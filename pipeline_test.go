package stream_test

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tr1v3r/stream"
	"github.com/tr1v3r/stream/types"
)

// The old factories, package functions and callback types keep their exact
// signatures. Returning Pipeline from an old factory would break these.
var (
	_ func(...int) stream.Streamer[int]                                                = stream.SliceOf[int]
	_ func(iter.Seq[int], int64) stream.Streamer[int]                                  = stream.From[int]
	_ func(stream.Streamer[int], types.Converter[int, string]) stream.Streamer[string] = stream.MapTo[int, string]
	_ func(stream.Streamer[int], func(int) int) stream.Streamer[int]                   = stream.DistinctBy[int, int]
	_ func(...int) stream.Pipeline[int]                                                = stream.Of[int]
)

func ExamplePipeline_MapTo() {
	names := stream.Of(1, 2, 2, 3).
		Filter(func(n int) bool { return n > 1 }).
		MapTo(strconv.Itoa).
		DistinctBy(func(s string) string { return s }).ToSlice()
	fmt.Println(names)
	// Output: [2 3]
}

func ExamplePipeline_FlatMapTo() {
	got := stream.Of(1, 2).FlatMapTo(func(n int) stream.Pipeline[string] {
		return stream.Of(strconv.Itoa(n), "!")
	}).Fold("", func(result, item string) string { return result + item })
	fmt.Println(got)
	// Output: 1!2!
}

func ExampleWrap() {
	legacy := stream.RepeatN(3, 2)
	typed := stream.Wrap(legacy).MapTo(strconv.Itoa)
	compatible := typed.Stream()
	fmt.Println(compatible.ToSlice())
	// Output: [3 3]
}

func TestPipelineLazinessAndTypedResults(t *testing.T) {
	mapped, flattened := 0, 0
	pipeline := stream.Of(1, 2, 3).MapTo(func(n int) string {
		mapped++
		return strconv.Itoa(n)
	}).FlatMapTo(func(s string) stream.Pipeline[string] {
		flattened++
		return stream.Of(s, s+"!")
	})
	if mapped != 0 || flattened != 0 {
		t.Fatal("intermediate methods executed eagerly")
	}
	got := pipeline.Limit(1).ToSlice()
	if !slices.Equal(got, []string{"1"}) || mapped != 1 || flattened != 1 {
		t.Fatalf("result=%v map=%d flatMap=%d", got, mapped, flattened)
	}
	total := stream.Of(1, 2, 3).Fold(int64(10), func(sum int64, n int) int64 { return sum + int64(n) })
	if total != 16 {
		t.Fatalf("Fold=%d", total)
	}
	index := stream.Of(1, 2).CollectTo(func(values ...int) map[int]string {
		result := make(map[int]string, len(values))
		for _, n := range values {
			result[n] = strconv.Itoa(n)
		}
		return result
	})
	if index[1] != "1" || index[2] != "2" {
		t.Fatalf("CollectTo=%v", index)
	}
}

func TestPipelineIntermediateMethodsRetainConcreteType(t *testing.T) {
	tests := []struct {
		name  string
		apply func(stream.Pipeline[int]) stream.Pipeline[int]
		want  []int
	}{
		{"Context", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.WithContext(context.Background()) }, []int{3, 1, 2, 2}},
		{"Filter", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Filter(func(n int) bool { return n > 1 }) }, []int{3, 2, 2}},
		{"Map", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Map(func(n int) int { return n * 2 }) }, []int{6, 2, 4, 4}},
		{"Peek", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Peek(func(int) {}) }, []int{3, 1, 2, 2}},
		{"Distinct", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Distinct() }, []int{3, 1, 2}},
		{"Sort", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Sort(cmp.Compare[int]) }, []int{1, 2, 2, 3}},
		{"Reverse-Sort", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.ReverseSort(cmp.Compare[int]) }, []int{3, 2, 2, 1}},
		{"Reverse", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Reverse() }, []int{2, 2, 1, 3}},
		{"Limit", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Limit(2) }, []int{3, 1}},
		{"Skip", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Skip(2) }, []int{2, 2}},
		{"Pick", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Pick(0, -1, 2) }, []int{3, 2}},
		{"Append", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Append(5) }, []int{3, 1, 2, 2, 5}},
		{"Execute", func(p stream.Pipeline[int]) stream.Pipeline[int] { return p.Execute() }, []int{3, 1, 2, 2}},
		{"Parallel", func(p stream.Pipeline[int]) stream.Pipeline[int] {
			return p.Parallel(2).Ordered().Map(func(n int) int { return n })
		}, []int{3, 1, 2, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calling a generic method afterwards also guards against interface returns.
			got := tt.apply(stream.Of(3, 1, 2, 2)).MapTo(strconv.Itoa).ToSlice()
			want := make([]string, len(tt.want))
			for i, n := range tt.want {
				want[i] = strconv.Itoa(n)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestPipelineLegacyTerminalsAndConversions(t *testing.T) {
	source := func() stream.Pipeline[int] { return stream.Of(1, 2, 3) }
	if got := source().Collect(func(v ...int) any { return len(v) }); got != 3 {
		t.Fatalf("Collect=%v", got)
	}
	if got := source().Reduce(func(a, b int) int { return a + b }); got != 6 {
		t.Fatalf("Reduce=%d", got)
	}
	if got := source().ReduceFrom(10, func(a, b int) int { return a + b }); got != 16 {
		t.Fatalf("ReduceFrom=%d", got)
	}
	if got := source().ReduceWith("", func(a any, b int) any { return a.(string) + strconv.Itoa(b) }); got != "123" {
		t.Fatalf("ReduceWith=%v", got)
	}
	if got := source().ReduceBy(func(n int) any { return n }, func(a any, b int) any { return a.(int) + b }); got != 9 {
		t.Fatalf("ReduceBy=%v", got)
	}
	if !source().AllMatch(func(n int) bool { return n > 0 }) || !source().NonMatch(func(n int) bool { return n < 0 }) || !source().AnyMatch(func(n int) bool { return n == 2 }) {
		t.Fatal("match result")
	}
	if source().First() != 1 || source().Last() != 3 || source().Count() != 3 {
		t.Fatal("element/count result")
	}
	if got := source().Take(); got < 1 || got > 3 {
		t.Fatalf("Take=%d", got)
	}
	if got := source().Any(); got < 1 || got > 3 {
		t.Fatalf("Any=%d", got)
	}
	var seen []int
	source().ForEach(func(n int) { seen = append(seen, n) })
	if !slices.Equal(seen, []int{1, 2, 3}) {
		t.Fatalf("ForEach=%v", seen)
	}
	if got := slices.Collect(source().Seq()); !slices.Equal(got, seen) {
		t.Fatalf("Seq=%v", got)
	}
	converted := source().Convert(func(n int) any { return strconv.Itoa(n) }).MapTo(func(v any) string { return v.(string) }).ToSlice()
	if !slices.Equal(converted, []string{"1", "2", "3"}) {
		t.Fatalf("Convert=%v", converted)
	}
	flat := source().FlatMap(func(n int) stream.Streamer[any] { return stream.SliceOf[any](n, n) }).Count()
	if flat != 6 {
		t.Fatalf("legacy FlatMap count=%d", flat)
	}
}

func TestPipelineZeroAndSnapshot(t *testing.T) {
	var zero stream.Pipeline[int]
	if zero.Count() != 0 || len(zero.ToSlice()) != 0 || zero.First() != 0 {
		t.Fatal("zero pipeline not empty")
	}
	if got := zero.Fold("initial", func(s string, n int) string { return s + strconv.Itoa(n) }); got != "initial" {
		t.Fatalf("empty Fold=%s", got)
	}
	calls := 0
	if got := zero.CollectTo(func(v ...int) int { calls++; return len(v) }); got != 0 || calls != 1 {
		t.Fatal("empty CollectTo must run once")
	}
	if zero.MapTo(strconv.Itoa).FlatMapTo(func(s string) stream.Pipeline[int] { return stream.Of(len(s)) }).Count() != 0 {
		t.Fatal("empty typed chain")
	}
	snapshot := stream.Of(1, 2, 3).MapTo(strconv.Itoa).Execute()
	for range 2 {
		if got := snapshot.ToSlice(); !slices.Equal(got, []string{"1", "2", "3"}) {
			t.Fatalf("snapshot=%v", got)
		}
	}
}

func TestPipelineWrapAndCardinality(t *testing.T) {
	calls := 0
	wrapped := stream.Wrap(stream.SliceOf(1, 2, 3)).MapTo(func(n int) string { calls++; return strconv.Itoa(n) })
	if got := wrapped.Skip(1).Limit(1).Count(); got != 1 || calls != 0 {
		t.Fatalf("size hint=%d calls=%d", got, calls)
	}
	if got := stream.Of(1, 2).FlatMapTo(func(n int) stream.Pipeline[int] { return stream.Of(n, n) }).Count(); got != 4 {
		t.Fatalf("FlatMapTo hint=%d", got)
	}
	foreign := func() stream.Pipeline[int] {
		return stream.Wrap(wrappedStreamer{Streamer: stream.SliceOf(1, 1, 2).Parallel(2)})
	}
	if got := foreign().DistinctBy(func(n int) int { return n }).ToSlice(); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("foreign=%v", got)
	}
	if got := foreign().First(); got != 1 {
		t.Fatalf("foreign First=%d", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := stream.Wrap(stream.SliceOf(1).WithContext(ctx)).MapTo(strconv.Itoa).ToSlice(); len(got) != 0 {
		t.Fatalf("Wrap lost context: %v", got)
	}
}

func TestPipelineTypedParallelBoundaries(t *testing.T) {
	values := make([]int, 256)
	for i := range values {
		values[i] = i
	}
	for _, boundary := range []string{"MapTo", "FlatMapTo", "Convert", "FlatMap"} {
		t.Run(boundary, func(t *testing.T) {
			source := stream.Of(values...).Parallel(2).Ordered().Map(func(n int) int { return n })
			typed := source.MapTo(func(n int) int64 { return int64(n) })
			switch boundary {
			case "FlatMapTo":
				typed = source.FlatMapTo(func(n int) stream.Pipeline[int64] { return stream.Of(int64(n)) })
			case "Convert":
				typed = source.Convert(func(n int) any { return int64(n) }).MapTo(func(v any) int64 { return v.(int64) })
			case "FlatMap":
				typed = source.FlatMap(func(n int) stream.Streamer[any] { return stream.SliceOf[any](int64(n)) }).MapTo(func(v any) int64 { return v.(int64) })
			}
			var active, peak, arrived atomic.Int64
			ready := make(chan struct{})
			got := typed.Map(func(n int64) int64 {
				a := active.Add(1)
				for old := peak.Load(); a > old; old = peak.Load() {
					if peak.CompareAndSwap(old, a) {
						break
					}
				}
				if n == 0 || n == 64 {
					if arrived.Add(1) == 2 {
						close(ready)
					}
					select {
					case <-ready:
					case <-time.After(3 * time.Second):
						t.Error("second worker did not enter the typed section")
					}
				}
				active.Add(-1)
				return n
			}).ToSlice()
			if peak.Load() < 2 {
				t.Fatal("typed boundary lost worker count")
			}
			if len(got) != len(values) {
				t.Fatalf("length=%d", len(got))
			}
			for i, n := range got {
				if n != int64(i) {
					t.Fatalf("lost Ordered at %d: %d", i, n)
				}
			}
		})
	}
}

func TestPipelineFoldCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := stream.Wrap(stream.Repeat(3)).WithContext(ctx).Fold(int64(0), func(sum int64, n int) int64 { cancel(); return sum + int64(n) })
	if got != 3 {
		t.Fatalf("partial Fold=%d", got)
	}
	calls := 0
	if got := stream.Of(1, 2).WithContext(ctx).CollectTo(func(v ...int) int { calls++; return len(v) }); got != 0 || calls != 1 {
		t.Fatal("cancelled CollectTo")
	}
}

func TestPipelineFlatMapCancellationAndShortCircuit(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		t.Run(fmt.Sprintf("parallel=%t", parallel), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			done := make(chan []int, 1)
			source := stream.Of(1).WithContext(ctx).FlatMapTo(func(int) stream.Pipeline[int] {
				inner := stream.Wrap(stream.From(func(yield func(int) bool) {
					close(started)
					for yield(1) {
					}
				}, -1))
				if parallel {
					inner = inner.Parallel(2)
				}
				return inner.Filter(func(int) bool { return false })
			})
			go func() { done <- source.ToSlice() }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("inner did not start")
			}
			cancel()
			select {
			case got := <-done:
				if len(got) != 0 {
					t.Fatalf("cancelled result=%v", got)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("inner ignored outer cancellation")
			}
		})
	}
	pulled := 0
	got := stream.Of(1).FlatMapTo(func(int) stream.Pipeline[int] {
		return stream.Wrap(stream.From(func(yield func(int) bool) {
			for i := 0; ; i++ {
				pulled++
				if !yield(i) {
					return
				}
			}
		}, -1))
	}).First()
	if got != 0 || pulled != 1 {
		t.Fatalf("First=%d pulls=%d", got, pulled)
	}
	// A child cancelled before iteration must remain empty after adaptation.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := stream.Of(1).FlatMapTo(func(int) stream.Pipeline[int] { return stream.Of(9).WithContext(ctx) }).ToSlice(); len(got) != 0 {
		t.Fatalf("child context=%v", got)
	}
}

func TestPipelineExplicitMethodInstantiation(t *testing.T) {
	mapper := stream.Pipeline[int].MapTo[string]
	got := mapper(stream.Of(1, 2), strconv.Itoa).ToSlice()
	if !slices.Equal(got, []string{"1", "2"}) {
		t.Fatalf("method expression=%v", got)
	}
	collect := stream.Of(3, 4).CollectTo[int]
	if got := collect(func(v ...int) int { return len(v) }); got != 2 {
		t.Fatalf("method value=%d", got)
	}
}

func TestPipelineFlatMapCancellationBeforeMapper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := stream.Of(1).Peek(func(int) { cancel() }).WithContext(ctx).FlatMapTo(func(n int) stream.Pipeline[int] {
		t.Error("mapper ran after cancellation")
		return stream.Of(n)
	}).ToSlice()
	if len(got) != 0 {
		t.Fatalf("cancelled result=%v", got)
	}
}
