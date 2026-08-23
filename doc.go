// Package stream provides Java Streams-like functional operations on Go collections.
//
// It enables true lazy evaluation, parallel processing, and functional-style pipelines
// using Go generics and the iter package (requires Go 1.26+, per go.mod).
//
// Intermediate operations compose iter.Seq[T] closures without executing any work.
// Processing is deferred until a terminal operation ranges over the pipeline.
// Short-circuit operations (First, AnyMatch, Limit) naturally stop early.
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
//   - From: from an iter.Seq[T] (supports infinite streams)
//   - From2: from an iter.Seq2[K, V]
//   - Repeat: infinite repeating element
//   - RepeatN: element repeated N times
//   - Concat: combine multiple streams
//
// # Operations
//
// Intermediate (lazy, return a new stream):
//   - Stateless: Filter, Map, Convert (deprecated: use generic MapTo), Peek, FlatMap
//   - Stateful: Distinct (or generic DistinctBy), Sort, ReverseSort, Reverse, Limit, Skip, Pick
//
// Terminal (eager, execute the pipeline):
//   - Collect: ToSlice, Collect
//   - Iterate: ForEach, Seq (native iter.Seq[T] for range loops)
//   - Reduce: Reduce, ReduceFrom, ReduceWith, ReduceBy
//   - Match: AllMatch, NonMatch, AnyMatch
//   - Element: First, Take, Any, Last
//   - Count: Count
//
// # iter.Seq Integration
//
// The Seq() method returns the underlying iter.Seq[T] for use with Go's range:
//
//	for v := range stream.SliceOf(1, 2, 3).Seq() {
//	    fmt.Println(v)
//	}
//
// # Parallel Processing
//
// Use Parallel(n) to enable concurrent processing. n controls concurrency:
//
//   - 0: no change (synchronous)
//
//   - 1+: concurrent workers
//
//     stream.SliceOf(data...).Parallel(4).Filter(...).ForEach(...)
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
//
// Infinite sources (Repeat, unbounded From) hang non-short-circuiting terminal
// operations such as ToSlice, Reduce, Count, Last, or Take without a cancellable
// context — bound them with Limit or WithContext.
package stream
