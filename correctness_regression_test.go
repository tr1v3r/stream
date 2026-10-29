package stream_test

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tr1v3r/stream"
)

func TestLimitCardinalityRegression(t *testing.T) {
	for _, count := range []int{0, 2, 20} {
		for _, limit := range []int64{-1, 0, 1, 10} {
			t.Run(fmt.Sprintf("count=%d/limit=%d", count, limit), func(t *testing.T) {
				source := func() stream.Streamer[int] {
					return stream.RepeatN(7, int64(count)).Filter(func(int) bool { return true })
				}
				want := int64(min(count, int(max(limit, 0))))
				if got := source().Limit(limit).Count(); got != want {
					t.Fatalf("Count=%d, want %d", got, want)
				}
				if got := int64(len(source().Limit(limit).ToSlice())); got != want {
					t.Fatalf("length=%d, want %d", got, want)
				}
			})
		}
	}
	if got := stream.SliceOf(1, 2).Filter(func(int) bool { return false }).Limit(10).Count(); got != 0 {
		t.Fatalf("empty filtered stream counted %d", got)
	}
}

func TestRepeatNRetainsExactCardinality(t *testing.T) {
	for _, count := range []int64{-1, 0, 3, math.MaxInt64} {
		if got := stream.RepeatN(7, count).Count(); got != max(count, 0) {
			t.Fatalf("RepeatN(%d).Count()=%d", count, got)
		}
	}
}

func TestLimitStopsPullingAtBoundary(t *testing.T) {
	for _, limit := range []int64{-1, 0, 1, 3} {
		calls := 0
		got := stream.From(func(yield func(int) bool) {
			for i := range 10 {
				calls++
				if calls > int(max(limit, 0)) {
					t.Fatalf("Limit(%d) pulled element %d", limit, calls)
				}
				if !yield(i) {
					return
				}
			}
		}, -1).Limit(limit).ToSlice()
		if calls != len(got) || int64(calls) != max(limit, 0) {
			t.Fatalf("Limit(%d): calls=%d result=%v", limit, calls, got)
		}
	}
}

func TestPickLargeIntervalRegression(t *testing.T) {
	for _, hint := range []int64{-1, 3} {
		for _, interval := range []int{2, 3, math.MaxInt} {
			got := stream.From(slices.Values([]int{1, 2, 3}), hint).Pick(1, -1, interval).ToSlice()
			if !slices.Equal(got, []int{2}) {
				t.Fatalf("hint=%d interval=%d: %v", hint, interval, got)
			}
		}
	}
}

func TestDistinctByForeignParallelRegression(t *testing.T) {
	values := make([]int, 8192)
	for i := range values {
		values[i] = i % 127
	}
	// Embedding hides the concrete implementation and exercises the fallback.
	foreign := wrappedStreamer{Streamer: stream.SliceOf(values...).Parallel(4)}
	got := stream.DistinctBy(foreign, func(n int) int { return n }).ToSlice()
	slices.Sort(got)
	want := make([]int, 127)
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(got, want) {
		t.Fatalf("dedup result=%v", got)
	}
	if got := stream.DistinctBy(wrappedStreamer{Streamer: stream.SliceOf(1, 1, 2)}, func(n int) int { return n }).First(); got != 1 {
		t.Fatalf("foreign short circuit=%d", got)
	}
}

func TestParallelMultiSectionShortCircuitRegression(t *testing.T) {
	for _, ordered := range []bool{false, true} {
		for _, boundary := range []string{"direct", "skip", "map-to", "concat", "flat-map"} {
			t.Run(fmt.Sprintf("ordered=%t/%s", ordered, boundary), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel() // also releases the old implementation on a test failure
				sourceStopped := make(chan struct{})
				source := stream.From(func(yield func(int) bool) {
					defer close(sourceStopped)
					for i := 0; ; i++ {
						if !yield(i) {
							return
						}
					}
				}, -1).WithContext(ctx).Parallel(1)
				if ordered {
					source = source.Ordered()
				}
				source = source.Filter(func(i int) bool { return i < 64 })
				switch boundary {
				case "skip":
					source = source.Skip(0)
				case "map-to":
					source = stream.MapTo(source, func(i int) int { return i })
				case "concat":
					source = stream.Concat(source, stream.SliceOf(99))
				case "flat-map":
					inner := source
					source = stream.MapTo(stream.SliceOf(1).FlatMap(func(int) stream.Streamer[any] {
						return stream.MapTo(inner, func(i int) any { return i })
					}), func(v any) int { return v.(int) })
				}
				done := make(chan int, 1)
				go func() { done <- source.Parallel(1).Map(func(i int) int { return i }).First() }()
				select {
				case got := <-done:
					if got != 0 {
						t.Fatalf("First=%d", got)
					}
				case <-time.After(3 * time.Second):
					cancel()
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Error("pipeline did not stop after cleanup cancellation")
					}
					t.Fatal("First did not finish after receiving its result")
				}
				select {
				case <-sourceStopped:
				default:
					t.Fatal("source still running after terminal returned")
				}
				if ctx.Err() != nil {
					t.Fatal("short circuit cancelled caller's context")
				}
			})
		}
	}
}

func TestParallelCancellationChecksEachElement(t *testing.T) {
	for _, ordered := range []bool{false, true} {
		for _, op := range []string{"map", "filter", "peek"} {
			t.Run(fmt.Sprintf("ordered=%t/%s", ordered, op), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				record := func(int) { calls++; cancel() }
				source := stream.RepeatN(1, 128).WithContext(ctx).Parallel(1)
				if ordered {
					source = source.Ordered()
				}
				switch op {
				case "map":
					source = source.Map(func(i int) int { record(i); return i })
				case "filter":
					source = source.Filter(func(i int) bool { record(i); return false })
				case "peek":
					source = source.Peek(record)
				}
				got := source.ToSlice()
				if calls != 1 || len(got) != 0 {
					t.Fatalf("callbacks=%d result=%v", calls, got)
				}
			})
		}
	}
}

func TestOrderedBoundedWindowRegression(t *testing.T) {
	for _, cancelEarly := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancelEarly), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			release := make(chan struct{})
			secondBatchDone := make(chan struct{})
			overflow := make(chan struct{}, 1)
			var processed atomic.Int64
			var pulled atomic.Int64
			done := make(chan []int, 1)
			go func() {
				done <- stream.From(func(yield func(int) bool) {
					for i := range 10000 {
						if pulled.Add(1) > 128 {
							select {
							case overflow <- struct{}{}:
							default:
							}
						}
						if !yield(i) {
							return
						}
					}
				}, -1).WithContext(ctx).Parallel(2).Ordered().Map(func(i int) int {
					if i == 0 {
						<-release
					}
					if processed.Add(1) > 128 {
						select {
						case overflow <- struct{}{}:
						default:
						}
					}
					if i == 127 {
						close(secondBatchDone)
					}
					return i
				}).ToSlice()
			}()
			// Cleanup releases the deliberately blocked callback even on failure.
			released, finished := false, false
			defer func() {
				cancel()
				if !released {
					close(release)
				}
				if !finished {
					select {
					case <-done:
					case <-time.After(3 * time.Second):
						t.Error("ordered cleanup hung")
					}
				}
			}()
			select {
			case <-secondBatchDone:
			case <-time.After(3 * time.Second):
				t.Fatal("second worker made no progress")
			}
			select {
			case <-overflow:
				t.Fatal("processed beyond two batches while first batch blocked")
			case <-time.After(30 * time.Millisecond):
			}
			if cancelEarly {
				cancel()
			}
			close(release)
			released = true
			select {
			case got := <-done:
				finished = true
				if cancelEarly {
					if len(got) != 0 {
						t.Fatalf("cancelled result length=%d", len(got))
					}
				} else {
					if len(got) != 10000 {
						t.Fatalf("result length=%d", len(got))
					}
					for i, v := range got {
						if i != v {
							t.Fatalf("element %d=%d", i, v)
						}
					}
				}
			case <-time.After(3 * time.Second):
				t.Fatal("ordered window failed to resume")
			}
		})
	}
	// Holes and a final partial batch must return their window credits too.
	got := stream.RepeatN(1, 193).Parallel(2).Ordered().Filter(func(int) bool { return false }).Append(9).ToSlice()
	if !slices.Equal(got, []int{9}) {
		t.Fatalf("holes/partial batch=%v", got)
	}
}

func TestShortCircuitPreservesFollowingStages(t *testing.T) {
	snapshot := stream.SliceOf(1, 2, 3).Execute()
	for range 2 {
		got := snapshot.Parallel(2).Ordered().Map(func(i int) int { return i }).Limit(1).Append(9).ToSlice()
		if !slices.Equal(got, []int{1, 9}) {
			t.Fatalf("snapshot/Append=%v", got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inner := stream.SliceOf[any](1).WithContext(ctx)
	if got := stream.SliceOf(0).FlatMap(func(int) stream.Streamer[any] { return inner }).ToSlice(); len(got) != 0 {
		t.Fatalf("FlatMap lost child's context: %v", got)
	}
	if got := stream.Concat(stream.SliceOf(1).WithContext(ctx), stream.SliceOf(2)).ToSlice(); !slices.Equal(got, []int{2}) {
		t.Fatalf("Concat lost child's context: %v", got)
	}
}

func TestCancellationAtSectionAndLimitBoundaries(t *testing.T) {
	for _, mode := range []string{"limit", "parallel", "ordered"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// Cancel after the source's check but before the next stage sees a value.
			source := stream.SliceOf(1, 2).Peek(func(int) { cancel() }).WithContext(ctx)
			if mode == "limit" {
				source = source.Limit(1)
			} else {
				source = source.Parallel(1)
				if mode == "ordered" {
					source = source.Ordered()
				}
				source = source.Map(func(i int) int { t.Error("callback ran after cancellation"); return i })
			}
			if got := source.ToSlice(); len(got) != 0 {
				t.Fatalf("cancelled result=%v", got)
			}
		})
	}
}

func TestCancelledDownstreamSection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	upstreamCtx, stopUpstream := context.WithCancel(context.Background())
	defer stopUpstream()
	source := stream.Repeat(1).WithContext(upstreamCtx).Parallel(1).Filter(func(int) bool { return false }).
		Skip(0).WithContext(ctx).Parallel(1).Map(func(i int) int { return i })
	done := make(chan []int, 1)
	go func() { done <- source.ToSlice() }()
	select {
	case got := <-done:
		if len(got) != 0 {
			t.Fatalf("cancelled result=%v", got)
		}
	case <-time.After(3 * time.Second):
		stopUpstream()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("upstream cleanup hung")
		}
		t.Fatal("pre-cancelled downstream section hung")
	}
}

func TestConcatForeignSource(t *testing.T) {
	source := func() stream.Streamer[int] {
		return stream.Concat(wrappedStreamer{Streamer: stream.SliceOf(1, 2)}, stream.SliceOf(3))
	}
	if got := source().ToSlice(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("Concat=%v", got)
	}
	if got := source().First(); got != 1 {
		t.Fatalf("First=%d", got)
	}
}
