//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-14 (TC-AUTH-019): endGameSession sets ended_at, end_reason = logout, records logout_at/end_reason on the
// login event once and publishes student.game_logged_out and game_session.ended; ending again is a no-op (204).
// F014-AC-15: heartbeat moves last_heartbeat_at; an ended session → 409.
func TestEndAndHeartbeat(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d := e.device(an)
	id := e.start(t, an, d, newKey(), nil).body["game_session_id"].(string)
	e.clock.Add(20 * time.Second)
	if r := e.heartbeat(t, an, d, id); r.status != http.StatusNoContent {
		t.Fatalf("heartbeat: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND last_heartbeat_at = $2`,
		id, e.clock.Now()); n != 1 {
		t.Fatal("heartbeat not recorded")
	}
	e.clock.Add(10 * time.Minute)
	for range 2 {
		if r := e.end(t, an, d, id); r.status != http.StatusNoContent {
			t.Fatalf("end: %d %s", r.status, r.raw)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND end_reason = 'logout'
		AND ended_at = $2`, id, e.clock.Now()); n != 1 {
		t.Fatal("session not ended with logout")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events WHERE game_session_id = $1 AND end_reason = 'logout'
		AND logout_at = $2`, id, e.clock.Now()); n != 1 {
		t.Fatal("login event not closed")
	}
	if e.events(ctx, t, "student.game_logged_out") != 1 || e.events(ctx, t, "game_session.ended") != 1 {
		t.Fatal("end events (exactly once)")
	}
	ended := e.payloads(ctx, t, "game_session.ended")[0]
	if ended["end_reason"] != "logout" || ended["duration_ms"] != float64(10*time.Minute/time.Millisecond+20000) {
		t.Fatalf("game_session.ended: %v", ended)
	}
	if r := e.heartbeat(t, an, d, id); r.status != http.StatusConflict || r.code() != string(problem.Conflict) {
		t.Fatalf("heartbeat after end: %d %s", r.status, r.raw)
	}
}

// F014-AC-17 (TC-TEN-001): endGameSession and heartbeatGameSession on another student's session (same tenant or
// another tenant) → 404, the session is untouched.
func TestEndHeartbeatOtherStudent(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	dDung := e.device(dung)
	idDung := e.start(t, dung, dDung, newKey(), nil).body["game_session_id"].(string)
	dBinh := e.device(binh)
	idBinh := e.start(t, binh, dBinh, newKey(), nil).body["game_session_id"].(string)
	dAn := e.device(an)
	for _, id := range []string{idDung, idBinh, uuid.NewString()} {
		if r := e.end(t, an, dAn, id); r.status != http.StatusNotFound {
			t.Fatalf("end %s: %d %s", id, r.status, r.raw)
		}
		if r := e.heartbeat(t, an, dAn, id); r.status != http.StatusNotFound {
			t.Fatalf("heartbeat %s: %d %s", id, r.status, r.raw)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE ended_at IS NULL`); n != 2 {
		t.Fatalf("active sessions = %d", n)
	}
	authztest.Cover(t, "endGameSession", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "heartbeatGameSession", authztest.CrossTenant, authztest.RoleDenied)
}
