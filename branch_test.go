package stream_test

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/tr1v3r/stream"
)

// --- downstream short-circuit: every intermediate op must stop when the
// --- consumer stops (Limit/First) instead of dragging the source along.

func TestBranch_ShortCircuitIntermediateOps(t *testing.T) {
	src := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	if got := stream.SliceOf(src...).Filter(func(int) bool { return true }).First(); got != 1 {
		t.Fatalf("Filter+First: expected 1, got %d", got)
	}
	if got := stream.SliceOf(src...).Peek(func(int) {}).First(); got != 1 {
		t.Fatalf("Peek+First: expected 1, got %d", got)
	}
	if got := stream.SliceOf(src...).Limit(8).First(); got != 1 {
		t.Fatalf("Limit+First: expected 1, got %d", got)
	}
	if got := stream.SliceOf(src...).Skip(2).First(); got != 3 {
		t.Fatalf("Skip+First: expected 3, got %d", got)
	}
	if got := stream.SliceOf(src...).Sort(func(a, b int) int { return a - b }).First(); got != 1 {
		t.Fatalf("Sort+First: expected 1, got %d", got)
	}
	if got := stream.SliceOf(src...).ReverseSort(func(a, b int) int { return a - b }).First(); got != 10 {
		t.Fatalf("ReverseSort+First: expected 10, got %d", got)
	}
	if got := stream.SliceOf(src...).Reverse().First(); got != 10 {
		t.Fatalf("Reverse+First: expected 10, got %d", got)
	}
	if got := stream.SliceOf(src...).Pick(0, 9, 2).First(); got != 1 {
		t.Fatalf("Pick+First: expected 1, got %d", got)
	}
	if got := stream.DistinctBy(stream.SliceOf(src...), func(n int) int { return n }).First(); got != 1 {
		t.Fatalf("DistinctBy+First: expected 1, got %d", got)
	}

	// Convert / FlatMap produce Streamer[any]
	if got := stream.SliceOf(src...).Convert(func(i int) any { return i }).Limit(2).ToSlice(); len(got) != 2 {
		t.Fatalf("Convert+Limit: expected 2 elements, got %v", got)
	}
	if got := stream.SliceOf(1, 2, 3).
		FlatMap(func(n int) stream.Streamer[any] { return stream.SliceOf[any](n, n*10) }).
		First(); got != any(1) {
		t.Fatalf("FlatMap+First: expected 1, got %v", got)
	}

	// Append: break inside the source loop, then inside the appended loop
	if got := stream.SliceOf(src...).Append(99).First(); got != 1 {
		t.Fatalf("Append+First (source break): expected 1, got %d", got)
	}
	if got := stream.SliceOf[int]().Append(7, 8, 9).First(); got != 7 {
		t.Fatalf("Append+First (data break): expected 7, got %d", got)
	}

	// factories: Concat and From2 must honor early exit
	if got := stream.Concat(stream.SliceOf(1, 2), stream.SliceOf(3, 4)).First(); got != 1 {
		t.Fatalf("Concat+First: expected 1, got %d", got)
	}
	pairs := func(yield func(string, int) bool) {
		for _, kv := range [][2]any{{"a", 1}, {"b", 2}} {
			if !yield(kv[0].(string), kv[1].(int)) {
				return
			}
		}
	}
	if got := stream.From2(pairs).First(); got != 1 {
		t.Fatalf("From2+First: expected 1, got %d", got)
	}
}

func TestBranch_PickMaterializePath(t *testing.T) {
	// end<0 with unknown sizeHint forces materialization
	seq := func(yield func(int) bool) {
		for i := 1; i <= 5; i++ {
			if !yield(i) {
				return
			}
		}
	}
	if got := stream.From(seq, -1).Pick(0, -1, 2).ToSlice(); !slices.Equal(got, []int{1, 3, 5}) {
		t.Fatalf("expected [1 3 5] via materialize path, got %v", got)
	}
	if got := stream.From(seq, -1).Pick(0, -1, 2).First(); got != 1 {
		t.Fatalf("expected early exit inside materialize path, got %d", got)
	}
	// Skip with unknown sizeHint keeps hint unknown
	if got := stream.From(seq, -1).Skip(3).ToSlice(); !slices.Equal(got, []int{4, 5}) {
		t.Fatalf("expected [4 5], got %v", got)
	}
	// Append with unknown sizeHint keeps hint unknown (Count iterates)
	if n := stream.From(seq, -1).Append(6).Count(); n != 6 {
		t.Fatalf("expected 6, got %d", n)
	}
}

func TestBranch_CancelledBranches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).
		Convert(func(i int) any { return i }).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled Convert must yield nothing, got %v", got)
	}
	var peeked int
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).
		Peek(func(int) { peeked++ }).ToSlice(); len(got) != 0 || peeked != 0 {
		t.Fatalf("cancelled Peek must not consume, got %v peeked=%d", got, peeked)
	}
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).
		FlatMap(func(n int) stream.Streamer[any] { return stream.SliceOf[any](n) }).
		ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled FlatMap must yield nothing, got %v", got)
	}
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).Skip(1).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled Skip must yield nothing, got %v", got)
	}
	if got := stream.DistinctBy(stream.SliceOf(1, 2, 3).WithContext(ctx), func(n int) int { return n }).
		ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled DistinctBy must yield nothing, got %v", got)
	}
	if stream.SliceOf(1, 2, 3).WithContext(ctx).AnyMatch(func(int) bool { return true }) {
		t.Fatal("cancelled AnyMatch must return false")
	}
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).Reduce(func(a, b int) int { return a + b }); got != 0 {
		t.Fatalf("cancelled Reduce must return zero value, got %d", got)
	}
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).ReduceFrom(10, func(a, b int) int { return a + b }); got != 10 {
		t.Fatalf("cancelled ReduceFrom must return init, got %d", got)
	}
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).
		ReduceWith("init", func(acc any, n int) any { return acc }).(string); got != "init" {
		t.Fatalf("cancelled ReduceWith must return init, got %q", got)
	}
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).
		ReduceBy(func(int) any { return "seed" }, func(acc any, n int) any { return acc }).(string); got != "seed" {
		t.Fatalf("cancelled ReduceBy must return builder seed, got %q", got)
	}
}

func TestBranch_ParallelZero(t *testing.T) {
	// Parallel(n<=0) is a no-op returning the same stream
	got := stream.SliceOf(1, 2, 3).Parallel(0).ToSlice()
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("Parallel(0) must be a no-op, got %v", got)
	}
}

// foreign Streamer implementation: DistinctBy must fall back to Filter
type wrappedStreamer struct {
	stream.Streamer[int]
}

func TestBranch_DistinctByForeignStreamer(t *testing.T) {
	s := stream.SliceOf(1, 1, 2, 2, 3)
	w := wrappedStreamer{Streamer: s}
	got := stream.DistinctBy(w, func(n int) int { return n }).ToSlice()
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("foreign impl fallback: expected [1 2 3], got %v", got)
	}
}

func TestBranch_MapTo(t *testing.T) {
	got := stream.MapTo(stream.SliceOf(1, 2, 3), func(n int) string {
		return fmt.Sprintf("#%d", n)
	}).ToSlice()
	if !slices.Equal(got, []string{"#1", "#2", "#3"}) {
		t.Fatalf("expected [#1 #2 #3], got %v", got)
	}

	// sizeHint + short-circuit preserved
	if n := stream.MapTo(stream.SliceOf(1, 2, 3), func(n int) bool { return n > 1 }).Count(); n != 3 {
		t.Fatalf("expected Count 3 via preserved sizeHint, got %d", n)
	}
	if got := stream.MapTo(stream.SliceOf(1, 2, 3, 4), func(n int) int { return n * 10 }).First(); got != 10 {
		t.Fatalf("expected early exit first 10, got %d", got)
	}

	// foreign Streamer implementation goes through the Seq fallback
	w := wrappedStreamer{Streamer: stream.SliceOf(1, 2)}
	got2 := stream.MapTo(w, strconv.Itoa).ToSlice()
	if !slices.Equal(got2, []string{"1", "2"}) {
		t.Fatalf("foreign fallback: expected [1 2], got %v", got2)
	}
	// ...and its short-circuit path stops pulling the source
	if got := stream.MapTo(wrappedStreamer{Streamer: stream.SliceOf(1, 2, 3)}, strconv.Itoa).First(); got != "1" {
		t.Fatalf("foreign fallback short-circuit: expected \"1\", got %q", got)
	}

	// Convert now delegates to MapTo
	if got := stream.SliceOf(1, 2).Convert(func(i int) any { return i * 10 }).ToSlice(); len(got) != 2 {
		t.Fatalf("Convert delegation broken, got %v", got)
	}
}

func TestBranch_TakeReservoir(t *testing.T) {
	// empty stream: zero value, no panic
	if got := stream.SliceOf[int]().Take(); got != 0 {
		t.Fatalf("empty Take must be zero value, got %d", got)
	}
	// single / small streams: element of the source
	if got := stream.SliceOf(7).Take(); got != 7 {
		t.Fatalf("single-element Take must return it, got %d", got)
	}
	src := []int{1, 2, 3}
	for range 20 {
		if got := stream.SliceOf(src...).Take(); !slices.Contains(src, got) {
			t.Fatalf("Take must return a member, got %d", got)
		}
	}
	// distribution over members must be roughly uniform (reservoir sampling)
	counts := map[int]int{}
	for range 3000 {
		counts[stream.SliceOf(src...).Take()]++
	}
	for _, v := range src {
		if counts[v] < 700 { // ~1000 expected, generous floor
			t.Fatalf("distribution not uniform: counts %v", counts)
		}
	}
	// pre-cancelled stream: zero value without consuming
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).Take(); got != 0 {
		t.Fatalf("cancelled Take must return zero value, got %d", got)
	}
}
