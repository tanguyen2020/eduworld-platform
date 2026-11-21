//go:build integration

package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	readrepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
)

// minDay is the earliest daily rollup bucket of a school or class (tenancyguard: test query on derived rollups, explicit tenant).
func (c *curEnv) minDay(ctx context.Context, t *testing.T, level string, tenant, id uuid.UUID) string {
	t.Helper()
	var d time.Time
	// tenancyguard:ok derived rollup (no deleted_at): explicit tenant_id of the test
	if err := c.platform.QueryRowContext(ctx, `SELECT min(bucket_date) FROM learning.rollup_`+level+`_daily WHERE tenant_id = $1 AND `+level+`_id = $2
		AND metric_version = 1`, tenant, id).Scan(&d); err != nil {
		t.Fatal(err)
	}
	return d.Format(time.DateOnly)
}

// laggingModel is a TEST DOUBLE of the dashboard catalog whose read replica lags by lag (no replica in the test cluster).
type laggingModel struct {
	readrepo.DashboardModel
	lag time.Duration
}

func (m laggingModel) ReplicaLag(context.Context) (time.Duration, error) { return m.lag, nil }

// memCache is a TEST DOUBLE of the dashboard cache that records the answers stored.
type memCache struct {
	mu   sync.Mutex
	sets int
}

func (m *memCache) Version(context.Context, uuid.UUID) (int64, error) { return 1, nil }
func (m *memCache) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (m *memCache) Set(context.Context, string, []byte, time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sets++
	return nil
}

// F043 T042 (F056-3): DashboardMeta carries available_from (earliest daily rollup of the scope of the answer: school, class,
// tenant summary) and replica_lag_warning (lag of the dashboard replica above dashboard_replica_lag_alert_seconds); an
// answer with the warning is not cached.
func TestDashboardMetaAvailableFromAndReplicaLag(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashReady(ctx, t)
	school := "/lms/dashboards/school?school_id=" + k.school.String() + "&" + dashRange
	r := must(t, c.do(t, http.MethodGet, school, adS, nil, nil), http.StatusOK, "school")
	if want := c.minDay(ctx, t, "school", c.tenantA, k.school); r.str("available_from") != want || r.str("replica_lag_warning") != "false" {
		t.Fatalf("school meta: available_from %q (want %q) lag %q", r.str("available_from"), want, r.str("replica_lag_warning"))
	}
	r = must(t, c.do(t, http.MethodGet, "/lms/classes/"+k.class3A.String()+"/dashboard?"+dashRange, adS, nil, nil), http.StatusOK, "class")
	if want := c.minDay(ctx, t, "class", c.tenantA, k.class3A); r.str("available_from") != want {
		t.Fatalf("class available_from %q, want %q", r.str("available_from"), want)
	}
	r = must(t, c.do(t, http.MethodGet, "/lms/schools/"+k.school.String()+"/analytics/series?metric=accuracy&"+dashRange, adS, nil, nil),
		http.StatusOK, "series")
	if r.str("available_from") == "" {
		t.Fatalf("series available_from: %s", r.raw)
	}

	// A replica 2 minutes behind (> 60 s): the numbers are still served, flagged, and the answer is not cached.
	mc := &memCache{}
	c.dashq.Model, c.dashq.Cache = laggingModel{lag: 2 * time.Minute}, mc
	r = must(t, c.do(t, http.MethodGet, school, adS, nil, nil), http.StatusOK, "lagging replica")
	if r.str("replica_lag_warning") != "true" || r.obj("widgets") == nil {
		t.Fatalf("lag warning: %s", r.raw)
	}
	if mc.sets != 0 {
		t.Fatalf("an answer with replica_lag_warning was cached (%d)", mc.sets)
	}
	// Under the threshold: no warning, cached as usual.
	c.dashq.Model = laggingModel{lag: 30 * time.Second}
	r = must(t, c.do(t, http.MethodGet, school, adS, nil, nil), http.StatusOK, "small lag")
	if r.str("replica_lag_warning") != "false" || mc.sets != 1 {
		t.Fatalf("small lag: warning %q, cached %d", r.str("replica_lag_warning"), mc.sets)
	}
	var _ read.DashboardReader = laggingModel{}
}
