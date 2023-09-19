package stream

var (
	_ iterator[any] = newIterator[any]([]any{})
	_ iterator[int] = newIterator[int]([]int{})
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
	return &staticIterator[T]{
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

// staticIterator 静态迭代器
type staticIterator[T any] struct {
	meta
	source []T
}

// Next return next element
func (iter *staticIterator[T]) Next() T {
	return iter.NextN(1)
}

// NextN return next n element
func (iter *staticIterator[T]) NextN(n int64) T {
	return iter.source[iter.nextIndexN(n)]
}

// Left return all left element
func (iter *staticIterator[T]) Left() (results []T) {
	for index := iter.nextIndexN(1); index != -1; index = iter.nextIndexN(1) {
		results = append(results, iter.source[index])
	}
	return
}

// Clone return a new iterator with same data
func (iter *staticIterator[T]) Clone() iterator[T] {
	return &staticIterator[T]{
		meta:   iter.meta,
		source: iter.source,
	}
}
