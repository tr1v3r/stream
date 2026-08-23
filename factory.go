package stream

import "iter"

// SliceOf creates a stream from a slice or variadic elements.
func SliceOf[T any](slice ...T) Streamer[T] {
	return newStreamer(seqFromSlice(slice), int64(len(slice)))
}

// Repeat creates an infinite stream of the same value.
func Repeat[T any](t T) Streamer[T] {
	return newStreamer(func(yield func(T) bool) {
		for {
			if !yield(t) {
				return
			}
		}
	}, -1)
}

// RepeatN creates a stream repeating t exactly n times.
func RepeatN[T any](t T, count int64) Streamer[T] {
	return Repeat(t).Limit(count)
}

// Concat concatenates srcs in order: all elements of the first stream, then
// the second, and so on. sizeHint is unknown (-1) after concatenation. An
// empty argument list yields an empty stream; short-circuiting the result
// stops pulling the remaining sources.
func Concat[T any](srcs ...Streamer[T]) Streamer[T] {
	return newStreamer(func(yield func(T) bool) {
		for _, src := range srcs {
			for v := range src.Seq() {
				if !yield(v) {
					return
				}
			}
		}
	}, -1)
}

// From wraps an existing iter.Seq[T] as a Streamer. sizeHint declares the
// known element count when non-negative (-1 for unknown or infinite); it
// feeds Count's O(1) fast path and capacity preallocation, so keep it honest.
// The seq is consumed once — streams are single-use. Infinite sequences are
// supported but must be bounded (Limit) or made cancellable (WithContext)
// before a non-short-circuiting terminal operation.
func From[T any](seq iter.Seq[T], sizeHint int64) Streamer[T] {
	return newStreamer(seq, sizeHint)
}

// From2 adapts an iter.Seq2[K, V] (e.g. maps.All) into a Streamer of the
// values only; keys are discarded. sizeHint is unknown (-1). Map iteration
// order is unspecified, so the resulting stream order varies.
func From2[K, V any](seq iter.Seq2[K, V]) Streamer[V] {
	return newStreamer(func(yield func(V) bool) {
		for _, v := range seq {
			if !yield(v) {
				return
			}
		}
	}, -1)
}
