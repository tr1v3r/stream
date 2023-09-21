package stream

import (
	"reflect"

	"github.com/tr1v3r/stream/types"
)

var (
	_ iterator[any] = new(staticIter[any])
	_ iterator[int] = new(staticIter[int])
	_ iterator[int] = new(supplyIter[int])
	_ iterator[any] = new(anyIterator[int])
)

// iterator 迭代器
type iterator[T any] interface {
	// Size return data size
	Size() int64
	// HasNext return true if iterator has next 1
	HasNext() bool
	// HasNextN return true if iterator has next n, HasNextN(1) == HasNext()
	HasNextN(n int64) bool
	// Next return next element
	Next() T
	// NextN return next n element, NextN(1) == Next()
	NextN(n int64) T
	// Left return all left element
	Left() []T

	// CurIndex return current index
	CurIndex() int64
	// Clone return a new iterator with same data
	Clone() iterator[T]
	// Concat concat iterators
	Concat(iters ...iterator[T]) iterator[T]
}

// newIterator return iterator
func newIterator[T any](data []T) iterator[T] {
	return &staticIter[T]{
		meta: meta{
			current: 0,
			size:    int64(len(data)),
		},
		source: data,
	}
}

// meta iterator元数据
type meta struct {
	current int64 // range 0 to size-1
	size    int64 // data size
}

// Size return meta size
func (m *meta) Size() int64 {
	return int64(m.size)
}

// HasNext return true if meta has next 1
func (m *meta) HasNext() bool {
	return m.HasNextN(1)
}

// HasNextN return true if meta has next n
func (m *meta) HasNextN(n int64) bool {
	return m.current+n-1 < m.size
}

// Cur return current index
func (m *meta) CurIndex() int64 { return m.current }

// nextIndexN return next index, return -1 if next index out of range
// n must be positive
func (m *meta) nextIndexN(n int64) (index int64) {
	if n <= 0 || m.current+n-1 >= m.size {
		return -1
	}
	m.current += n
	return m.current - 1
}

// staticIter 静态迭代器
type staticIter[T any] struct {
	meta
	source []T
}

func (i *staticIter[T]) Next() T         { return i.NextN(1) }
func (i *staticIter[T]) NextN(n int64) T { return i.source[i.nextIndexN(n)] }
func (i *staticIter[T]) Left() (results []T) {
	for index := i.nextIndexN(1); index != -1; index = i.nextIndexN(1) {
		results = append(results, i.source[index])
	}
	return
}
func (i staticIter[T]) Clone() iterator[T] { return &i }
func (i staticIter[T]) Concat(iters ...iterator[T]) iterator[T] {
	for _, iter := range iters {
		i.source = append(i.source, iter.Left()...)
		i.size += i.Size()
	}
	return &i
}

type supplyIter[T any] struct {
	curIndex int64
	supply   types.Supplier[T]
}

func (s *supplyIter[T]) Size() int64         { return -1 }
func (s *supplyIter[T]) Left() []T           { return nil }
func (s *supplyIter[T]) CurIndex() int64     { return s.curIndex }
func (s *supplyIter[T]) HasNext() bool       { return true }
func (s *supplyIter[T]) HasNextN(int64) bool { return true }
func (s *supplyIter[T]) Next() T             { return s.NextN(1) }
func (s *supplyIter[T]) NextN(n int64) T {
	s.curIndex += n
	for i := 0; i < int(n)-1; i++ {
		s.supply()
	}
	return s.supply()
}
func (s supplyIter[T]) Clone() iterator[T] { return &s }
func (s supplyIter[T]) Concat(iters ...iterator[T]) iterator[T] {
	for _, iter := range iters {
		supply := s.supply
		s.supply = func() T {
			if item := supply(); !s.isNil(item) { // return item if not nil
				return item
			}
			return iter.Next()
		}
	}
	return &s
}
func (s supplyIter[T]) isNil(t T) bool {
	v := reflect.ValueOf(t)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map,
		reflect.Pointer, reflect.UnsafePointer,
		reflect.Interface, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func wrapAny[T any](iter iterator[T]) iterator[any]   { return &anyIterator[T]{iter} }
func deWrapAny[T any](iter iterator[any]) iterator[T] { return iter.(*anyIterator[T]).iterator }

type anyIterator[T any] struct{ iterator[T] }

func (a *anyIterator[T]) Left() []any {
	return To[T, any](func(t T) any { return t })(a.iterator.Left()...).([]any)
}
func (a *anyIterator[T]) Next() any           { return a.iterator.NextN(1) }
func (a *anyIterator[T]) NextN(n int64) any   { return a.iterator.NextN(n) }
func (a anyIterator[T]) Clone() iterator[any] { return &anyIterator[T]{a.iterator.Clone()} }
func (a anyIterator[T]) Concat(iters ...iterator[any]) iterator[any] {
	var wrappedIters []iterator[T]
	for _, iter := range iters {
		wrappedIters = append(wrappedIters, deWrapAny[T](iter))
	}
	return wrapAny(a.iterator.Concat(wrappedIters...))
}

// Sortable implement sort.Interface
type Sortable[T any] struct {
	List []T
	Cmp  types.Comparator[T]
}

// implement sort.Interface
func (a *Sortable[T]) Len() int           { return len(a.List) }
func (a *Sortable[T]) Less(i, j int) bool { return a.Cmp(a.List[i], a.List[j]) < 0 }
func (a *Sortable[T]) Swap(i, j int)      { a.List[i], a.List[j] = a.List[j], a.List[i] }
