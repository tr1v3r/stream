package stream

import "github.com/tr1v3r/stream/types"

type Streamer[T, R any] interface {
	// stateless operate 无状态操作

	// Filter filter data by Judge result
	Filter(types.Judge[T]) Streamer[T, R]
	// Map convert every T to R
	Map(types.Mapper[T, R]) Streamer[R, any]
	// Peek peek ecah data
	Peek(types.Consumer[T]) Streamer[T, R]

	// stateful operate 有状态操作

	// Distinct deduplicate data in source
	Distinct() Streamer[T, R]
	Sort(types.Comparator[T]) Streamer[T, R]
	ReverseSort(types.Comparator[T]) Streamer[T, R]
	Reverse() Streamer[T, R]
	// Limit limit data
	Limit(int) Streamer[T, R]
	Skip(int) Streamer[T, R]
	Pick(start, end, interval int) Streamer[T, R]

	// terminal operate 终止操作

	// ForEach
	ForEach(types.Consumer[T])
	// ToSlice
	ToSlice() []T
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
	// Count return count result
	Count() int64
}
