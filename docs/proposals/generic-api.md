# Go 1.27 concrete pipeline API

Status: implemented alongside the legacy API. Requires Go 1.27; no external
runtime dependencies. The design uses Go 1.27's [generic concrete methods](https://go.dev/doc/go1.27),
not generic interface methods, which the language still does not support.

## API and compatibility

```go
p := stream.Of(1, 2, 2, 3).
    Filter(func(n int) bool { return n > 1 }).
    MapTo(strconv.Itoa).
    DistinctBy(func(s string) string { return s })
var result []string = p.ToSlice()
```

`Pipeline[T]` is a public concrete value holding a private pointer to the existing
stream implementation. Its zero value is empty. Intermediate methods return
Pipeline, so generic methods remain available throughout the chain:

```go
func (Pipeline[T]) MapTo[R any](types.Converter[T, R]) Pipeline[R]
func (Pipeline[T]) FlatMapTo[R any](func(T) Pipeline[R]) Pipeline[R]
func (Pipeline[T]) DistinctBy[K comparable](func(T) K) Pipeline[T]
func (Pipeline[T]) Fold[R any](R, types.Accumulator[T, R]) R
func (Pipeline[T]) CollectTo[R any](func(...T) R) R
```

The existing Streamer interface, factories, package-level generic functions,
and function types retain their exact signatures. In particular, SliceOf still
returns Streamer: changing it would break function values and interface-based
call sites. New code uses Of; existing sources (From, From2, Repeat, RepeatN,
Concat, custom Streamer implementations) enter with Wrap:

```go
concrete := stream.Wrap(stream.RepeatN(3, 2)).MapTo(strconv.Itoa)
var legacy stream.Streamer[string] = concrete.Stream()
```

Pipeline does not implement Streamer, because Go does not allow covariant
intermediate return types. Wrap and Stream neither consume nor duplicate a
source. They cannot make a single-use stream reusable; use Execute for snapshots.
Wrapping native streams preserves metadata and execution. Foreign implementations
are lazily adapted via Seq, with unknown cardinality and serial downstream
defaults; their Seq remains responsible for their own context and execution.
Nil Streamer inputs are unsupported.

This is source compatibility on Go 1.27, **not** continued support for compiling
the new release with Go 1.26. Consumers pinned to Go 1.26 need the previous release.
CI moves to Go 1.27 and golangci-lint v2.15.0.

## Execution choices

- Same-type Map retains its existing non-generic signature and fused parallel
  execution. Generalizing it to arbitrary R would change the monomorphic fused
  stage representation and risks removing an existing optimization.
- MapTo closes the current section and uses the existing typed package function.
  The concrete method carries Ordered preference, as well as context, worker
  count and exact cardinality, into its result.
- FlatMapTo closes the current section and drains typed children serially; children
  may declare their own parallel work. Its result has unknown cardinality and
  retains context, worker count and Ordered preference for later operations.
  Execution cancellation is connected to outer and native child sources, including
  children whose filters reject every subsequent value.
- The concrete Convert and FlatMap compatibility methods retain any-valued result
  types but delegate to the typed methods, sharing their option-preserving behavior.
  The old Streamer methods are unchanged.
- DistinctBy delegates to the existing serial exact-key implementation, with
  parallel options retained. It does not introduce shared parallel map access.
- Fold carries R directly through iteration, without any-valued accumulators.
  Cancellation returns the partial accumulator; empty input returns the initial
  value. CollectTo materializes input, then calls its typed collector once even
  for empty/cancelled input. It uses O(n) memory, like Collect; Fold is incremental.
- Callback panics, source ownership, and cooperative cancellation limitations are
  unchanged. Callbacks already running must cooperate; foreign sources that never
  yield cannot be interrupted through Seq.

## Validation

The original suite remains unchanged and compiles against the legacy signatures.
New tests cover typed chains, lazy short circuits, exact/unknown cardinality,
zero values, snapshots, explicit/inferred method type arguments, method values
and expressions, foreign adapters, cancellation and partial results, and preserved
worker counts and encounter order after type changes. Race runs include the
seven correctness regressions from PR #30. A deterministic allocation test guards
against reintroducing per-element accumulator boxing into Fold.

The existing G1–G7 performance gate is retained without changing thresholds.
BenchmarkGenericAPI compares equivalent results on the same compiler and data;
package-level generics are controls, not presumed slower baselines.

## Local performance comparison

Environment: Go 1.27.1, Apple M3 Pro, darwin/arm64, default GOMAXPROCS=12.
Each operation processes 4096 ints (1024 through 5119); FlatMap emits two int64s
per input. Five samples per case, 200ms benchmark duration per sample. Numbers
below are sample medians, not cross-machine promises or statistical significance
claims. Raw samples and commands are in [the benchmark log](../benchmarks/generic-api-go1.27.1.txt).

| Workload | Existing path ns/op | Concrete path ns/op | Existing allocs/op | Concrete allocs/op |
|---|---:|---:|---:|---:|
| Same-type Filter/Map/Reduce | 28,054 | 27,712 | 15 | 15 |
| MapTo: package function vs method | 35,552 | 35,780 | 22 | 22 |
| Convert + typed recovery vs MapTo | 77,132 | 35,780 | 4,121 | 22 |
| FlatMap + typed recovery vs FlatMapTo | 527,581 | 438,748 | 45,083 | 36,893 |
| ReduceWith vs Fold (int64 accumulator) | 42,046 | 18,966 | 4,104 | 7 |
| Collect vs CollectTo (int64 slice result) | 30,060 | 29,455 | 21 | 19 |
| DistinctBy: package function vs method | 38,048 | 38,093 | 26 | 26 |

MapTo and DistinctBy method syntax is effectively at parity with the already-typed
package functions. The useful gains come from removing any-valued data paths:
MapTo used 128,248 vs 305,081 B/op, Fold used 280 vs 33,064 B/op, and FlatMapTo used
1,570,300 vs 1,927,129 B/op. FlatMap still allocates many child pipelines; this
change does not claim to solve that separate cost.

Ordered Parallel(4) on the near-free Filter/Map/Reduce workload was noisy. The
first short run showed 112,506 ns/op for legacy and 128,089 for Pipeline (~14%
slower). A follow-up with 1s samples gave medians 112,168 and 115,282 (~2.8% slower),
with overlapping ranges (111,182–123,679 and 112,580–119,207). Allocations were
173 vs 172. Both rounds are retained: no parallel speedup is claimed, and the
short-run gap is not established as stable overhead.

These results isolate API paths under Go 1.27.1; they do not measure the effect
of upgrading the compiler from Go 1.26. No benchmark thresholds were relaxed.
