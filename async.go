package stream

import (
	"math/rand"

	"github.com/tr1v3r/pkg/pools"
	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = newAsyncStreamer[any](1, nil)
	_ Streamer[float64] = newAsyncStreamer[float64](1, nil)
)

type asyncStage[T any] func() <-chan T

func newAsyncStreamer[T any](parallelSize int, ch <-chan T) *asyncStreamer[T] {
	return &asyncStreamer[T]{parallelSize: parallelSize, stage: func() <-chan T { return ch }}
}

func wrapAsyncStreamer[T any](parallelSize int, stage asyncStage[T]) *asyncStreamer[T] {
	return &asyncStreamer[T]{parallelSize: parallelSize, stage: stage}
}

// asyncStreamer underlying p streamer implement for Streamer
type asyncStreamer[T any] struct {
	parallelSize int
	stage        asyncStage[T]
}

func (s *asyncStreamer[T]) Append(data ...T) Streamer[T] { return s.sync().Append(data...) }
func (s *asyncStreamer[T]) Execute() Streamer[T]         { return s.sync().Execute() }
func (s *asyncStreamer[T]) Parallel(n int) Streamer[T] {
	if n <= 0 {
		return s.sync()
	}
	s.parallelSize = n
	return s
}

func (s *asyncStreamer[T]) Filter(judge types.Judge[T]) Streamer[T] {
	return wrapAsyncStreamer[T](s.parallelSize, s.wrapAsyncStage(func(t T, ch chan<- T) {
		if judge(t) {
			ch <- t
		}
	}))
}
func (s *asyncStreamer[T]) Map(m types.Mapper[T]) Streamer[T] {
	return wrapAsyncStreamer[T](s.parallelSize, s.wrapAsyncStage(func(t T, ch chan<- T) {
		ch <- m(t)
	}))
}
func (s *asyncStreamer[T]) Peek(consumer types.Consumer[T]) Streamer[T] {
	return wrapAsyncStreamer[T](s.parallelSize, s.wrapAsyncStage(func(t T, ch chan<- T) {
		consumer(t)
		ch <- t
	}))
}

func (s *asyncStreamer[T]) Convert(convert types.Converter[T, any]) Streamer[any] {
	return wrapAsyncStreamer[any](s.parallelSize, func() <-chan any {
		ch := make(chan any, 1024)
		go func(size int) {
			defer close(ch)
			pool := pools.NewPool(size)
			for t := range s.stage() {
				pool.Wait()
				go func(t T) {
					defer pool.Done()
					ch <- convert(t)
				}(t)
			}
			pool.WaitAll()
		}(s.parallelSize)
		return ch
	})
}

func (s *asyncStreamer[T]) Distinct() Streamer[T] { return s.Filter(distinctJudge[T]()) }
func (s *asyncStreamer[T]) Sort(comparator types.Comparator[T]) Streamer[T] {
	return s.sync().Sort(comparator)
}
func (s *asyncStreamer[T]) ReverseSort(comparator types.Comparator[T]) Streamer[T] {
	return s.sync().ReverseSort(comparator)
}
func (s *asyncStreamer[T]) Reverse() Streamer[T]      { return s.sync().Reverse() }
func (s *asyncStreamer[T]) Limit(l int64) Streamer[T] { return s.sync().Limit(l) }
func (s *asyncStreamer[T]) Skip(n int64) Streamer[T]  { return s.sync().Skip(n) }
func (s *asyncStreamer[T]) Pick(start, end, interval int) Streamer[T] {
	return s.sync().Pick(start, end, interval)
}

func (s *asyncStreamer[T]) Collect(to types.Collector[T]) any { return s.sync().Collect(to) }

func (s *asyncStreamer[T]) ForEach(consumer types.Consumer[T]) {
	pool := pools.NewPool(s.parallelSize)
	for t := range s.stage() {
		pool.Wait()
		go func(t T) {
			defer pool.Done()
			consumer(t)
		}(t)
	}
	pool.WaitAll()
}
func (s *asyncStreamer[T]) ToSlice() []T { return s.fetchAll() }
func (s *asyncStreamer[T]) AllMatch(judge types.Judge[T]) bool {
	for t := range s.stage() {
		if !judge(t) {
			return false
		}
	}
	return true
}
func (s *asyncStreamer[T]) NonMatch(judge types.Judge[T]) bool {
	for t := range s.stage() {
		if judge(t) {
			return false
		}
	}
	return true
}
func (s *asyncStreamer[T]) AnyMatch(judge types.Judge[T]) bool {
	for t := range s.stage() {
		if judge(t) {
			return true
		}
	}
	return false
}
func (s *asyncStreamer[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for t := range s.stage() {
		result = accumulator(result, t)
	}
	return result
}
func (s *asyncStreamer[T]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	var result T = initValue
	for t := range s.stage() {
		result = accumulator(result, t)
	}
	return result
}
func (s *asyncStreamer[T]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	var result = initValue
	for t := range s.stage() {
		result = accumulator(result, t)
	}
	return result
}
func (s *asyncStreamer[T]) ReduceBy(initValueBulider func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	return s.sync().ReduceBy(initValueBulider, accumulator)
}

func (s *asyncStreamer[T]) First() T { return <-s.stage() }
func (s *asyncStreamer[T]) Take() T {
	data := s.fetchAll()
	return data[rand.Intn(len(data))]
}
func (s *asyncStreamer[T]) Last() T {
	data := s.fetchAll()
	return data[len(data)-1]
}

func (s *asyncStreamer[T]) Count() int64 { return s.sync().Count() }

func (s *asyncStreamer[T]) sync() Streamer[T] {
	return wrapStreamer[T](newIterator[T](nil), func(iter iterator[T]) iterator[T] { return iter.Concat(newIterator[T](s.fetchAll())) })
}
func (s *asyncStreamer[T]) fetchAll() (source []T) {
	for t := range s.stage() {
		source = append(source, t)
	}
	return source
}

func (s *asyncStreamer[T]) wrapAsyncStage(work func(T, chan<- T)) asyncStage[T] {
	return func() <-chan T {
		ch := make(chan T, 1024)
		go func(size int) {
			defer close(ch)
			pool := pools.NewPool(size)
			for t := range s.stage() {
				pool.Wait()
				go func(t T) {
					defer pool.Done()
					work(t, ch)
				}(t)
			}
			pool.WaitAll()
		}(s.parallelSize)
		return ch
	}
}
