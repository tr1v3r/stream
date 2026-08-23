package stream

import (
	"context"
	"iter"

	"github.com/tr1v3r/stream/types"
)

// Streamer is a lazily-evaluated pipeline over elements of type T, in the
// spirit of Java Streams. Intermediate operations compose iter.Seq[T]
// closures and do no work until a terminal operation iterates; short-circuit
// terminals (First, AnyMatch, Limit-fed pipelines) stop early.
//
// Streams are single-use: a terminal operation consumes the underlying
// sequence. Create a new stream for each pipeline.
//
// # Usage pattern
//
// Create (SliceOf, From, Repeat, Concat...), transform (Filter, Map...),
// terminate (ToSlice, Reduce, ForEach...):
//
//	sum := stream.SliceOf(1, 2, 3, 4, 5).
//	    Filter(func(n int) bool { return n%2 == 1 }).
//	    Map(func(n int) int { return n * n }).
//	    Reduce(func(a, b int) int { return a + b })
type Streamer[T any] interface {
	// WithContext sets the context consulted by later operations; a
	// cancelled context makes intermediate operations stop pulling and
	// terminals return promptly. Applies to the returned stream only.
	WithContext(context.Context) Streamer[T]

	// stateless operate

	// Filter keeps elements for which judge returns true. Lazy, order
	// preserving; sizeHint becomes unknown (-1).
	Filter(types.Judge[T]) Streamer[T]
	// Map transforms each element with a same-type function m.
	// For a different result type use the generic stream.MapTo.
	Map(types.Mapper[T]) Streamer[T]
	// Convert transforms elements to any. Deprecated: it loses the element
	// type and forces type assertions downstream; use the generic
	// stream.MapTo[T, R] instead. Kept for backward compatibility.
	//
	// Deprecated: use MapTo.
	Convert(types.Converter[T, any]) Streamer[any]
	// Peek applies consumer to each element as it passes through without
	// changing them; useful for debugging or side effects mid-pipeline.
	Peek(types.Consumer[T]) Streamer[T]
	// FlatMap flattens each element to a sub-stream and concatenates
	// sub-streams' elements. The result is Streamer[any]; use AnyTo or To
	// to recover concrete types. sizeHint becomes unknown.
	FlatMap(func(T) Streamer[any]) Streamer[any]

	// stateful operate

	// Distinct removes duplicate elements keeping first occurrences. Keys
	// come from fmt.Sprint (or types.Unique), so 1 and "1" collide —
	// prefer the generic stream.DistinctBy for exact comparable keys.
	Distinct() Streamer[T]
	// Sort orders elements ascending by comparator (slices.SortFunc).
	// Materializes the pipeline stage when iterated; sizeHint preserved.
	Sort(types.Comparator[T]) Streamer[T]
	// ReverseSort orders elements descending by comparator.
	ReverseSort(types.Comparator[T]) Streamer[T]
	// Reverse yields elements in reverse order; empty input stays empty.
	Reverse() Streamer[T]
	// Limit keeps at most the first n elements (n <= 0 yields empty).
	// Short-circuits the upstream pipeline.
	Limit(int64) Streamer[T]
	// Skip discards the first n elements (n <= 0 keeps everything).
	Skip(int64) Streamer[T]
	// Pick selects elements at absolute indices start, start+interval, ...
	// up to end inclusive; end < 0 means the last index (materializing the
	// stage when sizeHint is unknown); start < 0 or interval <= 0 yields
	// empty.
	Pick(startIndex, endIndex, interval int) Streamer[T]

	// Append yields the stream's elements followed by data. sizeHint grows
	// by len(data) when known.
	Append(...T) Streamer[T]
	// Execute eagerly materializes the pipeline so far and returns a
	// re-iterable snapshot stream; ctx and parallelSize carry over.
	Execute() Streamer[T]

	// Parallel sets worker-pool concurrency for a section of stateless
	// operations: n <= 0 keeps synchronous execution, n >= 1 runs n
	// workers on the section's fused stages (Filter/Map/Peek chain into
	// one pool). A mid-chain call closes the current section and opens a
	// new one; stateful ops and type changes close sections too. Order is
	// not preserved unless Ordered() follows. See
	// docs/proposals/parallel-v2.md.
	Parallel(int) Streamer[T]
	// Ordered marks the current (or next) parallel section as
	// order-preserving: elements are index-tagged and re-sequenced at the
	// consumer, so output matches serial execution order. No-op in serial
	// mode. Costs one index stamp and slot lookup per element.
	Ordered() Streamer[T]

	// terminal operate

	// ToSlice collects all elements into a new slice; hangs on infinite
	// sources unless bounded or cancellable.
	ToSlice() []T
	// Collect drains the stream into the caller-provided collector and
	// returns its result (type any — assert it back).
	Collect(types.Collector[T]) any
	// ForEach applies consumer to every element in order.
	ForEach(types.Consumer[T])
	// AllMatch reports whether judge holds for every element; false on the
	// first violation (short-circuit).
	AllMatch(types.Judge[T]) bool
	// NonMatch reports whether judge holds for no element; false on the
	// first match (short-circuit).
	NonMatch(types.Judge[T]) bool
	// AnyMatch reports whether judge holds for at least one element; true
	// on the first match (short-circuit).
	AnyMatch(types.Judge[T]) bool
	// Reduce folds elements with accumulator, starting from T's zero
	// value; empty input returns the zero value.
	Reduce(accumulator types.BinaryOperator[T]) T
	// ReduceFrom folds elements starting from initValue.
	ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T
	// ReduceWith folds elements into an any-typed accumulator, enabling
	// cross-type reduction; assert the result back to the concrete type.
	ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any
	// ReduceBy builds its initial value from the stream's sizeHint (which
	// may be negative = unknown, e.g. for capacity preallocation), then
	// folds like ReduceWith.
	ReduceBy(initValueBuilder func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any
	// First returns the first element or T's zero value when empty;
	// short-circuits the pipeline.
	First() T
	// Take returns a uniformly random element via reservoir sampling —
	// O(1) memory, honors cancellation; zero value when empty.
	Take() T
	// Any is an alias for Take.
	Any() T
	// Last returns the final element or T's zero value; consumes the whole
	// stream and hangs on infinite sources.
	Last() T
	// Count returns the element count in O(1) when sizeHint is known,
	// otherwise by full iteration.
	Count() int64
	// Seq returns the underlying iter.Seq[T] for native range loops.
	Seq() iter.Seq[T]
}
