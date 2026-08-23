# AGENTS.md

This file provides guidance for AI coding agents (Claude Code, Codex, etc.) working in this repository.

> Note: `CLAUDE.md` is a symlink to this file. Edit **AGENTS.md** only — never edit through the symlink target name.

## Project Overview

Go stream processing library (`github.com/tr1v3r/stream`) providing Java Streams-like functionality. Core pipeline representation is `iter.Seq[T]`, enabling true lazy evaluation with short-circuit support. Zero external dependencies; `go.sum` is empty. `go.mod` declares `go 1.26.0`.

## Development Commands

```bash
# Run all tests
go test ./...

# Run tests with verbose output / race detector
go test ./... -v
go test ./... -race   # recommended for anything touching Parallel()

# Run linting (tests/ dir is excluded via .golangci.yml skip-dirs)
golangci-lint run --config=.golangci.yml
```

## Architecture

### Core Components

1. **Streamer Interface** (`export.go`): Main interface defining stream operations
   - Stateless operations: `Filter`, `Map`, `Convert`, `Peek`, `FlatMap`
   - Stateful operations: `Distinct`, `Sort`, `ReverseSort`, `Reverse`, `Limit`, `Skip`, `Pick`
   - Terminal operations: `ToSlice`, `Collect`, `ForEach`, `Reduce`, `Count`, `Seq`
   - Match operations: `AllMatch`, `NonMatch`, `AnyMatch`
   - Element operations: `First`, `Take`, `Any`, `Last`
   - Reduce variants: `Reduce`, `ReduceFrom`, `ReduceWith`, `ReduceBy`

2. **Stream Implementation** (`stream.go`): Core `streamer[T]` struct
   - Holds `iter.Seq[T]` as internal pipeline
   - `ctx context.Context` for cancellation (`WithContext` to set)
   - `sizeHint int64` for known-size optimizations (-1 for unknown)
   - `parallelSize int` for concurrent section workers (0=sync)
   - `ordered bool` marks an order-preserving section
   - All intermediate ops compose `iter.Seq[T]` closures (true lazy)
   - Parallel sections: `fused func(T) (T, bool)` accumulates stateless stages; `flushFused` runs them on one worker pool (`fusedFeeder`/`fusedWorkers`, or `orderedFeeder`/`orderedWorkers`/`orderedYield` for `Ordered()`)
   - Sorting via `slices.SortFunc` (type-safe pdqsort, no sort.Interface adapter)

3. **Factory Functions** (`factory.go`): `SliceOf`, `Repeat`, `RepeatN`, `Concat`, `From` (from `iter.Seq[T]`), `From2` (from `iter.Seq2[K,V]`, values only)

4. **Helper Functions** (`helper.go`)
   - `To[T, R]`: Convert slice type with converter function
   - `AnyTo[T]`: Convert `[]any` to typed slice
   - Package-level generics in `stream.go`: `DistinctBy[T, K comparable]` (dedup by exact keys; `Distinct` delegates with `fmt.Sprint`/`types.Unique` keys) and `MapTo[T, R]` (type-safe transform; `Convert` delegates and is deprecated)

### Package Structure (flat layout — NOT nested under stream/)

- Root package `stream`: `export.go`, `stream.go`, `factory.go`, `helper.go`, `doc.go`, `export_test.go`
- `types/`: Functional interface type definitions
- `tests/`: Exercise-style test cases and examples (excluded from lint)

### Type Definitions (`types/type.go`)

`Judge[T]`, `Mapper[T]`, `Converter[T,R]`, `Comparator[T]`, `Consumer[T]`, `BinaryOperator[T]`, `Accumulator[T,R]`, `Collector[T]`, and `Unique` interface (custom distinct key via `Key() string`).

## Important Implementation Details

### Critical Gotchas

- **Streams are single-use**: A terminal operation consumes the underlying `iter.Seq`. Create a new stream for each pipeline.
- **Lazy evaluation**: Intermediate operations compose closures without executing. `Limit(1)` + `First()` on a million elements processes only 1 element.
- **Distinct uses `fmt.Sprint`** for hashing by default (`1` and `"1"` collide). Implement `types.Unique` for custom hash keys, or prefer the generic `DistinctBy[T, K comparable]` for exact keys (also much faster: no boxing).
- **`Convert` and `FlatMap` produce `Streamer[any]`**: Type info is lost; prefer the generic `stream.MapTo[T, R]` (Convert is deprecated and delegates to it). Trade-off: MapTo is a function, so it interrupts method chaining at the type-changing point — recommend it for head-of-pipeline type changes (chaining resumes below); Convert stays acceptable for mid-chain changes in throwaway code. For FlatMap, use `AnyTo[T]()` or `To[T, R]()` to convert back.
- **Parallel sections are unordered by default** (`Ordered()` opts into serial-order output; see docs/proposals/parallel-v2.md). Parallelism is section-scoped: `Parallel(n)` opens a fused section (single pool over Filter/Map/Peek chains, 64-element batches); sections close at stateful ops, type changes, terminals, and the next `Parallel` call. `Convert`/`FlatMap`/`Sort`/`Reverse`/`Distinct` therefore run serially after a section closes (Distinct is serial by design — shared key map).
- **v1 per-op pools are gone**: sections fuse stateless ops into one pool (machinery overhead went from 14-21x to 1-2x serial on near-free workloads). The leak-freedom pattern lives in `flushFused`/`orderedSeq`; ordered re-sequencing is batch-keyed with hole placeholders — see the proposal's design-journey note before touching it.
- **`Pick` with negative `end`** must materialize the entire stream to determine size.

### sizeHint Propagation

- `Filter`, `Distinct`, `DistinctBy`, `FlatMap`, `Pick`: hint becomes -1
- `Map`, `Peek`, `Convert`: hint preserved
- `Limit(n)`: min(hint, n) if hint >= 0
- `Skip(n)`: max(0, hint - n) if hint >= 0
- `Sort`, `ReverseSort`, `Reverse`, `Append`: hint preserved / additive
- `Count()` short-circuits to `sizeHint` when known — keep the hint honest when adding ops

## Known Issues

- None currently. Historical issues (parallel goroutine leak, seededRand race, empty-stream Take panic, negative Pick index, parallel Distinct crash, Execute dropping parallelSize) are fixed and guarded by regression tests in `parallel_test.go` / `export_test.go`.

## Deliberate Trade-offs

- **Leak detection stays NumGoroutine-based (no goleak)**: `TestParallel_ShortCircuitNoLeak` polls `runtime.NumGoroutine` with a 5s deadline instead of using `go.uber.org/goleak` stack-snapshot comparison. Decision: the zero-dependency property (empty go.sum) outweighs the precision gain. Parallel v2 has added goroutine paths (fused/ordered feeders and workers), but they all instantiate the one shared cancel/drain pattern in `flushFused`/`orderedSeq` and are exercised by the leak, short-circuit, and A3 property tests. Re-evaluate if section kinds multiply further (a hand-rolled ~40-line `runtime.Stack` diff remains the zero-dependency option).

## Common Development Tasks

When adding new stream operations:
1. Add method to `Streamer` interface in `export.go`
2. Implement in `streamer[T]` in `stream.go` (compose an `iter.Seq` closure; keep lazy)
3. Update `sizeHint` propagation deliberately (see table above)
4. For parallel support, accumulate into the fused section (`fused` + `thenFused`) and let `ensureFlushed`/`effectiveSeq` close it. Leak-freedom pattern (keep it): `flushFused`/`orderedSeq` derive a cancellable child ctx; the feeder has a single cancellation exit (top-of-loop check, plain blocking send — workers always drain `in`, discarding after cancel); the consumer drains `out` on any exit. Avoid select-with-Done on the feeder send: when both cases are ready Go picks randomly, which made coverage and exit paths nondeterministic.
5. Add assertion-based tests: `factory_test.go` / `ops_test.go` / `terminal_test.go` / `parallel_test.go` / `branch_test.go` hold the suite (statement coverage 100%); parallel results must be compared as sorted multisets (order is not preserved). `TestParallel_ShortCircuitNoLeak` and `TestParallel_ConcurrentTakeNoRace` guard the concurrency fixes — always run `-race` before shipping parallel changes. `branch_test.go` covers defensive branches (mid-stream cancellation, downstream short-circuit per op, Pick materialize path, foreign Streamer fallback) — extend it when adding new branches.
6. Update `README.md` and `doc.go` documentation
