package stream

// SliceOf receive array and initlize streamer
func SliceOf[T any](slice []T) Streamer[T] { return newStreamer[T](newIterator[T](slice)) }

// Of create a new stream with some data items
func Of[T any](items ...T) Streamer[T] { return newStreamer[T](newIterator[T](items)) }
