package stream_test

import (
	"fmt"
	"testing"

	"github.com/tr1v3r/stream"
)

func TestStream(t *testing.T) {
	array := []int{4, 1, 3, 3, 2}

	streamer := stream.SliceOf[int](array...).Distinct()

	fmt.Println("distinct: ", streamer.ToSlice())
	fmt.Println("sort: ", streamer.Sort(func(l, r int) int { return l - r }).ToSlice())
	fmt.Println("reverse sort: ", streamer.ReverseSort(func(l, r int) int { return l - r }).ToSlice())

	result := stream.SliceOf[int](array...).
		Map(func(i int) any { return float64(i + 1) }).
		Reduce(func(result, data any) any {
			if result == nil {
				return data.(float64)
			}
			return result.(float64) + data.(float64)
		}).(float64)
	fmt.Println("result: ", result)

	stream.SliceOf[int](array...).
		Map(func(i int) any { return float64(i + 1) }).
		ForEach(func(data ...any) { fmt.Println(data...) })

	floatResult := stream.SliceOf[int](array...).
		Map(func(i int) any { return float64(i + 1) }).Collect(func(data ...any) any {
		var floats []float64
		for _, item := range data {
			floats = append(floats, item.(float64))
		}
		return stream.SliceOf[float64](floats...)
	}).(stream.Streamer[float64]).ReduceFrom(99.99, func(result, data float64) float64 {
		return result + data
	})
	fmt.Println("collect new streamer result: ", floatResult)
}

func TestStream_1(t *testing.T) {
	array := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	stage := stream.SliceOf[int](array...)
	fmt.Println("stream First: ", stage.First())
	fmt.Println("stream Take: ", stage.Take())
	fmt.Println("stream Last: ", stage.Last())
	fmt.Println("stream ToSlice: ", stage.ToSlice())
	fmt.Println("stream Reverse: ", stage.Reverse().ToSlice())
	fmt.Println("stream Limit: ", stage.Limit(8).ToSlice())
	fmt.Println("stream Skip: ", stage.Skip(1).ToSlice())
	fmt.Println("stream Pick: ", stage.Pick(0, 8, 2).ToSlice())
	fmt.Println("stream Pick: ", stage.Pick(1, 9, 2).ToSlice())
	fmt.Println("stream Pick: ", stage.Pick(1, 99, 2).ToSlice())
	fmt.Println("stream Pick: ", stage.Pick(1, -1, 2).ToSlice())
	result := stage.Reduce(func(result, data int) int {
		return result + data
	})
	fmt.Println("stream Reduce sum: ", result)

	// var x []any = []any{"hello", "world"}
	// stream.SliceOf[string](x.([]string))
}
