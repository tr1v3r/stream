package stream_test

import (
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/tr1v3r/stream"
)

// Machine-independent performance gates for the parallel architecture
// (docs/proposals/parallel-v2.md acceptance A1/A2) and the other
// performance contracts of the library. Every check is either a RATIO of
// two same-process measurements (hardware cancels out) or an ALLOCATION
// COUNT (deterministic by construction) — never an absolute wall time.
//
// Gates (best-of-3 timings, GC settled before each run):
//
//	G1 near-free Filter+Map, Parallel(4) unordered <= 6x serial
//	G2 near-free, Parallel(4).Ordered()            <= 8x serial
//	G3 heavy work, Parallel(4) >= 1.5x faster than serial
//	G4 Sort pipeline <= 2.5x direct slices.SortFunc on the same data
//	G5 DistinctBy <= 0.6x Distinct time AND <= 5% of its allocations
//	G6 Take on 100k elements: <= 50 allocations (reservoir O(1) memory)
//	G7 Limit(2).ToSlice() on 100k elements: <= 200 allocations (lazy
//	   short-circuit must not traverse the source)
//
// Threshold calibration (measured; do not retune without cross-machine data):
//
//	G1: Apple M3 Pro 1.1-2.2x | GitHub CI shared runner 4.2x  -> gate 6.0
//	G2: Apple M3 Pro 0.8-3.2x | GitHub CI shared runner 5.0x  -> gate 8.0
//	G3: Apple M3 Pro 2.9-3.2x | GitHub CI shared runner 3.0x  -> gate 1.5
//	G4: Apple M3 Pro 1.17-1.22x (materialize overhead only)   -> gate 2.5
//	G5: Apple M3 Pro 0.24-0.27x time, 0.3% of Distinct allocs -> gate 0.6 / 0.05
//	G6: Apple M3 Pro 6 allocs/op                               -> gate 50
//	G7: Apple M3 Pro 12 allocs/op                              -> gate 200
//
// Healthy-v2 machinery overhead spans 1-5x across hardware (shared CI
// runners schedule channel machinery far worse than a laptop core); the v1
// per-op-pool regression sat at 14-21x and a fusion failure collapses G3 to
// ~1x — both remain far outside these bounds. Allocation gates are exact and
// hardware-free; a regression to materialize-all Take or eager traversal
// overshoots them by orders of magnitude.
//
// Deliberately NOT a time gate: proposal A6 (multi-section 16+2 vs single
// pool on IO-sim) — its speedup is core-count dependent (16 workers need
// 16 cores to beat 4), so it cannot be machine-independent; multi-section
// correctness is pinned by TestParallelV2_MidChainParallelReopens instead.
//
// Skipped under -race (the detector instruments channel operations and would
// skew machinery ratios) and under -short. CI runs it via the dedicated
// "Benchmark gate" step (plain build) — see .github/workflows/ci.yml.

func gateNearFree(parallel, ordered bool) func() {
	data := make([]int, 100000)
	for i := range data {
		data[i] = i
	}
	f := func(v int) bool { return v%2 == 0 }
	m := func(v int) int { return v + 1 }
	return func() {
		s := stream.SliceOf(data...)
		if parallel {
			s = s.Parallel(4)
		}
		if ordered {
			s = s.Ordered()
		}
		sink := 0
		for _, v := range s.Filter(f).Map(m).ToSlice() {
			sink += v
		}
		_ = sink
	}
}

func gateHeavy(parallel bool) func() {
	data := make([]int, 600)
	work := func(v int) int {
		x := v
		for range 150000 { // ~50us on a modern core; dominates machinery
			x = (x*31 + 7) % 1000000007
		}
		return x % 2
	}
	return func() {
		s := stream.SliceOf(data...)
		if parallel {
			s = s.Parallel(4)
		}
		sink := 0
		for _, v := range s.Map(work).ToSlice() {
			sink += v
		}
		_ = sink
	}
}

// gateBestOf returns the fastest of three timed runs (GC settled before
// each), damping scheduler and CI-neighbor noise.
func gateBestOf(f func()) time.Duration {
	const runs = 3
	var best time.Duration
	for range runs {
		runtime.GC()
		start := time.Now()
		f()
		if d := time.Since(start); best == 0 || d < best {
			best = d
		}
	}
	return best
}

func TestBenchmarkGate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	if gateRaceEnabled {
		t.Skip("skipped under -race: detector overhead skews machinery ratios")
	}

	serial := gateBestOf(gateNearFree(false, false))
	unordered := gateBestOf(gateNearFree(true, false))
	ordered := gateBestOf(gateNearFree(true, true))

	if r := float64(unordered) / float64(serial); r > 6.0 {
		t.Errorf("G1 FAIL: near-free Parallel(4) overhead %.2fx serial (gate <= 6.0x; a v1-style per-op-pool regression is 14-21x)", r)
	} else {
		t.Logf("G1 pass: unordered %.2fx serial", float64(unordered)/float64(serial))
	}
	if r := float64(ordered) / float64(serial); r > 8.0 {
		t.Errorf("G2 FAIL: ordered overhead %.2fx serial (gate <= 8.0x)", r)
	} else {
		t.Logf("G2 pass: ordered %.2fx serial", float64(ordered)/float64(serial))
	}

	serialHeavy := gateBestOf(gateHeavy(false))
	parallelHeavy := gateBestOf(gateHeavy(true))
	if sp := float64(serialHeavy) / float64(parallelHeavy); sp < 1.5 {
		t.Errorf("G3 FAIL: heavy-work speedup %.2fx (gate >= 1.5x; a fusion failure collapses to ~1x)", sp)
	} else {
		t.Logf("G3 pass: heavy speedup %.2fx", float64(serialHeavy)/float64(parallelHeavy))
	}

	// G4: the Sort pipeline is materialize + slices.SortFunc; it must track
	// a hand-rolled clone+sort of the same data. A reintroduced
	// sort.Interface adapter (v0.x) or an accidental O(n^2) comparator path
	// blows past this ratio.
	sortSrc := makeUnsorted(100000)
	cmpInt := func(a, b int) int { return a - b }
	pipelineSort := gateBestOf(func() {
		sink := 0
		for _, v := range stream.SliceOf(sortSrc...).Sort(cmpInt).ToSlice() {
			sink += v
		}
		_ = sink
	})
	directSort := gateBestOf(func() {
		data := slices.Clone(sortSrc)
		slices.SortFunc(data, cmpInt)
		sink := 0
		for _, v := range data {
			sink += v
		}
		_ = sink
	})
	if r := float64(pipelineSort) / float64(directSort); r > 2.5 {
		t.Errorf("G4 FAIL: Sort pipeline %.2fx direct slices.SortFunc (gate <= 2.5x)", r)
	} else {
		t.Logf("G4 pass: sort %.2fx direct", float64(pipelineSort)/float64(directSort))
	}

	// G5: DistinctBy (comparable keys) vs Distinct (fmt.Sprint keys) — both
	// time ratio and allocation ratio on the same input.
	distinctSrc := makeCyclic(5000, 1000)
	distinctBy := gateBestOf(func() {
		sink := 0
		for _, v := range stream.DistinctBy(stream.SliceOf(distinctSrc...), func(n int) int { return n }).ToSlice() {
			sink += v
		}
		_ = sink
	})
	distinct := gateBestOf(func() {
		sink := 0
		for _, v := range stream.SliceOf(distinctSrc...).Distinct().ToSlice() {
			sink += v
		}
		_ = sink
	})
	byAllocs := testing.AllocsPerRun(3, func() {
		stream.DistinctBy(stream.SliceOf(distinctSrc...), func(n int) int { return n }).ToSlice()
	})
	defAllocs := testing.AllocsPerRun(3, func() {
		stream.SliceOf(distinctSrc...).Distinct().ToSlice()
	})
	if r := float64(distinctBy) / float64(distinct); r > 0.6 {
		t.Errorf("G5 FAIL: DistinctBy %.2fx Distinct time (gate <= 0.6x; measured ~0.2x when healthy)", r)
	} else {
		t.Logf("G5 pass: DistinctBy %.2fx Distinct time", float64(distinctBy)/float64(distinct))
	}
	if r := byAllocs / defAllocs; r > 0.05 {
		t.Errorf("G5 FAIL: DistinctBy %.1f%% of Distinct allocations (%.0f vs %.0f; gate <= 5%%)", r*100, byAllocs, defAllocs)
	} else {
		t.Logf("G5 pass: DistinctBy %.1f%% of Distinct allocs", r*100)
	}

	// G6: Take is reservoir-sampled — O(1) memory regardless of source
	// size. A regression to materialize-all costs ~800KB / thousands of
	// allocs on this input.
	takeSrc := makeUnsorted(100000)
	if allocs := testing.AllocsPerRun(3, func() { stream.SliceOf(takeSrc...).Take() }); allocs > 50 {
		t.Errorf("G6 FAIL: Take on 100k elements allocated %.0f times (gate <= 50; reservoir is O(1) memory)", allocs)
	} else {
		t.Logf("G6 pass: Take %.0f allocs", allocs)
	}

	// G7: lazy short-circuit — Limit(2) must not traverse (let alone
	// materialize) the 100k source.
	if allocs := testing.AllocsPerRun(3, func() { stream.SliceOf(takeSrc...).Limit(2).ToSlice() }); allocs > 200 {
		t.Errorf("G7 FAIL: Limit(2).ToSlice() on 100k elements allocated %.0f times (gate <= 200; eager traversal would be orders more)", allocs)
	} else {
		t.Logf("G7 pass: lazy short-circuit %.0f allocs", allocs)
	}
}
