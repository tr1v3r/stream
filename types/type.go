package types

type (
	// Judge judge data, return true when match rule
	Judge[T any] func(T) bool

	// Mapper convert source from T to R
	Mapper[T, R any] func(T) R

	// Comparator comparator for sort []T
	// it is a BiFunction, which two input arguments are the type, and returns a int.
	// if left is greater then right, it returns a positive number;
	// if left is less then right, it returns a negative number; if the two input are equal, it returns
	Comparator[T any] func(left T, right T) int

	// Consumer consume T
	Consumer[T any] func(...T)

	// BinaryOperator operate two data T
	BinaryOperator[T any] func(T, T) T

	// Accumulator accumulate operator
	Accumulator[T, R any] func(R, T) R

	// Collector collect source to any data struct
	Collector[T any] func(...T) any

	// provide source data
	Supplier[T any] func() T

	// Unique unique item interface
	Unique interface{ Key() string }
)
