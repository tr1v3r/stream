package stream

import "github.com/tr1v3r/stream/types"

type Streamer[T any] interface {
	// stateless operate 无状态操作

	// Filter filter data by Judge result
	Filter(types.Judge[T]) Streamer[T]
	// Map convert every T to R
	Map(types.Mapper[T, any]) Streamer[any]
	// Peek peek ecah data
	Peek(types.Consumer[T]) Streamer[T]

	// stateful operate 有状态操作

	// Distinct deduplicate data in source
	Distinct() Streamer[T]
	Sort(types.Comparator[T]) Streamer[T]
	ReverseSort(types.Comparator[T]) Streamer[T]
	Reverse() Streamer[T]
	// Limit limit data
	Limit(int64) Streamer[T]
	Skip(int64) Streamer[T]
	Pick(start, end, interval int) Streamer[T]

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
	Last() T
	// Max() T
	// Min() T
	// Count return count result
	Count() int64
}
