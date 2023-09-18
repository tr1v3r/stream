package stream

// SliceOf receive array and initlize streamer
func SliceOf[T, R any](array []T) Streamer[T, R] { return newStreamer[T, R](array...) }
