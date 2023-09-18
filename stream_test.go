package stream_test

import (
	"fmt"
	"testing"

	"github.com/tr1v3r/stream"
)

// var mapper Mapper[int, any] = func(int) any { return nil }

func TestStream(t *testing.T) {
	array := []int{1, 2, 3, 4}
	result := stream.Slice[int, float64](array).
		Map(func(i int) float64 { return float64(i + 1) }).
		Map(func(f float64) any { return int(f) }).
		Reduce(func(result, data any) any {
			if result == nil {
				return data.(int)
			}
			return result.(int) + data.(int)
		}).(int)
	fmt.Println("result: ", result)
}
