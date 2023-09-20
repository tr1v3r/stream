package stream

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = newStreamer[any](nil)
	_ Streamer[float64] = newStreamer[float64](nil)
)

type stage[T any] func() iterator[T]

// newStreamer return streamer
func newStreamer[T any](iter iterator[T]) *streamer[T] {
	return &streamer[T]{stage: func() iterator[T] { return iter.Clone() }}
}

// wrapStreamer wrap stage to new streamer
func wrapStreamer[T any](stage stage[T]) *streamer[T] {
	return &streamer[T]{stage: stage}
}

// streamer underlying streamer implement for Streamer
type streamer[T any] struct {
	stage stage[T]
}

// Execute eager execute on source
func (s *streamer[T]) Execute() Streamer[T] { return newStreamer[T](s.stage()) }

func (s *streamer[T]) Filter(judge types.Judge[T]) Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source, results := s.stage(), []T{}
		for source.HasNext() {
			if item := source.Next(); judge(item) {
				results = append(results, item)
			}
		}
		return newIterator[T](results)
	})
}
func (s *streamer[T]) Map(mapper types.Mapper[T, any]) Streamer[any] {
	return wrapStreamer[any](func() iterator[any] {
		source, results := s.stage(), []any{}
		for source.HasNext() {
			results = append(results, mapper(source.Next()))
		}
		return newIterator[any](results)
	})
}
func (s *streamer[T]) Peek(consumer types.Consumer[T]) Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source, results := s.stage(), []T{}
		for source.HasNext() {
			consumer(source.Next())
		}
		return newIterator[T](results)
	})
}

func (s *streamer[T]) Distinct() Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source, results, keyMap := s.stage(), []T{}, map[string]T{}
		for source.HasNext() {
			item := source.Next()

			var key string
			if keyer, ok := any(item).(types.Unique); ok {
				key = keyer.Key()
			} else {
				key = fmt.Sprint(item)
			}

			if _, ok := keyMap[key]; !ok {
				keyMap[key] = item
				results = append(results, item)
			}
		}
		return newIterator[T](results)
	})
}
func (s *streamer[T]) Sort(comparator types.Comparator[T]) Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source, results := s.stage(), []T{}
		for source.HasNext() {
			results = append(results, source.Next())
		}
		sort.Sort(&Sortable[T]{List: results, Cmp: comparator})
		return newIterator[T](results)
	})
}
func (s *streamer[T]) ReverseSort(comparator types.Comparator[T]) Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source, results := s.stage(), []T{}
		for source.HasNext() {
			results = append(results, source.Next())
		}
		sort.Sort(sort.Reverse(&Sortable[T]{List: results, Cmp: comparator}))
		return newIterator[T](results)
	})
}
func (s *streamer[T]) Reverse() Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source := s.stage()
		results := source.Left()
		for i, length := 0, len(results)-1; i <= length/2; i++ {
			results[i], results[length-i] = results[length-i], results[i]
		}
		return newIterator[T](results)
	})
}
func (s *streamer[T]) Limit(l int64) Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source, results := s.stage(), []T{}
		for i := 0; i < int(l) && source.HasNext(); i++ {
			results = append(results, source.Next())
		}
		return newIterator[T](results)
	})
}
func (s *streamer[T]) Skip(n int64) Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source := s.stage()
		source.NextN(n) // skip n
		return newIterator[T](source.Left())
	})
}
func (s *streamer[T]) Pick(start, end, interval int) Streamer[T] {
	return wrapStreamer[T](func() iterator[T] {
		source, results := s.stage(), []T{}

		// if end is negative, set end to source size
		if end < 0 {
			end = int(source.Size())
		}

		// start out of range or start > end or interval == 0, return empty
		if start < 0 || int64(start) >= source.Size() || start > end || interval < 0 {
			return newIterator[T](results)
		}

		results = append(results, source.NextN(int64(start+1))) // pick first
		for source.CurIndex()+int64(interval)-1 <= int64(end) && source.HasNextN(int64(interval)) {
			results = append(results, source.NextN(int64(interval)))
		}
		return newIterator[T](results)
	})
}

// ============ terminal operate 终止操作 ============

func (s *streamer[T]) Collect(to types.Collector[T]) any {
	return to(s.stage().Left()...)
}
func (s *streamer[T]) ForEach(consumer types.Consumer[T]) {
	for source := s.stage(); source.HasNext(); {
		consumer(source.Next())
	}
}
func (s *streamer[T]) To(to func([]T) any) any {
	return to(s.ToSlice())
}
func (s *streamer[T]) ToSlice() []T {
	return s.stage().Left()
}
func (s *streamer[T]) AllMatch(judge types.Judge[T]) bool {
	for source := s.stage(); source.HasNext(); {
		if item := source.Next(); !judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T]) NonMatch(judge types.Judge[T]) bool {
	for source := s.stage(); source.HasNext(); {
		if item := source.Next(); judge(item) {
			return false
		}
	}
	return true
}
func (s *streamer[T]) AnyMatch(judge types.Judge[T]) bool {
	for source := s.stage(); source.HasNext(); {
		if item := source.Next(); judge(item) {
			return true
		}
	}
	return false
}
func (s *streamer[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for source := s.stage(); source.HasNext(); {
		result = accumulator(result, source.Next())
	}
	return result
}
func (s *streamer[T]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	var result T = initValue
	for source := s.stage(); source.HasNext(); {
		result = accumulator(result, source.Next())
	}
	return result
}
func (s *streamer[T]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	var result = initValue
	for source := s.stage(); source.HasNext(); {
		result = accumulator(result, source.Next())
	}
	return result
}
func (s *streamer[T]) ReduceBy(initValueBulider func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	source := s.stage()
	result := initValueBulider(int(source.Size()))
	for source.HasNext() {
		result = accumulator(result, source.Next())
	}
	return result
}
func (s *streamer[T]) First() T { return s.stage().Next() }
func (s *streamer[T]) Take() T {
	source := s.stage()
	return source.NextN(rand.Int63n(source.Size()) + 1)
}
func (s *streamer[T]) Last() T {
	source := s.stage()
	return source.NextN(source.Size())
}
func (s *streamer[T]) Count() int64 { return s.stage().Size() }
