//go:build integration

package integration

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// placementOf is a student.placement_changed.v1 payload (schema specs/events/payloads/identity, required fields + source).
func placementOf(student, tenant, school uuid.UUID, grade int, status string, version int) map[string]any {
	return map[string]any{"student_id": student, "tenant_id": tenant, "school_id": school, "grade_level": grade, "status": status,
		"changed_fields": []string{"tenant_id", "school_id", "grade_level", "status"}, "source": "platform_common",
		"sync_run_id": nil, "source_version": version}
}

// EVENT_DESIGN §4 (fix 2026-11-28): chat.projection_identity subscribes student.placement_changed, so a student created
// or moved outside the SIS (no student.synced) reaches ref.students and can use the social API; school and grade follow
// the newest placement, an inactive placement cancels the pending requests, an account status (locked) is kept by an
// active placement, an older version is ignored, and a placement of another tenant's student or a payload whose
// tenant differs from the envelope goes to the DLQ (never moves the row).
func TestProjectionPlacement(t *testing.T) {
	e := setup(t, envOpts{})
	found := false
	for _, ev := range messaging.ProjectionIdentityEvents {
		found = found || ev == "student.placement_changed"
	}
	if !found {
		t.Fatal("chat.projection_identity does not subscribe student.placement_changed")
	}
	p := command.Projection{Refs: persistence.Refs{}, Social: persistence.Social{}, Now: e.clock.Now}
	id := consumer.IdentityProjection{P: p}
	game := consumer.GameProjection{P: p}
	tenant, school, school2 := uuid.New(), uuid.New(), uuid.New()
	if err := e.handle(t, id.Handle, "tenant.created", tenant, map[string]any{"tenant_id": tenant, "tenant_type": "school", "status": "active",
		"changed_fields": []string{}, "source_version": 1}); err != nil {
		t.Fatal(err)
	}
	a := student{id: uuid.New(), tenant: tenant, school: school, sub: "kc-pa", nickname: "Pa"}
	b := student{id: uuid.New(), tenant: tenant, school: school, sub: "kc-pb", nickname: "Pb"}
	for _, s := range []*student{&a, &b} {
		if err := e.handle(t, id.Handle, "student.placement_changed", tenant, placementOf(s.id, tenant, school, 3, "active", 1)); err != nil {
			t.Fatal(err)
		}
		if err := e.handle(t, game.Handle, "game_profile.updated", tenant, map[string]any{"student_id": s.id, "tenant_id": tenant,
			"nickname": s.nickname, "avatar_config_hash": nil, "level": 1, "source_version": 1}); err != nil {
			t.Fatal(err)
		}
		tt := tenant
		e.res.Set(s.sub, authz.PrincipalData{UserID: s.id, UserType: authz.UserTypeStudent, TenantID: &tt, TenantType: authz.TenantTypeSchool})
		s.token = e.idp.TokenSID(s.sub, "eduworld-game-backend", uuid.NewString(), time.Now())
	}
	if n := e.count(`SELECT count(*) FROM ref.students WHERE student_id IN ($1, $2) AND tenant_id = $3 AND school_id = $4
		AND grade_level = 3 AND status = 'active' AND deleted_at IS NULL`, a.id, b.id, tenant, school); n != 2 {
		t.Fatalf("placement-only students projected: %d", n)
	}
	// The social API works on students known only from placements (no student.synced).
	ab := requestID(e.send(a, b, http.StatusCreated))

	// A newer placement moves b to another school and grade; an older one is ignored.
	if err := e.handle(t, id.Handle, "student.placement_changed", tenant, placementOf(b.id, tenant, school2, 4, "active", 3)); err != nil {
		t.Fatal(err)
	}
	if err := e.handle(t, id.Handle, "student.placement_changed", tenant, placementOf(b.id, tenant, school, 3, "active", 2)); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM ref.students WHERE student_id = $1 AND school_id = $2 AND grade_level = 4`, b.id, school2); n != 1 {
		t.Fatal("newest placement not kept")
	}
	// An account status (locked) survives an active placement.
	if err := e.handle(t, id.Handle, "user.status_changed", tenant, map[string]any{"user_id": a.id, "from": "active", "to": "locked"}); err != nil {
		t.Fatal(err)
	}
	if err := e.handle(t, id.Handle, "student.placement_changed", tenant, placementOf(a.id, tenant, school, 3, "active", 4)); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM ref.students WHERE student_id = $1 AND status = 'locked'`, a.id); n != 1 {
		t.Fatal("an active placement overwrote the account status")
	}
	// No longer placed: inactive, the pending request of b is cancelled.
	if err := e.handle(t, id.Handle, "student.placement_changed", tenant, placementOf(b.id, tenant, school2, 4, "inactive", 5)); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM ref.students WHERE student_id = $1 AND status = 'inactive'`, b.id); n != 1 {
		t.Fatal("inactive placement not projected")
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests WHERE request_id = $1 AND status = 'cancelled'`, ab); n != 1 {
		t.Fatal("request of a student no longer placed still pending")
	}
	// Placed again: active.
	if err := e.handle(t, id.Handle, "student.placement_changed", tenant, placementOf(b.id, tenant, school2, 4, "active", 6)); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM ref.students WHERE student_id = $1 AND status = 'active'`, b.id); n != 1 {
		t.Fatal("placement not revived")
	}

	// Tenant isolation: an event of another tenant about a's row, or a payload tenant that is not the envelope tenant → DLQ.
	other := uuid.New()
	e.tenant(other, "school", nil)
	if err := e.handle(t, id.Handle, "student.placement_changed", other, placementOf(a.id, other, uuid.New(), 5, "active", 9)); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("another tenant's student: %v", err)
	}
	if err := e.handle(t, id.Handle, "student.placement_changed", other, placementOf(uuid.New(), tenant, school, 3, "active", 1)); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("payload tenant ≠ envelope tenant: %v", err)
	}
	for _, bad := range []map[string]any{placementOf(uuid.New(), tenant, school, 0, "active", 1), placementOf(uuid.New(), tenant, school, 3, "gone", 1)} {
		if err := e.handle(t, id.Handle, "student.placement_changed", tenant, bad); !errors.Is(err, outbox.ErrPermanent) {
			t.Fatalf("invalid payload %v: %v", bad, err)
		}
	}
	if n := e.count(`SELECT count(*) FROM ref.students WHERE student_id = $1 AND tenant_id = $2 AND grade_level = 3`, a.id, tenant); n != 1 {
		t.Fatal("a's row moved by another tenant")
	}
	if n := e.count(`SELECT count(*) FROM ref.students WHERE tenant_id = $1`, other); n != 0 {
		t.Fatal("rows written for the other tenant")
	}
}
