# HISTORY.md — 修改历史（整理版）

组件：`github.com/tr1v3r/stream` 根包
更新：2026-10-12（覆盖基线 9735b20 → 1d05225 的 25 个提交）。证据分级：A=PR/commit/文档明写；B=维护者口述；C=推测（待确认）。提交号可 `git show <hash>` 回查。
发布线（2026-10-12 经 `git ls-remote --tags` 核实）：v0.0.1=f6dbc2e（2023-10-07）、v0.0.2=2c0681c（2023-10-10）、v0.0.3=1054737（2025-11-01）、v0.1.0=53b194e（2026-04-21）、v0.2.0=3ac3fcc、v0.3.0=d85303a（含 parallel v2 三阶段，段语义已随其发布）。⚠ 本地旧 clone 曾 fetch 不到 v0.2.0/v0.3.0——核对 tag 一律用 ls-remote，勿信本地引用。v0.3.0 之后 master 尚有 8 个提交（#18~#21 行为修复 + 文档两笔 + #27/#28 CI 门禁）未打 tag（见 T-007）。

## H-001：iterator 时代的地基（2023-09-18 ~ 2023-10-07）

变化：init module（0fe7b48）→ eager streamer（a1f3fba）→ 自研 `iterator[T]` 接口+静态迭代器（e230dd4）→ stage 机制（8b44eb1）→ `supplyIter` 供应器迭代器（5a56b90）→ concat（c86f4da）→ `async.go` 并行流（5baca73、908495e）；同期 7d096ce 精简泛型参数、5de87fd 加 Any、8df5a78 Consumer 改单参。
当前影响：内部体系已随 53b194e 全删，但公开 API（SliceOf/Filter/Sort/…）自那时起基本稳定——tests/ 用现行 API 且 `go test ./...` 通过（2026-10-12 于 1d05225 复测）。
关联：上列各提交。

## H-002：context 控制引入（2023-10-10，PR #1，tag v0.0.2）

变化：`streamer` 增加 ctx 字段与 `WithContext`，操作循环内加取消检查（2c0681c）。
原因（证据等级 A）：PR 标题 "implement context control"；README 先一日挂出 TODO 项（e9b9232→2c0681c 移除）。
当前影响：取消检查覆盖不均的格局在 800105d（H-012）才补齐终结操作一侧；Append 中间闭包仍是盲区（T-006）。

## H-003：iterator 时代的三次修复（拉取模型的坑）

变化：f6dbc2e concat 尺寸错误；ce83284 ForEach/Peek 行为；f742eea（2025-01-08）supplier 返回 false 后源迭代不停止——给 supplyIter 加 dead 标志。
原因（证据等级 A）：三个 commit message 均明写症状。
当前影响（含 C 级推测）：C——这类反复修补可能是推倒重写（H-004）的动机之一，重写 commit 未提及，待维护者确认。
关联：f6dbc2e、ce83284、f742eea。

## H-004：iter.Seq 惰性求值重写（2026-04-21，PR #3，tag v0.1.0）★ 方向性变化

变化：53b194e（净 -350 行）删 `iterator.go`/`async.go`/`types.Supplier`；`iter.Seq[T]` 成为唯一骨架；新增 FlatMap/Seq/From/From2；移除 tr1v3r/pkg 依赖；`Concat(srcs...)` 签名精简；工厂命名四次迭代（commit 明写）。
原因（证据等级 A）：PR #3 message 完整陈述动机。
当前影响：v0.0.x 消费方需适配（ivy 仅用 SliceOf，面窄）；该提交由 Claude Opus 4.7 协作完成（commit 尾注），message 可当设计讨论记录读。
关联：53b194e。

## H-005：parallel worker 取消泄漏修复（2026-04-21 02:10，重写后 11 分钟）

变化：9735b20：feed 发送改 select+ctx.Done、worker 取元素先查 cancelled()（commit 明写 "prevent goroutine leaks when context is cancelled"）。
当前影响：⚠ v0.1.0 tag 打在 53b194e 上**不含本修复**——tag 消费方带并行取消泄漏；skr 以伪版本锁定 9735b20 绕过（2026-10-12 复查仍如此）。~~消费端短路泄漏路径仍未覆盖（T-002）~~ → 2026-10-12 追记：T-002 已由 77d6ab0 修复（H-009），v2 又把该模式收敛为单点实现（H-008）。
关联：9735b20；与 53b194e 构成「重写当天补漏」时间对（01:59→02:10）。

## H-006：文档与测试补全（2026-04-21）

变化：52a285f 新增 doc.go、完整 README、修 single-use 测试——但只修了 TestStream_1，TestStream 仍复用已消费的 Distinct 流；6cb44d7 完善 CLAUDE.md。
当前影响：~~TestStream 残留是 T-005 已知样本~~ → 2026-10-12 追记：77d6ab0 已把两个测试重写为断言（H-009）。CLAUDE.md 已于 9023424 变为 AGENTS.md 的软链。
关联：52a285f、6cb44d7。

## H-007：工程化整备（2025-10-27 ~ 2025-11-01，tag v0.0.3）

变化：f97fd2d 加 golangci-lint 配置、弱随机数改 seeded 源、删 isNil/reflect；1054737 首次加 CLAUDE.md。
当前影响：lint 基线自此定型（635241f/#8 后为 v2 规范）；~~seededRand 为现行随机源~~ → 2026-10-12 追记：seededRand 已被 77d6ab0 换成 math/rand/v2 顶层 IntN（并发安全，gosec G404 豁免记录在 .golangci.yml）。
关联：f97fd2d、1054737。

## H-008：parallel v2——段融合重写（2026-08-24，#14 提案 → #15 → #16 → #17）★ 方向性变化

变化：v1「每 op 一池」（parallelSeq）近零负载开销实测 14-21× 串行（proposal §1 测量表）；23b3a79 提案（评审否决 fusion-only、保留按段定规模+段间流水线并行）→ a3f6499 段融合/单池/64 批（dead parallelSeq 删除，A1 门 14-21×→1-2.6×）→ 745ef5a Ordered() 批对齐重排（naive next-index/smallest-pending/ring slots 三结构被实测否决，设计旅程记于 commit）→ d85303a 文档收口（README/doc.go/AGENTS.md 重写为段语义，proposal 状态翻 Implemented）。
当前影响：并行语义从「invisible rolling range」变「显式段」（兼容性注记：老代码 `.Parallel(4).Filter(f)` 行为不变，多 op 链融合——段语义已随 v0.3.0=d85303a 发布，其后的行为修复 #18~#21 仍在未打 tag 区间）；修并行代码前必读 proposal。
关联：23b3a79、a3f6499、745ef5a、d85303a。

## H-009：并行硬化修复批（2026-08-23，43e328d + 77d6ab0/#4）——旧 T-001/T-002/T-003/T-005 在此了结

变化：43e328d：空流 Take/Any 返零值（原 panic）、负 Pick start 不再负索引、并行 Distinct 崩溃（改串行）。77d6ab0：parallelSeq 派生可取消子 ctx + 消费端任意退出排空 out（**T-002 短路泄漏修复**）、全局 rand 换 math/rand/v2 顶层 IntN（Take 并发安全）、Execute 快照保留 parallelSize（**T-003 Execute 半修复**，加断言测试）、TestStream/TestStream_1 重写为断言 + 覆盖 49%→88% + 修 Question1Sub2 倒置比较器（**T-005 大半了结**）、删死码 errors.go。
当前影响：AGENTS.md Known Issues 清空；本轮探针复测短路/取消 goroutine delta=0、Execute 峰值并发=8。
关联：43e328d、77d6ab0。

## H-010：Go 1.26 + slices + DistinctBy + 100% 覆盖（2026-08-23，f1e12f6/#6、10355a1/#7、f48e18a）

变化：go.mod 1.24→1.26（Green Tea GC）；Sort/ReverseSort 改 slices.SortFunc pdqsort（-38%/-30%）、Reverse 改 slices.Reverse；新增 `DistinctBy[T, K comparable]`（**D-004 重评条件触发**，Distinct 委托之，修 string 碰撞语义、5× 快 327× 少分配）；10355a1 branch_test 覆盖全部防御分支（100% 语句）；f48e18a feeder 出口改「循环头检查+阻塞发送」消除 select 双就绪随机性（100% 覆盖跨 10 次稳定，泄漏测试改无限源+慢 Map 让 feeder 确定性停在发送上）。
当前影响：CI 矩阵 1.26.x；改 feeder 结构前先读 f48e18a（AGENTS.md 同款警告）。
关联：f1e12f6、10355a1、f48e18a。

## H-011：MapTo + 蓄水池 Take + godoc 全覆盖（2026-08-24，7079b53/#9、cfbdf78/#10、ef9906e/#11、b7c1c4a）

变化：7079b53 新增 `MapTo[T,R]`（Convert 标 Deprecated 并委托）、Take 从全量物化改单槽蓄水池采样（O(1)、无限流可用、统计测试钉均匀性）、无限流文档警示；cfbdf78 MapTo vs Convert 取舍指南；ef9906e 全部 51 个导出符号 godoc（修正 Parallel 注释里过时的 "0 do nothing, 1 async"）；b7c1c4a tests/ 练习题答案表驱动断言化（**T-005 尾巴了结**）。
当前影响：跨类型变型的推荐路径是 MapTo；godoc 成为行为契约的日常入口。
关联：7079b53、cfbdf78、ef9906e、b7c1c4a。

## H-012：终结操作尊重 ctx + 段语义两修复 + 审计收尾（2026-08-24 ~ 09-14，800105d/#18、4c2f076/#21、2e77ff7/#19、76bd10c/#20）

变化：800105d：materialize(ctx,seq) 及六个调用点、First/Last/Count(迭代)/Seq 全部补 ctx 检查——裸管线上 `Repeat(1).WithContext(cancelled).ToSlice()` 原先永久挂起（本轮探针复测现返回 []；中途取消 ToSlice 66 元素/152ms）；cancel_bare_test.go 钉 12 个终结。4c2f076：Skip 负 n 钳零（原 hint 膨胀致 Count 谎报 4 vs 实际 3）。2e77ff7：中链 Parallel 采纳 flushFused 结果（原两段融进一个池、Ordered 段错选元素 [192 193 194]）。76bd10c：Ordered() 在 Parallel() 之前设置也能带入段。均出自同一轮 audit（H2/M1/M2 编号）。
当前影响：export.go WithContext 契约（元素边界协作式取消）自此可信；Append 中间闭包是仅存盲区（T-006，新发现）。
关联：800105d、4c2f076、2e77ff7、76bd10c。
