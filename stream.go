package stream

import (
	"math/rand"

	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = newStreamer[any]()
	_ Streamer[float64] = newStreamer[float64]()
)

func newStreamer[T any](data ...T) *streamer[T] { return &streamer[T]{data: data} }

// streamer underlying streamer implement for Streamer
type streamer[T any] struct {
	data []T
}

func (s *streamer[T]) Filter(judge types.Judge[T]) Streamer[T] {
	var results []T
	for _, item := range s.data {
		if judge(item) {
			results = append(results, item)
		}
	}
	return newStreamer[T](results...)
}
func (s *streamer[T]) Map(mapper types.Mapper[T, any]) Streamer[any] {
	var results []any
	for _, item := range s.data {
		results = append(results, mapper(item))
	}
	return newStreamer[any](results...)
}
func (s *streamer[T]) Peek(consumer types.Consumer[T]) Streamer[T] {
	for _, item := range s.data {
		consumer(item)
	}
	return s
}

func (s *streamer[T]) Distinct() Streamer[T] {
	for _, item := range s.data {
		// TODO implement distinct logic
		_ = item
	}
	return s
}
func (s *streamer[T]) Sort(comparator types.Comparator[T]) Streamer[T] {
	// TODO implement sort logic
	return s
}
func (s *streamer[T]) ReverseSort(comparator types.Comparator[T]) Streamer[T] {
	// TODO implement reverse sort logic
	return s
}
func (s *streamer[T]) Reverse() Streamer[T] {
	for i, length := 0, len(s.data)-1; i <= length/2; i++ {
		s.data[i], s.data[length-i] = s.data[length-i], s.data[i]
	}
	return s
}
func (s *streamer[T]) Limit(l int) Streamer[T] {
	if int(l) < len(s.data) {
		s.data = s.data[:l]
	}
	return s
}
func (s *streamer[T]) Skip(i int) Streamer[T] {
	if int(i) < len(s.data) {
		s.data = s.data[i:]
	} else {
		s.data = nil
	}
	return s
}
func (s *streamer[T]) Pick(start, end, interval int) Streamer[T] {
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

func (s *streamer[T]) ForEach(consumer types.Consumer[T]) {
	for _, item := range s.data {
		consumer(item)
	}
}
func (s *streamer[T]) ToSlice() []T {
	return s.data
}
func (s *streamer[T]) AllMatch(judge types.Judge[T]) bool {
	for _, item := range s.data {
		if !judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T]) NonMatch(judge types.Judge[T]) bool {
	for _, item := range s.data {
		if judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T]) AnyMatch(judge types.Judge[T]) bool {
	for _, item := range s.data {
		if judge(item) {
			return true
		}
	}
	return false
}
func (s *streamer[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	var result T = initValue
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	var result = initValue
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T]) ReduceBy(initValueBulider func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	var result = initValueBulider(len(s.data))
	for _, item := range s.data {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T]) First() T {
	var t T
	if s.Count() > 0 {
		t = s.data[0]
	}
	return t
}
func (s *streamer[T]) Take() T {
	var t T
	if count := s.Count(); count > 0 {
		t = s.data[rand.Int63n(count)]
	}
	return t
}
func (s *streamer[T]) Last() T {
	var t T
	if count := s.Count(); count > 0 {
		t = s.data[count-1]
	}
	return t
}
func (s *streamer[T]) Count() int64 {
	return int64(len(s.data))
}
