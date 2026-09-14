package stream_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/tr1v3r/stream"
)

// Machine-independent performance gates for the parallel architecture
// (docs/proposals/parallel-v2.md acceptance A1/A2). Every check compares
// parallel against serial execution IN THE SAME PROCESS, so the ratio — not
// absolute time — is the contract; identical thresholds hold on any runner.
//
// Gates (best-of-3 timings, GC settled before each run):
//
//	G1 near-free Filter+Map, Parallel(4) unordered <= 6x serial
//	G2 near-free, Parallel(4).Ordered()            <= 8x serial
//	G3 heavy work, Parallel(4) >= 1.5x faster than serial
//
// Threshold calibration (measured; do not retune without cross-machine data):
//
//	G1: Apple M3 Pro 1.1-2.2x | GitHub CI shared runner 4.2x  -> gate 6.0
//	G2: Apple M3 Pro 0.8-3.2x | GitHub CI shared runner 5.0x  -> gate 8.0
//	G3: Apple M3 Pro 2.9-3.2x | GitHub CI shared runner 3.0x  -> gate 1.5
//
// Healthy-v2 machinery overhead spans 1-5x across hardware (shared CI
// runners schedule channel machinery far worse than a laptop core); the v1
// per-op-pool regression sat at 14-21x and a fusion failure collapses G3 to
// ~1x — both remain far outside these bounds.
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
}
