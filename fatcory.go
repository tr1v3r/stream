package stream

import "github.com/tr1v3r/stream/types"

// SliceOf receive array and initlize streamer
func SliceOf[T any](slice ...T) Streamer[T] {
	return newStreamer[T](newIterator[T](slice))
}

// Of create a new stream with supply
func Of[T any](supply types.Supplier[T]) Streamer[T] {
	return newStreamer[T](&supplyIter[T]{supply: supply})
}

// Repeat create a new stream with unlimit repeated data items
func Repeat[T any](t T) Streamer[T] {
	return newStreamer[T](&supplyIter[T]{supply: func() T { return t }})
}

// RepeatN create a new stream with n times repeated data items
func RepeatN[T any](t T, count int64) Streamer[T] {
	return Repeat[T](t).Limit(count)
}

// Concat concat streamers
func Concat[T any](dst Streamer[T], srcs ...Streamer[T]) Streamer[T] {
	for _, src := range srcs {
		dst = dst.Append(src.ToSlice()...)
	}
	return dst
}
