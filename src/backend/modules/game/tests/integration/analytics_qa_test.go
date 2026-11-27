//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// hasValue: a series point with a number (a suppressed or empty point is null).
var hasValue = regexp.MustCompile(`"value":[0-9]`)

// F134-AC-08, AC-09, AC-10 (TC-ANL-001, TC-ANL-004): tenant A is flagged QA after its golden data was rolled up. Its new
// domain events are dropped with eduworld_analytics_events_dropped_total{reason="qa_tenant"}; every platform query leaves
// it out — data written before the flag included — even when tenant_ids names it (empty answer + excluded_tenant_ids);
// the Tenant Admin of the QA tenant gets 200 with an empty summary, another tenant keeps the isolation rules.
func TestAnalyticsQATenantExcluded(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, nil)
	g := e.seedGolden(ctx, t)
	e.closeGolden(ctx, t, k)
	lo := e.staffToken(e.platformStaff("liveops"))
	get := func(path string) resp { return e.do(t, http.MethodGet, path, lo, nil, nil) }
	metrics := "/admin/analytics/game/metrics?metric_codes=dau&from=" + g.d0 + "&to=" + g.d1
	named := metrics + "&tenant_ids=" + e.tenantA.String()
	before := get(named)
	if before.status != http.StatusOK || len(before.body["data"].([]any)) == 0 || before.body["excluded_tenant_ids"] != nil {
		t.Fatalf("before the flag: %d %s", before.status, before.raw)
	}
	if !hasValue.MatchString(before.raw) {
		t.Fatalf("golden data expected before the flag: %s", before.raw)
	}

	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.tenants SET is_qa = true WHERE tenant_id = $1`, e.tenantA); err != nil {
		t.Fatal(err)
	}
	// AC-08: a domain event of the QA tenant is not written and counted as dropped.
	dropped := testutil.ToFloat64(observability.AnalyticsDroppedCounter().WithLabelValues("qa_tenant"))
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tenant := e.tenantA
	payload, _ := json.Marshal(map[string]any{"student_id": s.id.String(), "game_session_id": uuid.NewString(), "platform": "android",
		"client_version": "1.3.0", "started_at": "2026-11-18T01:00:00Z"})
	ev := outbox.Envelope{EventID: uuid.New(), EventType: "game_session.started", OccurredAt: ictAt("2026-11-18", "09:00"), TenantID: &tenant, Payload: payload}
	if err := (consumer.Analytics{Ingest: k.ingest}).Handle(tenancy.WithScope(ctx, mustEventScope(t, e.tenantA)), ev); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE event_id = $1`, ev.EventID); n != 0 {
		t.Fatal("event of the QA tenant stored")
	}
	if got := testutil.ToFloat64(observability.AnalyticsDroppedCounter().WithLabelValues("qa_tenant")); got != dropped+1 {
		t.Fatalf("eduworld_analytics_events_dropped_total{qa_tenant} %v → %v", dropped, got)
	}

	// AC-09: named QA tenant → empty answer with excluded_tenant_ids; every tenant → QA tenant listed as excluded.
	r := get(named)
	if r.status != http.StatusOK || hasValue.MatchString(r.raw) || !strings.Contains(r.raw, `"excluded_tenant_ids":["`+e.tenantA.String()+`"]`) {
		t.Fatalf("named QA tenant: %d %s", r.status, r.raw)
	}
	all := get(metrics)
	if all.status != http.StatusOK || !strings.Contains(all.raw, e.tenantA.String()) {
		t.Fatalf("all tenants: %d %s", all.status, all.raw)
	}
	for _, p := range []string{
		"/admin/analytics/game/retention?cohort_from=" + g.d0 + "&cohort_to=" + g.d0 + "&tenant_ids=" + e.tenantA.String(),
		"/admin/analytics/game/funnels/onboarding?from=" + g.d0 + "&to=" + g.d0 + "&tenant_ids=" + e.tenantA.String(),
		"/admin/analytics/game/content-stats?kind=quest&from=" + g.d0 + "&to=" + g.d1 + "&tenant_ids=" + e.tenantA.String(),
		"/admin/analytics/game/performance?from=" + g.d0 + "&to=" + g.d0 + "&tenant_ids=" + e.tenantA.String(),
	} {
		r := get(p)
		if r.status != http.StatusOK || !strings.Contains(r.raw, `"excluded_tenant_ids":["`+e.tenantA.String()+`"]`) {
			t.Fatalf("%s: %d %s", p, r.status, r.raw)
		}
	}

	// AC-10: the Tenant Admin of the QA tenant → 200 with an empty summary; tenant B keeps its own rules.
	hA := map[string]string{"X-Tenant-Id": e.tenantA.String()}
	admin := e.staffToken(e.staff(e.tenantA, "tenant_admin"))
	sum := e.do(t, http.MethodGet, "/tenant/analytics/game/summary?from="+g.d0+"&to="+g.d1, admin, hA, nil)
	if sum.status != http.StatusOK || sum.body["active_students"] != nil || hasValue.MatchString(sum.raw) || sum.body["available_from"] != nil || len(sum.body["by_school"].([]any)) != 0 {
		t.Fatalf("QA tenant summary: %d %s", sum.status, sum.raw)
	}
	other := e.staffToken(e.staff(e.tenantB, "tenant_admin"))
	if r := e.do(t, http.MethodGet, "/tenant/analytics/game/summary?from="+g.d0+"&to="+g.d1, other, hA, nil); r.status != http.StatusForbidden {
		t.Fatalf("tenant B admin asking tenant A: %d %s", r.status, r.raw)
	}

	// Unflagged: the data kept before the flag is counted again (no backfill of the flagged period).
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.tenants SET is_qa = false WHERE tenant_id = $1`, e.tenantA); err != nil {
		t.Fatal(err)
	}
	if r := get(named); r.status != http.StatusOK || !hasValue.MatchString(r.raw) || r.body["excluded_tenant_ids"] != nil {
		t.Fatalf("after unflagging: %d %s", r.status, r.raw)
	}
}
