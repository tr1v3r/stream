package stream

import "github.com/tr1v3r/stream/types"

var (
	_ iterator[any] = new(staticIter[any])
	_ iterator[int] = new(staticIter[int])
	_ iterator[int] = new(supplyIter[int])
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

// Next return next element
func (iter *staticIter[T]) Next() T {
	return iter.NextN(1)
}

// NextN return next n element
func (iter *staticIter[T]) NextN(n int64) T {
	return iter.source[iter.nextIndexN(n)]
}

// Left return all left element
func (iter *staticIter[T]) Left() (results []T) {
	for index := iter.nextIndexN(1); index != -1; index = iter.nextIndexN(1) {
		results = append(results, iter.source[index])
	}
	return
}

// Clone return a new iterator with same data
func (iter staticIter[T]) Clone() iterator[T] { return &iter }

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

// Sortable implement sort.Interface
type Sortable[T any] struct {
	List []T
	Cmp  types.Comparator[T]
}

// implement sort.Interface
func (a *Sortable[T]) Len() int           { return len(a.List) }
func (a *Sortable[T]) Less(i, j int) bool { return a.Cmp(a.List[i], a.List[j]) < 0 }
func (a *Sortable[T]) Swap(i, j int)      { a.List[i], a.List[j] = a.List[j], a.List[i] }
