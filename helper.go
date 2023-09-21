package stream

import "github.com/tr1v3r/stream/types"

// To converts a slice of T to a slice of R
func To[T, R any](converter types.Converter[T, R]) types.Collector[T] {
	return func(data ...T) any {
		var results []R
		for _, item := range data {
			results = append(results, converter(item))
		}
		return results
	}
}

// AnyTo converts a slice of any to a slice of T
func AnyTo[T any](data ...any) types.Collector[any] {
	return To[any, T](func(d any) T { return d.(T) })
}
