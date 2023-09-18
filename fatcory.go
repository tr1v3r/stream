package stream

// SliceOf receive array and initlize streamer
func SliceOf[T any](array []T) Streamer[T] { return newStreamer[T](array...) }

// Of create a new stream with some data items
func Of[T any](items ...T) Streamer[T] { return newStreamer[T](items...) }
