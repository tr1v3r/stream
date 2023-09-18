package stream

import (
	"math/rand"

	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any, int]     = newStreamer[any, int]()
	_ Streamer[float64, int] = newStreamer[float64, int]()
)

func newStreamer[T, R any](data ...T) *streamer[T, R] { return &streamer[T, R]{data: data} }

// streamer underlying streamer implement for Streamer
type streamer[T, R any] struct {
	data []T
}

func (s *streamer[T, R]) Filter(judge types.Judge[T]) Streamer[T, R] {
	var results []T
	for _, item := range s.data {
		if judge(item) {
			results = append(results, item)
		}
	}
	return newStreamer[T, R](results...)
}
func (s *streamer[T, R]) Map(mapper types.Mapper[T, R]) Streamer[R, any] {
	var results []R
	for _, item := range s.data {
		results = append(results, mapper(item))
	}
	return newStreamer[R, any](results...)
}
func (s *streamer[T, R]) Peek(consumer types.Consumer[T]) Streamer[T, R] {
	for _, item := range s.data {
		consumer(item)
	}
	return s
}

func (s *streamer[T, R]) Distinct() Streamer[T, R] {
	for _, item := range s.data {
		// TODO implement distinct logic
		_ = item
	}
	return s
}
func (s *streamer[T, R]) Sort(comparator types.Comparator[T]) Streamer[T, R] {
	// TODO implement sort logic
	return s
}
func (s *streamer[T, R]) ReverseSort(comparator types.Comparator[T]) Streamer[T, R] {
	// TODO implement reverse sort logic
	return s
}
func (s *streamer[T, R]) Reverse() Streamer[T, R] {
	for i, length := 0, len(s.data)-1; i <= length/2; i++ {
		s.data[i], s.data[length-i] = s.data[length-i], s.data[i]
	}
	return s
}
func (s *streamer[T, R]) Limit(l int) Streamer[T, R] {
	if int(l) < len(s.data) {
		s.data = s.data[:l]
	}
	return s
}
func (s *streamer[T, R]) Skip(i int) Streamer[T, R] {
	if int(i) < len(s.data) {
		s.data = s.data[i:]
	} else {
		s.data = nil
	}
	return s
}
func (s *streamer[T, R]) Pick(start, end, interval int) Streamer[T, R] {
	length := len(s.data)
	if start >= length || end < 0 || interval < 0 || interval >= length {
		s.data = nil
		return s
	}

	if start < 0 {
		start = 0
	}
	if end >= length {
		end = length - 1
	}

	var results []T
	for i := start; i <= end; i += interval {
		results = append(results, s.data[i])
	}
	s.data = results

	return s
}

func (s *streamer[T, R]) ForEach(consumer types.Consumer[T]) {
	for _, item := range s.data {
		consumer(item)
	}
}
func (s *streamer[T, R]) ToSlice() []T {
	return s.data
}
func (s *streamer[T, R]) AllMatch(judge types.Judge[T]) bool {
	for _, item := range s.data {
		if !judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T, R]) NonMatch(judge types.Judge[T]) bool {
	for _, item := range s.data {
		if judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T, R]) AnyMatch(judge types.Judge[T]) bool {
	for _, item := range s.data {
		if judge(item) {
			return true
		}
	}
	return false
}
func (s *streamer[T, R]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T, R]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	var result T = initValue
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T, R]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	var result = initValue
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T, R]) ReduceBy(initValueBulider func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	var result = initValueBulider(len(s.data))
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T, R]) First() T {
	var t T
	if s.Count() > 0 {
		t = s.data[0]
	}
	return t
}
func (s *streamer[T, R]) Take() T {
	var t T
	if count := s.Count(); count > 0 {
		t = s.data[rand.Int63n(count)]
	}
	return t
}
func (s *streamer[T, R]) Last() T {
	var t T
	if count := s.Count(); count > 0 {
		t = s.data[count-1]
	}
	return t
}
func (s *streamer[T, R]) Count() int64 {
	return int64(len(s.data))
}
