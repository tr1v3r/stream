package stream

import "github.com/tr1v3r/stream/types"

func AnyTo[T any](data ...any) (results []T) {
	return To[any, T](func(d any) T { return d.(T) }, data...)
}

func To[T, R any](mapper types.Mapper[T, R], data ...T) (results []R) {
	for _, item := range data {
		results = append(results, mapper(item))
	}
	return
}
