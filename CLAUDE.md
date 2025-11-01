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
# Run linter
golangci-lint run --config=.golangci.yml

# Fix linting issues (if supported by linters)
golangci-lint run --config=.golangci.yml --fix
```

## Architecture

### Core Components

1. **Streamer Interface** (`export.go`): Main interface defining stream operations
   - Stateless operations: `Filter`, `Map`, `Convert`, `Peek`
   - Stateful operations: `Distinct`, `Sort`, `Reverse`, `Limit`, `Skip`, `Pick`
   - Terminal operations: `ToSlice`, `Collect`, `ForEach`, `Reduce`, `Count`

2. **Iterator Pattern** (`iterator.go`): Core abstraction for data traversal
   - `staticIter`: For finite collections
   - `supplyIter`: For infinite/streaming data sources
   - `anyIter`: For type conversion between `T` and `any`

3. **Stream Implementations** (`stream.go`, `async.go`):
   - `streamer`: Synchronous stream processing
   - `asyncStreamer`: Parallel stream processing with worker pools

4. **Factory Functions** (`fatcory.go`): Stream creation utilities
   - `SliceOf`: Create stream from slice
   - `Of`: Create stream from supplier function
   - `Repeat`: Create infinite repeating stream
   - `Concat`: Combine multiple streams

### Key Design Patterns

- **Lazy Evaluation**: Operations are chained and only executed when terminal operation is called
- **Iterator Abstraction**: Unified interface for different data sources (static collections, generators, etc.)
- **Type Safety**: Generic types with proper type constraints
- **Context Support**: All operations respect context cancellation
- **Parallel Processing**: Worker pool-based parallel execution for async streams

### Package Structure

- `stream/`: Main package with core stream functionality
- `stream/types/`: Type definitions for functional interfaces
- `stream/tests/`: Test cases and examples

## Important Implementation Details

### Iterator Types
- **Static Iterator**: For finite collections, supports random access
- **Supply Iterator**: For infinite data sources, uses supplier functions
- **Dead Iterator**: Empty iterator for edge cases

### Stream Operations
- **Intermediate Operations**: Return new streams (lazy evaluation)
- **Terminal Operations**: Execute the pipeline and return results
- **Parallel Operations**: Use `Parallel(n)` to enable concurrent processing

### Error Handling
- Context cancellation is respected throughout the pipeline
- Panic recovery should be implemented for production use
- Type conversion errors may occur when using `Convert` operations

## Testing

Test files are located in:
- `export_test.go`: Core stream functionality tests
- `tests/`: Additional test cases and examples

Tests demonstrate various stream operations including filtering, mapping, reduction, and parallel processing.

## Dependencies

- `github.com/tr1v3r/pkg`: For worker pool implementation in async streams
- Go 1.20+: Required for generic types

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