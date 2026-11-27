//go:build race

package bench

// raceDetector is true when the test binary is built with -race: the detector instruments every memory access, so
// wall-clock timings no longer describe the product build (the budget is then enforced by make test-latency).
const raceDetector = true
