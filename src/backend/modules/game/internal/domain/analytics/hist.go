package analytics

import "sort"

// Hist counts values into the buckets of bounds: bucket i holds values ≤ bounds[i] (and > bounds[i-1]); the last bucket
// (index len(bounds)) holds values above the last bound.
func Hist(bounds []float64, values []float64) []int64 {
	out := make([]int64, len(bounds)+1)
	for _, v := range values {
		out[sort.SearchFloat64s(bounds, v)]++
	}
	return out
}

// MergeHist adds two histograms of the same bounds (nil-safe).
func MergeHist(a, b []int64) []int64 {
	if len(a) == 0 {
		return append([]int64(nil), b...)
	}
	if len(b) == 0 {
		return append([]int64(nil), a...)
	}
	n := max(len(a), len(b))
	out := make([]int64, n)
	for i := range n {
		if i < len(a) {
			out[i] += a[i]
		}
		if i < len(b) {
			out[i] += b[i]
		}
	}
	return out
}

// Percentile interpolates the q-quantile (0 < q ≤ 1) of a histogram linearly inside its bucket (DASHBOARD_DATA §3.2:
// error within one bucket); the open last bucket answers its lower bound. ok = false for an empty histogram.
func Percentile(bounds []float64, hist []int64, q float64) (float64, bool) {
	var total int64
	for _, c := range hist {
		total += c
	}
	if total == 0 || len(bounds) == 0 {
		return 0, false
	}
	rank := q * float64(total)
	var cum float64
	for i, c := range hist {
		if c == 0 {
			continue
		}
		if cum+float64(c) >= rank {
			lo := 0.0
			if i > 0 {
				lo = bounds[i-1]
			}
			if i >= len(bounds) {
				return bounds[len(bounds)-1], true
			}
			hi := bounds[i]
			return lo + (hi-lo)*(rank-cum)/float64(c), true
		}
		cum += float64(c)
	}
	return bounds[len(bounds)-1], true
}
