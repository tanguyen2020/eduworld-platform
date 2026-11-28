//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
)

// placementPayload is a student.placement_changed.v1 payload (schema specs/events/payloads/identity, required fields).
func placementPayload(student, tenant, school uuid.UUID, grade int, status string, version int) map[string]any {
	return map[string]any{"student_id": student.String(), "tenant_id": tenant.String(), "school_id": school.String(),
		"grade_level": grade, "status": status, "changed_fields": []string{"tenant_id", "school_id", "grade_level", "status"},
		"source": "platform_common", "sync_run_id": nil, "source_version": version}
}

// EVENT_DESIGN §4 (fix 2026-11-28): game.projection_identity subscribes student.placement_changed through NATS, so a
// student created or moved outside the SIS (no student.synced) reaches ref.students; the newest placement sets school
// and grade, an active placement keeps an account status (locked) and the SIS connection, inactive marks the student
// inactive (game commands then refuse it), an older version is ignored, and a placement of another tenant's student
// or a payload tenant that is not the envelope tenant is dead-lettered without moving the row.
func TestProjectionIdentityPlacement(t *testing.T) {
	ctx := context.Background()
	if !slices.Contains(messaging.ProjectionIdentityEvents, "student.placement_changed") {
		t.Fatal("game.projection_identity does not subscribe student.placement_changed")
	}
	e := setup(ctx, t, envOpts{})
	b := newBus(ctx, t)
	b.run(ctx, t, consumer.IdentityProjectionOptions(consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}},
		e.uow, e.app, b.js))
	name := messaging.ProjectionIdentityName
	tenant, school, school2, student, sis := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tn := b.publish(ctx, t, "tenant.created", &tenant, map[string]any{"tenant_id": tenant.String(), "code": "placed", "tenant_type": "school",
		"status": "active", "changed_fields": []string{}, "source_version": 1})
	e.processed(ctx, t, name, tn)

	first := b.publish(ctx, t, "student.placement_changed", &tenant, placementPayload(student, tenant, school, 3, "active", 1))
	e.processed(ctx, t, name, first)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND tenant_id = $2 AND school_id = $3
		AND grade_level = 3 AND status = 'active' AND deleted_at IS NULL`, student, tenant, school); n != 1 {
		t.Fatal("placement-only student not projected")
	}
	// SIS link and account status come from their own events; a newer active placement keeps both.
	link := b.publish(ctx, t, "student.sis_link_created", &tenant, map[string]any{"user_id": student.String(), "tenant_id": tenant.String(),
		"sis_connection_id": sis.String()})
	e.processed(ctx, t, name, link)
	lk := b.publish(ctx, t, "user.status_changed", nil, map[string]any{"user_id": student.String(), "from": "active", "to": "locked"})
	e.processed(ctx, t, name, lk)
	moved := b.publish(ctx, t, "student.placement_changed", &tenant, placementPayload(student, tenant, school2, 4, "active", 3))
	e.processed(ctx, t, name, moved)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND school_id = $2 AND grade_level = 4
		AND status = 'locked' AND sis_connection_id = $3`, student, school2, sis); n != 1 {
		t.Fatal("newer placement: school/grade not applied, or account status/SIS link lost")
	}
	old := b.publish(ctx, t, "student.placement_changed", &tenant, placementPayload(student, tenant, school, 3, "active", 2))
	e.processed(ctx, t, name, old)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND grade_level = 4`, student); n != 1 {
		t.Fatal("older placement applied")
	}
	off := b.publish(ctx, t, "student.placement_changed", &tenant, placementPayload(student, tenant, school2, 4, "inactive", 4))
	e.processed(ctx, t, name, off)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND status = 'inactive'`, student); n != 1 {
		t.Fatal("inactive placement not projected")
	}
	back := b.publish(ctx, t, "student.placement_changed", &tenant, placementPayload(student, tenant, school2, 4, "active", 5))
	e.processed(ctx, t, name, back)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND status = 'active'`, student); n != 1 {
		t.Fatal("placement not revived")
	}

	// Tenant isolation: tenant A claims the student of tenant "placed"; a payload tenant differs from the envelope tenant.
	claim := b.publish(ctx, t, "student.placement_changed", &e.tenantA, placementPayload(student, e.tenantA, uuid.New(), 5, "active", 9))
	e.deadLettered(ctx, t, name, claim)
	stranger := uuid.New()
	mismatch := b.publish(ctx, t, "student.placement_changed", &e.tenantA, placementPayload(stranger, tenant, school, 3, "active", 1))
	e.deadLettered(ctx, t, name, mismatch)
	bad := b.publish(ctx, t, "student.placement_changed", &tenant, placementPayload(uuid.New(), tenant, school, 10, "active", 1))
	e.deadLettered(ctx, t, name, bad)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND tenant_id = $2 AND school_id = $3`,
		student, tenant, school2); n != 1 {
		t.Fatal("student moved by an event of another tenant")
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1`, stranger); n != 0 {
		t.Fatal("mismatched payload projected")
	}
}
