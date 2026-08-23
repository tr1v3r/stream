package stream_test

import (
	"context"
	"slices"
	"testing"

	"github.com/tr1v3r/stream"
)

func TestOps_Filter(t *testing.T) {
	got := stream.SliceOf(1, 2, 3, 4, 5).Filter(func(n int) bool { return n > 2 }).ToSlice()
	if !slices.Equal(got, []int{3, 4, 5}) {
		t.Fatalf("expected [3 4 5], got %v", got)
	}
}

func TestOps_Map(t *testing.T) {
	got := stream.SliceOf(1, 2, 3).Map(func(n int) int { return n * 10 }).ToSlice()
	if !slices.Equal(got, []int{10, 20, 30}) {
		t.Fatalf("expected [10 20 30], got %v", got)
	}
}

func TestOps_Convert(t *testing.T) {
	got := stream.SliceOf(1, 2, 3).Convert(func(i int) any { return i * 10 }).ToSlice()
	if len(got) != 3 || got[0] != any(10) || got[2] != any(30) {
		t.Fatalf("expected [10 20 30] as any, got %v", got)
	}
}

func TestOps_Peek(t *testing.T) {
	var seen []int
	got := stream.SliceOf(1, 2, 3).
		Peek(func(n int) { seen = append(seen, n*10) }).
		ToSlice()
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("peek must pass elements through, got %v", got)
	}
	if !slices.Equal(seen, []int{10, 20, 30}) {
		t.Fatalf("expected side effects [10 20 30], got %v", seen)
	}
}

func TestOps_Append(t *testing.T) {
	got := stream.SliceOf(1, 2).Append(3, 4).ToSlice()
	if !slices.Equal(got, []int{1, 2, 3, 4}) {
		t.Fatalf("expected [1 2 3 4], got %v", got)
	}
	if n := stream.SliceOf(1, 2).Append(3, 4).Count(); n != 4 {
		t.Fatalf("expected Count 4, got %d", n)
	}
}

func TestOps_Execute(t *testing.T) {
	src := []int{1, 2, 3}
	s := stream.SliceOf(src...).Map(func(n int) int { return n * 2 }).Execute()
	src[0] = 99 // mutating the source after Execute must not affect the snapshot
	first := s.ToSlice()
	second := s.ToSlice() // Execute snapshot is re-iterable, unlike lazy pipelines
	if !slices.Equal(first, []int{2, 4, 6}) || !slices.Equal(second, []int{2, 4, 6}) {
		t.Fatalf("expected stable snapshot [2 4 6], got %v then %v", first, second)
	}
}

func TestOps_Distinct(t *testing.T) {
	got := stream.SliceOf(4, 1, 3, 3, 2, 4).Distinct().ToSlice()
	if !slices.Equal(got, []int{4, 1, 3, 2}) {
		t.Fatalf("expected first-occurrence order [4 1 3 2], got %v", got)
	}
}

func TestOps_DistinctBy(t *testing.T) {
	// comparable keys: exact Go equality, no string coercion
	got := stream.DistinctBy(stream.SliceOf(1, 1, 2, 3, 3, 1), func(n int) int { return n }).ToSlice()
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("expected [1 2 3], got %v", got)
	}

	// key by struct field: same key -> first occurrence wins
	type user struct{ dept, name string }
	got2 := stream.DistinctBy(
		stream.SliceOf(user{"a", "x"}, user{"b", "y"}, user{"a", "z"}),
		func(u user) string { return u.dept },
	).ToSlice()
	if len(got2) != 2 || got2[0].name != "x" || got2[1].name != "y" {
		t.Fatalf("expected first-per-dept [a/x b/y], got %v", got2)
	}

	// unlike Distinct, the key function decides: int 1 and string "1" coexist
	type typedKey struct {
		isInt bool
		intV  int
		strV  string
	}
	got3 := stream.DistinctBy(stream.SliceOf[any](1, "1"), func(v any) typedKey {
		switch x := v.(type) {
		case int:
			return typedKey{isInt: true, intV: x}
		case string:
			return typedKey{strV: x}
		}
		return typedKey{}
	}).ToSlice()
	if len(got3) != 2 {
		t.Fatalf("expected both kept with exact keys, got %v", got3)
	}

	// sizeHint semantics match Distinct: unknown after dedup
	if n := stream.DistinctBy(stream.SliceOf(1, 1, 2), func(n int) int { return n }).Count(); n != 2 {
		t.Fatalf("expected Count 2 by iteration, got %d", n)
	}
}

func TestOps_DistinctAnyCollision(t *testing.T) {
	// Documented quirk: distinct keys come from fmt.Sprint, so int 1 and
	// string "1" collide and the later one is dropped.
	got := stream.SliceOf[any](1, "1", 2).Distinct().ToSlice()
	if len(got) != 2 {
		t.Fatalf("expected collision to drop \"1\", got %v", got)
	}
}

type keyObj struct{ id string }

func (k keyObj) Key() string { return k.id }

func TestOps_DistinctUnique(t *testing.T) {
	got := stream.SliceOf(keyObj{"a"}, keyObj{"b"}, keyObj{"a"}).Distinct().ToSlice()
	if len(got) != 2 || got[0].id != "a" || got[1].id != "b" {
		t.Fatalf("expected [a b] via types.Unique, got %v", got)
	}
}

func TestOps_SortAndReverse(t *testing.T) {
	asc := func(l, r int) int { return l - r }
	if got := stream.SliceOf(4, 1, 3, 2).Sort(asc).ToSlice(); !slices.Equal(got, []int{1, 2, 3, 4}) {
		t.Fatalf("expected [1 2 3 4], got %v", got)
	}
	if got := stream.SliceOf(4, 1, 3, 2).ReverseSort(asc).ToSlice(); !slices.Equal(got, []int{4, 3, 2, 1}) {
		t.Fatalf("expected [4 3 2 1], got %v", got)
	}
	if got := stream.SliceOf(1, 2, 3).Reverse().ToSlice(); !slices.Equal(got, []int{3, 2, 1}) {
		t.Fatalf("expected [3 2 1], got %v", got)
	}
	// empty inputs must be safe
	if got := stream.SliceOf[int]().Sort(asc).ToSlice(); len(got) != 0 {
		t.Fatalf("expected empty sort, got %v", got)
	}
	if got := stream.SliceOf[int]().Reverse().ToSlice(); len(got) != 0 {
		t.Fatalf("expected empty reverse, got %v", got)
	}
}

func TestOps_LimitSkip(t *testing.T) {
	if got := stream.SliceOf(1, 2, 3, 4, 5).Limit(2).ToSlice(); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("expected [1 2], got %v", got)
	}
	if got := stream.SliceOf(1, 2).Limit(99).ToSlice(); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("limit beyond size must keep all, got %v", got)
	}
	if got := stream.SliceOf(1, 2, 3).Limit(0).ToSlice(); len(got) != 0 {
		t.Fatalf("expected empty for Limit(0), got %v", got)
	}
	if got := stream.SliceOf(1, 2, 3).Skip(1).ToSlice(); !slices.Equal(got, []int{2, 3}) {
		t.Fatalf("expected [2 3], got %v", got)
	}
	if got := stream.SliceOf(1, 2).Skip(99).ToSlice(); len(got) != 0 {
		t.Fatalf("skip beyond size must yield empty, got %v", got)
	}
	if got := stream.SliceOf(1, 2).Skip(0).ToSlice(); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("Skip(0) must pass through, got %v", got)
	}
}

func TestOps_PickIntervalGuard(t *testing.T) {
	// known-size path
	if got := stream.SliceOf(1, 2, 3).Pick(0, 2, 0).ToSlice(); len(got) != 0 {
		t.Fatalf("interval <= 0 must yield empty, got %v", got)
	}
	// unknown-size materialize path
	seq := func(yield func(int) bool) {
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				return
			}
		}
	}
	if got := stream.From(seq, -1).Pick(0, 2, 0).ToSlice(); len(got) != 0 {
		t.Fatalf("interval <= 0 must yield empty, got %v", got)
	}
}

func TestCtx_CancelledSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).
		Filter(func(int) bool { return true }).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled ctx must yield empty, got %v", got)
	}
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).
		Map(func(n int) int { return n }).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled ctx must yield empty, got %v", got)
	}
	calls := 0
	stream.SliceOf(1, 2, 3).WithContext(ctx).ForEach(func(int) { calls++ })
	if calls != 0 {
		t.Fatalf("cancelled ctx must not consume, got %d calls", calls)
	}
}
