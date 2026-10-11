# SCENARIOS.md — 自然语言测试案例库

组件：`github.com/tr1v3r/stream` 根包
更新：2026-10-12，基线 commit 1d05225。标〔实测〕的数值来自 2026-10-12 探针程序（replace 指向本仓库，置于暂存区 .gotmp）与仓库内 `go test ./...`（含 -cover 100.0%）；修改管线组合逻辑后应重跑。多数案例已有固化测试，附对照名。

## S-001：惰性求值——中间操作不超额执行

前置：计数器闭包。
动作：`SliceOf(1..5).Map(count++; n*2).Limit(2).ToSlice()`。
预期：1. 结果 `[2 4]`；2. Map 恰好执行 3 次。〔实测：result=[2 4] mapCalls=3〕
验证：对照 export_test.go 惰性断言；门禁 G7（100k 上 Limit(2) ≤200 allocs）钉住不遍历全源。

## S-002：AllMatch 短路

动作：`SliceOf(1..5).AllMatch(count++; n<3)`。
预期：false；judge 恰好执行 3 次。
验证：对照套件短路断言（branch_test.go 覆盖取消+短路组合）。

## S-003：single-use——Distinct 流第二次终结得到空结果

前置：`src := SliceOf(1,2,3,2).Distinct()` 一次建流。动作：连续两次 `src.ToSlice()`。
预期：第一次 `[1 2 3]`，第二次 `[]`（DistinctBy 的 seen map 建流时创建一次）。〔实测：[1 2 3] / []〕
验证：直接断言（77d6ab0 起 suite 以断言钉住 single-use）。

## S-004：并行段退出路径均无 goroutine 泄漏（v2 回归案例）

前置：无限慢速源（每元素 1ms）+ 慢 Map（5ms）；NumGoroutine 基线。
动作 A：`WithContext(ctx).Parallel(4).Map(f).ForEach(noop)`，150ms 后 cancel。
动作 B：同管线不 cancel，改 `.Limit(3)` 短路。
预期：两条路径 goroutine 均回基线。〔实测：A delta=0，B delta=0（9735b20 时 B 为 delta=6——T-002 已修）〕
验证：对照 TestParallel_ShortCircuitNoLeak；改 feeder/worker/消费循环结构后必跑（AGENTS.md 约定）。

## S-005：并行段默认不保序

前置：`From(0..1999, -1)`；`Parallel(4)`；Map 轻负载；`.Limit(3).ToSlice()`。
预期：结果是某 3 个已完成元素的映射，顺序**无保证**——64 批对齐使轻负载下常出现有序前缀假象〔实测：100 次中 86 次 [0 1 2]、14 次乱序〕，不可断言有序。
验证：排序后与源子集比对；保序需求用 S-009。

## S-006：Count 的 sizeHint 快路径与回退路径

动作：A `SliceOf(1..10).Count()`；B `.Filter(odd).Count()`；C `.Limit(3).Count()`。
预期：A=10（不消费不查 ctx）；B=5（遍历）；C=3。〔实测：10/5/3〕
验证：修改任何 op 的 hint 传播后必跑（含 S-014 负 Skip）。

## S-007：Distinct 跨类型 key 碰撞（文档化契约）与 DistinctBy 逃生口

动作：A `SliceOf[any](1,"1",2).Distinct().ToSlice()`；B 同流 `DistinctBy(s, func(t any) any { return t })`。
预期：A=`[1 2]`（`fmt.Sprint(1)==fmt.Sprint("1")`，D-004）；B=`[1 1 2]`（map 精确 key）。〔实测均符〕
验证：直接断言；A 的结果变化=行为破坏（不是修复），B 由门禁 G5 钉性能。

## S-008：Pick 负数 end 在未知尺寸流上物化

前置：`From(埋计数器的 5 元素 seq, -1)`。动作：`.Pick(1, -1, 2).ToSlice()`。
预期：返回下标 1、3 → `[1 3]`；上游完整消费一次。〔实测：result=[1 3] consumed=5〕
验证：断言消费次数==5；⚠ 不要在无限流上执行——物化永不返回（现可用 WithContext 中断）。

## S-009：Ordered() 并行段保遭遇序（含 Filter 丢弃）

前置：`SliceOf(0..99)`；`Parallel(4).Ordered().Filter(n%3!=0).Map(n*2)`。
预期：输出与串行执行逐元素相等（Filter 丢弃以 hole 占位推进序号，不因空洞卡死/跳批）。〔实测：== 串行，66 元素〕
验证：对照 TestParallelV2_Ordered* 系与 A3 性质测试（30 随机管线含对抗性批头过滤）。

## S-010：Ordered() 与 Parallel() 先后皆可（76bd10c 回归）

动作：A `SliceOf(0..99).Ordered().Parallel(4).Map(id).Limit(3)`；B `...Parallel(4).Ordered()...` 同构。
预期：A==B==`[0 1 2]`（标志带入段，与调用顺序无关）。〔实测：A=[0 1 2]〕
验证：对照 TestParallelV2_OrderedBeforeParallel。

## S-011：中链 Parallel 关闭旧段（2e77ff7 回归）

动作：`SliceOf(0..199).Parallel(2).Ordered().Map(id).Parallel(4).Limit(3).ToSlice()`。
预期：`[0 1 2]`——两个段各自成池（2 worker 段 + 4 worker 段，经 channel 流水线交叠），Ordered 标志存活；修复前两段融进一个池且错选 `[192 193 194]`。〔实测：[0 1 2]〕
验证：对照 TestParallelV2_MidChainParallelReopens 及三条 H2 回归（分池、保序存活、链式段短路无泄漏）。

## S-012：终结操作尊重 ctx（800105d 回归）——裸管线

动作 A：`ctx` 预取消；`Repeat(1).WithContext(ctx).ToSlice()` / `.Last()` / `.First()`。
动作 B：中途取消（150ms）；无限 2ms 源 `.WithContext(ctx).ToSlice()`。
预期：A 立即返回 [] / 零值〔实测：ToSlice []、Last=0、First=0〕；B 返回已拉取的部分结果并 promptly 退出〔实测：66 元素 / 152ms〕。
验证：对照 cancel_bare_test.go（12 个终结 × 预取消/中途取消，5s watchdog）；语义=元素边界协作式。

## S-013：空流 Take/Any 返零值且不 panic（43e328d 回归）

动作：`SliceOf[int]().Take()`；`.Any()`。
预期：均返回 0，无 panic。〔实测：0 / 0〕
验证：export_test.go 空流断言；Take 均匀性由统计测试钉住（7079b53）。

## S-014：负 Skip 钳零保 hint 诚实（4c2f076 回归）

动作：`SliceOf(1,2,3).Skip(-1)` 后 `.Count()` 与 `.ToSlice()`。
预期：Count=3 且 len(ToSlice)=3（文档 "n<=0 keeps everything"；修复前 Count 谎报 4）。〔实测：3 / 3〕
验证：对照 TestOps_SkipNonPositiveKeepsAll。
