package stream_test

import (
	"math"
	"slices"
	"testing"

	"github.com/tr1v3r/stream"
)

func TestStream(t *testing.T) {
	array := []int{4, 1, 3, 3, 2}

	sorted := stream.SliceOf(array...).Distinct().
		Sort(func(l, r int) int { return l - r }).ToSlice()
	if !slices.Equal(sorted, []int{1, 2, 3, 4}) {
		t.Fatalf("expected [1 2 3 4], got %v", sorted)
	}

	revSorted := stream.SliceOf(array...).Distinct().
		ReverseSort(func(l, r int) int { return l - r }).ToSlice()
	if !slices.Equal(revSorted, []int{4, 3, 2, 1}) {
		t.Fatalf("expected [4 3 2 1], got %v", revSorted)
	}

	result := stream.SliceOf(array...).
		Convert(func(i int) any { return float64(i + 1) }).
		Reduce(func(acc, data any) any {
			if acc == nil {
				return data.(float64)
			}
			return acc.(float64) + data.(float64)
		}).(float64)
	if result != 18 {
		t.Fatalf("expected 18, got %v", result)
	}

	floatResult := stream.SliceOf(array...).
		Convert(func(i int) any { return float64(i + 1) }).Collect(func(data ...any) any {
		floats := make([]float64, 0, len(data))
		for _, item := range data {
			floats = append(floats, item.(float64))
		}
		return stream.SliceOf(floats...)
	}).(stream.Streamer[float64]).ReduceFrom(99.99, func(result, data float64) float64 {
		return result + data
	})
	if math.Abs(floatResult-117.99) > 1e-9 {
		t.Fatalf("expected 117.99, got %v", floatResult)
	}
}

func TestStream_1(t *testing.T) {
	array := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	if got := stream.SliceOf(array...).First(); got != 1 {
		t.Fatalf("expected First 1, got %d", got)
	}
	if got := stream.SliceOf(array...).Last(); got != 10 {
		t.Fatalf("expected Last 10, got %d", got)
	}
	if got := stream.SliceOf(array...).Take(); !slices.Contains(array, got) {
		t.Fatalf("Take must return a member, got %d", got)
	}
	if got := stream.SliceOf(array...).Any(); !slices.Contains(array, got) {
		t.Fatalf("Any must return a member, got %d", got)
	}
	if got := stream.SliceOf(array...).ToSlice(); !slices.Equal(got, array) {
		t.Fatalf("expected %v, got %v", array, got)
	}
	if got := stream.SliceOf(array...).Reverse().ToSlice(); !slices.Equal(got, []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}) {
		t.Fatalf("unexpected reverse: %v", got)
	}
	if got := stream.SliceOf(array...).Limit(8).ToSlice(); !slices.Equal(got, array[:8]) {
		t.Fatalf("unexpected limit: %v", got)
	}
	if got := stream.SliceOf(array...).Skip(1).ToSlice(); !slices.Equal(got, array[1:]) {
		t.Fatalf("unexpected skip: %v", got)
	}
	if got := stream.SliceOf(array...).Pick(0, 8, 2).ToSlice(); !slices.Equal(got, []int{1, 3, 5, 7, 9}) {
		t.Fatalf("unexpected pick: %v", got)
	}
	if got := stream.SliceOf(array...).Pick(1, 9, 2).ToSlice(); !slices.Equal(got, []int{2, 4, 6, 8, 10}) {
		t.Fatalf("unexpected pick: %v", got)
	}
	if got := stream.SliceOf(array...).Pick(1, 99, 2).ToSlice(); !slices.Equal(got, []int{2, 4, 6, 8, 10}) {
		t.Fatalf("end beyond size must clamp: %v", got)
	}
	if got := stream.SliceOf(array...).Pick(1, -1, 2).ToSlice(); !slices.Equal(got, []int{2, 4, 6, 8, 10}) {
		t.Fatalf("negative end must mean last index: %v", got)
	}
	if got := stream.SliceOf(array...).Reduce(func(a, b int) int { return a + b }); got != 55 {
		t.Fatalf("expected sum 55, got %d", got)
	}
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

func TestStream_From(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				return
			}
		}
	}
	result := stream.From(seq, 3).Map(func(n int) int { return n * 10 }).ToSlice()
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

func TestStream_TakeEmpty(t *testing.T) {
	if v := stream.SliceOf[int]().Take(); v != 0 {
		t.Fatalf("expected zero value from Take on empty stream, got %v", v)
	}
	if v := stream.SliceOf[int]().Any(); v != 0 {
		t.Fatalf("expected zero value from Any on empty stream, got %v", v)
	}
}

func TestStream_PickNegativeStart(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				return
			}
		}
	}
	// Unknown sizeHint with negative start must not panic
	got := stream.From(seq, -1).Pick(-1, -1, 1).ToSlice()
	if len(got) != 0 {
		t.Fatalf("expected empty result for negative start, got %v", got)
	}
}

func TestStream_ParallelDistinct(t *testing.T) {
	data := make([]int, 0, 20000)
	for i := 0; i < 20000; i++ {
		data = append(data, i%1000)
	}
	got := stream.SliceOf(data...).Parallel(4).Distinct().ToSlice()
	if len(got) != 1000 {
		t.Fatalf("expected 1000 unique elements, got %d", len(got))
	}
}
