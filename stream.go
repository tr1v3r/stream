package stream

type (
	// Judge calculate t and return true or false
	Judge[T any] func(T) bool

	Mapper[T, R any] func(T) R

	Comparator[T any] func(T, T) int

	Consumer[T any] func(T) error

	BinaryOperator[T any] func(T, T) T

	Accumulator[T, R any] func(R, T) R
)

type Streamer[T, R any] interface {
	// stateless operate 无状态操作

	// // Filter filter data by Judge result
	// Filter(Judge[T]) Streamer[T, R]
	// Map convert every T to R
	Map(Mapper[T, R]) Streamer[R, any]
	// Peek(Consumer[T]) Streamer[T, R]

	// stateful operate 有状态操作

	// // Distinct()
	// Sort(Comparator[T]) Streamer[T, R]
	// Reverse(Comparator[T]) Streamer[T, R]
	// // Limit limit data
	// Limit(int64) Streamer[T, R]
	// Skip(int64) Streamer[T, R]
	// Pick(start, end, interval int64) Streamer[T, R]

	// // terminal operate 终止操作

	// // ForEach
	// ForEach(Consumer[T])
	// // ToSlice
	// ToSlice() []T
	// // AllMatch
	// AllMatch(Judge[T]) bool
	// NonMatch(Judge[T]) bool
	// AnyMatch(Judge[T]) bool
	Reduce(accumulator BinaryOperator[T]) T
	// ReduceFrom(initValue T, accumulator BinaryOperator[T]) T
	// ReduceWith(initValue any, accumulator Accumulator[T, any]) any
	// // Count return count result
	// Count() int64
}

// Slice receive array and initlize streamer
func Slice[T, R any](array []T) Streamer[T, R] { return &streamer[T, R]{data: array} }

// var _ Streamer[any, int] = new(stream[int])
// var _ Streamer[float64, int] = new(stream[float64])
type streamer[T, R any] struct {
	data []T
}

func (s *streamer[T, R]) Map(mapper Mapper[T, R]) Streamer[R, any] {
	var results []R
	for _, item := range s.data {
		results = append(results, mapper(item))
	}
	return &streamer[R, any]{data: results}
}

func (s *streamer[T, R]) Reduce(accumulator BinaryOperator[T]) T {
	var result T
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
