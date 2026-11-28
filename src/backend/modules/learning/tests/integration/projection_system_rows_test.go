//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
)

// F010 Clarifications 2026-11-28: the events the identity migrations write for the rows they create without a use case
// (tenant common, its virtual school, the province catalog), relayed as the relay would, reach the projection of
// learning: ref.tenants and ref.schools hold `common`, ref.provinces the whole catalog; a re-delivery changes nothing.
func TestProjectionReceivesIdentitySystemRows(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, false)
	p := startProjection(ctx, t, e)
	evs := e.cl.IdentitySystemEvents(ctx, t)
	provinces := 0
	for _, env := range evs {
		if !slices.Contains(messaging.ProjectionIdentityEvents, env.EventType) {
			t.Fatalf("learning does not project %s", env.EventType)
		}
		if env.EventType == "province.updated" {
			provinces++
		}
		p.publishEnvelope(ctx, t, env)
	}
	for _, env := range evs {
		p.processed(ctx, t, env)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.tenants WHERE code = 'common' AND tenant_type = 'common'
		AND status = 'active'`); n != 1 {
		t.Fatal("ref.tenants has no common tenant")
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.schools s JOIN ref.tenants t ON t.tenant_id = s.tenant_id
		WHERE t.code = 'common' AND s.is_virtual AND s.name = 'EduWorld chung'`); n != 1 {
		t.Fatal("ref.schools has no virtual school of common")
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.provinces`); n != provinces || n == 0 {
		t.Fatalf("ref.provinces = %d, want %d", n, provinces)
	}
	// Re-delivery (the relay publishes again after a crash): deduplicated by processed_events, the rows do not change.
	before, dup := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.tenants`), consumed("tenant.created", "duplicate")
	p.publishEnvelope(ctx, t, evs[0])
	waitFor(t, 15*time.Second, "duplicate ack", func() bool { return consumed("tenant.created", "duplicate") == dup+1 })
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = 'learning.projection_identity'
		AND event_id = $1`, evs[0].EventID); n != 1 || count(ctx, t, e.migrator, `SELECT count(*) FROM ref.tenants`) != before {
		t.Fatal("re-delivery not idempotent")
	}
}
