package stream_test

import (
	"fmt"
	"testing"

	"github.com/tr1v3r/stream"
)

func TestStream(t *testing.T) {
	array := []int{4, 1, 3, 3, 2}

	streamer := stream.SliceOf(array...).Distinct()

	fmt.Println("distinct: ", streamer.ToSlice())
	fmt.Println("sort: ", streamer.Sort(func(l, r int) int { return l - r }).ToSlice())
	fmt.Println("reverse sort: ", streamer.ReverseSort(func(l, r int) int { return l - r }).ToSlice())

	result := stream.SliceOf(array...).
		Convert(func(i int) any { return float64(i + 1) }).
		Reduce(func(result, data any) any {
			if result == nil {
				return data.(float64)
			}
			return result.(float64) + data.(float64)
		}).(float64)
	fmt.Println("result: ", result)

	stream.SliceOf(array...).
		Convert(func(i int) any { return float64(i + 1) }).
		ForEach(func(data any) { fmt.Println(data) })

	floatResult := stream.SliceOf(array...).
		Convert(func(i int) any { return float64(i + 1) }).Collect(func(data ...any) any {
		var floats []float64
		for _, item := range data {
			floats = append(floats, item.(float64))
		}
		return stream.SliceOf(floats...)
	}).(stream.Streamer[float64]).ReduceFrom(99.99, func(result, data float64) float64 {
		return result + data
	})
	fmt.Println("collect new streamer result: ", floatResult)
}

func TestStream_1(t *testing.T) {
	array := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	fmt.Println("stream First: ", stream.SliceOf(array...).First())
	fmt.Println("stream Take: ", stream.SliceOf(array...).Take())
	fmt.Println("stream Last: ", stream.SliceOf(array...).Last())
	fmt.Println("stream ToSlice: ", stream.SliceOf(array...).ToSlice())
	fmt.Println("stream Reverse: ", stream.SliceOf(array...).Reverse().ToSlice())
	fmt.Println("stream Limit: ", stream.SliceOf(array...).Limit(8).ToSlice())
	fmt.Println("stream Skip: ", stream.SliceOf(array...).Skip(1).ToSlice())
	fmt.Println("stream Pick: ", stream.SliceOf(array...).Pick(0, 8, 2).ToSlice())
	fmt.Println("stream Pick: ", stream.SliceOf(array...).Pick(1, 9, 2).ToSlice())
	fmt.Println("stream Pick: ", stream.SliceOf(array...).Pick(1, 99, 2).ToSlice())
	fmt.Println("stream Pick: ", stream.SliceOf(array...).Pick(1, -1, 2).ToSlice())
	result := stream.SliceOf(array...).Reduce(func(result, data int) int {
		return result + data
	})
	fmt.Println("stream Reduce sum: ", result)
}

func TestStream_LazyEval(t *testing.T) {
	// Verify true lazy evaluation: Map should only be called for elements actually consumed
	count := 0
	result := stream.SliceOf(1, 2, 3, 4, 5).
		Map(func(n int) int { count++; return n * 2 }).
		Limit(2).
		ToSlice()
	if len(result) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(result))
	}
	if result[0] != 2 || result[1] != 4 {
		t.Fatalf("expected [2, 4], got %v", result)
	}
	if count > 3 {
		t.Fatalf("expected at most 3 Map calls (lazy), got %d", count)
	}
}

func TestStream_Seq(t *testing.T) {
	sum := 0
	for v := range stream.SliceOf(1, 2, 3).Seq() {
		sum += v
	}
	if sum != 6 {
		t.Fatalf("expected 6, got %d", sum)
	}
}

func TestStream_Of(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				return
			}
		}
	}
	result := stream.Of(seq, 3).Map(func(n int) int { return n * 10 }).ToSlice()
	if len(result) != 3 || result[0] != 10 || result[1] != 20 || result[2] != 30 {
		t.Fatalf("expected [10, 20, 30], got %v", result)
	}
}

func TestStream_FlatMap(t *testing.T) {
	result := stream.SliceOf(1, 2, 3).
		FlatMap(func(n int) stream.Streamer[any] {
			return stream.SliceOf[any](n, n*10)
		}).ToSlice()
	if len(result) != 6 {
		t.Fatalf("expected 6 elements, got %d: %v", len(result), result)
	}
	if result[0] != 1 || result[1] != 10 || result[2] != 2 || result[3] != 20 || result[4] != 3 || result[5] != 30 {
		t.Fatalf("expected [1, 10, 2, 20, 3, 30], got %v", result)
	}
}

func TestStream_ShortCircuit(t *testing.T) {
	// AllMatch should short-circuit on first non-match
	count := 0
	ok := stream.SliceOf(1, 2, 3, 4, 5).AllMatch(func(n int) bool {
		count++
		return n < 3
	})
	if ok {
		t.Fatal("expected false")
	}
	if count != 3 {
		t.Fatalf("expected 3 checks (short-circuit at 3), got %d", count)
	}
}

