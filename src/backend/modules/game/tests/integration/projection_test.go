//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
)

// T008 (TC-SVC-008): the consumer game.projection_identity keeps ref.tenants, ref.schools (curfew window) and
// ref.students: newer source_version only, user.status_changed and cleanup/restore applied, a payload referencing
// another tenant's row refused (DLQ), and a student whose tenant arrives late is projected once the tenant is
// (catch-up by retry).
func TestProjectionIdentity(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	b := newBus(ctx, t)
	b.run(ctx, t, consumer.IdentityProjectionOptions(consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}},
		e.uow, e.app, b.js))
	name := messaging.ProjectionIdentityName
	late := uuid.New()
	school, student := uuid.New(), uuid.New()
	// The student arrives before its tenant: retried until the tenant is projected.
	st := b.publish(ctx, t, "student.synced", &late, map[string]any{"student_id": student.String(), "school_id": school.String(),
		"action": "create", "sync_run_id": uuid.NewString(), "grade_level": 3, "status": "active", "source_version": 2})
	tn := b.publish(ctx, t, "tenant.created", &late, map[string]any{"tenant_id": late.String(), "code": "late", "tenant_type": "school",
		"status": "active", "changed_fields": []string{}, "source_version": 1})
	e.processed(ctx, t, name, tn)
	e.processed(ctx, t, name, st)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND tenant_id = $2 AND status = 'active'`,
		student, late); n != 1 {
		t.Fatal("student not projected after its tenant")
	}
	// Older version ignored.
	old := b.publish(ctx, t, "student.synced", &late, map[string]any{"student_id": student.String(), "school_id": school.String(),
		"action": "update", "sync_run_id": uuid.NewString(), "status": "deactivated", "source_version": 1})
	e.processed(ctx, t, name, old)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND status = 'active'`, student); n != 1 {
		t.Fatal("older version applied")
	}
	// School curfew window from school.updated.
	window := map[string]any{"curfew_window": map[string]any{"start": "20:00", "end": "07:00"}}
	sc := b.publish(ctx, t, "school.updated", &late, map[string]any{"school_id": school.String(), "tenant_id": late.String(),
		"status": "active", "settings_subset": window, "changed_fields": []string{"curfew_window"}, "source_version": 3})
	e.processed(ctx, t, name, sc)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.schools WHERE school_id = $1
		AND settings_subset->'curfew_window'->>'start' = '20:00'`, school); n != 1 {
		t.Fatal("school curfew window not projected")
	}
	// user.status_changed (platform event) and cleanup/restore.
	lk := b.publish(ctx, t, "user.status_changed", nil, map[string]any{"user_id": student.String(), "from": "active", "to": "locked"})
	e.processed(ctx, t, name, lk)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND status = 'locked'`, student); n != 1 {
		t.Fatal("status not projected")
	}
	cu := b.publish(ctx, t, "student.cleaned_up", &late, map[string]any{"student_id": student.String(), "tenant_id": late.String(),
		"last_login_at": nil, "cutoff_days": 180, "run_id": uuid.NewString()})
	e.processed(ctx, t, name, cu)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND deleted_at IS NOT NULL`, student); n != 1 {
		t.Fatal("cleanup not projected")
	}
	rs := b.publish(ctx, t, "student.restored", &late, map[string]any{"student_id": student.String(), "tenant_id": late.String(),
		"cleaned_up_at": "2026-11-19T00:00:00Z", "restored_via": "login"})
	e.processed(ctx, t, name, rs)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND deleted_at IS NULL`, student); n != 1 {
		t.Fatal("restore not projected")
	}
	// A student event of tenant A about the student of tenant "late" is refused (tenant mismatch → DLQ).
	bad := b.publish(ctx, t, "student.synced", &e.tenantA, map[string]any{"student_id": student.String(), "school_id": school.String(),
		"action": "update", "sync_run_id": uuid.NewString(), "status": "active", "source_version": 9})
	e.deadLettered(ctx, t, name, bad)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.students WHERE student_id = $1 AND tenant_id = $2`, student, late); n != 1 {
		t.Fatal("student moved to another tenant")
	}
}
