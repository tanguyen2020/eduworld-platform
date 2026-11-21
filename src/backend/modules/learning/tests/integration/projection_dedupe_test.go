//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
)

// F020-AC-03 (TC-SVC-003): a redelivered event is processed exactly once thanks to infra.processed_events.
func TestProjectionRedeliveryProcessedOnce(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, false)
	p := startProjection(ctx, t, e)
	id := uuid.New()

	before := consumed("tenant.created", "duplicate")
	env := p.publish(ctx, t, "tenant.created", &id, tenantPayload(id, "th-x", "active", 1))
	p.processed(ctx, t, env)
	p.publishEnvelope(ctx, t, env) // same event_id delivered again
	waitFor(t, 15*time.Second, "duplicate ack", func() bool { return consumed("tenant.created", "duplicate") == before+1 })
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM infra.processed_events WHERE event_id = $1`, env.EventID); n != 1 {
		t.Fatalf("processed_events rows = %d", n)
	}
}

// F020-AC-04: an event whose tenant differs from the tenant of the projection row it references is refused,
// sent to DLQ_LEARNING and counted in eduworld_consumer_rejected_total. Edge case: an event arriving before
// tenant.created is retried (NAK with delay) and applied once the tenant exists.
func TestProjectionTenantMismatchDeadLettered(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, false)
	p := startProjection(ctx, t, e)
	a, b, school := uuid.New(), uuid.New(), uuid.New()
	p.processed(ctx, t, p.publish(ctx, t, "tenant.created", &a, tenantPayload(a, "t-a", "active", 1)))
	p.processed(ctx, t, p.publish(ctx, t, "tenant.created", &b, tenantPayload(b, "t-b", "active", 1)))
	p.processed(ctx, t, p.publish(ctx, t, "school.updated", &a, map[string]any{"school_id": school.String(), "tenant_id": a.String(),
		"status": "active", "name": "School A", "settings_subset": map[string]any{}, "changed_fields": []string{}, "source_version": 1}))

	rejected := func() float64 {
		return testutil.ToFloat64(observability.ConsumerRejected().WithLabelValues("learning.projection_identity", "tenant_mismatch"))
	}
	before := rejected()
	// A placement of tenant B in a school of tenant A (ADR-0095: student.placement_changed).
	cls := p.publish(ctx, t, "student.placement_changed", &b, map[string]any{"student_id": uuid.NewString(), "tenant_id": b.String(),
		"school_id": school.String(), "grade_level": 1, "status": "active", "source": "sis", "source_version": 1})
	p.deadLettered(ctx, t, cls)
	// The school itself re-published under tenant B.
	sch := p.publish(ctx, t, "school.updated", &b, map[string]any{"school_id": school.String(), "tenant_id": b.String(),
		"status": "active", "name": "Hijack", "settings_subset": map[string]any{}, "changed_fields": []string{}, "source_version": 9})
	p.deadLettered(ctx, t, sch)
	if got := rejected() - before; got != 2 {
		t.Fatalf("eduworld_consumer_rejected_total += %v, want 2", got)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students`); n != 0 {
		t.Fatalf("ref.students rows = %d", n)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.schools WHERE school_id = $1 AND tenant_id = $2 AND name = 'School A'`, school, a); n != 1 {
		t.Fatal("school of tenant A was changed")
	}

	// Edge case: school.updated of tenant C before tenant.created of C → retried, then applied.
	c, schoolC := uuid.New(), uuid.New()
	early := p.publish(ctx, t, "school.updated", &c, map[string]any{"school_id": schoolC.String(), "tenant_id": c.String(),
		"status": "active", "name": "School C", "settings_subset": map[string]any{}, "changed_fields": []string{}, "source_version": 1})
	time.Sleep(500 * time.Millisecond)
	p.processed(ctx, t, p.publish(ctx, t, "tenant.created", &c, tenantPayload(c, "t-c", "active", 1)))
	p.processed(ctx, t, early)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.schools WHERE school_id = $1 AND tenant_id = $2`, schoolC, c); n != 1 {
		t.Fatal("early school.updated was not applied after tenant.created")
	}
}
