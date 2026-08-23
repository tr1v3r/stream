package stream_test

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/tr1v3r/stream"
)

// Phase 1 of docs/proposals/parallel-v2.md: consecutive stateless ops in a
// parallel section fuse into ONE pool (not one pool per op), and a mid-chain
// Parallel(n) closes the current section and opens a new one.

func TestParallelV2_FusionSinglePool(t *testing.T) {
	// One pool executes the whole Filter+Map+Peek section: with 4 workers,
	// the stateless stages must all run on the worker side, observable as
	// concurrent execution of the "slow" stage with wall time < serial.
	data := make([]int, 400)
	for i := range data {
		data[i] = i
	}
	var mu sync.Mutex
	active, peak := 0, 0
	slow := func(v int) int {
		mu.Lock()
		active++
		if active > peak {
			peak = active
		}
		mu.Unlock()
		time.Sleep(2 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return v
	}

	start := time.Now()
	got := stream.SliceOf(data...).Parallel(4).
		Filter(func(int) bool { return true }).
		Map(slow).
		Peek(func(int) {}).
		ToSlice()
	elapsed := time.Since(start)

	slices.Sort(got)
	if len(got) != 400 || got[0] != 0 || got[399] != 399 {
		t.Fatalf("fusion lost elements: %d elems", len(got))
	}
	mu.Lock()
	defer mu.Unlock()
	if peak < 3 { // 4 workers fused => up to 4 concurrent slow() calls
		t.Fatalf("fused section ran serially: peak concurrency %d", peak)
	}
	if elapsed > time.Duration(400)*2*time.Millisecond/2 { // serial would be ~800ms; 4 workers ~200ms
		t.Fatalf("fusion did not parallelize: %v (peak %d)", elapsed, peak)
	}
}

func TestParallelV2_MidChainParallelReopens(t *testing.T) {
	// Parallel(8) then Parallel(4): two sections, second closes the first.
	// Results must still be complete and correct as a multiset.
	data := make([]int, 2000)
	for i := range data {
		data[i] = i
	}
	got := stream.SliceOf(data...).
		Parallel(8).
		Filter(func(v int) bool { return v%2 == 0 }).
		Parallel(4).
		Map(func(v int) int { return v * 10 }).
		ToSlice()
	if len(got) != 1000 {
		t.Fatalf("expected 1000 evens, got %d", len(got))
	}
	slices.Sort(got)
	if got[0] != 0 || got[999] != 19980 {
		t.Fatalf("two-section pipeline corrupted data: [%d..%d]", got[0], got[len(got)-1])
	}
}

func TestParallelV2_SectionClosesAtStatefulOp(t *testing.T) {
	// Sort after a parallel section: section flushes (single pool), sort
	// then sees correct data and produces sorted output.
	data := make([]int, 500)
	for i := range data {
		data[i] = (i * 7919) % 500
	}
	got := stream.SliceOf(data...).Parallel(4).
		Filter(func(v int) bool { return v < 250 }).
		Sort(func(a, b int) int { return a - b }).
		ToSlice()
	if !slices.IsSorted(got) {
		t.Fatal("sorted output expected after parallel section")
	}
	if len(got) != 250 {
		t.Fatalf("expected 250 elements < 250, got %d", len(got))
	}
}

func TestParallelV2_TerminalFlushesSection(t *testing.T) {
	// Every terminal must flush the open section: verified implicitly by
	// result correctness for the main terminals.
	data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if n := stream.SliceOf(data...).Parallel(3).
		Filter(func(int) bool { return true }).Count(); n != 10 {
		t.Fatalf("Count after section: expected 10, got %d", n)
	}
	even := func(v int) bool { return v%2 == 0 }
	if !stream.SliceOf(data...).Parallel(3).Filter(even).AllMatch(even) {
		t.Fatal("AllMatch after section expected true")
	}
	if got := stream.SliceOf(data...).Parallel(3).
		Filter(even).Reduce(func(a, b int) int { return a + b }); got != 30 {
		t.Fatalf("Reduce after section: expected 30, got %d", got)
	}
	if got := stream.SliceOf(data...).Parallel(3).
		Filter(even).First(); got != 2 && got != 4 && got != 6 && got != 8 && got != 10 {
		t.Fatalf("First after section returned non-member: %d", got)
	}
}

func TestParallelV2_ShortCircuitStillLean(t *testing.T) {
	// Short-circuit inside/after a fused section must not hang or leak;
	// infinite source + slow section + Limit.
	done := make(chan []int, 1)
	go func() {
		done <- stream.Repeat(1).Parallel(4).
			Map(func(n int) int { time.Sleep(5 * time.Millisecond); return n + 1 }).
			Limit(3).ToSlice()
	}()
	select {
	case got := <-done:
		if len(got) != 3 {
			t.Fatalf("expected 3, got %d", len(got))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("fused short-circuit hung")
	}
}
