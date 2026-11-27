package observability

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
)

// F132-AC-24 (ANTI_CHEAT §10): a corrected speed hack counts in eduworld_rt_speed_violations_total (no label), an
// out-of-bounds correction only in eduworld_hub_violations_total{reason}.
func TestSpeedViolationsMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	once = sync.Once{} // register on this test registry
	Register(reg)
	before := testutil.ToFloat64(speedViolations)
	Hub{}.Violation(hub.ReasonSpeed)
	Hub{}.Violation(hub.ReasonSpeed)
	Hub{}.Violation(hub.ReasonOutOfBounds)
	if got := testutil.ToFloat64(speedViolations) - before; got != 2 {
		t.Fatalf("speed violations += %v, want 2", got)
	}
	want := `
# HELP eduworld_rt_speed_violations_total Hub moves faster than the area max speed (× tolerance), corrected by the room host (speed hack signal).
# TYPE eduworld_rt_speed_violations_total counter
eduworld_rt_speed_violations_total ` + strconv.FormatFloat(before+2, 'g', -1, 64) + `
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "eduworld_rt_speed_violations_total"); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(violations.WithLabelValues(hub.ReasonOutOfBounds)); got < 1 {
		t.Fatalf("out_of_bounds not counted in eduworld_hub_violations_total: %v", got)
	}
}
