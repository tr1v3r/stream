package stream

import (
	"context"
	"iter"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = newStreamer[any](nil, 0)
	_ Streamer[float64] = newStreamer[float64](nil, 0)

	seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func materialize[T any](seq iter.Seq[T]) []T {
	result := make([]T, 0, 64)
	for v := range seq {
		result = append(result, v)
	}
	return result
}

func seqFromSlice[T any](s []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range s {
			if !yield(v) {
				return
			}
		}
	}
}

type streamer[T any] struct {
	ctx          context.Context
	seq          iter.Seq[T]
	sizeHint     int64 // known size, or -1 for unknown
	parallelSize int   // 0=sync, >0=parallel workers
}

func newStreamer[T any](seq iter.Seq[T], sizeHint int64) *streamer[T] {
	return &streamer[T]{ctx: context.Background(), seq: seq, sizeHint: sizeHint}
}

func (s *streamer[T]) cancelled() bool { return s.ctx.Err() != nil }

func (s streamer[T]) WithContext(ctx context.Context) Streamer[T] {
	s.ctx = ctx
	return &s
}

// parallelSeq fans out elements to N workers and collects results.
func (s *streamer[T]) parallelSeq(work func(T, chan<- T)) iter.Seq[T] {
	prev := s.seq
	n := s.parallelSize
	return func(yield func(T) bool) {
		in := make(chan T, 1024)
		out := make(chan T, 1024)

		go func() {
			defer close(in)
			for v := range prev {
				in <- v
			}
		}()

		var wg sync.WaitGroup
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for v := range in {
					work(v, out)
				}
			}()
		}

		go func() {
			wg.Wait()
			close(out)
		}()

		for v := range out {
			if s.cancelled() || !yield(v) {
				return
			}
		}
	}
}

func (s *streamer[T]) wrap(newSeq iter.Seq[T], newHint int64) *streamer[T] {
	return &streamer[T]{ctx: s.ctx, seq: newSeq, sizeHint: newHint, parallelSize: s.parallelSize}
}

func (s *streamer[T]) Filter(judge types.Judge[T]) Streamer[T] {
	if s.parallelSize > 0 {
		return s.wrap(s.parallelSeq(func(t T, ch chan<- T) {
			if judge(t) {
				ch <- t
			}
		}), -1)
	}
	prev := s.seq
	return s.wrap(func(yield func(T) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			if judge(v) && !yield(v) {
				return
			}
		}
	}, -1)
}

func (s *streamer[T]) Map(m types.Mapper[T]) Streamer[T] {
	if s.parallelSize > 0 {
		return s.wrap(s.parallelSeq(func(t T, ch chan<- T) {
			ch <- m(t)
		}), s.sizeHint)
	}
	prev := s.seq
	return s.wrap(func(yield func(T) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			if !yield(m(v)) {
				return
			}
		}
	}, s.sizeHint)
}

func (s *streamer[T]) Convert(convert types.Converter[T, any]) Streamer[any] {
	prev := s.seq
	parallel := s.parallelSize
	return &streamer[any]{ctx: s.ctx, seq: func(yield func(any) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			if !yield(convert(v)) {
				return
			}
		}
	}, sizeHint: s.sizeHint, parallelSize: parallel}
}

func (s *streamer[T]) Peek(consumer types.Consumer[T]) Streamer[T] {
	if s.parallelSize > 0 {
		return s.wrap(s.parallelSeq(func(t T, ch chan<- T) {
			consumer(t)
			ch <- t
		}), s.sizeHint)
	}
	prev := s.seq
	return s.wrap(func(yield func(T) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			consumer(v)
			if !yield(v) {
				return
			}
		}
	}, s.sizeHint)
}

func (s *streamer[T]) FlatMap(f func(T) Streamer[any]) Streamer[any] {
	prev := s.seq
	return &streamer[any]{ctx: s.ctx, seq: func(yield func(any) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			for item := range f(v).Seq() {
				if !yield(item) {
					return
				}
			}
		}
	}, sizeHint: -1}
}

func (s *streamer[T]) Distinct() Streamer[T] { return s.Filter(distinctJudge[T]()) }

func (s *streamer[T]) Sort(comparator types.Comparator[T]) Streamer[T] {
	prev := s.seq
	hint := s.sizeHint
	return s.wrap(func(yield func(T) bool) {
		data := materialize(prev)
		sort.Sort(&Sortable[T]{List: data, Cmp: comparator})
		for _, v := range data {
			if !yield(v) {
				return
			}
		}
	}, hint)
}

func (s *streamer[T]) ReverseSort(comparator types.Comparator[T]) Streamer[T] {
	prev := s.seq
	hint := s.sizeHint
	return s.wrap(func(yield func(T) bool) {
		data := materialize(prev)
		sort.Sort(sort.Reverse(&Sortable[T]{List: data, Cmp: comparator}))
		for _, v := range data {
			if !yield(v) {
				return
			}
		}
	}, hint)
}

func (s *streamer[T]) Reverse() Streamer[T] {
	prev := s.seq
	hint := s.sizeHint
	return s.wrap(func(yield func(T) bool) {
		data := materialize(prev)
		for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
			data[i], data[j] = data[j], data[i]
		}
		for _, v := range data {
			if !yield(v) {
				return
			}
		}
	}, hint)
}

func (s *streamer[T]) Limit(l int64) Streamer[T] {
	prev := s.seq
	newHint := l
	if s.sizeHint >= 0 && s.sizeHint < l {
		newHint = s.sizeHint
	}
	return s.wrap(func(yield func(T) bool) {
		count := int64(0)
		for v := range prev {
			if s.cancelled() || count >= l {
				return
			}
			count++
			if !yield(v) {
				return
			}
		}
	}, newHint)
}

func (s *streamer[T]) Skip(n int64) Streamer[T] {
	prev := s.seq
	newHint := int64(-1)
	if s.sizeHint >= 0 {
		newHint = max(s.sizeHint-n, 0)
	}
	return s.wrap(func(yield func(T) bool) {
		skipped := int64(0)
		for v := range prev {
			if s.cancelled() {
				return
			}
			if skipped < n {
				skipped++
				continue
			}
			if !yield(v) {
				return
			}
		}
	}, newHint)
}

func (s *streamer[T]) Pick(start, end, interval int) Streamer[T] {
	prev := s.seq
	return s.wrap(func(yield func(T) bool) {
		// Resolve negative end: use sizeHint if known, otherwise materialize
		if end < 0 {
			if s.sizeHint >= 0 {
				end = int(s.sizeHint) - 1
			} else {
				data := materialize(prev)
				for i := start; i < len(data); i += interval {
					if !yield(data[i]) {
						return
					}
				}
				return
			}
		}
		if start < 0 || interval <= 0 {
			return
		}
		idx := 0
		for v := range prev {
			if idx > end {
				return
			}
			if idx >= start && (idx-start)%interval == 0 {
				if !yield(v) {
					return
				}
			}
			idx++
		}
	}, -1)
}

func (s *streamer[T]) Append(data ...T) Streamer[T] {
	prev := s.seq
	newHint := s.sizeHint + int64(len(data))
	if s.sizeHint < 0 {
		newHint = -1
	}
	return s.wrap(func(yield func(T) bool) {
		for v := range prev {
			if !yield(v) {
				return
			}
		}
		for _, v := range data {
			if !yield(v) {
				return
			}
		}
	}, newHint)
}

func (s *streamer[T]) Execute() Streamer[T] {
	data := materialize(s.seq)
	return newStreamer(seqFromSlice(data), int64(len(data))).WithContext(s.ctx)
}

func (s streamer[T]) Parallel(n int) Streamer[T] {
	if n <= 0 {
		return &s
	}
	s.parallelSize = n
	return &s
}

func (s *streamer[T]) ToSlice() []T {
	return materialize(s.seq)
}

func (s *streamer[T]) Collect(to types.Collector[T]) any {
	return to(s.ToSlice()...)
}

func (s *streamer[T]) ForEach(consumer types.Consumer[T]) {
	for v := range s.seq {
		if s.cancelled() {
			return
		}
		consumer(v)
	}
}

func (s *streamer[T]) AllMatch(judge types.Judge[T]) bool {
	for v := range s.seq {
		if s.cancelled() || !judge(v) {
			return false
		}
	}
	return true
}

func (s *streamer[T]) NonMatch(judge types.Judge[T]) bool {
	for v := range s.seq {
		if s.cancelled() || judge(v) {
			return false
		}
	}
	return true
}

func (s *streamer[T]) AnyMatch(judge types.Judge[T]) bool {
	for v := range s.seq {
		if s.cancelled() {
			return false
		}
		if judge(v) {
			return true
		}
	}
	return false
}

func (s *streamer[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for v := range s.seq {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

func (s *streamer[T]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	result := initValue
	for v := range s.seq {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

func (s *streamer[T]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	result := initValue
	for v := range s.seq {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

func (s *streamer[T]) ReduceBy(initValueBuilder func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	result := initValueBuilder(int(s.sizeHint))
	for v := range s.seq {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

func (s *streamer[T]) First() T {
	var zero T
	for v := range s.seq {
		return v
	}
	return zero
}

func (s *streamer[T]) Take() T {
	data := materialize(s.seq)
	return data[seededRand.Int63n(int64(len(data)))]
}

func (s *streamer[T]) Any() T { return s.Take() }

func (s *streamer[T]) Last() T {
	var result T
	for v := range s.seq {
		result = v
	}
	return result
}

func (s *streamer[T]) Count() int64 {
	if s.sizeHint >= 0 {
		return s.sizeHint
	}
	var count int64
	for range s.seq {
		count++
	}
	return count
}

func (s *streamer[T]) Seq() iter.Seq[T] {
	return s.seq
}

type Sortable[T any] struct {
	List []T
	Cmp  types.Comparator[T]
}

func (a *Sortable[T]) Len() int           { return len(a.List) }
func (a *Sortable[T]) Less(i, j int) bool { return a.Cmp(a.List[i], a.List[j]) < 0 }
func (a *Sortable[T]) Swap(i, j int)      { a.List[i], a.List[j] = a.List[j], a.List[i] }
