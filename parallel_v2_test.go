package stream_test

import (
	"context"
	"math/rand/v2"
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

func TestParallelV2_OrderedMatchesSerial(t *testing.T) {
	// A3 property: Ordered() output must equal serial output element-for-element.
	// Random pipelines over a fixed multiset, with heterogeneous stage costs
	// (sleep jitter) to force real out-of-order completion.
	rng := rand.New(rand.NewPCG(42, 2026))
	for trial := range 30 {
		n := 50 + rng.IntN(300)
		serialSrc := make([]int, n)
		for i := range serialSrc {
			serialSrc[i] = i % 17
		}

		seed := rng.Uint64()
		jitter := func(v int) int {
			r := rand.New(rand.NewPCG(seed, uint64(v)+1))
			time.Sleep(time.Duration(r.IntN(300)) * time.Microsecond)
			return v * 2 // observable transform
		}

		serial := stream.SliceOf(serialSrc...).
			Filter(func(v int) bool { return v%3 != 0 }).
			Map(jitter).
			ToSlice()

		ordered := stream.SliceOf(serialSrc...).Parallel(4).Ordered().
			Filter(func(v int) bool { return v%3 != 0 }).
			Map(jitter).
			ToSlice()

		if !slices.Equal(serial, ordered) {
			t.Fatalf("trial %d: ordered diverged from serial\nserial : %v\nordered: %v", trial, serial, ordered)
		}
	}
}

func TestParallelV2_OrderedVsUnordered(t *testing.T) {
	// unordered may permute (or coincide); ordered must not — smoke test
	// with visible divergence under sleep jitter, then verify Ordered is
	// exactly sorted-back-to-input-order after a non-reordering pipeline.
	src := make([]int, 500)
	for i := range src {
		src[i] = i
	}
	got := stream.SliceOf(src...).Parallel(8).Ordered().
		Map(func(v int) int { time.Sleep(time.Duration(v%7) * time.Millisecond); return v }).
		ToSlice()
	if !slices.Equal(got, src) {
		t.Fatal("Ordered() must reproduce input order exactly for identity pipeline")
	}
}

func TestParallelV2_OrderedShortCircuitAndCancel(t *testing.T) {
	// Ordered section + Limit short-circuit: must not hang, must respect order
	done := make(chan []int, 1)
	go func() {
		done <- stream.SliceOf(make([]int, 100000)...).Parallel(4).Ordered().
			Map(func(v int) int { return v + 1 }).
			Limit(5).ToSlice()
	}()
	select {
	case got := <-done:
		if len(got) != 5 {
			t.Fatalf("expected 5, got %d", len(got))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ordered short-circuit hung")
	}

	// pre-cancelled ctx: ordered section yields nothing
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := stream.SliceOf(1, 2, 3).WithContext(ctx).Parallel(2).Ordered().
		Map(func(v int) int { return v }).ToSlice(); len(got) != 0 {
		t.Fatalf("cancelled ordered section must yield nothing, got %v", got)
	}
}

func TestParallelV2_OrderedAdversarialBatchHeads(t *testing.T) {
	// Filter drops EVERY batch head (idx%64==0): batches must still
	// re-sequenence correctly because filtered elements travel as holes,
	// keeping batch boundaries and lengths intact.
	src := make([]int, 1000)
	for i := range src {
		src[i] = i
	}
	done := make(chan []int, 1)
	go func() {
		done <- stream.SliceOf(src...).Parallel(4).Ordered().
			Filter(func(v int) bool { return v%64 != 0 }).
			Map(func(v int) int { return v }).
			ToSlice()
	}()
	select {
	case got := <-done:
		if len(got) != 1000-16 {
			t.Fatalf("expected %d elements, got %d", 1000-16, len(got))
		}
		for i, v := range got {
			want := i + i/63 + 1 // original order minus multiples of 64
			if v != want {
				t.Fatalf("pos %d: got %d want %d", i, v, want)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("adversarial ordered pipeline hung")
	}
}

func TestParallelV2_OrderedAllFiltered(t *testing.T) {
	// every element filtered: all-hole batches must flow through without
	// hanging and yield nothing
	done := make(chan []int, 1)
	go func() {
		done <- stream.SliceOf(make([]int, 1000)...).Parallel(4).Ordered().
			Filter(func(int) bool { return false }).
			ToSlice()
	}()
	select {
	case got := <-done:
		if len(got) != 0 {
			t.Fatalf("expected empty, got %v", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("all-hole ordered pipeline hung")
	}
}
