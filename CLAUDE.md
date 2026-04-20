# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go stream processing library (`github.com/tr1v3r/stream`) that provides Java Streams-like functionality for Go. It enables functional-style operations on collections with support for lazy evaluation, parallel processing, and various stream operations.

## Development Commands

### Building and Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run tests for specific package
go test ./tests

# Run linting
golangci-lint run --config=.golangci.yml
```

### Code Quality
```bash
# Fix linting issues (if supported by linters)
golangci-lint run --config=.golangci.yml --fix
```

## Architecture

### Core Components

1. **Streamer Interface** (`export.go`): Main interface defining stream operations
   - Stateless operations: `Filter`, `Map`, `Convert`, `Peek`
   - Stateful operations: `Distinct`, `Sort`, `ReverseSort`, `Reverse`, `Limit`, `Skip`, `Pick`
   - Terminal operations: `ToSlice`, `Collect`, `ForEach`, `Reduce`, `Count`
   - Match operations: `AllMatch`, `NonMatch`, `AnyMatch`
   - Element operations: `First`, `Take`, `Any`, `Last`
   - Reduce variants: `Reduce`, `ReduceFrom`, `ReduceWith`, `ReduceBy`
   - Eager operations: `Append`, `Execute`

2. **Iterator Pattern** (`iterator.go`): Core abstraction for data traversal
   - `staticIter`: For finite collections
   - `supplyIter`: For infinite/streaming data sources (Size() returns -1)
   - `anyIter`: For type conversion between `T` and `any`
   - `deadIter`: Empty iterator sentinel

3. **Stream Implementations** (`stream.go`, `async.go`):
   - `streamer`: Synchronous stream processing
   - `asyncStreamer`: Parallel stream processing with worker pools

4. **Factory Functions** (`fatcory.go`): Stream creation utilities (note: filename is a typo)
   - `SliceOf`: Create stream from slice
   - `Of`: Create stream from supplier function
   - `Repeat`: Create infinite repeating stream
   - `RepeatN`: Create stream with N repetitions
   - `Concat`: Combine multiple streams

5. **Helper Functions** (`helper.go`): Utility functions
   - `To[T, R]`: Convert slice type with converter function
   - `AnyTo[T]`: Convert `[]any` to typed slice
   - `distinctJudge`: Internal distinct filter using `fmt.Sprint` or `types.Unique`

### Package Structure

- `stream/`: Main package with core stream functionality
- `stream/types/`: Type definitions for functional interfaces
- `stream/tests/`: Test cases and examples

## Important Implementation Details

### Critical Gotchas

- **Streams are single-use**: Calling a terminal operation consumes the underlying iterator. Subsequent terminal calls on the same stream produce incorrect or empty results. Create a new stream for each pipeline.
- **supplyIter panics on size-dependent ops**: `Count()`, `Take()`, `Last()` will panic on supply-based streams because `Size()` returns -1.
- **Distinct uses `fmt.Sprint` for hashing**: By default, `Distinct()` stringifies elements. Implement the `types.Unique` interface for custom hash keys.
- **`Pick` with negative end**: Uses source size as end bound. Only works on static iterators.
- **`Convert` produces `Streamer[any]`**: Type information is lost; use `AnyTo[T]()` or `To[T, R]()` to convert back.

### Iterator Types
- **Static Iterator**: For finite collections, supports random access
- **Supply Iterator**: For infinite data sources, uses supplier functions
- **Dead Iterator**: Empty iterator for edge cases

### Stream Operations
- **Intermediate Operations**: Return new streams (lazy evaluation)
- **Terminal Operations**: Execute the pipeline and return results
- **Parallel Operations**: Use `Parallel(n)` to enable concurrent processing

## Testing

Test files are located in:
- `export_test.go`: Core stream functionality tests
- `tests/`: Additional test cases and examples

Tests demonstrate various stream operations including filtering, mapping, reduction, and parallel processing.

## Dependencies

- `github.com/tr1v3r/pkg`: For worker pool implementation in async streams
- Go 1.20+: Required for generic types

### Type Definitions (`types/type.go`)
- `Judge[T]`, `Mapper[T]`, `Converter[T,R]`, `Comparator[T]`, `Consumer[T]`
- `BinaryOperator[T]`, `Accumulator[T,R]`, `Collector[T]`, `Supplier[T]`
- `Unique`: Interface for custom distinct hashing (`Key() string`)

## Common Development Tasks

When adding new stream operations:
1. Add method to `Streamer` interface in `export.go`
2. Implement in both `streamer` (sync) and `asyncStreamer` (async)
3. Add appropriate tests
4. Update documentation

When working with iterators:
- Use `newIterator()` for static data
- Use `supplyIter` for generator functions
- Handle context cancellation in long-running operations