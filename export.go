package stream

import (
	"context"

	"github.com/tr1v3r/stream/types"
)

type Streamer[T any] interface {
	// WithContext set Streamer context
	WithContext(context.Context) Streamer[T]

	// stateless operate 无状态操作

	// Filter filter data by Judge result
	Filter(types.Judge[T]) Streamer[T]
	Map(types.Mapper[T]) Streamer[T]
	Convert(types.Converter[T, any]) Streamer[any]
	Peek(types.Consumer[T]) Streamer[T]
	// FlatMap(func(T) Streamer[any]) Streamer[any]

	// stateful operate 有状态操作

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

	// terminal operate 终止操作

	// ToSlice
	ToSlice() []T
	Collect(types.Collector[T]) any
	// ForEach
	ForEach(types.Consumer[T])
	// Match methods
	AllMatch(types.Judge[T]) bool
	NonMatch(types.Judge[T]) bool
	AnyMatch(types.Judge[T]) bool
	// Reduce reduce calculate
	Reduce(accumulator types.BinaryOperator[T]) T
	ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T
	ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any
	// ReduceBy use `buildInitValue` to build the initValue, which parameter is a int64 means element size, or -1 if unknown size.
	// Then use `accumulator` to add each element to previous result
	ReduceBy(initValueBulider func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any
	// Pick one
	First() T
	Take() T
	Any() T
	Last() T
	// Cout return count result
	Count() int64
}
