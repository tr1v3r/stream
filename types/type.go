package types

type (
	// Judge judge data, return true when match rule
	Judge[T any] func(T) bool

	// Mapper convert source from T to R
	Mapper[T, R any] func(T) R

	// Comparator comparator for sort []T
	Comparator[T any] func(T, T) int

	// Consumer consume T
	Consumer[T any] func(...T)

	// BinaryOperator operate two data T
	BinaryOperator[T any] func(T, T) T

	// Accumulator accumulate operator
	Accumulator[T, R any] func(R, T) R

	// Collector collect source to any data struct
	Collector[T any] func(...T) any
)
