package stream

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"slices"
	"sync"

	"github.com/tr1v3r/stream/types"
)

var (
	_ Streamer[any]     = newStreamer[any](nil, 0)
	_ Streamer[float64] = newStreamer[float64](nil, 0)
)

func materialize[T any](seq iter.Seq[T]) []T {
	result := make([]T, 0, 64)
	for v := range seq {
		result = append(result, v)
	}
	return result
}

func seqFromSlice[T any](s []T) iter.Seq[T] { return slices.Values(s) }

type streamer[T any] struct {
	ctx          context.Context
	seq          iter.Seq[T]
	sizeHint     int64 // known size, or -1 for unknown
	parallelSize int   // 0=sync, >0=parallel workers

	// fused accumulates the stateless stages of the currently open parallel
	// section (proposal docs/proposals/parallel-v2.md). It is nil in serial
	// mode and until a parallel section opens; flushFused turns it into the
	// section's single-pool execution boundary.
	fused func(T) (T, bool)

	// ordered marks the current section as order-preserving: elements are
	// index-tagged and re-sequenced at the consumer (proposal 3.3).
	ordered bool
}

func newStreamer[T any](seq iter.Seq[T], sizeHint int64) *streamer[T] {
	return &streamer[T]{ctx: context.Background(), seq: seq, sizeHint: sizeHint}
}

func (s *streamer[T]) cancelled() bool { return s.ctx.Err() != nil }

// WithContext implements Streamer.WithContext; the context is consulted by later operations on the returned stream.
func (s streamer[T]) WithContext(ctx context.Context) Streamer[T] {
	s.ctx = ctx
	return &s
}

// thenFused returns the fused-stage composition of s with next appended.
// Filters compose as short-circuit drops; Map/Peek as transforms.
func (s *streamer[T]) thenFused(next func(T) (T, bool)) func(T) (T, bool) {
	if s.fused == nil {
		return next
	}
	prev := s.fused
	return func(t T) (T, bool) {
		v, ok := prev(t)
		if !ok {
			var zero T
			return zero, false
		}
		return next(v)
	}
}

// parallelBatchSize is the feeder's batching granularity: amortizes channel
// and scheduling cost across the batch (proposal 3.2, gate A1).
const parallelBatchSize = 64

// fusedFeeder pulls upstream into the feeder goroutine and emits
// parallelBatchSize batches on in, stopping on ctx cancellation.
func fusedFeeder[T any](ctx context.Context, prev iter.Seq[T], in chan<- []T, batch int) {
	defer close(in)
	buf := make([]T, 0, batch)
	flush := func() {
		if len(buf) > 0 {
			in <- buf
			buf = make([]T, 0, batch)
		}
	}
	defer flush()
	for v := range prev {
		if ctx.Err() != nil {
			return
		}
		buf = append(buf, v)
		if len(buf) == batch {
			flush()
		}
	}
}

// fusedWorkers runs n workers applying stage to whole batches from in onto
// out. After cancellation workers keep draining in (discarding) so the
// feeder never blocks forever.
func fusedWorkers[T any](ctx context.Context, stage func(T) (T, bool), in <-chan []T, out chan<- []T, n int, wg *sync.WaitGroup) {
	for range n {
		wg.Go(func() {
			for items := range in {
				if ctx.Err() != nil {
					continue // cancelled: discard, keep draining
				}
				res := make([]T, 0, len(items))
				for _, v := range items {
					if r, ok := stage(v); ok {
						res = append(res, r)
					}
				}
				if len(res) > 0 {
					out <- res
				}
			}
		})
	}
}

// indexedValue pairs an element with its input index for ordered sections.
// A hole marks an index dropped by a fused Filter: it carries no value but
// must still advance the consumer's sequence position.
type indexedValue[T any] struct {
	idx  int
	val  T
	hole bool
}

// orderedFeeder pulls upstream, stamps input indices, and emits batches of
// indexed elements (proposal 3.3).
func orderedFeeder[T any](ctx context.Context, prev iter.Seq[T], in chan<- []indexedValue[T], batch int) {
	defer close(in)
	buf := make([]indexedValue[T], 0, batch)
	flush := func() {
		if len(buf) > 0 {
			in <- buf
			buf = make([]indexedValue[T], 0, batch)
		}
	}
	defer flush()
	i := 0
	for v := range prev {
		if ctx.Err() != nil {
			return
		}
		buf = append(buf, indexedValue[T]{idx: i, val: v})
		i++
		if len(buf) == batch {
			flush()
		}
	}
}

// orderedWorkers runs n workers applying stage to indexed batches; the
// index travels with the result for re-sequencing. Indices dropped by a
// fused Filter are forwarded as holes so the consumer can advance past them.
func orderedWorkers[T any](ctx context.Context, stage func(T) (T, bool), in <-chan []indexedValue[T], out chan<- []indexedValue[T], n int, wg *sync.WaitGroup) {
	for range n {
		wg.Go(func() {
			for items := range in {
				if ctx.Err() != nil {
					continue // cancelled: discard, keep draining
				}
				res := make([]indexedValue[T], 0, len(items))
				for _, iv := range items {
					if r, ok := stage(iv.val); ok {
						res = append(res, indexedValue[T]{idx: iv.idx, val: r})
					} else {
						res = append(res, indexedValue[T]{idx: iv.idx, hole: true})
					}
				}
				out <- res
			}
		})
	}
}

// orderedYield re-sequences batch-indexed elements into encounter order.
// Batches arrive out of order but are internally contiguous (the feeder
// stamps indices sequentially), so re-sequencing works per batch: aligned
// batches yield straight through with zero per-element bookkeeping; only
// the gap-filling partial batches at stream end need element-level holes.
// pending holds at most the in-flight window of batches.
func orderedYield[T any](ctx context.Context, out <-chan []indexedValue[T], yield func(T) bool) bool {
	next := 0                              // next absolute index to emit
	pending := map[int][]indexedValue[T]{} // batch start idx -> batch
	nextBatchStart := 0
	emit := func(v T) bool {
		if ctx.Err() != nil || !yield(v) {
			return false
		}
		return true
	}
	for items := range out {
		pending[items[0].idx] = items
		for {
			b, ok := pending[nextBatchStart]
			if !ok {
				break
			}
			delete(pending, nextBatchStart)
			for _, iv := range b {
				if !iv.hole && !emit(iv.val) {
					return false
				}
			}
			nextBatchStart += len(b)
			_ = next
		}
	}
	return true
}

// flushFused materializes the open parallel section as a single worker-pool
// stage: one feeder, n workers applying the fused function, one consumer —
// reusing the leak-free cancel/drain pattern. Elements flow in
// parallelBatchSize batches to amortize channel machinery; the consumer
// un-batches before yielding. Ordered sections stamp input indices and
// re-sequence at the consumer (proposal 3.3). After the flush the streamer
// keeps parallelSize so a following Parallel call or stateless op can open a
// new section.
func (s *streamer[T]) flushFused() *streamer[T] {
	stage := s.fused
	prev := s.seq
	n := s.parallelSize
	ordered := s.ordered
	next := &streamer[T]{ctx: s.ctx, sizeHint: s.sizeHint, parallelSize: s.parallelSize, ordered: s.ordered}
	if ordered {
		next.seq = s.orderedSeq(stage, prev, n)
	} else {
		next.seq = s.unorderedSeq(stage, prev, n)
	}
	return next
}

func (s *streamer[T]) unorderedSeq(stage func(T) (T, bool), prev iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()

		in := make(chan []T, n)
		out := make(chan []T, n)

		go fusedFeeder(ctx, prev, in, parallelBatchSize)

		var wg sync.WaitGroup
		fusedWorkers(ctx, stage, in, out, n, &wg)
		go func() {
			wg.Wait()
			close(out)
		}()

		// On any exit (natural, short-circuit, cancellation) stop the
		// pipeline and drain out until it is closed, so workers blocked
		// on send are released.
		defer func() {
			cancel()
			for range out {
			}
		}()

		for items := range out {
			for _, v := range items {
				if ctx.Err() != nil || !yield(v) {
					return
				}
			}
		}
	}
}

func (s *streamer[T]) orderedSeq(stage func(T) (T, bool), prev iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()

		in := make(chan []indexedValue[T], n)
		out := make(chan []indexedValue[T], n)

		go orderedFeeder(ctx, prev, in, parallelBatchSize)

		var wg sync.WaitGroup
		orderedWorkers(ctx, stage, in, out, n, &wg)
		go func() {
			wg.Wait()
			close(out)
		}()

		defer func() {
			cancel()
			for range out {
			}
		}()

		orderedYield(ctx, out, yield)
	}
}

// ensureFlushed closes any open parallel section before a stage that cannot
// fuse (stateful ops, type changes, terminals, section re-open) executes.
func (s *streamer[T]) ensureFlushed() *streamer[T] {
	if s.fused == nil {
		return s
	}
	return s.flushFused()
}

// effectiveSeq is the sequence terminal operations iterate: any open
// parallel section is flushed into its single-pool stage first.
func (s *streamer[T]) effectiveSeq() iter.Seq[T] {
	return s.ensureFlushed().seq
}

func (s *streamer[T]) wrap(newSeq iter.Seq[T], newHint int64) *streamer[T] {
	return &streamer[T]{ctx: s.ctx, seq: newSeq, sizeHint: newHint, parallelSize: s.parallelSize, ordered: s.ordered}
}

// Filter implements Streamer.Filter. In a parallel section the judge fuses
// into the section's single stage (one pool per section); order is not
// preserved.
func (s *streamer[T]) Filter(judge types.Judge[T]) Streamer[T] {
	if s.parallelSize > 0 {
		next := &streamer[T]{ctx: s.ctx, seq: s.seq, sizeHint: -1, parallelSize: s.parallelSize, ordered: s.ordered}
		next.fused = s.thenFused(func(t T) (T, bool) {
			if judge(t) {
				return t, true
			}
			var zero T
			return zero, false
		})
		return next
	}
	prev := s.seq
	return s.wrap(func(yield func(T) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			if judge(v) && !yield(v) {
				return
			}
		}
	}, -1)
}

// Map implements Streamer.Map. In a parallel section m fuses into the
// section's single stage; order is not preserved.
func (s *streamer[T]) Map(m types.Mapper[T]) Streamer[T] {
	if s.parallelSize > 0 {
		next := &streamer[T]{ctx: s.ctx, seq: s.seq, sizeHint: s.sizeHint, parallelSize: s.parallelSize, ordered: s.ordered}
		next.fused = s.thenFused(func(t T) (T, bool) { return m(t), true })
		return next
	}
	prev := s.seq
	return s.wrap(func(yield func(T) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			if !yield(m(v)) {
				return
			}
		}
	}, s.sizeHint)
}

// Convert implements the deprecated Streamer.Convert; it delegates to MapTo.
func (s *streamer[T]) Convert(convert types.Converter[T, any]) Streamer[any] {
	return MapTo(s, convert)
}

// MapTo transforms each element of s from T to R, preserving ctx, sizeHint
// and parallelSize. Unlike Convert it keeps the result type, so no
// Streamer[any] round-trip with type assertions is needed:
//
//	names := stream.MapTo(stream.SliceOf(1, 2, 3), func(n int) string {
//	    return fmt.Sprintf("#%d", n)
//	}).ToSlice()
//
// As a package function it interrupts method chaining at the type-changing
// point; Convert chains fluently but erases the element type. Prefer MapTo,
// especially for head-of-pipeline type changes where chaining resumes right
// below it; Convert stays acceptable for mid-chain changes despite being
// deprecated.
func MapTo[T, R any](s Streamer[T], m types.Converter[T, R]) Streamer[R] {
	st, ok := s.(*streamer[T])
	if !ok {
		return From(func(yield func(R) bool) {
			for v := range s.Seq() {
				if !yield(m(v)) {
					return
				}
			}
		}, -1)
	}
	prev := st.ensureFlushed().seq // type change closes the parallel section
	return &streamer[R]{ctx: st.ctx, seq: func(yield func(R) bool) {
		for v := range prev {
			if st.cancelled() {
				return
			}
			if !yield(m(v)) {
				return
			}
		}
	}, sizeHint: st.sizeHint, parallelSize: st.parallelSize}
}

// Peek implements Streamer.Peek. In a parallel section consumer fuses into
// the section's single stage; order is not preserved.
func (s *streamer[T]) Peek(consumer types.Consumer[T]) Streamer[T] {
	if s.parallelSize > 0 {
		next := &streamer[T]{ctx: s.ctx, seq: s.seq, sizeHint: s.sizeHint, parallelSize: s.parallelSize, ordered: s.ordered}
		next.fused = s.thenFused(func(t T) (T, bool) {
			consumer(t)
			return t, true
		})
		return next
	}
	prev := s.seq
	return s.wrap(func(yield func(T) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			consumer(v)
			if !yield(v) {
				return
			}
		}
	}, s.sizeHint)
}

// FlatMap implements Streamer.FlatMap; sub-streams are drained in order, sequentially. A parallel section closes first.
func (s *streamer[T]) FlatMap(f func(T) Streamer[any]) Streamer[any] {
	prev := s.ensureFlushed().seq
	return &streamer[any]{ctx: s.ctx, seq: func(yield func(any) bool) {
		for v := range prev {
			if s.cancelled() {
				return
			}
			for item := range f(v).Seq() {
				if !yield(item) {
					return
				}
			}
		}
	}, sizeHint: -1}
}

// Distinct removes duplicate elements, keeping first occurrences. Keys come
// from fmt.Sprint (or types.Unique), so int 1 and string "1" collide — prefer
// DistinctBy for exact keys.
func (s *streamer[T]) Distinct() Streamer[T] {
	return DistinctBy(s, func(t T) string {
		if keyer, ok := any(t).(types.Unique); ok {
			return keyer.Key()
		}
		return fmt.Sprint(t)
	})
}

// DistinctBy removes elements whose key, produced by key, has already been
// seen, keeping first occurrences. Keys use Go map equality (K comparable),
// avoiding the string-coercion collisions of Distinct. Like Distinct it runs
// serially — the shared key map is not concurrency-safe — while preserving
// parallelSize for downstream operations.
func DistinctBy[T any, K comparable](s Streamer[T], key func(T) K) Streamer[T] {
	seen := make(map[K]struct{})
	judge := func(t T) bool {
		k := key(t)
		if _, dup := seen[k]; dup {
			return false
		}
		seen[k] = struct{}{}
		return true
	}
	st, ok := s.(*streamer[T])
	if !ok {
		return s.Filter(judge) // foreign Streamer implementations
	}
	prev := st.ensureFlushed().seq
	return st.wrap(func(yield func(T) bool) {
		for v := range prev {
			if st.cancelled() {
				return
			}
			if judge(v) && !yield(v) {
				return
			}
		}
	}, -1)
}

// Sort implements Streamer.Sort via slices.SortFunc; materializes the stage when iterated. A parallel section closes first.
func (s *streamer[T]) Sort(comparator types.Comparator[T]) Streamer[T] {
	prev := s.ensureFlushed().seq
	hint := s.sizeHint
	return s.wrap(func(yield func(T) bool) {
		data := materialize(prev)
		slices.SortFunc(data, comparator)
		for _, v := range data {
			if !yield(v) {
				return
			}
		}
	}, hint)
}

// ReverseSort implements Streamer.ReverseSort via slices.SortFunc with inverted comparator. A parallel section closes first.
func (s *streamer[T]) ReverseSort(comparator types.Comparator[T]) Streamer[T] {
	prev := s.ensureFlushed().seq
	hint := s.sizeHint
	return s.wrap(func(yield func(T) bool) {
		data := materialize(prev)
		slices.SortFunc(data, func(a, b T) int { return comparator(b, a) })
		for _, v := range data {
			if !yield(v) {
				return
			}
		}
	}, hint)
}

// Reverse implements Streamer.Reverse by materializing and reversing in place. A parallel section closes first.
func (s *streamer[T]) Reverse() Streamer[T] {
	prev := s.ensureFlushed().seq
	hint := s.sizeHint
	return s.wrap(func(yield func(T) bool) {
		data := materialize(prev)
		slices.Reverse(data)
		for _, v := range data {
			if !yield(v) {
				return
			}
		}
	}, hint)
}

// Limit implements Streamer.Limit; stops pulling upstream once the count is reached. A parallel section closes first.
func (s *streamer[T]) Limit(l int64) Streamer[T] {
	prev := s.ensureFlushed().seq
	newHint := l
	if s.sizeHint >= 0 && s.sizeHint < l {
		newHint = s.sizeHint
	}
	return s.wrap(func(yield func(T) bool) {
		count := int64(0)
		for v := range prev {
			if s.cancelled() || count >= l {
				return
			}
			count++
			if !yield(v) {
				return
			}
		}
	}, newHint)
}

// Skip implements Streamer.Skip; discards the first n elements before yielding. A parallel section closes first.
func (s *streamer[T]) Skip(n int64) Streamer[T] {
	prev := s.ensureFlushed().seq
	newHint := int64(-1)
	if s.sizeHint >= 0 {
		newHint = max(s.sizeHint-n, 0)
	}
	return s.wrap(func(yield func(T) bool) {
		skipped := int64(0)
		for v := range prev {
			if s.cancelled() {
				return
			}
			if skipped < n {
				skipped++
				continue
			}
			if !yield(v) {
				return
			}
		}
	}, newHint)
}

// Pick implements Streamer.Pick over absolute indices with interval stepping. A parallel section closes first.
func (s *streamer[T]) Pick(start, end, interval int) Streamer[T] {
	prev := s.ensureFlushed().seq
	return s.wrap(func(yield func(T) bool) {
		if start < 0 || interval <= 0 {
			return
		}
		// Resolve negative end (last index): use sizeHint if known,
		// otherwise materialize to find out
		if end < 0 {
			if s.sizeHint >= 0 {
				end = int(s.sizeHint) - 1
			} else {
				data := materialize(prev)
				for i := start; i < len(data); i += interval {
					if !yield(data[i]) {
						return
					}
				}
				return
			}
		}
		idx := 0
		for v := range prev {
			if idx > end {
				return
			}
			if idx >= start && (idx-start)%interval == 0 {
				if !yield(v) {
					return
				}
			}
			idx++
		}
	}, -1)
}

// Append implements Streamer.Append; yields upstream first, then data. A parallel section closes first.
func (s *streamer[T]) Append(data ...T) Streamer[T] {
	prev := s.ensureFlushed().seq
	newHint := s.sizeHint + int64(len(data))
	if s.sizeHint < 0 {
		newHint = -1
	}
	return s.wrap(func(yield func(T) bool) {
		for v := range prev {
			if !yield(v) {
				return
			}
		}
		for _, v := range data {
			if !yield(v) {
				return
			}
		}
	}, newHint)
}

// Execute implements Streamer.Execute; ctx and parallelSize carry over to the snapshot.
func (s *streamer[T]) Execute() Streamer[T] {
	data := materialize(s.ensureFlushed().seq)
	// keep ctx and parallelSize so downstream ops behave as before the snapshot
	return &streamer[T]{ctx: s.ctx, seq: seqFromSlice(data), sizeHint: int64(len(data)), parallelSize: s.parallelSize, ordered: s.ordered}
}

// Parallel implements Streamer.Parallel: n <= 0 is a no-op returning the
// same stream; otherwise a mid-chain call closes the current parallel
// section (if any) and opens a new one with n workers (unordered; follow
// with Ordered() to preserve encounter order). Consecutive stateless ops
// inside the section fuse into one pool (proposal
// docs/proposals/parallel-v2.md).
func (s streamer[T]) Parallel(n int) Streamer[T] {
	if n <= 0 {
		return &s
	}
	s.ensureFlushed() // close current section if open (value receiver is addressable)
	s.parallelSize = n
	s.ordered = false // new section starts unordered; Ordered() opts back in
	return &s
}

// Ordered implements Streamer.Ordered: the current parallel section (opened
// by the most recent Parallel call, or the next one if called before it)
// preserves encounter order via index-tagged re-sequencing at the consumer.
// No-op in serial mode.
func (s streamer[T]) Ordered() Streamer[T] {
	s.ordered = true
	return &s
}

// ToSlice implements Streamer.ToSlice.
func (s *streamer[T]) ToSlice() []T {
	return materialize(s.effectiveSeq())
}

// Collect implements Streamer.Collect by draining into the caller's collector.
func (s *streamer[T]) Collect(to types.Collector[T]) any {
	return to(s.ToSlice()...)
}

// ForEach implements Streamer.ForEach.
func (s *streamer[T]) ForEach(consumer types.Consumer[T]) {
	for v := range s.effectiveSeq() {
		if s.cancelled() {
			return
		}
		consumer(v)
	}
}

// AllMatch implements Streamer.AllMatch; short-circuits on the first failing element.
func (s *streamer[T]) AllMatch(judge types.Judge[T]) bool {
	for v := range s.effectiveSeq() {
		if s.cancelled() || !judge(v) {
			return false
		}
	}
	return true
}

// NonMatch implements Streamer.NonMatch; short-circuits on the first matching element.
func (s *streamer[T]) NonMatch(judge types.Judge[T]) bool {
	for v := range s.effectiveSeq() {
		if s.cancelled() || judge(v) {
			return false
		}
	}
	return true
}

// AnyMatch implements Streamer.AnyMatch; short-circuits on the first matching element.
func (s *streamer[T]) AnyMatch(judge types.Judge[T]) bool {
	for v := range s.effectiveSeq() {
		if s.cancelled() {
			return false
		}
		if judge(v) {
			return true
		}
	}
	return false
}

// Reduce implements Streamer.Reduce from T's zero value.
func (s *streamer[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	var result T
	for v := range s.effectiveSeq() {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

// ReduceFrom implements Streamer.ReduceFrom from an explicit initial value.
func (s *streamer[T]) ReduceFrom(initValue T, accumulator types.BinaryOperator[T]) T {
	result := initValue
	for v := range s.effectiveSeq() {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

// ReduceWith implements Streamer.ReduceWith with an any-typed accumulator.
func (s *streamer[T]) ReduceWith(initValue any, accumulator types.Accumulator[T, any]) any {
	result := initValue
	for v := range s.effectiveSeq() {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

// ReduceBy implements Streamer.ReduceBy; the initial value is built from sizeHint (may be negative = unknown).
func (s *streamer[T]) ReduceBy(initValueBuilder func(sizeMayNegative int) any, accumulator types.Accumulator[T, any]) any {
	result := initValueBuilder(int(s.sizeHint))
	for v := range s.effectiveSeq() {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

// First implements Streamer.First; stops the pipeline after one element.
func (s *streamer[T]) First() T {
	var zero T
	for v := range s.effectiveSeq() {
		return v
	}
	return zero
}

// Take returns a uniformly-sampled element without materializing the whole
// stream: reservoir sampling keeps O(1) memory (a single slot here) and works
// on infinite streams, terminating as soon as the source is exhausted or
// cancelled. On an empty stream it returns the zero value of T.
func (s *streamer[T]) Take() T {
	var pick T
	seen := int64(0)
	for v := range s.effectiveSeq() {
		if s.cancelled() {
			return pick
		}
		seen++
		// first element seeds the reservoir, later ones replace it with
		// probability 1/seen — every element ends up equally likely
		if seen == 1 || rand.Int64N(seen) == 0 {
			pick = v
		}
	}
	return pick
}

// Any implements Streamer.Any as an alias for Take.
func (s *streamer[T]) Any() T { return s.Take() }

// Last implements Streamer.Last by consuming the whole stream.
func (s *streamer[T]) Last() T {
	var result T
	for v := range s.effectiveSeq() {
		result = v
	}
	return result
}

// Count implements Streamer.Count; O(1) when sizeHint is known.
func (s *streamer[T]) Count() int64 {
	if s.sizeHint >= 0 {
		return s.sizeHint
	}
	var count int64
	for range s.effectiveSeq() {
		count++
	}
	return count
}

// Seq implements Streamer.Seq.
func (s *streamer[T]) Seq() iter.Seq[T] {
	return s.effectiveSeq()
}
