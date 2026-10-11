# DECISIONS.md — 架构与选型说明

组件：`github.com/tr1v3r/stream` 根包
更新：2026-10-12，基线 commit 1d05225。证据分级：A=PR/commit message/仓库文档明写或代码/实验可回查；B=维护者口述；C=推测（待确认）。

## D-001：以 iter.Seq[T] 作为管线骨架（推倒 iterator 接口体系）

选择：标准库 `iter.Seq[T]` 作唯一管线表示，中间操作组合闭包零执行；并行用 goroutine worker 表达（53b194e，PR #3）。f1e12f6 起要求 **Go 1.26**（Green Tea GC + go fix 现代化）。
替代方案：自研 `iterator[T]` 拉取接口 + 独立 async.go——已随重写删除（净 -350 行，commit 明写）。
权衡：语言级 range-over-func、天然短路、与标准库互操作；代价是消费方工具链须 ≥1.26（skr 若升级需同步，见 T-007）。
证据：A——53b194e、f1e12f6 commit message。
动机补充（C，推测）：旧拉取模型的反复修补（H-003）是重写动机之一——重写 commit 未提及，待确认。
重新评估条件：需支持旧 Go 的消费方，或标准库迭代器语义不兼容变化。
状态：生效中（2026-10-12）

## D-002：single-use 语义（流不可重放）

选择：文档明确流是一次性的，复用需重新建流。
证据：A——doc.go/export.go "single-use"；2026-10-12 探针复测 Distinct 流二次消费得空。
权衡（C，推测）：避免「每次终端操作前重置内部状态」的成本——待确认。
重新评估条件：稳定的重放需求出现，且愿意为 stateful op 实现状态快照。
状态：生效中（2026-10-12）

## D-003：Convert / FlatMap 返回 Streamer[any]（类型擦除），MapTo 为类型安全替代

选择：`Convert`/`FlatMap` 落到 `any`，由 `AnyTo`/`To` 转回；7079b53 新增包级泛型 `MapTo[T,R]`（保 ctx/sizeHint/parallelSize，无 any 往返），`Convert` 标记 Deprecated 并委托 MapTo（cfbdf78 给出取舍指南：MapTo 在变型点打断链式、适合管线头部变型；Convert 中链顺手但擦除类型）。
证据：A——export.go Deprecated 标注、AGENTS.md gotcha、cfbdf78。
动机（C，推测）：Go 泛型无 HKT，跨类型变换无法在 `Streamer[T]` 接口方法内表达——合理化推测，待确认。
重新评估条件：Go 泛型能力演进，或跨类型使用成为主要场景。
状态：生效中（2026-10-12；推荐路径已换为 MapTo）

## D-004：Distinct 默认用 fmt.Sprint 做去重 key；DistinctBy 为精确 key 变体

选择：默认 `fmt.Sprint(t)`（或 `types.Unique`）做 key；f1e12f6 新增 `DistinctBy[T, K comparable]`（map 精确相等 key，快 5×、少 327× 分配），`Distinct` 委托它。
证据：A——f1e12f6 commit 明写 "fixing Distinct's string-collision semantics (int 1 vs string \"1\")"；godoc 警示碰撞；2026-10-12 探针复测 `Distinct` 仍 `[1 2]`、`DistinctBy` 恒等 key `[1 1 2]`。
2026-10-12 更新：**重新评估条件已触发**——官方变体 DistinctBy 已落地，但 Distinct 的 fmt.Sprint 语义为兼容保留（godoc 明写碰撞）。T-004 就此关闭：新代码该用 DistinctBy，Distinct 行为是契约而非缺陷。
动机（C，推测）：`T any` 无 comparable 约束、不能做 map key——待确认。
重新评估条件：v2 拆分 `ComparableStreamer[T comparable]`，或 Distinct 碰撞在实际使用中造成故障。
状态：生效中（2026-10-12；Distinct 本体不变，替代路径已提供）

## D-005：并行段默认不保序，Ordered() 是显式例外

选择：`Parallel(n>0)` 段默认乱序（v1 兼容）；745ef5a 新增 `Ordered()`——当前（或下一个）并行段经索引盖章 + 消费端重排保持遭遇序（见 D-009）。
证据：A——proposal 3.3、#16、AGENTS.md；2026-10-12 探针实测：无序段轻负载下 100 次有 14 次乱序前缀（64 批对齐造成的有序假象，无保证）；Ordered 段含 Filter 丢弃时与串行逐元素相等。
权衡：保序有每元素盖章+重排成本（门禁 G2 允许更高开销比），默认关闭保 v1 行为。
重新评估条件：依赖顺序的并行消费方成为主流（那时考虑默认保序的大版本切换）。
状态：生效中（2026-10-12，v2 起语义为段作用域）

## D-006：Count() 信任 sizeHint（O(1) 快路径）

选择：hint≥0 时 Count 直接返回 hint 不消费管线（也不查 ctx）；迭代路径每元素查 ctx、取消时返回已拉取数（800105d）。
证据：A——stream.go Count；2026-10-12 探针复测 10/5/3。
动机（C，推测）：性能优先于防御性校验——待确认。
重新评估条件：因错误 hint 出现故障，可在 From 文档或 debug 模式加校验。
状态：生效中（2026-10-12）

## D-007：零外部依赖

选择：SDK 不引入第三方依赖（53b194e 移除 tr1v3r/pkg），go.mod 仅标准库、go.sum 为空。
证据：A——commit message；go.mod/go.sum 现状。
重新评估条件：无（除非标准库缺口巨大）。
状态：生效中（2026-10-12）

## D-008：并行 v2——段融合、一池一段、64 元素批（a3f6499，#15）

选择：段内连续无状态 op（Filter/Map/Peek）融合为单函数、单 worker 池；feeder 按 64 元素批喂入。v1「每 op 一池」在近零负载下开销 14-21× 串行，v2 实测 1-2×。
替代方案（A，proposal §6 明写被否决）：fusion-only 初稿——评审指出会静默丢掉按段定规模能力（异构 IO/CPU 段的正确模型），改为「中链 Parallel(n) 关旧段开新段，相邻段经 channel 流水线交叠」；保留 v1 仅文档化开销——把 API 留成陷阱；显式 `.Pool(p)` 生命周期——破坏声明式一行流；errgroup 外部组合——放弃库的存在意义。
权衡：段边界（有状态 op/类型变化/终结/下一 Parallel）明确化，覆盖面从「任意」变显式；Sort/Distinct 等仍串行（documented）。
证据：A——docs/proposals/parallel-v2.md（Implemented 状态）、#14/#15/#17、ARCHITECTURE.md §3.2。
重新评估条件：出现需要并行有状态 op 的负载（当前 Non-Goal）。
状态：生效中（2026-10-12）

## D-009：Ordered() 用批对齐重排而非逐元素滑窗（745ef5a，#16）

选择：feeder 盖章、Filter 丢弃转发为 hole 占位、消费端按「批起点 idx→批」pending map 重排：对齐批直通，仅流尾残批逐元素过洞。
被否决的中间结构（A，#16 commit 设计旅程明写）：naive next-index map（hole 上死锁等待）、smallest-pending（批竞态下提前发射）、ring slots（在飞距离超窗时损坏——消费端持半批会拉大窗口）。
证据：A——#16 message；A3 性质测试（30 随机管线含对抗性批头过滤逐元素等于串行）。
重新评估条件：批大小或窗口语义需要变化时，先读 proposal 3.3 与该设计旅程再动手（AGENTS.md 同款警告）。
状态：生效中（2026-10-12）

## D-010：泄漏检测保持 NumGoroutine 轮询，不用 goleak（465f380，#13）

选择：`TestParallel_ShortCircuitNoLeak` 轮询 `runtime.NumGoroutine`（5s 截止），不做 goleak 栈快照对比。
权衡：零依赖属性（空 go.sum）优先于精度；v2 新增 feeder/worker 路径复用同一 cancel/drain 单点模式，由泄漏/短路/A3 测试共同覆盖。
证据：A——465f380、AGENTS.md Deliberate Trade-offs 节。
重新评估条件：并发原语种类继续增加（保留 ~40 行 runtime.Stack diff 作为零依赖选项）。
状态：生效中（2026-10-12）

## D-011：性能门禁机器无关化——比值与分配数，绝不裸墙钟（16f9d42/#27、1d05225/#28）

选择：`TestBenchmarkGate` G1-G7 只用同进程比值（G1 无序 ≤6×、G2 保序 ≤8×、G3 重负载加速 ≥1.5×、G4 Sort ≤2.5× 直排、G5 DistinctBy ≤0.6× Distinct 时间且 ≤5% 分配）与确定性分配数（G6 Take ≤50 allocs、G7 惰性 Limit ≤200 allocs）；阈值按双机实测标定记录在文件头，**无提案级理由不得放宽**；A6（多段加速）刻意不做时间门禁（依赖核数），正确性由 TestParallelV2_MidChainParallelReopens 钉住。
证据：A——benchmark_gate_test.go 头注释（标定表）、#27/#28 message。
重新评估条件：无；改阈值本身需要新标定数据。
状态：生效中（2026-10-12）
