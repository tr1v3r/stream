package stream_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/tr1v3r/stream"
)

// Regression tests for the WithContext cancellation contract on bare
// pipelines (H1): terminals without intermediate ops never consulted ctx
// and hung forever on infinite sources, contrary to the promises in
// export.go (WithContext: "every terminal return promptly"), doc.go
// ("bound them with Limit or WithContext") and README.md ("Important
// Notes" — cancellable: ok). Cancellation is cooperative, checked at
// element boundaries.

// finite yields 1, 2, 3 and stops.
func finite(yield func(int) bool) {
	for i := 1; i <= 3; i++ {
		if !yield(i) {
			return
		}
	}
}

// slowInfinite yields 1 forever, one element per millisecond, so mid-run
// cancellation unblocks with a small, bounded partial result.
func slowInfinite(yield func(int) bool) {
	for {
		if !yield(1) {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// Pre-cancelled context: every terminal on a bare pipeline must return
// promptly with an empty / zero-value result.
func TestCancelBare_PrecancelledTerminalsReturnEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := func() stream.Streamer[int] { return stream.From(finite, -1).WithContext(ctx) }

	if got := src().ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled ToSlice: want empty, got %v", got)
	}
	if got := src().Collect(func(nums ...int) any { return nums }).([]int); len(got) != 0 {
		t.Fatalf("cancelled Collect: want empty, got %v", got)
	}
	if got := src().Last(); got != 0 {
		t.Fatalf("cancelled Last: want zero, got %d", got)
	}
	if got := src().Count(); got != 0 {
		t.Fatalf("cancelled Count (unknown hint): want 0, got %d", got)
	}
	if got := src().First(); got != 0 {
		t.Fatalf("cancelled First: want zero, got %d", got)
	}
	cmp := func(a, b int) int { return a - b }
	if got := src().Sort(cmp).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled Sort: want empty, got %v", got)
	}
	if got := src().ReverseSort(cmp).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled ReverseSort: want empty, got %v", got)
	}
	if got := src().Reverse().ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled Reverse: want empty, got %v", got)
	}
	// Pick end<0 with unknown hint takes the materialize path
	if got := src().Pick(0, -1, 2).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled Pick (materialize path): want empty, got %v", got)
	}
	// Pick end>=0 takes the counting-loop path
	if got := src().Pick(0, 2, 1).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled Pick (loop path): want empty, got %v", got)
	}
	if got := src().Execute().ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled Execute: want empty snapshot, got %v", got)
	}
	n := 0
	for range src().Seq() {
		n++
	}
	if n != 0 {
		t.Fatalf("cancelled Seq: want 0 elements, got %d", n)
	}
}

// Infinite source: cancelling the context mid-run must unblock every
// previously-hanging terminal (watchdog keeps the test itself finite).
func TestCancelBare_InfiniteSourceTerminalsUnblock(t *testing.T) {
	cases := []struct {
		name string
		run  func(stream.Streamer[int])
	}{
		{"ToSlice", func(s stream.Streamer[int]) { s.ToSlice() }},
		{"Collect", func(s stream.Streamer[int]) { s.Collect(func(nums ...int) any { return len(nums) }) }},
		{"Count", func(s stream.Streamer[int]) { s.Count() }},
		{"Last", func(s stream.Streamer[int]) { s.Last() }},
		{"First", func(s stream.Streamer[int]) { s.First() }},
		{"Sort", func(s stream.Streamer[int]) { s.Sort(func(a, b int) int { return a - b }).ToSlice() }},
		{"Reverse", func(s stream.Streamer[int]) { s.Reverse().ToSlice() }},
		{"PickNegativeEnd", func(s stream.Streamer[int]) { s.Pick(0, -1, 2).ToSlice() }},
		{"Execute", func(s stream.Streamer[int]) { s.Execute() }},
		{"SeqRange", func(s stream.Streamer[int]) {
			for range s.Seq() {
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				tc.run(stream.From(slowInfinite, -1).WithContext(ctx))
			}()
			time.Sleep(30 * time.Millisecond) // let the terminal start pulling
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("%s did not return after cancellation", tc.name)
			}
		})
	}
}

// The exact documented repro: Repeat + WithContext + ToSlice (README
// "Infinite streams" note). A tight infinite source must still unblock.
func TestCancelBare_RepeatToSliceUnblocks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan []int, 1)
	go func() { done <- stream.Repeat(1).WithContext(ctx).ToSlice() }()
	time.Sleep(5 * time.Millisecond)
	cancel()
	select {
	case got := <-done:
		if len(got) == 0 {
			t.Log("note: cancelled before any element was appended")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Repeat(1).WithContext(ctx).ToSlice() did not return after cancellation")
	}
}

// Control: an uncancelled context must not change any terminal's result.
func TestCancelBare_UncancelledControl(t *testing.T) {
	src := func() stream.Streamer[int] { return stream.From(finite, -1).WithContext(context.Background()) }

	if got := src().ToSlice(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("ToSlice: want [1 2 3], got %v", got)
	}
	if got := src().Count(); got != 3 {
		t.Fatalf("Count: want 3, got %d", got)
	}
	if got := src().Last(); got != 3 {
		t.Fatalf("Last: want 3, got %d", got)
	}
	if got := src().First(); got != 1 {
		t.Fatalf("First: want 1, got %d", got)
	}
	if got := src().Sort(func(a, b int) int { return a - b }).ToSlice(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("Sort: want [1 2 3], got %v", got)
	}
	if got := src().Pick(0, -1, 2).ToSlice(); !slices.Equal(got, []int{1, 3}) {
		t.Fatalf("Pick(0,-1,2): want [1 3], got %v", got)
	}
	if got := src().Execute().ToSlice(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("Execute: want [1 2 3], got %v", got)
	}
	n := 0
	for v := range src().Seq() {
		n++
		if v != n {
			t.Fatalf("Seq: want %d at position %d", n, v)
		}
	}
	if n != 3 {
		t.Fatalf("Seq: want 3 elements, got %d", n)
	}
}
