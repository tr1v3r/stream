// Package stream provides Java Streams-like functional operations on Go collections.
//
// It enables lazy evaluation, parallel processing, and functional-style pipelines
// using Go generics (requires Go 1.20+).
//
// # Quick Start
//
// Create a stream from a slice, apply intermediate operations, and collect results:
//
//	sum := stream.SliceOf(1, 2, 3, 4, 5).
//	    Filter(func(n int) bool { return n%2 == 1 }).
//	    Map(func(n int) int { return n * n }).
//	    Reduce(func(a, b int) int { return a + b })
//	// sum == 35
//
// # Stream Creation
//
// Use factory functions to create streams:
//   - SliceOf: from a slice or variadic elements
//   - Of: from a supplier function (supports infinite streams)
//   - Repeat: infinite repeating element
//   - RepeatN: element repeated N times
//   - Concat: combine multiple streams
//
// # Operations
//
// Intermediate (lazy, return a new stream):
//   - Stateless: Filter, Map, Convert, Peek
//   - Stateful: Distinct, Sort, ReverseSort, Reverse, Limit, Skip, Pick
//
// Terminal (eager, execute the pipeline):
//   - Collect: ToSlice, Collect
//   - Iterate: ForEach
//   - Reduce: Reduce, ReduceFrom, ReduceWith, ReduceBy
//   - Match: AllMatch, NonMatch, AnyMatch
//   - Element: First, Take, Any, Last
//   - Count: Count
//
// # Parallel Processing
//
// Use Parallel(n) to enable concurrent processing. n controls concurrency:
//   - 0: no change (synchronous)
//   - 1: asynchronous single-worker
//   - 2+: concurrent workers
//
//	stream.SliceOf(data...).Parallel(4).Filter(...).ForEach(...)
//
// # Helper Functions
//
//   - To[T, R]: converts a slice of T to a slice of R via a converter
//   - AnyTo[T]: converts []any to []T via type assertion
//
// # Important
//
// Streams are single-use. Each terminal operation consumes the underlying iterator.
// Create a new stream for each pipeline.
package stream
