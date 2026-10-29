package stream_test

import (
	"testing"

	"github.com/tr1v3r/stream"
)

var (
	genericBenchInt    int
	genericBenchInt64  int64
	genericBenchValues []int64
)

// BenchmarkGenericAPI compares equivalent results under the same compiler.
// Package-level generics are controls: method syntax alone is not an expected
// speedup. Erased cases include the assertions needed to recover typed output.
func BenchmarkGenericAPI(b *testing.B) {
	values := make([]int, 4096)
	for i := range values {
		values[i] = i + 1024
	}
	keep := func(n int) bool { return n%2 == 0 }
	twice := func(n int) int { return n * 2 }
	add := func(a, b int) int { return a + b }
	convert := func(n int) int64 { return int64(n) }
	key := func(n int) int { return n % 128 }
	typedCollect := func(v ...int) []int64 {
		result := make([]int64, len(v))
		for i, n := range v {
			result[i] = int64(n)
		}
		return result
	}
	tests := []struct {
		name string
		run  func()
	}{
		{"SameType/Legacy", func() { genericBenchInt = stream.SliceOf(values...).Filter(keep).Map(twice).Reduce(add) }},
		{"SameType/Pipeline", func() { genericBenchInt = stream.Of(values...).Filter(keep).Map(twice).Reduce(add) }},
		{"Parallel/Legacy", func() {
			genericBenchInt = stream.SliceOf(values...).Parallel(4).Ordered().Filter(keep).Map(twice).Reduce(add)
		}},
		{"Parallel/Pipeline", func() {
			genericBenchInt = stream.Of(values...).Parallel(4).Ordered().Filter(keep).Map(twice).Reduce(add)
		}},
		{"MapTo/Package", func() { genericBenchValues = stream.MapTo(stream.SliceOf(values...), convert).ToSlice() }},
		{"MapTo/Method", func() { genericBenchValues = stream.Of(values...).MapTo(convert).ToSlice() }},
		{"MapTo/Erased", func() {
			genericBenchValues = stream.SliceOf(values...).Convert(func(n int) any { return int64(n) }).Collect(stream.AnyTo[int64]()).([]int64)
		}},
		{"FlatMap/Typed", func() {
			genericBenchValues = stream.Of(values...).FlatMapTo(func(n int) stream.Pipeline[int64] { return stream.Of(int64(n), int64(n+1)) }).ToSlice()
		}},
		{"FlatMap/Erased", func() {
			genericBenchValues = stream.SliceOf(values...).FlatMap(func(n int) stream.Streamer[any] { return stream.SliceOf[any](int64(n), int64(n+1)) }).Collect(stream.AnyTo[int64]()).([]int64)
		}},
		{"Fold/Typed", func() {
			genericBenchInt64 = stream.Of(values...).Fold(int64(0), func(sum int64, n int) int64 { return sum + int64(n) })
		}},
		{"Fold/Erased", func() {
			genericBenchInt64 = stream.SliceOf(values...).ReduceWith(int64(0), func(sum any, n int) any { return sum.(int64) + int64(n) }).(int64)
		}},
		{"Collect/Typed", func() { genericBenchValues = stream.Of(values...).CollectTo(typedCollect) }},
		{"Collect/Erased", func() {
			genericBenchValues = stream.SliceOf(values...).Collect(func(v ...int) any { return typedCollect(v...) }).([]int64)
		}},
		{"DistinctBy/Package", func() { genericBenchInt = int(stream.DistinctBy(stream.SliceOf(values...), key).Count()) }},
		{"DistinctBy/Method", func() { genericBenchInt = int(stream.Of(values...).DistinctBy(key).Count()) }},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				tt.run()
			}
		})
	}
}

func TestPipelineFoldAvoidsAccumulatorBoxing(t *testing.T) {
	values := make([]int, 1024)
	for i := range values {
		values[i] = i + 1024
	}
	typed := func(input []int) {
		genericBenchInt64 = stream.Of(input...).Fold(int64(0), func(sum int64, n int) int64 { return sum + int64(n) })
	}
	typed(values)
	want := genericBenchInt64
	got := stream.SliceOf(values...).ReduceWith(int64(0), func(sum any, n int) any { return sum.(int64) + int64(n) }).(int64)
	if got != want {
		t.Fatal("typed and erased sums differ")
	}
	small := testing.AllocsPerRun(3, func() { typed(values[:16]) })
	large := testing.AllocsPerRun(3, func() { typed(values) })
	if large > small+1 {
		t.Fatalf("Fold allocations grow with input size: 16 elements %.0f, 1024 elements %.0f", small, large)
	}
}
