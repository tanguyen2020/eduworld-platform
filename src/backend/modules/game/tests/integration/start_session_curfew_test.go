//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-25 (TC-AUTH-092, ADR-0078): inside the effective curfew (platform 21:30–06:30 ICT, or a school window that
// extends it) createGameSession and takeover → 403 CURFEW_ACTIVE with reopen_at; no session, no login event.
func TestStartSessionCurfew(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{curfew: "21:30-06:30"})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	b := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active")
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.schools SET settings_subset = '{"curfew_window":{"start":"20:00","end":"07:00"}}'
		WHERE school_id = $1`, e.schoolA2); err != nil {
		t.Fatal(err)
	}
	// 20:30 ICT: open for the platform, closed for school A2 until 07:00 the next morning.
	e.clock.Set(time.Date(2026, 11, 19, 20, 30, 0, 0, ict))
	d := e.device(an)
	if r := e.start(t, an, d, newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("before curfew: %d %s", r.status, r.raw)
	}
	r := e.start(t, b, e.device(b), newKey(), nil)
	if r.status != http.StatusForbidden || r.code() != string(problem.CurfewActive) {
		t.Fatalf("school curfew: %d %s", r.status, r.raw)
	}
	if at, _ := time.Parse(time.RFC3339, r.body["reopen_at"].(string)); !at.Equal(time.Date(2026, 11, 20, 7, 0, 0, 0, ict)) {
		t.Fatalf("reopen_at %v", r.body["reopen_at"])
	}
	// 21:35 ICT: platform curfew for everyone, takeover included.
	e.clock.Set(time.Date(2026, 11, 19, 21, 35, 0, 0, ict))
	r = e.takeover(t, an, e.device(an))
	if r.status != http.StatusForbidden || r.code() != string(problem.CurfewActive) {
		t.Fatalf("takeover in curfew: %d %s", r.status, r.raw)
	}
	if at, _ := time.Parse(time.RFC3339, r.body["reopen_at"].(string)); !at.Equal(time.Date(2026, 11, 20, 6, 30, 0, 0, ict)) {
		t.Fatalf("reopen_at %v", r.body["reopen_at"])
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions`); n != 1 {
		t.Fatalf("sessions = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events`); n != 1 {
		t.Fatalf("login events = %d", n)
	}
}
