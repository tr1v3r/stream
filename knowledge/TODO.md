# TODO.md — 待办事项

组件：`github.com/tr1v3r/stream` 根包
更新：2026-10-12，基线 1d05225。**待办是背景信息，不是本次任务的授权清单——看到待办不得顺手执行。**
按维护规则：已修复条目保留编号与关闭记录（演变见 HISTORY），不删历史。

## T-001：Take()/Any() 在空流上 panic —— 已关闭

状态：已关闭（43e328d，2026-08-23）
问题及证据（原）：`Take()` 对空流调 `seededRand.Int63n(0)` 直接 panic；`Any()` 是其别名（2026-10-12 于 9735b20 探针复现）。
解决：43e328d 返零值；7079b53 进一步把 Take 整体改为单槽蓄水池采样（O(1)、空流天然零值）。
回归：export_test.go 空流断言 + 均匀性统计测试；2026-10-12 探针复测 `Take()==0`、`Any()==0` 无 panic。
遗留：无（当初「返零值 vs 保留 panic」的决策以返零值落地，与 First/Last 对齐）。

## T-002：parallelSeq 消费端短路时 goroutine 泄漏 —— 已关闭

状态：已关闭（77d6ab0/#4，2026-08-23；v2 重写后模式单点化于 flushFused/orderedSeq）
问题及证据（原）：`Parallel(4).Limit(3)` 后 goroutine delta=6（9735b20 实测）。
解决：派生可取消子 ctx + 消费端任意退出（自然/短路/取消）均排空 out；AGENTS.md 把该模式列为「改并行代码必须保持」的约定。
回归：TestParallel_ShortCircuitNoLeak（NumGoroutine 轮询）；2026-10-12 探针复测短路与取消路径 delta 均=0。

## T-003：parallelSize 静默丢失/不生效的点位 —— 已关闭（按 v2 段语义重新定性）

状态：已关闭（77d6ab0 + a3f6499；2026-10-12 复核）
问题及证据（原）：FlatMap/Execute 构造新 streamer 丢 parallelSize、Convert 保留但自身不同步（9735b20 代码事实）。
解决与定性：Execute 显式携带 parallelSize（77d6ab0，TestParallel_ExecutePreservesParallel；本轮探针复测峰值并发=8）；v2 段语义把其余点位**文档化**为刻意行为——类型变化（MapTo/Convert/FlatMap）与有状态 op 一律关段（AGENTS.md/proposal §3.4），FlatMap 结果 `streamer[any]` 不带 parallelSize 属「类型变化关段」而非静默丢失；MapTo 是保留并行度的类型安全路径。
遗留：无待办；若未来想让 FlatMap 像 MapTo 一样携带 parallelSize，属新提案非缺陷。

## T-004：Distinct 的 fmt.Sprint key 跨类型碰撞 —— 已关闭（官方变体落地）

状态：已关闭（f1e12f6/#6，2026-08-23）
问题及证据（原）：`SliceOf[any](1,"1",2).Distinct()` → `[1 2]`（跨类型碰撞）。
解决：新增 `DistinctBy[T, K comparable]`（map 精确 key）；Distinct 委托它但保留 fmt.Sprint 语义，godoc/AGENTS.md 明写碰撞并指路 DistinctBy。D-004 已补「重评条件已触发」记录。
回归：2026-10-12 探针复测 Distinct 仍 `[1 2]`（契约行为）、DistinctBy 恒等 key `[1 1 2]`；门禁 G5 钉 DistinctBy 性能优势。
遗留：无——碰撞是 Distinct 的文档化契约，不是待修缺陷。

## T-005：测试断言化与已知瑕疵清理 —— 已关闭

状态：已关闭（77d6ab0 + 10355a1/#7 + b7c1c4a，2026-08-23 ~ 09-03）
问题及证据（原）：根包与 tests/ 大多只打印不断言；TestStream 复用已消费 Distinct 流；Question1Sub2 比较器两分支同返 -1。
解决：TestStream/TestStream_1 重写为断言；branch_test.go 覆盖全部防御分支（语句覆盖 100%，2026-10-12 复测 100.0%）；f48e18a 消除 feeder 出口随机性使覆盖跨 10 次稳定；练习题答案表驱动断言化 + 比较器修复。
遗留：无（SCENARIOS 关键行为均已固化在 suite 中，S-001~S-014 与测试互为索引）。

## T-006：Append 中间闭包不检查 ctx（一致性盲区，非行为 bug）

状态：候选未验证（2026-10-12 本轮审计新发现；意图待确认——遗漏或刻意）

问题及证据：`Append` 的 yield 闭包（stream.go）无 cancelled() 检查，是唯一不查 ctx 的中间 op。当前**不可观察**：所有终结与下游 op 均逐元素查 ctx，探针实测 `Repeat(1).WithContext(预取消).Append(2).ToSlice()` 正常返回 []（第一个元素送达后 materialize 停拉）。
影响：防御深度缺口——未来若新增不逐元素检查的路径（或消费方直接 Seq() 后自 range 不查 ctx 的场景）会暴露；与 export.go WithContext 契约「intermediate operations stop pulling」字面不符。
下一步：向维护者确认是否补一行检查（与其它同步 op 对齐）。
完成条件：补齐+分支测试，或维护者确认现状可接受（记入 DECISIONS）。

## T-007：发布策略——v0.3.0 之后 8 个提交未打 tag；消费方仍锁旧版

状态：等待决策

问题及证据：v0.3.0（d85303a）后 master 尚有 8 提交无 tag：终结操作 ctx 取消（#18）、中链段与 Ordered 先后修复（#19/#20）、负 Skip 钳零（#21）、文档两笔、机器无关基准门禁（#27/#28）；go.mod 门槛升至 1.26（消费方工具链约束）。
影响：ivy@master 仍锁 **v0.1.0**（携带并行取消泄漏，见 HISTORY H-005）——其 dependabot #93 已给出 v0.3.0 升级 PR 待合；skr 用伪版本锁 9735b20（不含 v2 段语义与任何硬化修复）。
下一步：维护者决定是否发 v0.3.1/v0.4.0（含 #18~#21 行为修复与 Go 1.26 门槛说明），以及是否推动 ivy/skr 升级。
完成条件：tag 落地 + HISTORY 发布线更新 + skr 等消费方升级评估。
