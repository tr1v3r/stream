package stream

import (
	"context"
	"iter"

	"github.com/tr1v3r/stream/types"
)

// Pipeline is a concrete, lazily evaluated stream with generic methods.
// Use Of to create one or Wrap to adapt a legacy Streamer. Intermediate
// methods retain Pipeline, allowing type-changing operations to chain.
//
// The zero value is an empty pipeline. Pipelines are single-use, except for
// Execute snapshots. Copying or adapting a pipeline does not copy its source.
// Pipeline does not implement Streamer: Go does not support covariant method
// return types or generic interface methods. Stream explicitly bridges back.
type Pipeline[T any] struct {
	stream *streamer[T]
}

// Of creates a concrete pipeline from variadic values or a slice.
func Of[T any](values ...T) Pipeline[T] {
	return Pipeline[T]{newStreamer(seqFromSlice(values), int64(len(values)))}
}

// Wrap adapts a Streamer without consuming it. Native streams retain context,
// cardinality, and parallel options. Foreign implementations are consumed via
// Seq with unknown cardinality and serial downstream defaults; their own Seq
// remains responsible for their context and execution. s must be non-nil.
func Wrap[T any](s Streamer[T]) Pipeline[T] {
	if native, ok := s.(*streamer[T]); ok {
		return Pipeline[T]{native}
	}
	return Pipeline[T]{newStreamer(func(yield func(T) bool) {
		for v := range s.Seq() {
			if !yield(v) {
				return
			}
		}
	}, -1)}
}

func (p Pipeline[T]) native() *streamer[T] {
	if p.stream == nil {
		return newStreamer(seqFromSlice([]T(nil)), 0)
	}
	return p.stream
}

// Stream returns the legacy interface over the same pipeline without
// consuming it. It does not provide an independent copy of the source.
func (p Pipeline[T]) Stream() Streamer[T] { return p.native() }

// MapTo lazily transforms T to R, retaining the concrete result type.
// It closes the current fused section and runs the mapper serially. Context,
// cardinality, worker count, and Ordered preference carry to subsequent ops.
func (p Pipeline[T]) MapTo[R any](mapper types.Converter[T, R]) Pipeline[R] {
	s := p.native()
	result := Wrap(MapTo[T, R](s, mapper))
	result.stream.ordered = s.ordered
	return result
}

// FlatMapTo lazily concatenates typed child pipelines in encounter order.
// It closes the current section and runs the mapper and child traversal
// serially (children may have their own parallel sections). Cardinality becomes
// unknown; context, worker count, and Ordered preference carry forward.
// Cancellation propagates into native children as well as the outer source.
func (p Pipeline[T]) FlatMapTo[R any](mapper func(T) Pipeline[R]) Pipeline[R] {
	s := p.native()
	prev := s.ensureFlushed().seq
	return Pipeline[R]{&streamer[R]{ctx: s.ctx, sizeHint: -1,
		parallelSize: s.parallelSize, ordered: s.ordered,
		seq: func(parent context.Context, yield func(R) bool) {
			ctx, cancel := sectionContext(parent, s.ctx)
			defer cancel()
			for v := range prev.bind(ctx) {
				if ctx.Err() != nil {
					return
				}
				for item := range streamSeq(mapper(v).native()).bind(ctx) {
					if ctx.Err() != nil || !yield(item) {
						return
					}
				}
			}
		},
	}}
}

// DistinctBy removes duplicate comparable keys, keeping first occurrences.
// Deduplication is serial; downstream parallel options are preserved.
func (p Pipeline[T]) DistinctBy[K comparable](key func(T) K) Pipeline[T] {
	return Wrap(DistinctBy(p.native(), key))
}

// Fold reduces into a typed accumulator without boxing it into any.
// Empty input returns initial; cancellation returns the partial accumulator.
func (p Pipeline[T]) Fold[R any](initial R, accumulator types.Accumulator[T, R]) R {
	s := p.native()
	result := initial
	for v := range s.effectiveSeq() {
		if s.cancelled() {
			return result
		}
		result = accumulator(result, v)
	}
	return result
}

// CollectTo materializes the pipeline and calls collector once, including
// for empty or cancelled input. The collector's result retains type R.
// Like Collect it uses O(n) input memory; use Fold for incremental reduction.
func (p Pipeline[T]) CollectTo[R any](collector func(...T) R) R {
	return collector(p.ToSlice()...)
}

// WithContext sets the context for later stages; see Streamer.WithContext.
func (p Pipeline[T]) WithContext(ctx context.Context) Pipeline[T] {
	return Wrap(p.native().WithContext(ctx))
}

// Filter keeps matching elements, lazily; the resulting count is unknown.
func (p Pipeline[T]) Filter(judge types.Judge[T]) Pipeline[T] {
	return Wrap(p.native().Filter(judge))
}

// Map maps within T and preserves stateless parallel fusion. Use MapTo to change type.
func (p Pipeline[T]) Map(mapper types.Mapper[T]) Pipeline[T] {
	return Wrap(p.native().Map(mapper))
}

// Peek observes each element; callbacks in parallel sections must be concurrency-safe.
func (p Pipeline[T]) Peek(consumer types.Consumer[T]) Pipeline[T] {
	return Wrap(p.native().Peek(consumer))
}

// Distinct keeps the first element per string key; prefer DistinctBy for exact comparable keys.
func (p Pipeline[T]) Distinct() Pipeline[T] {
	return Wrap(p.native().Distinct())
}

// Sort lazily materializes and sorts with the given comparator.
func (p Pipeline[T]) Sort(comparator types.Comparator[T]) Pipeline[T] {
	return Wrap(p.native().Sort(comparator))
}

// ReverseSort lazily sorts in descending comparator order.
func (p Pipeline[T]) ReverseSort(comparator types.Comparator[T]) Pipeline[T] {
	return Wrap(p.native().ReverseSort(comparator))
}

// Reverse lazily materializes and reverses the encounter order.
func (p Pipeline[T]) Reverse() Pipeline[T] {
	return Wrap(p.native().Reverse())
}

// Limit keeps at most n elements and short-circuits; n <= 0 yields empty.
func (p Pipeline[T]) Limit(n int64) Pipeline[T] {
	return Wrap(p.native().Limit(n))
}

// Skip discards the first n elements; n <= 0 keeps everything.
func (p Pipeline[T]) Skip(n int64) Pipeline[T] {
	return Wrap(p.native().Skip(n))
}

// Pick selects absolute indices; see Streamer.Pick for bounds and materialization.
func (p Pipeline[T]) Pick(start, end, interval int) Pipeline[T] {
	return Wrap(p.native().Pick(start, end, interval))
}

// Append yields the current pipeline followed by values.
func (p Pipeline[T]) Append(values ...T) Pipeline[T] {
	return Wrap(p.native().Append(values...))
}

// Execute eagerly materializes a re-iterable snapshot, retaining context and parallel options.
func (p Pipeline[T]) Execute() Pipeline[T] {
	return Wrap(p.native().Execute())
}

// Parallel opens a fused stateless section; see Streamer.Parallel for section boundaries.
func (p Pipeline[T]) Parallel(n int) Pipeline[T] {
	return Wrap(p.native().Parallel(n))
}

// Ordered preserves encounter order for current and subsequent parallel sections.
func (p Pipeline[T]) Ordered() Pipeline[T] {
	return Wrap(p.native().Ordered())
}

// Convert retains the legacy any-valued conversion with concrete chaining.
//
// Deprecated: use MapTo to retain the result element type.
func (p Pipeline[T]) Convert(convert types.Converter[T, any]) Pipeline[any] {
	return p.MapTo(convert)
}

// FlatMap retains the legacy any-valued flattening with concrete chaining.
// Prefer FlatMapTo to retain the result element type. Like FlatMapTo, this
// concrete method preserves downstream parallel and ordering preferences.
func (p Pipeline[T]) FlatMap(mapper func(T) Streamer[any]) Pipeline[any] {
	return p.FlatMapTo(func(v T) Pipeline[any] { return Wrap(mapper(v)) })
}

// ToSlice collects into a new slice, with the same cancellation semantics as Streamer.ToSlice.
func (p Pipeline[T]) ToSlice() []T {
	return p.native().ToSlice()
}

// Seq exposes native range iteration with context cancellation.
func (p Pipeline[T]) Seq() iter.Seq[T] {
	return p.native().Seq()
}

// Count uses the exact size hint when known; otherwise it iterates. See Streamer.Count.
func (p Pipeline[T]) Count() int64 {
	return p.native().Count()
}

// Collect retains the legacy any-valued collector. Prefer CollectTo for a typed result.
func (p Pipeline[T]) Collect(collector types.Collector[T]) any {
	return p.native().Collect(collector)
}

// ForEach visits elements serially in encounter order, after upstream parallel work.
func (p Pipeline[T]) ForEach(consumer types.Consumer[T]) {
	p.native().ForEach(consumer)
}

// AllMatch short-circuits at the first nonmatching element.
func (p Pipeline[T]) AllMatch(judge types.Judge[T]) bool {
	return p.native().AllMatch(judge)
}

// NonMatch short-circuits at the first matching element.
func (p Pipeline[T]) NonMatch(judge types.Judge[T]) bool {
	return p.native().NonMatch(judge)
}

// AnyMatch reports whether any element matches, short-circuiting on success.
func (p Pipeline[T]) AnyMatch(judge types.Judge[T]) bool {
	return p.native().AnyMatch(judge)
}

// Reduce folds from T's zero value.
func (p Pipeline[T]) Reduce(accumulator types.BinaryOperator[T]) T {
	return p.native().Reduce(accumulator)
}

// ReduceFrom folds from an explicit initial value.
func (p Pipeline[T]) ReduceFrom(initial T, accumulator types.BinaryOperator[T]) T {
	return p.native().ReduceFrom(initial, accumulator)
}

// ReduceWith retains legacy any-valued reduction. Prefer Fold for a typed accumulator.
func (p Pipeline[T]) ReduceWith(initial any, accumulator types.Accumulator[T, any]) any {
	return p.native().ReduceWith(initial, accumulator)
}

// ReduceBy retains legacy size-aware initialization; see Streamer.ReduceBy.
func (p Pipeline[T]) ReduceBy(builder func(int) any, accumulator types.Accumulator[T, any]) any {
	return p.native().ReduceBy(builder, accumulator)
}

// First returns the first element, or T's zero value when empty.
func (p Pipeline[T]) First() T {
	return p.native().First()
}

// Take samples uniformly using O(1) memory, or returns T's zero value when empty.
func (p Pipeline[T]) Take() T {
	return p.native().Take()
}

// Any is an alias for Take.
func (p Pipeline[T]) Any() T {
	return p.native().Any()
}

// Last returns the final element, or T's zero value when empty.
func (p Pipeline[T]) Last() T {
	return p.native().Last()
}
