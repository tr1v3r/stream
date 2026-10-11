# FLOW.md — 根包（stream）运行流程

组件：`github.com/tr1v3r/stream` 根包（stream.go / factory.go / helper.go / export.go / doc.go / types/；errors.go 已删）
更新：2026-10-12，基线 commit 1d05225（master HEAD），行号以该版本为准。

## 组件职责（边界）

- 职责：Java Streams 风格的 Go 流式管线 SDK；`iter.Seq[T]` 骨架，惰性中间操作 + 短路终结 + 段式并行 fan-out + 协作式 ctx 取消。
- 入口：工厂（factory.go：`SliceOf`/`From`/`From2`/`Repeat`/`RepeatN`/`Concat`）与 `Streamer[T]` 接口（export.go）。
- 上下游：零第三方依赖（go.mod 仅标准库，要求 **Go 1.26**，f1e12f6）；已核实消费方（2026-10-12，读各自 origin/master 的 go.mod）：ivy 锁 **v0.1.0**——恰是携带并行取消泄漏的 tag（见 HISTORY H-005），其 dependabot #93 已给出升 v0.3.0 的 PR；skr 锁 9735b20 伪版本。
- 权威行为文档（A 级证据来源，本目录只索引不重复）：AGENTS.md（9023424；CLAUDE.md 是它的软链）、ARCHITECTURE.md（b5042c2）、docs/proposals/parallel-v2.md（并行段语义权威参考）、export.go/doc.go godoc（ef9906e 全符号覆盖）。

## 管线生命周期（主路径 / 异常路径）

```text
【工厂建流】factory.go
  SliceOf ─ From(seq,hint) ─ From2 ─ Repeat/RepeatN ─ Concat(srcs...)
     │  只包装 iter.Seq 闭包；newStreamer 置 ctx=Background、sizeHint（-1=未知）
     ▼
【中间操作链】stream.go —— wrap/直接构造派生新 streamer，仍零执行
  同步闭包：Filter/Map/Peek/MapTo/Distinct(By)/Limit/Skip/Pick —— 每收到一个元素查一次 ctx
  物化型：Sort/ReverseSort/Reverse 及 Pick(end<0 且 hint<0) —— 终结触发时 materialize(ctx,seq)
          整段上游 ⇒ 无限流上永不返回（文档化风险；7079b53 起 README/doc.go 警示并给 Limit/WithContext 解法）
  异常路径（ctx 取消）：提前 return，已 yield 元素保留；预先取消 ⇒ 集合/快照为空、
    归化保留部分结果（800105d 统一语义，export.go WithContext 契约）
     ▼
【并行段 v2】stream.go —— parallelSize>0 时 Filter/Map/Peek 融合进 fused func(T)(T,bool)（thenFused 累积）
  段边界（ensureFlushed/effectiveSeq → flushFused）：有状态 op（Distinct*/Sort 系/Reverse/Limit/
    Skip/Pick/Execute）、类型变化（MapTo/Convert/FlatMap）、终结 op、下一个 Parallel(n)
  flushFused → unorderedSeq / orderedSeq（Ordered() 段）：
    feeder 协程: range 上游 → 64 元素批（parallelBatchSize）→ in；ordered 版逐元素盖章 idx
    worker ×N:  range in（批粒度）→ 取批后查 ctx（取消则丢弃整批、继续排空 in 防 feeder 卡死）
                → 逐元素 stage(v) → out（批）；closer: wg.Wait → close(out)
    消费循环:   range out → 逐元素 cancelled()‖!yield ⇒ return；defer cancel()+排空 out
                ⇒ 短路/取消/自然结束三条退出路径均无 goroutine 泄漏（77d6ab0 模式，v2 单点实现）
    ordered:    orderedYield 按「批起点 idx→批」pending map 重排；Filter 丢弃 → hole 占位推进序号；
                对齐批直通零簿记，仅流尾残批需逐元素过洞
  中链 Parallel(m)：先 flushFused 旧段（其输出成新上游）再开 m worker 新段；相邻段经 channel 惰性
    交叠 = 流水线并行。⚠ #19 修复前旧段不关闭：两段融进一个池且 Ordered 失效（错选元素）
     ▼
【终结操作】export.go/stream.go —— 全部先 effectiveSeq()（冲刷未关闭的并行段）
  ToSlice/Collect | ForEach/Match×3 | Reduce×4 | First/Take/Any/Last | Count | Seq
  短路：First/AnyMatch 及 Limit+下游 break 由 yield=false 逐层向上传播
  Take = 单槽蓄水池采样：O(1) 内存、无限流可用、均匀性有统计测试钉住（7079b53）
  Count(hint≥0)：O(1) 返回 hint，不消费管线也不查 ctx
     ▼
【终态】single-use —— 纯闭包源（SliceOf 等）可重放（碰巧，非契约）；带闭包状态的 op
  （Distinct 委托 DistinctBy 后 seen map 仍在建流时创建一次）二次消费得空
```

## ctx 取消检查覆盖（800105d 之后的现状）

| 类别 | 覆盖 |
|---|---|
| 同步中间 op | Filter/Map/Peek/MapTo/Distinct(By)/Limit/Skip/Pick 每元素查；FlatMap 外层查（内层经子流 Seq）；**Append 不查**——唯一盲区，当前被所有下游逐元素检查掩盖（探针实测预先取消+无限源+Append+ToSlice 返回 []），见 T-006 |
| 物化路径 | materialize(ctx,seq) 每元素查：Sort/ReverseSort/Reverse/Pick(end<0)/Execute/ToSlice/Collect（预先取消→空，中途→部分结果） |
| 终结 op | ForEach/Match×3/Reduce×4/First/Take/Last/Count(迭代路径) 每元素查；Seq 返回包装序列（裸管线 range 也尊重 WithContext）；Count 快路径例外（不消费故不查） |
| 并行段 | feeder 循环头、worker 取批后、消费循环、orderedYield emit 均查 |

全部是「元素边界」协作式：卡住不产元素的源无法中断（export.go WithContext 契约明写）。800105d 之前 ToSlice/Collect/Count/Last/First/Seq/物化阶段全部不查——基线 9735b20 的旧覆盖表已过时。

## 并行标志与 Ordered 的作用域

- `Parallel(n>0)` 开段；`Parallel(n≤0)` 原样返回副本（并行「关不掉」仍成立，现为文档化 no-op）。
- `Ordered()` 置 ordered=true 经 wrap 传播：**粘性**、无取消开关；#20 修复后 Ordered() 与 Parallel() 先后任意（标志带入段）。类型变化（MapTo/FlatMap）丢 ordered、Execute 保留（代码现状）。
- 段内仅 Filter/Map/Peek 融合并行执行；Distinct/Sort 系/Reverse/Limit/Skip/Pick/Execute/FlatMap/MapTo/Convert 一律先关段再同步执行（AGENTS.md 事实表；43e328d 起 Distinct 串行是刻意的——共享 key map 非并发安全）。

## sizeHint 传播（影响 Count/预分配）

| op | hint |
|---|---|
| Filter / Distinct / DistinctBy / FlatMap / Pick | -1 |
| Map / Peek / MapTo（Convert 委托 MapTo） | 保留 |
| Sort / ReverseSort / Reverse | 保留 |
| Limit(n) | hint≥0 时 min(hint, n) |
| Skip(n) | n<0 先钳零（4c2f076 修复负数膨胀）再 max(0, hint−n) |
| Append | hint+len（hint<0 保持 -1） |
| Execute | 物化后精确 len（parallelSize/ordered 一并保留，77d6ab0） |

`Count()` 信任 hint：`From(seq, sizeHint)` 传错 → Count 静默返回错值，无校验路径（代码现状，D-006）。

## single-use 的真实边界

纯闭包源可重放——每次终端调用重 range 底层；带闭包状态的 op 不可重放：Distinct(DistinctBy) 的 seen map 在 `Distinct()` 调用时创建一次，首次终端消费填满后，第二次终端得到空结果（2026-10-12 探针复测 [1 2 3]→[]）。doc.go 约定一律按 single-use 对待，不要依赖「碰巧可重放」。
