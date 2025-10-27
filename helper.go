package stream

import (
	"fmt"

	"github.com/tr1v3r/stream/types"
)

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
	return To(func(d any) T { return d.(T) })
}

func distinctJudge[T any]() types.Judge[T] {
	var keyMap = make(map[string]struct{})
	return func(t T) bool {
		var key string
		if keyer, ok := any(t).(types.Unique); ok {
			key = keyer.Key()
		} else {
			key = fmt.Sprint(t)
		}

		if _, ok := keyMap[key]; !ok {
			keyMap[key] = struct{}{}
			return true
		}
		return false
	}
}
