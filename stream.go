package stream

import (
	"context"
	"math/rand"
	"sort"

	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = newStreamer[any](nil)
	_ Streamer[float64] = newStreamer[float64](nil)
)

var ctx = context.Background()

type stage[T any] func(iterator[T]) iterator[T]

// newStreamer return streamer
func newStreamer[T any](iter iterator[T]) *streamer[T] {
	return &streamer[T]{ctx: ctx, source: iter, stage: func(iter iterator[T]) iterator[T] { return iter }}
}

// wrapStreamer wrap stage to new streamer
func wrapStreamer[T any](source iterator[T], stage stage[T]) *streamer[T] {
	return &streamer[T]{ctx: ctx, source: source, stage: stage}
}

// streamer underlying streamer implement for Streamer
type streamer[T any] struct {
	ctx context.Context

	source iterator[T]
	stage  stage[T]
}

// WithContext set stream context
func (s streamer[T]) WithContext(ctx context.Context) Streamer[T] {
	s.ctx = ctx
	return &s
}

func (s *streamer[T]) cancelled() bool { return s.ctx.Err() != nil }

// Append append data to streamer source
func (s *streamer[T]) Append(data ...T) Streamer[T] {
	return newStreamer(s.source.Concat(newIterator(data))).WithContext(s.ctx)
}

// Execute eager execute on source
func (s *streamer[T]) Execute() Streamer[T] {
	return newStreamer(s.stage(s.source)).WithContext(s.ctx)
}

func (s streamer[T]) Parallel(n int) Streamer[T] {
	if n <= 0 {
		return &s
	}
	return wrapAsyncStreamer(n, func() <-chan T {
		ch := make(chan T, 1024)
		go func() {
			defer close(ch)
			for source := s.stage(s.source); !s.cancelled() && source.HasNext(); {
				ch <- source.Next()
			}
		}()
		return ch
	}).WithContext(s.ctx)
}

func (s *streamer[T]) Filter(judge types.Judge[T]) Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		source, results := s.stage(source), []T{}
		for !s.cancelled() && source.HasNext() {
			if item := source.Next(); judge(item) {
				results = append(results, item)
			}
		}
		return newIterator(results)
	}).WithContext(s.ctx)
}
func (s *streamer[T]) Map(m types.Mapper[T]) Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		source, results := s.stage(source), []T{}
		for !s.cancelled() && source.HasNext() {
			results = append(results, m(source.Next()))
		}
		return newIterator(results)
	}).WithContext(s.ctx)
}
func (s *streamer[T]) Convert(convert types.Converter[T, any]) Streamer[any] {
	return wrapStreamer(wrapAny(s.source), func(source iterator[any]) iterator[any] {
		source, results := wrapAny(s.stage(deWrapAny[T](source))), []any{}
		for !s.cancelled() && source.HasNext() {
			results = append(results, convert(source.Next().(T)))
		}
		return newIterator(results)
	}).WithContext(s.ctx)
}
func (s *streamer[T]) Peek(consumer types.Consumer[T]) Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		source, results := s.stage(source), []T{}
		for !s.cancelled() && source.HasNext() {
			item := source.Next()
			consumer(item)
			results = append(results, item)
		}
		return newIterator(results)
	}).WithContext(s.ctx)
}

func (s *streamer[T]) Distinct() Streamer[T] { return s.Filter(distinctJudge[T]()) }
func (s *streamer[T]) Sort(comparator types.Comparator[T]) Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		source, results := s.stage(source), []T{}
		for !s.cancelled() && source.HasNext() {
			results = append(results, source.Next())
		}
		sort.Sort(&Sortable[T]{List: results, Cmp: comparator})
		return newIterator(results)
	}).WithContext(s.ctx)
}
func (s *streamer[T]) ReverseSort(comparator types.Comparator[T]) Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		source, results := s.stage(source), []T{}
		for !s.cancelled() && source.HasNext() {
			results = append(results, source.Next())
		}
		sort.Sort(sort.Reverse(&Sortable[T]{List: results, Cmp: comparator}))
		return newIterator(results)
	}).WithContext(s.ctx)
}
func (s *streamer[T]) Reverse() Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		results := s.stage(source).Left()
		for i, length := 0, len(results)-1; i <= length/2; i++ {
			results[i], results[length-i] = results[length-i], results[i]
		}
		return newIterator(results)
	}).WithContext(s.ctx)
}
func (s *streamer[T]) Limit(l int64) Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		source, results := s.stage(source), []T{}
		for i := 0; i < int(l) && !s.cancelled() && source.HasNext(); i++ {
			results = append(results, source.Next())
		}
		return newIterator(results)
	}).WithContext(s.ctx)
}
func (s *streamer[T]) Skip(n int64) Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		source = s.stage(source)
		source.NextN(n) // skip n
		return source
		// return newIterator[T](source.Left()) Left cannot be called on supply iterator
	}).WithContext(s.ctx)
}
func (s *streamer[T]) Pick(start, end, interval int) Streamer[T] {
	return wrapStreamer(s.source, func(source iterator[T]) iterator[T] {
		source, results := s.stage(source), []T{}

		// if end is negative, set end to source size
		if end < 0 {
			end = int(source.Size())
		}

		// start out of range or start > end or interval == 0, return empty
		if start < 0 || int64(start) >= source.Size() || start > end || interval < 0 {
			return newIterator(results)
		}

		results = append(results, source.NextN(int64(start+1))) // pick first
		for !s.cancelled() && source.CurIndex()+int64(interval)-1 <= int64(end) && source.HasNextN(int64(interval)) {
			results = append(results, source.NextN(int64(interval)))
		}
		return newIterator(results)
	}).WithContext(s.ctx)
}

// ============ terminal operate 终止操作 ============

func (s *streamer[T]) Collect(to types.Collector[T]) any {
	return to(s.ToSlice()...)
}
func (s *streamer[T]) ForEach(consumer types.Consumer[T]) {
	for source := s.stage(s.source); !s.cancelled() && source.HasNext(); {
		consumer(source.Next())
	}
}
func (s *streamer[T]) ToSlice() []T {
	return s.stage(s.source).Left()
}
func (s *streamer[T]) AllMatch(judge types.Judge[T]) bool {
	for source := s.stage(s.source); !s.cancelled() && source.HasNext(); {
		if item := source.Next(); !judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T]) NonMatch(judge types.Judge[T]) bool {
	for source := s.stage(s.source); !s.cancelled() && source.HasNext(); {
		if item := source.Next(); judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T]) AnyMatch(judge types.Judge[T]) bool {
	for source := s.stage(s.source); !s.cancelled() && source.HasNext(); {
		if item := source.Next(); judge(item) {
			return true
		}
	}
	return false
}
func (s *streamer[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for source := s.stage(s.source); !s.cancelled() && source.HasNext(); {
		result = accumulator(result, source.Next())
	}
	return result
}
func (s *streamer[T]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	var result T = initValue
	for source := s.stage(s.source); !s.cancelled() && source.HasNext(); {
		result = accumulator(result, source.Next())
	}
	return result
}
func (s *streamer[T]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	var result any = initValue
	for source := s.stage(s.source); !s.cancelled() && source.HasNext(); {
		result = accumulator(result, source.Next())
	}
	return result
}
func (s *streamer[T]) ReduceBy(initValueBulider func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	source := s.stage(s.source)
	result := initValueBulider(int(source.Size()))
	for !s.cancelled() && source.HasNext() {
		result = accumulator(result, source.Next())
	}
	return result
}
func (s *streamer[T]) First() T { return s.stage(s.source).Next() }
func (s *streamer[T]) Take() T {
	source := s.stage(s.source)
	return source.NextN(rand.Int63n(source.Size()) + 1)
}
func (s *streamer[T]) Any() T { return s.Take() }
func (s *streamer[T]) Last() T {
	source := s.stage(s.source)
	return source.NextN(source.Size())
}
func (s *streamer[T]) Count() int64 { return s.stage(s.source).Size() }
