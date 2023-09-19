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
	// AllLeft return all left element
	AllLeft() []T
	// Reset
	Reset()
}

// newIterator return iterator
func newIterator[T any](data []T) iterator[T] {
	return &staticIterator[T]{
		meta: meta{
			current: 0,
			size:    int64(len(data)),
		},
		data: data,
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

// Reset reset meta
func (m *meta) Reset() { m.current = 0 }

// nextIndexN return next index, return -1 if next index out of range
// n must be positive
func (m *meta) nextIndexN(n int64) (index int64) {
	if n <= 0 || m.current+n >= m.size {
		return -1
	}
	m.current += n
	return m.current - 1
}

// staticIterator 静态迭代器
type staticIterator[T any] struct {
	meta
	data []T
}

// Next return next element
func (i *staticIterator[T]) Next() T {
	return i.NextN(1)
}

// NextN return next n element
func (i *staticIterator[T]) NextN(n int64) T {
	return i.data[i.nextIndexN(n)]
}

// AllLeft return all left element
func (i *staticIterator[T]) AllLeft() (results []T) {
	for index := i.nextIndexN(1); index != -1; index = i.nextIndexN(1) {
		results = append(results, i.data[index])
	}
	return
}
