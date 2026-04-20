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

// Of creates a stream from an iter.Seq[T].
func Of[T any](seq iter.Seq[T], sizeHint int64) Streamer[T] {
	return newStreamer(seq, sizeHint)
}

// OfSeq2 creates a stream from an iter.Seq2[K, V], projecting to values only.
func OfSeq2[K, V any](seq iter.Seq2[K, V]) Streamer[V] {
	return newStreamer(func(yield func(V) bool) {
		for _, v := range seq {
			if !yield(v) {
				return
			}
		}
	}, -1)
}
