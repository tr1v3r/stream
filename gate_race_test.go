//go:build race

package stream_test

// gateRaceEnabled reports whether the race detector is active. The benchmark
// gate is skipped under -race because detector instrumentation of channel
// operations inflates machinery overhead and would skew the ratio gates.
const gateRaceEnabled = true
