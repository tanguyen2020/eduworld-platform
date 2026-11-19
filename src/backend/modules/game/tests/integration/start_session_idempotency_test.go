//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-07 (API_DESIGN §6): no Idempotency-Key → 428; the same key and body replays the stored 201 without a second
// session; the same key with another body (takeover: true) → 409 IDEMPOTENCY_KEY_REUSED.
func TestStartSessionIdempotency(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d := e.device(an)
	if r := e.start(t, an, d, "", nil); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	key := newKey()
	first := e.start(t, an, d, key, nil)
	again := e.start(t, an, d, key, nil)
	if first.status != http.StatusCreated || again.status != http.StatusCreated ||
		again.body["game_session_id"] != first.body["game_session_id"] || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %v / %d %v", first.status, first.body, again.status, again.body)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE student_id = $1`, an.id); n != 1 {
		t.Fatalf("sessions = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events WHERE student_id = $1`, an.id); n != 1 {
		t.Fatalf("login events = %d", n)
	}
	r := e.start(t, an, d, key, map[string]any{"client_platform": "windows", "client_version": "1.2.3", "device_model": "Máy tính Windows",
		"takeover": true})
	if r.status != http.StatusConflict || r.code() != string(problem.IdempotencyKeyReused) {
		t.Fatalf("other body: %d %s", r.status, r.raw)
	}
}
