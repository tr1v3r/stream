package stream

import (
	"math/rand"

	"github.com/tr1v3r/pkg/pools"
	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = &asyncStreamer[any]{}
	_ Streamer[float64] = &asyncStreamer[float64]{}
)

func newParallelStreamer[T any](parallelSize int, ch <-chan T) *asyncStreamer[T] {
	return &asyncStreamer[T]{parallelSize: parallelSize, ch: ch}
}

// asyncStreamer underlying p streamer implement for Streamer
type asyncStreamer[T any] struct {
	parallelSize int
	ch           <-chan T
}

func (s *asyncStreamer[T]) Append(data ...T) Streamer[T] { return s.sync().Append(data...) }
func (s *asyncStreamer[T]) Execute() Streamer[T]         { return s.sync() }
func (s *asyncStreamer[T]) Parallel(n int) Streamer[T] {
	if n <= 0 {
		return s.sync()
	}
	s.parallelSize = n
	return s
}

func (s *asyncStreamer[T]) Filter(judge types.Judge[T]) Streamer[T] {
	ch, pool := make(chan T, 1024), pools.NewPool(s.parallelSize)
	go func() {
		defer close(ch)
		for t := range s.ch {
			pool.Wait()
			go func(t T) {
				defer pool.Done()
				if judge(t) {
					ch <- t
				}
			}(t)
		}
		pool.WaitAll()
	}()
	return &asyncStreamer[T]{ch: ch, parallelSize: s.parallelSize}
}
func (s *asyncStreamer[T]) Map(m types.Mapper[T]) Streamer[T] {
	ch, pool := make(chan T, 1024), pools.NewPool(s.parallelSize)
	go func() {
		defer close(ch)
		for t := range s.ch {
			pool.Wait()
			go func(t T) {
				defer pool.Done()
				ch <- m(t)
			}(t)
		}
		pool.WaitAll()
	}()
	return &asyncStreamer[T]{ch: ch, parallelSize: s.parallelSize}
}
func (s *asyncStreamer[T]) Convert(convert types.Converter[T, any]) Streamer[any] {
	ch, pool := make(chan any, 1024), pools.NewPool(s.parallelSize)
	go func() {
		defer close(ch)
		for t := range s.ch {
			pool.Wait()
			go func(t T) {
				defer pool.Done()
				ch <- convert(t)
			}(t)
		}
		pool.WaitAll()
	}()
	return &asyncStreamer[any]{ch: ch, parallelSize: s.parallelSize}
}
func (s *asyncStreamer[T]) Peek(consumer types.Consumer[T]) Streamer[T] {
	ch, pool := make(chan T, 1024), pools.NewPool(s.parallelSize)
	go func() {
		defer close(ch)
		for t := range s.ch {
			pool.Wait()
			go func(t T) {
				defer pool.Done()
				consumer(t)
				ch <- t
			}(t)
		}
		pool.WaitAll()
	}()
	return &asyncStreamer[T]{ch: ch, parallelSize: s.parallelSize}
}

func (s *asyncStreamer[T]) Distinct() Streamer[T] { return s.sync().Distinct() }
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
	go func() {
		for t := range s.ch {
			pool.Wait()
			go func(t T) {
				defer pool.Done()
				consumer(t)
			}(t)
		}
		pool.WaitAll()
	}()
}
func (s *asyncStreamer[T]) ToSlice() []T { return s.fetchAll() }
func (s *asyncStreamer[T]) AllMatch(judge types.Judge[T]) bool {
	for t := range s.ch {
		if !judge(t) {
			return false
		}
	}
	return true
}
func (s *asyncStreamer[T]) NonMatch(judge types.Judge[T]) bool {
	for t := range s.ch {
		if judge(t) {
			return false
		}
	}
	return true
}
func (s *asyncStreamer[T]) AnyMatch(judge types.Judge[T]) bool {
	for t := range s.ch {
		if judge(t) {
			return true
		}
	}
	return false
}
func (s *asyncStreamer[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for t := range s.ch {
		result = accumulator(result, t)
	}
	return result
}
func (s *asyncStreamer[T]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	var result T = initValue
	for t := range s.ch {
		result = accumulator(result, t)
	}
	return result
}
func (s *asyncStreamer[T]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	var result = initValue
	for t := range s.ch {
		result = accumulator(result, t)
	}
	return result
}
func (s *asyncStreamer[T]) ReduceBy(initValueBulider func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	return s.sync().ReduceBy(initValueBulider, accumulator)
}

func (s *asyncStreamer[T]) First() T { return <-s.ch }
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
	return newStreamer[T](newIterator[T](s.fetchAll()))
}
func (s *asyncStreamer[T]) fetchAll() (source []T) {
	for t := range s.ch {
		source = append(source, t)
	}
	return source
}
