package stream

import (
	"context"
	"iter"

	"github.com/tr1v3r/stream/types"
)

type Streamer[T any] interface {
	// WithContext set Streamer context
	WithContext(context.Context) Streamer[T]

	// stateless operate

	// Filter filter data by Judge result
	Filter(types.Judge[T]) Streamer[T]
	Map(types.Mapper[T]) Streamer[T]
	// Convert transforms elements to any. Deprecated: it loses the element
	// type and forces type assertions downstream; use the generic
	// stream.MapTo[T, R] instead. Kept for backward compatibility.
	//
	// Deprecated: use MapTo.
	Convert(types.Converter[T, any]) Streamer[any]
	Peek(types.Consumer[T]) Streamer[T]
	// FlatMap flattens each element to a sub-stream and concatenates
	FlatMap(func(T) Streamer[any]) Streamer[any]

	// stateful operate

	Distinct() Streamer[T]
	Sort(types.Comparator[T]) Streamer[T]
	ReverseSort(types.Comparator[T]) Streamer[T]
	Reverse() Streamer[T]
	Limit(int64) Streamer[T]
	Skip(int64) Streamer[T]
	Pick(startIndex, endIndex, interval int) Streamer[T]

	// Append append data to streamer source
	Append(...T) Streamer[T]
	// Execute eager execute streamer stage
	Execute() Streamer[T]

	// Parallel 0 do nothing, 1 async work, 2-n concurrent work
	Parallel(int) Streamer[T]

	// terminal operate

	ToSlice() []T
	Collect(types.Collector[T]) any
	ForEach(types.Consumer[T])
	// Match methods
	AllMatch(types.Judge[T]) bool
	NonMatch(types.Judge[T]) bool
	AnyMatch(types.Judge[T]) bool
	// Reduce reduce calculate
	Reduce(accumulator types.BinaryOperator[T]) T
	ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T
	ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any
	ReduceBy(initValueBuilder func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any
	// Pick one
	First() T
	Take() T
	Any() T
	Last() T
	// Count return count result
	Count() int64
	// Seq returns the underlying iter.Seq[T] for native range loops
	Seq() iter.Seq[T]
}
