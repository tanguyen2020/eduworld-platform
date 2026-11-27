//go:build integration

package integration

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// metricSum adds the series of a counter vector whose labels match want (MONITORING §3 assertions on process-global
// counters: callers compare a before/after delta).
func metricSum(t *testing.T, c prometheus.Collector, want map[string]string) float64 {
	t.Helper()
	reg := prometheus.NewRegistry()
	if err := reg.Register(c); err != nil {
		t.Fatal(err)
	}
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	sum := 0.0
	for _, mf := range mfs {
	next:
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if v, ok := want[l.GetName()]; ok && v != l.GetValue() {
					continue next
				}
			}
			switch {
			case m.GetCounter() != nil:
				sum += m.GetCounter().GetValue()
			case m.GetHistogram() != nil:
				sum += float64(m.GetHistogram().GetSampleCount())
			}
		}
	}
	return sum
}

// metricLabelValues lists the values of label among the series of c matching want.
func metricLabelValues(t *testing.T, c prometheus.Collector, label string, want map[string]string) []string {
	t.Helper()
	reg := prometheus.NewRegistry()
	if err := reg.Register(c); err != nil {
		t.Fatal(err)
	}
	mfs, _ := reg.Gather()
	var out []string
	for _, mf := range mfs {
	next:
		for _, m := range mf.GetMetric() {
			val := ""
			for _, l := range m.GetLabel() {
				if v, ok := want[l.GetName()]; ok && v != l.GetValue() {
					continue next
				}
				if l.GetName() == label {
					val = l.GetValue()
				}
			}
			out = append(out, val)
		}
	}
	return out
}

// histogramSum adds the sample sums of the histogram series of c matching want (before/after deltas).
func histogramSum(t *testing.T, c prometheus.Collector, want map[string]string) float64 {
	t.Helper()
	reg := prometheus.NewRegistry()
	if err := reg.Register(c); err != nil {
		t.Fatal(err)
	}
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	sum := 0.0
	for _, mf := range mfs {
	next:
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if v, ok := want[l.GetName()]; ok && v != l.GetValue() {
					continue next
				}
			}
			if h := m.GetHistogram(); h != nil {
				sum += h.GetSampleSum()
			}
		}
	}
	return sum
}
