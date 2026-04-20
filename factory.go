package stream

import (
	"iter"

	"github.com/tr1v3r/stream/types"
)

// SliceOf creates a stream from a slice or variadic elements.
func SliceOf[T any](slice ...T) Streamer[T] {
	return newStreamer(seqFromSlice(slice), int64(len(slice)))
}

// Of creates a stream from a supplier function.
func Of[T any](supply types.Supplier[T]) Streamer[T] {
	return newStreamer(func(yield func(T) bool) {
		for {
			v, ok := supply()
			if !ok || !yield(v) {
				return
			}
		}
	}, -1)
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

// Concat concatenates multiple streams.
func Concat[T any](dst Streamer[T], srcs ...Streamer[T]) Streamer[T] {
	seqs := []iter.Seq[T]{dst.Seq()}
	for _, src := range srcs {
		seqs = append(seqs, src.Seq())
	}
	return newStreamer(func(yield func(T) bool) {
		for _, seq := range seqs {
			for v := range seq {
				if !yield(v) {
					return
				}
			}
		}
	}, -1)
}

// FromSeq creates a stream from an iter.Seq[T].
func FromSeq[T any](seq iter.Seq[T], sizeHint int64) Streamer[T] {
	return newStreamer(seq, sizeHint)
}

// FromSeq2 creates a stream from an iter.Seq2[K, V], projecting to values only.
func FromSeq2[K, V any](seq iter.Seq2[K, V]) Streamer[V] {
	return newStreamer(func(yield func(V) bool) {
		for _, v := range seq {
			if !yield(v) {
				return
			}
		}
	}, -1)
}
