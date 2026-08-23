package stream_test

import (
	"testing"

	"github.com/tr1v3r/stream"
)

var (
	benchSortData    = makeUnsorted(10000)
	benchDistinctSrc = makeCyclic(5000, 1000)
)

func makeUnsorted(n int) []int {
	data := make([]int, n)
	for i := range data {
		data[i] = (i*7919 + 13) % n // deterministic pseudo-shuffle
	}
	return data
}

func makeCyclic(n, mod int) []int {
	data := make([]int, n)
	for i := range data {
		data[i] = i % mod
	}
	return data
}

func BenchmarkSort(b *testing.B) {
	cmp := func(l, r int) int { return l - r }
	b.ResetTimer()
	for b.Loop() {
		stream.SliceOf(benchSortData...).Sort(cmp).ToSlice()
	}
}

func BenchmarkReverseSort(b *testing.B) {
	cmp := func(l, r int) int { return l - r }
	b.ResetTimer()
	for b.Loop() {
		stream.SliceOf(benchSortData...).ReverseSort(cmp).ToSlice()
	}
}

func BenchmarkDistinct(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		stream.SliceOf(benchDistinctSrc...).Distinct().ToSlice()
	}
}

func BenchmarkDistinctBy(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		stream.DistinctBy(stream.SliceOf(benchDistinctSrc...), func(n int) int { return n }).ToSlice()
	}
}

func BenchmarkTake(b *testing.B) {
	data := makeUnsorted(100000)
	b.ResetTimer()
	for b.Loop() {
		stream.SliceOf(data...).Take() // reservoir sampling, O(1) memory
	}
}

func BenchmarkPipeline(b *testing.B) {
	src := makeUnsorted(20000)
	b.ResetTimer()
	for b.Loop() {
		stream.SliceOf(src...).
			Filter(func(n int) bool { return n%2 == 0 }).
			Map(func(n int) int { return n * 3 }).
			Reduce(func(a, b int) int { return a + b })
	}
}

// Parallel v2 section benchmarks (proposal gates A1/A2). The near-free
// workload pins machinery overhead; compare Parallel(4) against serial.
func BenchmarkParallelSection(b *testing.B) {
	src := makeUnsorted(100000)
	f := func(n int) bool { return n%2 == 0 }
	m := func(n int) int { return n + 1 }

	cases := []struct {
		name    string
		workers int
		ordered bool
	}{
		{"serial", 0, false},
		{"fused-4", 4, false},
		{"fused-4-ordered", 4, true},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				s := stream.SliceOf(src...)
				if tc.workers > 0 {
					s = s.Parallel(tc.workers)
				}
				if tc.ordered {
					s = s.Ordered()
				}
				sink := 0
				for _, v := range s.Filter(f).Map(m).ToSlice() {
					sink += v
				}
				_ = sink
			}
		})
	}
}
