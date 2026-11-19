package lifecyclemetrics

import (
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := New(reg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(reg); err != nil { // registering twice is tolerated
		t.Fatal(err)
	}
	tenant := uuid.New()
	m.Cleanup(tenant, "cleaned", 3)
	m.LargeBatch(tenant)
	m.Finished("student_cleanup", "succeeded")
	m.Stuck(2)
	if v := testutil.ToFloat64(m.cleanup.WithLabelValues(tenant.String(), "cleaned")); v != 3 {
		t.Fatalf("cleanup = %v", v)
	}
	if testutil.ToFloat64(m.largeBatch.WithLabelValues(tenant.String())) != 1 || testutil.ToFloat64(m.sagaStuck) != 2 ||
		testutil.ToFloat64(m.sagaRuns.WithLabelValues("student_cleanup", "succeeded")) != 1 {
		t.Fatal("metrics")
	}
}
