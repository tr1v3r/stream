# stream

A Go stream processing library that brings Java Streams-like functional operations to Go collections using generics and `iter.Seq`.

Requires Go 1.26+ (see `go.mod`).

## Features

- **True lazy evaluation** — intermediate operations compose `iter.Seq[T]` closures; nothing runs until a terminal operation iterates
- **Short-circuiting** — `First()`, `AnyMatch()`, `Limit()` stop processing as soon as the result is known
- **Generics** — type-safe streams with `Streamer[T]`
- **iter.Seq integration** — `Seq()` method and `From`/`From2` factory functions for native `for range` interop
- **Parallel processing** — concurrent execution via goroutine worker pools
- **Functional pipelines** — filter, map, flatmap, reduce, sort, distinct, and more
- **Infinite streams** — supplier-based streams for generator patterns

## Installation

```bash
go get github.com/tr1v3r/stream
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/tr1v3r/stream"
)

func main() {
    // Filter odd numbers, square them, sum the result
    sum := stream.SliceOf(1, 2, 3, 4, 5).
        Filter(func(n int) bool { return n%2 == 1 }).
        Map(func(n int) int { return n * n }).
        Reduce(func(a, b int) int { return a + b })
    fmt.Println(sum) // 35
}
```

## Stream Creation

| Function | Description |
|----------|-------------|
| `SliceOf[T](slice ...T)` | Create a stream from a slice or variadic elements |
| `From[T](seq, sizeHint)` | Create from an `iter.Seq[T]` (supports infinite streams) |
| `From2[K, V](seq)` | Create from an `iter.Seq2[K, V]` |
| `Repeat[T](t T)` | Create an infinite stream repeating `t` |
| `RepeatN[T](t T, n int64)` | Create a stream repeating `t` exactly `n` times |
| `Concat[T](dst, ...src)` | Concatenate multiple streams |

```go
// From an iter.Seq
fib := stream.From(func(yield func(int) bool) {
    a, b := 0, 1
    for yield(a) { a, b = b, a+b }
}, -1).Limit(10)

// Repeat
fives := stream.RepeatN(5, 10) // [5, 5, 5, 5, 5, 5, 5, 5, 5, 5]
```

## Intermediate Operations

All intermediate operations are lazy — they compose closures without processing elements.

### Stateless

| Method | Signature | Description |
|--------|-----------|-------------|
| `Filter` | `(Judge[T]) Streamer[T]` | Keep elements matching the predicate |
| `Map` | `(Mapper[T]) Streamer[T]` | Transform each element (same type) |
| `Convert` | `(Converter[T, any]) Streamer[any]` | Transform to a different type. Deprecated: use `stream.MapTo` |
| `Peek` | `(Consumer[T]) Streamer[T]` | Apply an action without modifying elements |
| `FlatMap` | `(func(T) Streamer[any]) Streamer[any]` | Flatten each element to a sub-stream |

Package-level generic (methods cannot add type parameters): `stream.MapTo[T, R](s, func(T) R) Streamer[R]` — the type-safe replacement for `Convert`.

### When to use MapTo vs Convert

`MapTo` keeps the result type at compile time — no `Streamer[any]` round-trip, no `Collect(AnyTo[T]())` assertion that can panic at runtime. The trade-off: as a function it interrupts method chaining at the type-changing point, while `Convert` chains fluently but erases types.

```go
// MapTo: type-safe, result is Streamer[string] — recommended default
names := stream.MapTo(stream.SliceOf(1, 2, 3), func(n int) string {
    return fmt.Sprintf("#%d", n)
})

// Head-of-pipeline type change: MapTo costs nothing — chain continues below it
stream.MapTo(stream.SliceOf(employees...), func(e *Employee) Dept { return e.Dept }).
    Filter(func(d Dept) bool { return d.Active }).  // normal chaining resumes
    Map(func(d Dept) string { return d.Name })

// Mid-pipeline type change in a long chain: Convert keeps it readable,
// at the cost of any + a runtime assertion to come back
stream.SliceOf(1, 2, 3, 4).
    Filter(func(n int) bool { return n > 2 }).
    Convert(func(n int) any { return float64(n) * 1.5 }).
    Map(func(x any) any { return x }).              // still Streamer[any] down here
    Collect(stream.AnyTo[float64]()).([]float64)     // runtime type assertion
```

Rule of thumb: prefer `MapTo` (type change at the pipeline head, or safety matters more than fluency); `Convert` remains valid for mid-chain type changes in throwaway code — it is deprecated, not removed, and still works. When Go ships generic methods, a `Map[R](func(T) R) Streamer[R]` method can offer both.

```go
stream.SliceOf(1, 2, 3, 4).
    Filter(func(n int) bool { return n > 2 }).   // [3, 4]
    Map(func(n int) int { return n * 10 }).       // [30, 40]
    Peek(func(n int) { fmt.Println(n) })          // prints 30, 40

// FlatMap
stream.SliceOf(1, 2, 3).
    FlatMap(func(n int) stream.Streamer[any] {
        return stream.SliceOf[any](n, n*10)
    }) // [1, 10, 2, 20, 3, 30]
```

### Stateful

| Method | Signature | Description |
|--------|-----------|-------------|
| `Distinct` | `() Streamer[T]` | Remove duplicate elements |
| `DistinctBy` | `(Streamer[T], func(T) K) Streamer[T]` | Dedup by comparable key (no string coercion) |
| `Sort` | `(Comparator[T]) Streamer[T]` | Sort ascending |
| `ReverseSort` | `(Comparator[T]) Streamer[T]` | Sort descending |
| `Reverse` | `() Streamer[T]` | Reverse element order |
| `Limit` | `(int64) Streamer[T]` | Take at most N elements |
| `Skip` | `(int64) Streamer[T]` | Skip first N elements |
| `Pick` | `(start, end, interval int) Streamer[T]` | Pick elements at intervals |

```go
stream.SliceOf(3, 1, 4, 1, 5).
    Distinct().                                    // [3, 1, 4, 5]
    Sort(func(a, b int) int { return a - b }).     // [1, 3, 4, 5]
    Limit(2)                                       // [1, 3]

// Dedup with exact comparable keys (5x faster, 300x fewer allocs than Distinct)
byDept := stream.DistinctBy(users, func(u User) string { return u.Dept })
```

## Terminal Operations

### Collecting

| Method | Signature | Description |
|--------|-----------|-------------|
| `ToSlice` | `() []T` | Collect all elements into a slice |
| `Collect` | `(Collector[T]) any` | Collect using a custom collector |
| `ForEach` | `(Consumer[T])` | Iterate over each element |
| `Count` | `() int64` | Return the number of elements |

### Reduce

| Method | Signature | Description |
|--------|-----------|-------------|
| `Reduce` | `(BinaryOperator[T]) T` | Reduce with zero-value init |
| `ReduceFrom` | `(T, BinaryOperator[T]) T` | Reduce with explicit init value |
| `ReduceWith` | `(any, Accumulator[T, any]) any` | Reduce with different accumulator type |
| `ReduceBy` | `(initBuilder, Accumulator[T, any]) any` | Reduce with size-aware init builder |

### Match

| Method | Signature | Description |
|--------|-----------|-------------|
| `AllMatch` | `(Judge[T]) bool` | True if all elements match |
| `NonMatch` | `(Judge[T]) bool` | True if no elements match |
| `AnyMatch` | `(Judge[T]) bool` | True if any element matches |

### Element

| Method | Signature | Description |
|--------|-----------|-------------|
| `First` | `() T` | First element |
| `Take` | `() T` | Random element (uniform reservoir sampling, O(1) memory) |
| `Any` | `() T` | Alias for Take |
| `Last` | `() T` | Last element |

## iter.Seq Integration

```go
// Convert a stream to iter.Seq for native range loops
for v := range stream.SliceOf(1, 2, 3).Filter(func(n int) bool { return n > 1 }).Seq() {
    fmt.Println(v) // 2, 3
}

// Create a stream from an existing iter.Seq
seq := slices.Values([]int{10, 20, 30})
stream.From(seq, 3).Map(func(n int) int { return n * 2 }).ToSlice() // [20, 40, 60]

// Create a stream from iter.Seq2 (uses values only)
m := map[string]int{"a": 1, "b": 2}
stream.From2(maps.All(m)).ToSlice() // [1, 2] (order varies)
```

## Parallel Processing

```go
stream.SliceOf(largeData...).
    Parallel(4).                        // 4 concurrent workers
    Filter(heavyPredicate).
    Map(heavyTransform).
    ForEach(process)
```

`Parallel(n)` behavior:
- `n <= 0`: synchronous (no change)
- `n >= 1`: concurrent workers with goroutine pools

Use `WithContext(ctx)` to support cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
stream.SliceOf(data...).WithContext(ctx).Parallel(4).ForEach(work)
```

## Helper Functions

```go
// To converts []T to []R
floats := stream.To(func(n int) float64 { return float64(n) })(1, 2, 3).([]float64)

// AnyTo converts []any to []T
items := stream.AnyTo[int]()(1, 2, 3).([]int)
```

## Type Definitions

The `types` package defines functional interfaces as function types:

```go
type Judge[T any] func(T) bool                    // Predicate
type Mapper[T any] func(T) T                      // Same-type transform
type Converter[T, R any] func(T) R                // Type transform
type Comparator[T any] func(T, T) int             // Ordering
type Consumer[T any] func(T)                      // Side-effect action
type BinaryOperator[T any] func(T, T) T           // Same-type accumulator
type Accumulator[T, R any] func(R, T) R           // Cross-type accumulator
type Collector[T any] func(...T) any              // Collect to result
type Unique interface{ Key() string }             // Custom distinct key
```

## Important Notes

- **Infinite streams hang non-short-circuiting terminals.** `ToSlice`, `ForEach`, `Reduce*`, `Count`, `Last`, and `Take`/`Any` (without a cancellable context) never finish on `Repeat` or an infinite `From` source. Bound them with `Limit` or `WithContext`:

```go
stream.Repeat(1).Limit(100).ToSlice() // bounded: ok

ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
stream.Repeat(1).WithContext(ctx).Take() // cancellable: ok
```

- **Streams are single-use.** A terminal operation consumes the stream. Create a new stream for each pipeline.
- **Lazy evaluation** — intermediate operations compose closures; work happens only during terminal operations. `Limit(1).First()` on a million elements only processes one element.
- **Distinct uses `fmt.Sprint`** by default for hashing. Implement the `types.Unique` interface (`Key() string`) for custom hash keys, or use the generic `stream.DistinctBy` with comparable keys for exact equality without string coercion.
- **Parallel mode does not preserve order.** Elements may be processed out of order when using `Parallel(n)` with `n > 1`. Use `Sort` after parallel operations if order matters.

## License

[MIT](LICENSE)
