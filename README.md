# stream

A Go stream processing library that brings Java Streams-like functional operations to Go collections using generics.

Requires Go 1.20+.

## Features

- **Lazy evaluation** — intermediate operations build a pipeline; nothing runs until a terminal operation
- **Generics** — type-safe streams with `Streamer[T]`
- **Parallel processing** — concurrent execution via worker pools
- **Functional pipelines** — filter, map, reduce, sort, distinct, and more
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
| `Of[T](supply Supplier[T])` | Create a stream from a supplier function |
| `Repeat[T](t T)` | Create an infinite stream repeating `t` |
| `RepeatN[T](t T, n int64)` | Create a stream repeating `t` exactly `n` times |
| `Concat[T](dst, ...src)` | Concatenate multiple streams |

```go
// From a supplier
nums := stream.Of(func() (int, bool) { return rand.Intn(100), true })

// Repeat
fives := stream.RepeatN(5, 10) // [5, 5, 5, 5, 5, 5, 5, 5, 5, 5]
```

## Intermediate Operations

### Stateless

| Method | Signature | Description |
|--------|-----------|-------------|
| `Filter` | `(Judge[T]) Streamer[T]` | Keep elements matching the predicate |
| `Map` | `(Mapper[T]) Streamer[T]` | Transform each element (same type) |
| `Convert` | `(Converter[T, any]) Streamer[any]` | Transform to a different type |
| `Peek` | `(Consumer[T]) Streamer[T]` | Apply an action without modifying elements |

```go
stream.SliceOf(1, 2, 3, 4).
    Filter(func(n int) bool { return n > 2 }).   // [3, 4]
    Map(func(n int) int { return n * 10 }).       // [30, 40]
    Peek(func(n int) { fmt.Println(n) })          // prints 30, 40
```

### Stateful

| Method | Signature | Description |
|--------|-----------|-------------|
| `Distinct` | `() Streamer[T]` | Remove duplicate elements |
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
```

## Terminal Operations

### Collecting

| Method | Signature | Description |
|--------|-----------|-------------|
| `ToSlice` | `() []T` | Collect all elements into a slice |
| `Collect` | `(Collector[T]) any` | Collect using a custom collector |
| `ForEach` | `(Consumer[T])` | Iterate over each element |
| `Count` | `() int64` | Return the number of elements |

```go
results := stream.SliceOf(1, 2, 3).ToSlice()
count := stream.SliceOf(1, 2, 3).Count() // 3
```

### Reduce

| Method | Signature | Description |
|--------|-----------|-------------|
| `Reduce` | `(BinaryOperator[T]) T` | Reduce with zero-value init |
| `ReduceFrom` | `(T, BinaryOperator[T]) T` | Reduce with explicit init value |
| `ReduceWith` | `(any, Accumulator[T, any]) any` | Reduce with different accumulator type |
| `ReduceBy` | `(initBuilder, Accumulator[T, any]) any` | Reduce with size-aware init builder |

```go
sum := stream.SliceOf(1, 2, 3).Reduce(func(a, b int) int { return a + b }) // 6

joined := stream.SliceOf("a", "b", "c").
    ReduceWith("", func(acc any, s string) any { return acc.(string) + s }).(string)
// "abc"
```

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
| `Take` | `() T` | Random element |
| `Any` | `() T` | Alias for Take |
| `Last` | `() T` | Last element |

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
- `n == 1`: asynchronous single worker
- `n >= 2`: concurrent workers with a worker pool

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
type Supplier[T any] func() (T, bool)             // Element source (false = done)
type Unique interface{ Key() string }             // Custom distinct key
```

## Important Notes

- **Streams are single-use.** A terminal operation consumes the stream. Create a new stream for each pipeline.
- **Distinct uses `fmt.Sprint`** by default for hashing. Implement the `types.Unique` interface (`Key() string`) for custom hash keys.
- **Supplier-based streams** (created with `Of`, `Repeat`) have unknown size. Operations like `Count()`, `Take()`, and `Last()` will panic on these streams. Use `Limit()` first to bound them.

## License

[MIT](LICENSE)
