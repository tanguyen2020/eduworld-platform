//go:build !race

package bench

// raceDetector is false in a normal build: the latency budget is asserted (see race_on_test.go).
const raceDetector = false
