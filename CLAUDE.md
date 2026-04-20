# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go stream processing library (`github.com/tr1v3r/stream`) that provides Java Streams-like functionality for Go. It uses `iter.Seq[T]` from Go 1.23+ as the core pipeline representation, enabling true lazy evaluation with short-circuit support.

## Development Commands

### Building and Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run linting
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
   - `sizeHint int64` for known-size optimizations (-1 for unknown)
   - `parallelSize int` for concurrent processing mode
   - All intermediate ops compose `iter.Seq[T]` closures (true lazy)
   - `parallelSeq` helper for N-worker concurrent processing
   - `Sortable[T]` for sort.Interface support

3. **Factory Functions** (`factory.go`): Stream creation utilities
   - `SliceOf`: Create stream from slice
   - `Of`: Create stream from supplier function
   - `Repeat`/`RepeatN`: Infinite/finite repeating streams
   - `Concat`: Combine multiple streams
   - `Of`: Create from `iter.Seq[T]`
   - `OfSeq2`: Create from `iter.Seq2[K, V]`

4. **Helper Functions** (`helper.go`): Utility functions
   - `To[T, R]`: Convert slice type with converter function
   - `AnyTo[T]`: Convert `[]any` to typed slice
   - `distinctJudge`: Internal distinct filter using `fmt.Sprint` or `types.Unique`

### Package Structure

- `stream/`: Main package with core stream functionality
- `stream/types/`: Type definitions for functional interfaces
- `stream/tests/`: Test cases and examples

## Important Implementation Details

### Critical Gotchas

- **Streams are single-use**: Calling a terminal operation consumes the underlying `iter.Seq`. Create a new stream for each pipeline.
- **Lazy evaluation**: Intermediate operations compose closures without executing. Work happens only in terminal operations. `Limit(1)` followed by `First()` on a million-element stream processes only 1-2 elements.
- **Distinct uses `fmt.Sprint`** for hashing by default. Implement the `types.Unique` interface for custom hash keys.
- **`Convert` and `FlatMap` produce `Streamer[any]`**: Type information is lost; use `AnyTo[T]()` or `To[T, R]()` to convert back.
- **Parallel mode does not preserve order**: Elements may arrive out of order with `Parallel(n)` where n > 1.
- **`Pick` with negative `end`**: Must materialize the entire stream to determine size.

### sizeHint Propagation

- `Filter`, `Distinct`, `FlatMap`: hint becomes -1
- `Map`, `Peek`: hint preserved
- `Limit(n)`: min(hint, n) if hint >= 0
- `Skip(n)`: max(0, hint - n) if hint >= 0
- `Sort`, `ReverseSort`, `Reverse`: hint preserved

## Dependencies

- No external dependencies (removed `github.com/tr1v3r/pkg`)
- Go 1.23+: Required for `iter.Seq[T]` and generic types

### Type Definitions (`types/type.go`)
- `Judge[T]`, `Mapper[T]`, `Converter[T,R]`, `Comparator[T]`, `Consumer[T]`
- `BinaryOperator[T]`, `Accumulator[T,R]`, `Collector[T]`, `Supplier[T]`
- `Unique`: Interface for custom distinct hashing (`Key() string`)

## Common Development Tasks

When adding new stream operations:
1. Add method to `Streamer` interface in `export.go`
2. Implement in `streamer[T]` in `stream.go`
3. For parallel support, use `parallelSeq` helper or handle `parallelSize > 0` case
4. Add appropriate tests
5. Update documentation
