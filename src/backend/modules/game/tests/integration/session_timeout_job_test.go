//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
)

// F014-AC-16 (TC-AUTH-017, ADR-0083): the job session-timeout ends with timeout every active session without a
// heartbeat beyond game_session_heartbeat_timeout and records the end on its login event — including a client that
// only called gameLogout (identity) and never endGameSession; sessions with a recent heartbeat stay; a second run
// (or a second instance) ends nothing more.
func TestSessionTimeoutJob(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	quiet := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active") // gameLogout only: module game was never told
	alive := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	ids := map[string]string{}
	devs := map[string]device{}
	for name, s := range map[string]student{"quiet": quiet, "other": other, "alive": alive} {
		devs[name] = e.device(s)
		r := e.start(t, s, devs[name], newKey(), nil)
		if r.status != http.StatusCreated {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
		ids[name] = r.body["game_session_id"].(string)
	}
	e.clock.Add(40 * time.Second)
	if r := e.heartbeat(t, alive, devs["alive"], ids["alive"]); r.status != http.StatusNoContent {
		t.Fatalf("heartbeat: %d", r.status)
	}
	e.clock.Add(10 * time.Second) // quiet/other: 50 s without heartbeat; alive: 10 s
	n, err := e.sessions.TimeoutStaleSessions(ctx, 100)
	if err != nil || n != 2 {
		t.Fatalf("timeout run: %d %v", n, err)
	}
	if g := testutil.ToFloat64(observability.ActiveSessions().WithLabelValues("windows")); g != 3 {
		t.Fatalf("eduworld_game_sessions_active{windows} = %v before the stale ones end", g)
	}
	for _, name := range []string{"quiet", "other"} {
		if c := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions s JOIN game.login_events l USING (game_session_id)
			WHERE s.game_session_id = $1 AND s.end_reason = 'timeout' AND l.end_reason = 'timeout' AND l.logout_at = s.ended_at`, ids[name]); c != 1 {
			t.Fatalf("%s not timed out", name)
		}
	}
	if c := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND ended_at IS NULL`, ids["alive"]); c != 1 {
		t.Fatal("alive session ended")
	}
	if n, err := e.sessions.TimeoutStaleSessions(ctx, 100); err != nil || n != 0 {
		t.Fatalf("second run: %d %v", n, err)
	}
	if e.events(ctx, t, "game_session.ended") != 2 || e.events(ctx, t, "student.game_logged_out") != 2 {
		t.Fatal("timeout events")
	}
	// The ended events carry the tenant of each session.
	if c := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'game_session.ended' AND tenant_id = $1`, e.tenantB); c != 1 {
		t.Fatal("tenant of the event")
	}
}
