package stream

import (
	"math/rand"

	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = newStreamer[any]()
	_ Streamer[float64] = newStreamer[float64]()
)

// newStreamer return streamer
func newStreamer[T any](data ...T) *streamer[T] {
	return &streamer[T]{source: newIterator[T](data)}
}

// streamer underlying streamer implement for Streamer
type streamer[T any] struct {
	source iterator[T]
}

func (s *streamer[T]) Filter(judge types.Judge[T]) Streamer[T] {
	var results []T
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		if judge(item) {
			results = append(results, item)
		}
	}
	return newStreamer[T](results...)
}
func (s *streamer[T]) Map(mapper types.Mapper[T, any]) Streamer[any] {
	var results []any
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		results = append(results, mapper(item))
	}
	return newStreamer[any](results...)
}
func (s *streamer[T]) Peek(consumer types.Consumer[T]) Streamer[T] {
	s.source.Reset()
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		consumer(item)
	}
	return s
}

func (s *streamer[T]) Distinct() Streamer[T] {
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
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
	var data = s.source.AllLeft()
	for i, length := 0, len(data)-1; i <= length/2; i++ {
		data[i], data[length-i] = data[length-i], data[i]
	}
	return newStreamer[T](data...)
}
func (s *streamer[T]) Limit(l int64) Streamer[T] {
	s.source.Reset()

	var data []T
	for i := 0; i < int(l) && s.source.HasNext(); i++ {
		data = append(data, s.source.Next())
	}
	return newStreamer[T](data...)
}
func (s *streamer[T]) Skip(n int64) Streamer[T] {
	s.source.NextN(n) // skip n
	return newStreamer[T](s.source.AllLeft()...)
}
func (s *streamer[T]) Pick(start, end, interval int) Streamer[T] {
	s.source.NextN(int64(start)) // skip start

	var results []T
	for i := 0; i < end && s.source.HasNext(); i++ { // pick
		item := s.source.Next()
		if i%interval != 0 {
			results = append(results, item)
		}
	}

	return newStreamer[T](results...)
}

func (s *streamer[T]) Collect(to types.Collector[T]) any {
	return to(s.source.AllLeft()...)
}
func (s *streamer[T]) ForEach(consumer types.Consumer[T]) {
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		consumer(item)
	}
}
func (s *streamer[T]) ToSlice() []T {
	return s.source.AllLeft()
}
func (s *streamer[T]) AllMatch(judge types.Judge[T]) bool {
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		if !judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T]) NonMatch(judge types.Judge[T]) bool {
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		if judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T]) AnyMatch(judge types.Judge[T]) bool {
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		if judge(item) {
			return true
		}
	}
	return false
}
func (s *streamer[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	var result T = initValue
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	var result = initValue
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T]) ReduceBy(initValueBulider func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	var result = initValueBulider(int(s.source.Size()))
	for item := s.source.Next(); s.source.HasNext(); item = s.source.Next() {
		result = accumulator(result, item)
	}
	return result
}
func (s *streamer[T]) First() T {
	return s.source.Next()
}
func (s *streamer[T]) Take() T {
	return s.source.NextN(rand.Int63n(s.source.Size()))
}
func (s *streamer[T]) Last() T {
	return s.source.NextN(s.source.Size() - 1)
}
func (s *streamer[T]) Count() int64 {
	return s.source.Size()
}
