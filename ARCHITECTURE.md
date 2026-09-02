# ARCHITECTURE — github.com/tr1v3r/stream

> 一句话定位：基于 Go 1.26 `iter.Seq[T]` 的零依赖惰性流式计算 SDK，提供 Java Streams 风格的类型安全组合子，以及"段作用域"（section-scoped）的融合式并行执行模型。
>
> 本文面向维护者与下游（skr / ivy）使用者；用法示例见 README.md，包级 godoc 见 doc.go，并行语义的权威参考是 docs/proposals/parallel-v2.md。

## 1. 它解决什么问题

- **Go 泛型流式计算**：标准库 `slices`/`maps` 只有零散的即时求值函数，缺少可组合的惰性管道（Filter→Map→Limit 短路、无限流、reservoir 采样等）。本库以 `iter.Seq[T]` 为管道表示，把整套 Java Streams 语义搬进 Go 泛型世界。
- **声明式并行编排**：让 `Parallel(n)` 像 Java 的 `.parallel()` 一样是链上的一行声明，而不是手工 `errgroup` + channel 的样板代码；同时通过"段融合"避免每个算子各起一个池的巨额机器开销。

仓库结构（扁平布局，非嵌套 `stream/` 目录）：

| 文件 | 职责 |
|---|---|
| `export.go` | `Streamer[T]` 接口（公开 API 契约） |
| `stream.go` | `streamer[T]` 实现：惰性闭包组合、并行段执行器 |
| `factory.go` | 工厂函数 |
| `helper.go` | `To` / `AnyTo` 收集辅助 |
| `doc.go` | 包级 godoc |
| `types/type.go` | 函数式接口类型（`Judge`、`Mapper`、…） |
| `tests/` | 练习场（lint 排除） |
| `docs/proposals/parallel-v2.md` | 并行 v2 设计提案（权威语义参考） |

`go.mod` 仅声明 `go 1.26.0`，无任何依赖（`go.sum` 为空）。

## 2. 公开 API 面

### 2.1 核心类型

- **`Streamer[T any]`**（export.go）：唯一核心接口，由不可导出的 `streamer[T]` 实现（`var _ Streamer[any] = newStreamer[any](nil, 0)` 编译期断言）。流是**单次使用**的：终结操作消费底层 `iter.Seq`。
- **`types` 包**（types/type.go）：全部是函数类型——`Judge[T] func(T) bool`、`Mapper[T] func(T) T`、`Converter[T,R] func(T) R`、`Comparator[T]`、`Consumer[T]`、`BinaryOperator[T]`、`Accumulator[T,R]`、`Collector[T] func(...T) any`，以及 `Unique interface{ Key() string }`（自定义去重键）。

### 2.2 工厂函数（factory.go）

| 函数 | 说明 |
|---|---|
| `SliceOf[T](slice ...T)` | 从切片/变参创建，sizeHint = len |
| `From[T](seq iter.Seq[T], sizeHint int64)` | 包装任意 `iter.Seq`，支持无限流（sizeHint 传 -1） |
| `From2[K,V](seq iter.Seq2[K,V])` | 适配 `maps.All` 等，只取 value |
| `Repeat[T](t)` / `RepeatN[T](t, n)` | 无限重复 / 有限重复（RepeatN = Repeat+Limit） |
| `Concat[T](srcs ...Streamer[T])` | 顺序拼接，短路会停止拉取后续源 |

### 2.3 中间算子（惰性，返回新流）

- **无状态**：`Filter(Judge)`、`Map(Mapper)`（同类型）、`Peek(Consumer)`（旁路副作用）、`Convert`（Deprecated，产出 `Streamer[any]`）、`FlatMap`（子流拼接，产出 `Streamer[any]`）。
- **包级泛型**（Go 方法不能新增类型参数，故为函数）：`MapTo[T,R](s, Converter) Streamer[R]` —— `Convert` 的类型安全替代；`DistinctBy[T,K comparable](s, key)` —— 精确键去重（`Distinct` 用 `fmt.Sprint` 键，`1` 与 `"1"` 碰撞）。
- **有状态**：`Distinct`、`Sort` / `ReverseSort`（`slices.SortFunc`，pdqsort）、`Reverse`、`Limit` / `Skip`、`Pick(start, end, interval)`（绝对索引步进采样，end<0 表示末元素）。
- **其他**：`Append(...T)`、`Execute()`（立即物化为可重迭代快照，保留 ctx 与 parallelSize）。
- **并发控制**：`WithContext(ctx)`、`Parallel(n)`、`Ordered()`。

### 2.4 终结操作

- 收集：`ToSlice()`、`Collect(Collector[T]) any`（配套 `helper.go` 的 `To[T,R]`、`AnyTo[T]`）。
- 归约：`Reduce` / `ReduceFrom` / `ReduceWith` / `ReduceBy`（初始值由 sizeHint 构建，可预分配容量）。
- 匹配（短路）：`AllMatch` / `NonMatch` / `AnyMatch`。
- 元素：`First`（短路）、`Take`（**均匀蓄水池采样**，O(1) 内存，可作用于无限流）、`Any`（Take 别名）、`Last`、`Count`（sizeHint 已知时 O(1)）。
- 互操作：`Seq() iter.Seq[T]`（原生 `for range`）。

## 3. 核心流程：构建 → 执行 → 终结

一条流的完整生命周期：

```
工厂（SliceOf/From/...）          中间算子（惰性组合）                终结操作
┌────────────────┐   ┌─────────────────────────────────┐   ┌──────────────┐
│ streamer[T]{   │   │ 串行: wrap() 包闭包, 不执行        │   │ effectiveSeq │
│  seq, sizeHint,│ → │ 并行段: thenFused() 累积融合函数   │ → │ = ensure-    │
│  parallelSize, │   │ (Filter/Map/Peek → 一个 stage)    │   │  Flushed.seq │
│  ordered       │   └─────────────────────────────────┘   └──────┬───────┘
└────────────────┘                                                ↓
                                                  串行: range 闭包链逐元素流动
                                                  并行: flushFused → feeder/workers/consumer
```

### 3.1 惰性构建（串行路径）

`streamer[T]` 的核心字段是 `seq iter.Seq[T]`。每个中间算子都是**包装闭包**：例如 `Filter` 返回 `s.wrap(func(yield){ for v := range prev { if s.cancelled(){return}; if judge(v) && !yield(v) { return } } }, -1)` —— 不做任何工作，只把旧 `seq` 包成新 `seq`。`wrap` 沿途传播 `ctx`、`parallelSize`、`ordered`。

终结操作触发求值：`ToSlice` 调 `materialize(s.effectiveSeq())`；短路终结（`First`）在 yield 一次后直接 return，短路信号沿闭包链向上传播（`branch_test.go` 逐算子验证）。

**sizeHint 传播表**（实现真实语义，新增算子时须显式决策）：

| 算子 | hint 变化 |
|---|---|
| `Filter` / `Distinct` / `DistinctBy` / `FlatMap` / `Pick` | -1（未知） |
| `Map` / `Peek` / `Convert` | 保留 |
| `Limit(n)` | hint ≥ 0 时取 min(hint, n) |
| `Skip(n)` | hint ≥ 0 时取 max(0, hint−n) |
| `Sort` / `ReverseSort` / `Reverse` / `Append` | 保留 / 加 len(data) |

`Count()` 在 hint ≥ 0 时直接返回 hint（O(1)），`ReduceBy` 用 hint（可为负）构建初始容器容量——所以 hint 必须诚实。

一个走透的例子（`Limit(1).First()` 在百万元素上只处理 1 个）：

```go
stream.SliceOf(million...).      // seq = slices.Values, sizeHint = 1e6
    Filter(odd).                 // 新闭包包旧 seq；hint → -1
    Map(square).                 // 再包一层；hint 保持 -1
    Limit(1).                    // 闭包内 count>=l 即 return
    First()                      // range 一次后 return → 短路逐层上传
```

### 3.2 并行路径：段融合执行器（v2）

`Parallel(n)` 打开一个**并行段**：设 `parallelSize = n`，此后段内的无状态算子不再包装闭包，而是累积到 `fused func(T) (T, bool)` 字段（`thenFused` 组合：Filter 短路丢弃、Map/Peek 变换）。执行时 `flushFused` 把整段变成**单个 worker 池**的一次执行：

```
上游 seq ──fusedFeeder(1 goroutine, 64/批)──> in chan []T
         ──fusedWorkers(n goroutines)────────> out chan []T
         ──consumer(调用方的 range，逐元素 yield)
```

真实函数链：`flushFused` → `unorderedSeq(stage, prev, n)` → `fusedFeeder` + `fusedWorkers` + `wg.Wait(); close(out)`。

**取消与无泄漏语义**（`unorderedSeq`/`orderedSeq` 共享同一模式）：
- 消费端 `context.WithCancel(s.ctx)` 派生子 ctx，`defer cancel()`；
- 任何退出路径（自然结束、下游短路、ctx 取消）都 `cancel()` 后**把 out 排干**，释放阻塞在发送上的 worker；
- worker 在取消后继续 drain `in`（丢弃元素），保证 feeder 永不永久阻塞。feeder 的取消检查在循环顶部、发送是纯阻塞 send——刻意不用 `select`+`Done`（两 case 同时就绪时 Go 随机选择，曾导致退出路径不确定）。

**段边界**：`ensureFlushed()` 在不可融合的算子（有状态算子、`MapTo`/`Convert`/`FlatMap` 类型变更、终结操作 via `effectiveSeq`、下一次 `Parallel` 调用）前先物化当前段。因此 `Parallel(4).Sort(cmp)` 是"并行前置段 + 串行排序"，显式且文档化（v1 是静默忽略）。链中第二次 `Parallel(m)` 关闭旧段、开新段——相邻段经 channel 衔接，天然获得**流水线并行**（IO 段 16 worker、CPU 段 2 worker 的异构配比，见提案 3.0 与 A6 门）。

**错误传播**：组合子签名不携带 error；下游约定在回调内部处理错误（记日志/置空元素），ctx 取消是唯一的流级异常通道（`WithContext`）。这也是 skr 的真实写法：Map 回调里重试抓取、失败返回 nil 哨兵，ForEach 回调里跳过 nil 并记日志。

**测试地图**（行为契约的权威来源）：

| 文件 | 覆盖 |
|---|---|
| `parallel_v2_test.go` | 融合单池、中途 Parallel 关段、Ordered 与串行等价（A3 性质测试） |
| `parallel_test.go` | 并行 Filter/Map/Peek、短路无泄漏（NumGoroutine 轮询）、并发 Take 无竞争 |
| `branch_test.go` | 逐算子下游短路、流中取消、Pick 物化路径、外来 Streamer 回退 |
| `ops_test.go` / `factory_test.go` / `terminal_test.go` / `export_test.go` | 算子语义、工厂、终结操作，语句覆盖率 100% |
| `benchmark_test.go` | A1/A2 开销与扩展性门 |

### 3.3 Ordered 路径：保序重排

`Ordered()` 把当前段标记为 `ordered`，执行时走 `orderedSeq`：`orderedFeeder` 给每个元素盖上输入索引（`indexedValue[T]{idx, val, hole}`），`orderedWorkers` 让索引随结果流动（被融合 Filter 丢弃的元素以 **hole** 占位以推进序号），`orderedYield` 在消费端按"批起始索引"重排——批内部连续，对齐的批零逐元素簿记直通，只有末尾残批需要 hole 填充。输出与串行执行逐元素一致（提案 A3 用 1k 条随机管道做性质测试）。

## 4. 设计演化史

**v1（已移除）**：每个并行感知算子各调一次 `parallelSeq`，即**每算子一个池**。提案（docs/proposals/parallel-v2.md）实测：近零负载 `Filter+Map` × 100k 元素上，`Parallel(2)` 慢 14×、`Parallel(8)` 慢 21×——纯机器开销吞掉并行收益，且无保序模式、并行覆盖不可见、作用域语义隐晦。

**v2（现状，PR #15/#16 落地，里程碑 v0.3.0）**，提案定案的关键决策：
- **段融合、一池一段**（G1/G3）：`Filter→Map→Peek` 链融合为单函数、单池、64 元素批；机器开销从 14–21× 降到 1–2×（G4 门 ≤3× 达标）。
- **拒绝"只融合"方案**（提案 §8）：初稿只保留融合，评审发现会静默丢掉 v1 的按段配比能力——异构负载（IO 重 + CPU 轻）下逐段定容 + 段间重叠才是对的模型。故定案为"段作用域 + 中途 `Parallel(m)` 开新段"。
- **`Ordered()` 作为可选保序**（G2）：默认无序兼容 v1；有序模式 = 索引盖章 + 消费端重排（§3.3）。
- **段边界显式化**：有状态算子/类型变更/终结/新 Parallel 都关段（§3.4），替代 v1 的不可见滚动作用域。
- **批大小 64**（§3.2，门 A1）摊销 channel 成本；**泄漏安全复用 v1 的 cancel/drain 模式且只实现一次**（G5）。

历史缺陷（goroutine 泄漏、seededRand 数据竞争、空流 Take panic、负数 Pick、并行 Distinct 崩溃、Execute 丢失 parallelSize）均已修复并有回归测试（`parallel_test.go` / `export_test.go`）。`parallel_v2_test.go` 验证融合单池、段关闭、Ordered 等价性；`branch_test.go` 覆盖中途取消、逐算子短路、Pick 物化路径、外来 `Streamer` 实现回退等防御分支；语句覆盖率 100%。刻意权衡：泄漏检测用 `runtime.NumGoroutine` 轮询而非 goleak，保住零依赖（空 `go.sum`）。

## 5. 与标准库 / 第三方的定位对比

- **vs `slices`/`maps`/`iter`（标准库）**：标准库是即时求值的零散函数；本库在其上提供**可组合的惰性管道 + 声明式并行**（`slices.Values`、`slices.SortFunc` 被内部复用，`Seq()`/`From`/`From2` 双向互操作）。
- **vs 社区流库（如 Java Streams、各 Go stream 克隆）**：语义对标 Java Streams（组合子集合几乎一一对应，`doc.go`/README 均自述 "Java Streams-like"），但并行模型不同——Java 的 `.parallel()` 是全局 ForkJoinPool 开关，本库是**段作用域 + 显式 `n` + 可选保序**，且 v2 融合设计针对"轻负载下并行反而更慢（14–21×）"这一实测问题。仓库自身未对标引用任何具体第三方流库。
- **零依赖 + Go 1.26 `iter` 原生**：不引 channel-free 以外的并发框架（拒绝 `errgroup` 外部组合——那正是本库存在的理由，提案 §8）。

## 6. 下游真实使用（skr / ivy）

- **skr**（`github.com/tr1v3r/stream v0.1.1-...`，go.mod）——IO 密集场景的典型用法：
  - `internal/service/notion/rss.go:36`：`stream.SliceOf(resources...).WithContext(ctx).Parallel(8).ForEach(...)` —— RSS 元数据并发刷新，回调内自管超时与错误。
  - `internal/service/notion/stocks.go:58`：`SliceOf(stocks...).WithContext(ctx).Parallel(3).ForEach(...)` —— 股票元数据并发查询。
  - `internal/service/notion/stocks.go:88-98`：`stream.From(generator, -1).WithContext(ctx).Parallel(6).Map(重试抓行情).Parallel(6).ForEach(回写 Notion)` —— 惰性源 + **两段式 Parallel**（Map 段抓取、ForEach 段回写），回调内 select ctx.Done 实现协作取消，nil 元素作哨兵跳过。
- **ivy**（`v0.0.1`，go.mod）——`export_test.go:64`：`stream.SliceOf(urls[0], urls[1]).Parallel(64).Convert(...).Collect(...)`，因无序输出改用 map 按内容匹配断言。

共同模式：`WithContext` + `Parallel(n)` + `ForEach/Map` 处理批量 IO；错误处理下沉到回调；依赖无序语义时在消费端按内容匹配。

## 8. 边界与非目标

- **流是单次使用的**：终结操作消费底层 `iter.Seq`；需重迭代请先 `Execute()` 物化快照。
- **无限源 × 非短路终结 = 挂起**：`Repeat`/无界 `From` 配 `ToSlice`/`Reduce*`/`Count`/`Last` 会永不返回，必须 `Limit` 或 `WithContext` 加界。
- **`Distinct`/`DistinctBy` 刻意串行**：共享 key map 非并发安全；它们是段边界，事后保留 parallelSize 供下游续段。
- **v2 非目标**（提案 §2）：不追求多终结复用池（单次使用语义下每终结一池可接受）、不并行化有状态算子本体（Sort 仍是物化后排序）、不改动串行路径性能与惰性闭包核心。
- **`Peek` 参与融合**（提案 Q1）：回调在 worker goroutine 上并发执行，副作用须自行保证并发安全——与 v1 一致，文档化而非禁止。
- **已知问题**：无（历史缺陷均有回归测试守护）。

## 9. 扩展新算子的约定

1. 在 `export.go` 的 `Streamer` 接口加方法；2. 在 `stream.go` 组合 `iter.Seq` 闭包保持惰性；3. 显式决定 sizeHint 传播；4. 需要并行时累积进 `fused`（`thenFused`），靠 `ensureFlushed`/`effectiveSeq` 关段，并保持 cancel/drain 模式（feeder 单一取消出口、纯阻塞发送；worker 取消后 drain；消费端退出时排干 out）；
5. 补 `*_test.go` 断言（并行结果按排序多重集比较，顺序不保证），涉及并行的改动必须跑 `-race`；
6. 更新 README 与 doc.go，保持三者语义一致（README 的用法示例、doc.go 的 godoc 分节、本文件的架构叙述）。
