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
   - `parallelSize int` for concurrent processing mode (0=sync)
   - All intermediate ops compose `iter.Seq[T]` closures (true lazy)
   - `parallelSeq` helper: feeder goroutine → N workers → out channel
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
- **`Convert` and `FlatMap` produce `Streamer[any]`**: Type info is lost; prefer the generic `stream.MapTo[T, R]` (Convert is deprecated and delegates to it) — for FlatMap, use `AnyTo[T]()` or `To[T, R]()` to convert back.
- **Parallel mode does not preserve order**. Also note: only `Filter`/`Map`/`Peek` have parallel branches — `Convert`, `FlatMap`, `Sort`, `Reverse`, and `Distinct` (serial by design, see stream.go) silently ignore `Parallel(n)`.
- **Each parallel-aware op spawns its own worker pool**: `Parallel(4).Filter(...).Map(...)` = two chained pools; keep pipelines short or expect overhead.
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

## Common Development Tasks

When adding new stream operations:
1. Add method to `Streamer` interface in `export.go`
2. Implement in `streamer[T]` in `stream.go` (compose an `iter.Seq` closure; keep lazy)
3. Update `sizeHint` propagation deliberately (see table above)
4. For parallel support, use `parallelSeq` helper or handle `parallelSize > 0`. Leak-freedom pattern (keep it): `parallelSeq` derives a cancellable child ctx; the feeder has a single cancellation exit (top-of-loop check, plain blocking send — workers always drain `in`, discarding after cancel); the consumer drains `out` on any exit. Avoid select-with-Done on the feeder send: when both cases are ready Go picks randomly, which made coverage and exit paths nondeterministic.
5. Add assertion-based tests: `factory_test.go` / `ops_test.go` / `terminal_test.go` / `parallel_test.go` / `branch_test.go` hold the suite (statement coverage 100%); parallel results must be compared as sorted multisets (order is not preserved). `TestParallel_ShortCircuitNoLeak` and `TestParallel_ConcurrentTakeNoRace` guard the concurrency fixes — always run `-race` before shipping parallel changes. `branch_test.go` covers defensive branches (mid-stream cancellation, downstream short-circuit per op, Pick materialize path, foreign Streamer fallback) — extend it when adding new branches.
6. Update `README.md` and `doc.go` documentation
