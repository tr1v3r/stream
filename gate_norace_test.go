//go:build !race

package stream_test

// gateRaceEnabled reports whether the race detector is active (see
// gate_race_test.go for the race build).
const gateRaceEnabled = false
