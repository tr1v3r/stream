package stream_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/tr1v3r/stream"
)

func TestFactory_Repeat(t *testing.T) {
	got := stream.Repeat(9).Limit(3).ToSlice()
	if !slices.Equal(got, []int{9, 9, 9}) {
		t.Fatalf("expected [9 9 9], got %v", got)
	}
}

func TestFactory_RepeatN(t *testing.T) {
	if got := stream.RepeatN(5, 3).ToSlice(); !slices.Equal(got, []int{5, 5, 5}) {
		t.Fatalf("expected [5 5 5], got %v", got)
	}
	if got := stream.RepeatN(5, 0).ToSlice(); len(got) != 0 {
		t.Fatalf("expected empty for zero count, got %v", got)
	}
	if n := stream.RepeatN(7, 4).Count(); n != 4 {
		t.Fatalf("expected Count 4 via sizeHint, got %d", n)
	}
}

func TestFactory_Concat(t *testing.T) {
	got := stream.Concat(stream.SliceOf(1, 2), stream.SliceOf[int](), stream.SliceOf(3)).ToSlice()
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("expected [1 2 3], got %v", got)
	}
	if got := stream.Concat[int]().ToSlice(); len(got) != 0 {
		t.Fatalf("expected empty concat, got %v", got)
	}
}

func TestFactory_From2(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	got := stream.From2(maps.All(m)).ToSlice() // values only, order varies
	slices.Sort(got)
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("expected values [1 2 3], got %v", got)
	}
}

func TestHelper_To(t *testing.T) {
	collector := stream.To(func(s string) int { return len(s) })
	got := collector("a", "bb", "ccc").([]int)
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("expected [1 2 3], got %v", got)
	}
}

func TestHelper_AnyTo(t *testing.T) {
	got := stream.SliceOf(1, 2, 3).
		Convert(func(i int) any { return i * 10 }).
		Collect(stream.AnyTo[int]()).([]int)
	if !slices.Equal(got, []int{10, 20, 30}) {
		t.Fatalf("expected [10 20 30], got %v", got)
	}
}
