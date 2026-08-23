package stream_test

import (
	"context"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tr1v3r/stream"
)

// Parallel mode does not preserve order; compare as sorted multisets.
func TestParallel_Filter(t *testing.T) {
	data := make([]int, 0, 1000)
	for i := 0; i < 1000; i++ {
		data = append(data, i)
	}
	got := stream.SliceOf(data...).Parallel(4).
		Filter(func(n int) bool { return n%2 == 0 }).ToSlice()
	slices.Sort(got)
	if len(got) != 500 || got[0] != 0 || got[499] != 998 {
		t.Fatalf("expected the 500 evens in [0,998], got %d elems: %v", len(got), got[:min(5, len(got))])
	}
}

func TestParallel_MapPeek(t *testing.T) {
	data := make([]int, 0, 300)
	for i := 0; i < 300; i++ {
		data = append(data, i)
	}
	var calls atomic.Int64
	got := stream.SliceOf(data...).Parallel(3).
		Map(func(n int) int { return n * 2 }).
		Peek(func(int) { calls.Add(1) }).
		ToSlice()
	slices.Sort(got)
	if len(got) != 300 || got[0] != 0 || got[299] != 598 {
		t.Fatalf("expected doubled values, got %d elems", len(got))
	}
	if calls.Load() != 300 {
		t.Fatalf("expected 300 Peek calls, got %d", calls.Load())
	}
}

func TestParallel_Count(t *testing.T) {
	data := make([]int, 0, 1000)
	for i := 0; i < 1000; i++ {
		data = append(data, i)
	}
	if n := stream.SliceOf(data...).Parallel(4).
		Filter(func(n int) bool { return n%2 == 0 }).Count(); n != 500 {
		t.Fatalf("expected 500, got %d", n)
	}
}

func TestCtx_CancelledParallel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// must terminate promptly instead of hanging; at most the 3 source
	// elements can ever escape before cancellation is observed
	got := stream.SliceOf(1, 2, 3).WithContext(ctx).Parallel(2).
		Map(func(n int) int { return n * 2 }).ToSlice()
	if len(got) > 3 {
		t.Fatalf("cancelled parallel stream must not over-produce, got %v", got)
	}
}

func TestParallel_ExecutePreservesParallel(t *testing.T) {
	var mu sync.Mutex
	inside, peak := 0, 0
	data := make([]int, 200)
	got := stream.SliceOf(data...).Parallel(4).Execute().
		Filter(func(n int) bool {
			mu.Lock()
			inside++
			if inside > peak {
				peak = inside
			}
			mu.Unlock()
			time.Sleep(2 * time.Millisecond)
			mu.Lock()
			inside--
			mu.Unlock()
			return true
		}).ToSlice()
	if len(got) != 200 {
		t.Fatalf("expected all 200 elements, got %d", len(got))
	}
	mu.Lock()
	defer mu.Unlock()
	if peak < 2 {
		t.Fatalf("Execute dropped parallelSize: peak filter concurrency %d, expected >= 2", peak)
	}
}

func TestParallel_ShortCircuitNoLeak(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	data := make([]int, 5000)
	if got := stream.SliceOf(data...).Parallel(4).
		Map(func(n int) int { return n + 1 }).
		Limit(2).ToSlice(); len(got) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(got))
	}
	if got := stream.SliceOf(data...).Parallel(4).
		Filter(func(int) bool { return true }).
		First(); got != 0 {
		t.Fatalf("expected first element 0, got %d", got)
	}

	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Fatalf("goroutine leak: before=%d after=%d", before, n)
	}
}

func TestParallel_ConcurrentTakeNoRace(t *testing.T) {
	src := make([]int, 100)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stream.SliceOf(src...).Take()
		}()
	}
	wg.Wait()
}
