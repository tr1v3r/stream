package stream

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"slices"
	"sync"

	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = newStreamer[any](nil, 0)
	_ Streamer[float64] = newStreamer[float64](nil, 0)
)

func materialize[T any](seq iter.Seq[T]) []T {
	result := make([]T, 0, 64)
	for v := range seq {
		result = append(result, v)
	}
	return result
}

func seqFromSlice[T any](s []T) iter.Seq[T] { return slices.Values(s) }

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
// It derives a cancellable child context so that an early consumer exit
// (short-circuit) or parent cancellation releases the feeder, workers,
// and closer goroutines instead of leaking them.
func (s *streamer[T]) parallelSeq(work func(T, chan<- T)) iter.Seq[T] {
	prev := s.seq
	n := s.parallelSize
	return func(yield func(T) bool) {
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()

		in := make(chan T, 1024)
		out := make(chan T, 1024)

		go func() {
			defer close(in)
			for v := range prev {
				if ctx.Err() != nil {
					return
				}
				in <- v // workers always drain in, even after cancellation
			}
		}()

		var wg sync.WaitGroup
		for range n {
			wg.Go(func() {
				for v := range in {
					if ctx.Err() != nil {
						continue // cancelled: discard, keep draining so the feeder never blocks
					}
					work(v, out)
				}
			})
		}

		go func() {
			wg.Wait()
			close(out)
		}()

		// On any exit (natural, short-circuit, cancellation) stop the
		// pipeline and drain out until it is closed, so workers blocked
		// on send are released.
		defer func() {
			cancel()
			for range out {
			}
		}()

		for v := range out {
			if ctx.Err() != nil || !yield(v) {
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
	return MapTo(s, convert)
}

// MapTo transforms each element of s from T to R, preserving ctx, sizeHint
// and parallelSize. Unlike Convert it keeps the result type, so no
// Streamer[any] round-trip with type assertions is needed:
//
//	names := stream.MapTo(stream.SliceOf(1, 2, 3), func(n int) string {
//	    return fmt.Sprintf("#%d", n)
//	}).ToSlice()
func MapTo[T, R any](s Streamer[T], m types.Converter[T, R]) Streamer[R] {
	st, ok := s.(*streamer[T])
	if !ok {
		return From(func(yield func(R) bool) {
			for v := range s.Seq() {
				if !yield(m(v)) {
					return
				}
			}
		}, -1)
	}
	prev := st.seq
	return &streamer[R]{ctx: st.ctx, seq: func(yield func(R) bool) {
		for v := range prev {
			if st.cancelled() {
				return
			}
			if !yield(m(v)) {
				return
			}
		}
	}, sizeHint: st.sizeHint, parallelSize: st.parallelSize}
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

// Distinct removes duplicate elements, keeping first occurrences. Keys come
// from fmt.Sprint (or types.Unique), so int 1 and string "1" collide — prefer
// DistinctBy for exact keys.
func (s *streamer[T]) Distinct() Streamer[T] {
	return DistinctBy(s, func(t T) string {
		if keyer, ok := any(t).(types.Unique); ok {
			return keyer.Key()
		}
		return fmt.Sprint(t)
	})
}

// DistinctBy removes elements whose key, produced by key, has already been
// seen, keeping first occurrences. Keys use Go map equality (K comparable),
// avoiding the string-coercion collisions of Distinct. Like Distinct it runs
// serially — the shared key map is not concurrency-safe — while preserving
// parallelSize for downstream operations.
func DistinctBy[T any, K comparable](s Streamer[T], key func(T) K) Streamer[T] {
	seen := make(map[K]struct{})
	judge := func(t T) bool {
		k := key(t)
		if _, dup := seen[k]; dup {
			return false
		}
		seen[k] = struct{}{}
		return true
	}
	st, ok := s.(*streamer[T])
	if !ok {
		return s.Filter(judge) // foreign Streamer implementations
	}
	prev := st.seq
	return st.wrap(func(yield func(T) bool) {
		for v := range prev {
			if st.cancelled() {
				return
			}
			if judge(v) && !yield(v) {
				return
			}
		}
	}, -1)
}

func (s *streamer[T]) Sort(comparator types.Comparator[T]) Streamer[T] {
	prev := s.seq
	hint := s.sizeHint
	return s.wrap(func(yield func(T) bool) {
		data := materialize(prev)
		slices.SortFunc(data, comparator)
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
		slices.SortFunc(data, func(a, b T) int { return comparator(b, a) })
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
		slices.Reverse(data)
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
		if start < 0 || interval <= 0 {
			return
		}
		// Resolve negative end (last index): use sizeHint if known,
		// otherwise materialize to find out
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
	// keep ctx and parallelSize so downstream ops behave as before the snapshot
	return &streamer[T]{ctx: s.ctx, seq: seqFromSlice(data), sizeHint: int64(len(data)), parallelSize: s.parallelSize}
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

// Take returns a uniformly-sampled element without materializing the whole
// stream: reservoir sampling keeps O(1) memory (a single slot here) and works
// on infinite streams, terminating as soon as the source is exhausted or
// cancelled. On an empty stream it returns the zero value of T.
func (s *streamer[T]) Take() T {
	var pick T
	seen := int64(0)
	for v := range s.seq {
		if s.cancelled() {
			return pick
		}
		seen++
		// first element seeds the reservoir, later ones replace it with
		// probability 1/seen — every element ends up equally likely
		if seen == 1 || rand.Int64N(seen) == 0 {
			pick = v
		}
	}
	return pick
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
