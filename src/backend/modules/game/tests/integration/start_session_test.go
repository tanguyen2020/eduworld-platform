//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-01 (TC-AUTH-019): a student game token with X-Device-Id, X-Tenant-Id and Idempotency-Key creates a session
// recording platform, client version, shortened device, keycloak_sid; 201 GameSession; F014-AC-09 one login event.
func TestStartSessionCreates(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d := e.device(an)
	r := e.start(t, an, d, newKey(), nil)
	if r.status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	id, _ := r.body["game_session_id"].(string)
	if _, err := uuid.Parse(id); err != nil || r.body["started_at"] == nil || r.body["login_at"] == nil || r.body["replaced_session_id"] != nil {
		t.Fatalf("body: %v", r.body)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND tenant_id = $2
		AND student_id = $3 AND school_id = $4 AND device_id = $5 AND client_platform = 'windows' AND client_version = '1.2.3'
		AND device_model = 'Máy tính Windows' AND keycloak_sid = $6 AND ended_at IS NULL`, id, an.tenant, an.id, an.school, d.id, d.sid); n != 1 {
		t.Fatal("session row not as expected")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events WHERE game_session_id = $1`, id); n != 1 {
		t.Fatalf("login events = %d", n)
	}
	if e.events(ctx, t, "game_session.started") != 1 || e.events(ctx, t, "student.game_logged_in") != 1 {
		t.Fatal("start events missing")
	}
}

// F014-AC-02 (TC-AUTH-013) and F014-AC-18 (TC-AUTH-014): staff tokens (game client or portal) and inactive or
// cleaned-up students get 403 GAME_ACCESS_STUDENT_ONLY, no session, no login event; the refusal is audited.
func TestStartSessionStudentOnly(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	locked := e.newStudent(ctx, t, e.tenantA, e.schoolA, "locked")
	cleaned := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.students SET deleted_at = now() WHERE student_id = $1`, cleaned.id); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]student{"locked": locked, "cleaned up": cleaned} {
		if r := e.start(t, s, e.device(s), newKey(), nil); r.status != http.StatusForbidden || r.code() != string(problem.GameAccessStudentOnly) {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if e.audits(ctx, t, command.AuditAccessDenied) != 2 {
		t.Fatalf("denial audits = %d", e.audits(ctx, t, command.AuditAccessDenied))
	}
	// Staff with a game-client token (the game client is for students only) are stopped by the game_player guard.
	staff := e.staff(e.tenantA, "school_admin")
	tok := e.idp.Token(staff, httpxtest.GameClient, false, time.Now())
	r := e.do(t, http.MethodPost, "/game/sessions", tok, map[string]string{"X-Tenant-Id": e.tenantA.String(),
		"X-Device-Id": uuid.NewString(), "Idempotency-Key": newKey()}, map[string]any{"client_platform": "windows", "client_version": "1.2.3"})
	if r.status != http.StatusForbidden || r.code() != string(problem.GameAccessStudentOnly) {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions`) + count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events`); n != 0 {
		t.Fatalf("rows written for refused calls: %d", n)
	}
	if count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'authz.denied'`) == 0 {
		t.Fatal("guard denial not audited")
	}
	authztest.Cover(t, "createGameSession", authztest.RoleDenied)
}

// F014-AC-08 (TC-TEN-011): X-Tenant-Id of another tenant → 403 TENANT_MISMATCH; the session always carries the
// student's tenant.
func TestStartSessionTenantMismatch(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d := e.device(an)
	r := e.do(t, http.MethodPost, "/game/sessions", d.token, map[string]string{"X-Tenant-Id": e.tenantB.String(),
		"X-Device-Id": d.id.String(), "Idempotency-Key": newKey()}, map[string]any{"client_platform": "windows", "client_version": "1.2.3"})
	if r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
		t.Fatalf("mismatch: %d %s", r.status, r.raw)
	}
	if r := e.start(t, an, d, newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("own tenant: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE tenant_id <> $1`, e.tenantA); n != 0 {
		t.Fatal("session in another tenant")
	}
	authztest.Cover(t, "createGameSession", authztest.CrossTenant)
}

// Edge case (spec): the student is not projected yet → 503 UNAVAILABLE with retryable, no session.
func TestStartSessionNotProjected(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	ghost := student{id: uuid.New(), tenant: e.tenantA, school: e.schoolA, sub: "kc-ghost"}
	tt := e.tenantA
	e.res.Set(ghost.sub, authztestStudent(ghost.id, tt))
	r := e.start(t, ghost, e.device(ghost), newKey(), nil)
	if r.status != http.StatusServiceUnavailable || r.code() != string(problem.Unavailable) || r.body["retryable"] != true {
		t.Fatalf("not projected: %d %s", r.status, r.raw)
	}
}
