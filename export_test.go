package stream_test

import (
	"fmt"
	"testing"

	"github.com/tr1v3r/stream"
)

// var mapper Mapper[int, any] = func(int) any { return nil }

func TestStream(t *testing.T) {
	array := []int{1, 2, 3, 4}
	result := stream.SliceOf[int](array).
		Map(func(i int) any { return float64(i + 1) }).
		Reduce(func(result, data any) any {
			if result == nil {
				return data.(float64)
			}
			return result.(float64) + data.(float64)
		}).(int)
	fmt.Println("result: ", result)
}

func TestStream_1(t *testing.T) {
	array := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	stage := stream.SliceOf[int](array)
	fmt.Println("stream First: ", stage.First())
	fmt.Println("stream Take: ", stage.Take())
	fmt.Println("stream Last: ", stage.Last())
	fmt.Println("stream ToSlice: ", stage.ToSlice())
	fmt.Println("stream Reverse: ", stage.Reverse().ToSlice())
	fmt.Println("stream ToSlice: ", stage.ToSlice())
	fmt.Println("stream Limit: ", stage.Limit(8).ToSlice())
	fmt.Println("stream ToSlice: ", stage.ToSlice())
	fmt.Println("stream Skip: ", stage.Skip(1).ToSlice())
	fmt.Println("stream ToSlice: ", stage.ToSlice())
	fmt.Println("stream Pick: ", stage.Pick(1, 7, 2).ToSlice())
	fmt.Println("stream ToSlice: ", stage.ToSlice())
	result := stage.Reduce(func(result, data int) int {
		return result + data
	})
	fmt.Println("stream Reduce sum: ", result)

	// var x []any = []any{"hello", "world"}
	// stream.SliceOf[string](x.([]string))
}
