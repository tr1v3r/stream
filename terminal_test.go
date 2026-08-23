package stream_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/tr1v3r/stream"
)

func TestTerminal_ForEach(t *testing.T) {
	var got []int
	stream.SliceOf(1, 2, 3).ForEach(func(n int) { got = append(got, n) })
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("expected in-order [1 2 3], got %v", got)
	}
}

func TestTerminal_CollectCustom(t *testing.T) {
	joined := stream.SliceOf("a", "b", "c").Collect(func(data ...string) any {
		return strings.Join(data, "-")
	}).(string)
	if joined != "a-b-c" {
		t.Fatalf("expected a-b-c, got %q", joined)
	}
}

func TestTerminal_CountFastPath(t *testing.T) {
	calls := 0
	seq := func(yield func(int) bool) {
		calls++
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				return
			}
		}
	}
	if n := stream.From(seq, 3).Count(); n != 3 {
		t.Fatalf("expected 3, got %d", n)
	}
	if calls != 0 {
		t.Fatalf("known sizeHint must skip iteration, seq was pulled %d times", calls)
	}
	if n := stream.From(seq, -1).Count(); n != 3 {
		t.Fatalf("expected 3 by iteration, got %d", n)
	}
	if calls != 1 {
		t.Fatalf("unknown hint must iterate once, got %d pulls", calls)
	}
}

func TestTerminal_MatchOps(t *testing.T) {
	even := func(n int) bool { return n%2 == 0 }
	if !stream.SliceOf(2, 4, 6).AllMatch(even) {
		t.Fatal("AllMatch expected true")
	}
	if stream.SliceOf(2, 5, 6).AllMatch(even) {
		t.Fatal("AllMatch expected false")
	}
	if !stream.SliceOf(1, 3, 5).NonMatch(even) {
		t.Fatal("NonMatch expected true")
	}
	if stream.SliceOf(1, 4, 5).NonMatch(even) {
		t.Fatal("NonMatch expected false")
	}
	if !stream.SliceOf(1, 4, 5).AnyMatch(even) {
		t.Fatal("AnyMatch expected true")
	}
	if stream.SliceOf(1, 3, 5).AnyMatch(even) {
		t.Fatal("AnyMatch expected false")
	}
}

func TestTerminal_ReduceVariants(t *testing.T) {
	sum := func(a, b int) int { return a + b }
	if got := stream.SliceOf(1, 2, 3).Reduce(sum); got != 6 {
		t.Fatalf("expected 6, got %d", got)
	}
	if got := stream.SliceOf[int]().Reduce(sum); got != 0 {
		t.Fatalf("empty Reduce must return zero value, got %d", got)
	}
	if got := stream.SliceOf[int]().ReduceFrom(10, sum); got != 10 {
		t.Fatalf("empty ReduceFrom must return init, got %d", got)
	}
	if got := stream.SliceOf(1, 2, 3).ReduceFrom(10, sum); got != 16 {
		t.Fatalf("expected 16, got %d", got)
	}

	concat := stream.SliceOf("a", "b").ReduceWith("", func(acc any, s string) any {
		return acc.(string) + s
	}).(string)
	if concat != "ab" {
		t.Fatalf("expected ab, got %q", concat)
	}

	res := stream.SliceOf(1, 2, 3).ReduceBy(
		func(sizeMayNegative int) any { return make([]int, 0, sizeMayNegative) },
		func(acc any, v int) any { return append(acc.([]int), v) },
	).([]int)
	if !slices.Equal(res, []int{1, 2, 3}) {
		t.Fatalf("expected [1 2 3], got %v", res)
	}
	if cap(res) != 3 {
		t.Fatalf("init builder must receive sizeHint 3, got cap %d", cap(res))
	}
}

func TestTerminal_FirstLastEmpty(t *testing.T) {
	if got := stream.SliceOf[int]().First(); got != 0 {
		t.Fatalf("empty First must be zero value, got %d", got)
	}
	if got := stream.SliceOf[int]().Last(); got != 0 {
		t.Fatalf("empty Last must be zero value, got %d", got)
	}
}

func TestTerminal_TakeSingle(t *testing.T) {
	if got := stream.SliceOf(7).Take(); got != 7 {
		t.Fatalf("single-element Take must return it, got %d", got)
	}
	if got := stream.SliceOf(7).Any(); got != 7 {
		t.Fatalf("single-element Any must return it, got %d", got)
	}
}
