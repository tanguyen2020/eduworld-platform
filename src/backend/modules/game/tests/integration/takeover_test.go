//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-03 (TC-AUTH-015): a live session on another device → 409 ACTIVE_SESSION_ON_OTHER_DEVICE with other_device
// (platform, shortened label, last seen) and never the device id. F014-AC-04: takeover with a new Idempotency-Key and
// the same game token (no password) ends the old session (replaced_by_other_device, replaced_by_session_id) and
// creates the new one in one transaction; game_session.replaced carries old_device_id and old_keycloak_sid; the old
// device gets session.session.replaced and 409 at its next heartbeat; exactly one active session remains.
func TestTakeover(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	a, b := e.device(an), e.device(an)
	ra := e.start(t, an, a, newKey(), nil)
	if ra.status != http.StatusCreated {
		t.Fatalf("A: %d %s", ra.status, ra.raw)
	}
	oldID := ra.body["game_session_id"].(string)
	e.clock.Add(10 * time.Second)

	rb := e.start(t, an, b, newKey(), map[string]any{"client_platform": "android", "client_version": "1.2.3"})
	if rb.status != http.StatusConflict || rb.code() != string(problem.ActiveSessionOnOtherDevice) {
		t.Fatalf("B without takeover: %d %s", rb.status, rb.raw)
	}
	other, _ := rb.body["other_device"].(map[string]any)
	if other["platform"] != "windows" || other["device_label"] != "Máy tính Windows" || other["last_seen_at"] == nil {
		t.Fatalf("other_device: %v", rb.body)
	}
	if strings.Contains(rb.raw, a.id.String()) || strings.Contains(rb.raw, "device_id") {
		t.Fatalf("409 leaks the device id: %s", rb.raw)
	}

	rt := e.takeover(t, an, b)
	if rt.status != http.StatusCreated || rt.body["replaced_session_id"] != oldID {
		t.Fatalf("takeover: %d %s", rt.status, rt.raw)
	}
	newID := rt.body["game_session_id"].(string)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1
		AND end_reason = 'replaced_by_other_device' AND replaced_by_session_id = $2 AND ended_at IS NOT NULL`, oldID, newID); n != 1 {
		t.Fatal("old session not replaced")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE student_id = $1 AND ended_at IS NULL`, an.id); n != 1 {
		t.Fatalf("active sessions = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events WHERE game_session_id = $1
		AND end_reason = 'replaced_by_other_device' AND logout_at IS NOT NULL`, oldID); n != 1 {
		t.Fatal("old login event not closed")
	}
	ps := e.payloads(ctx, t, "game_session.replaced")
	if len(ps) != 1 || ps[0]["old_game_session_id"] != oldID || ps[0]["new_game_session_id"] != newID ||
		ps[0]["old_device_id"] != a.id.String() || ps[0]["old_keycloak_sid"] != a.sid || ps[0]["new_platform"] != "android" {
		t.Fatalf("game_session.replaced: %v", ps)
	}
	if e.audits(ctx, t, command.AuditSessionReplaced) != 1 {
		t.Fatal("takeover not audited")
	}
	if got := e.pushes.all(); len(got) != 1 || got[0] != "replaced:"+an.id.String()+":"+oldID+":" {
		t.Fatalf("realtime pushes: %v", got)
	}
	if r := e.heartbeat(t, an, a, oldID); r.status != http.StatusConflict {
		t.Fatalf("old device heartbeat: %d %s", r.status, r.raw)
	}
	if r := e.heartbeat(t, an, b, newID); r.status != http.StatusNoContent {
		t.Fatalf("new device heartbeat: %d %s", r.status, r.raw)
	}
	if e.end(t, an, b, newID).status != http.StatusNoContent {
		t.Fatal("end")
	}
	seen := e.checkEventSchemas(ctx, t)
	for _, typ := range gameEventTypes {
		if seen[typ] == 0 {
			t.Errorf("no %s event checked", typ)
		}
	}
}

// F014-AC-06 (TC-AUTH-017): a session without heartbeat beyond the timeout ends with timeout and the new device starts
// without confirmation; the same installation starting again ends its old session with logout.
func TestStartAfterTimeoutAndSameDevice(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	a, b := e.device(an), e.device(an)
	first := e.start(t, an, a, newKey(), nil).body["game_session_id"].(string)
	e.clock.Add(46 * time.Second)
	rb := e.start(t, an, b, newKey(), nil)
	if rb.status != http.StatusCreated || rb.body["replaced_session_id"] != nil {
		t.Fatalf("after timeout: %d %s", rb.status, rb.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND end_reason = 'timeout'`, first); n != 1 {
		t.Fatal("stale session not ended with timeout")
	}
	second := rb.body["game_session_id"].(string)
	e.clock.Add(5 * time.Second)
	if r := e.start(t, an, b, newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("same device: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND end_reason = 'logout'`, second); n != 1 {
		t.Fatal("same device session not ended with logout")
	}
	if e.events(ctx, t, "game_session.replaced") != 0 || len(e.pushes.all()) != 0 {
		t.Fatal("no takeover expected")
	}
	if e.events(ctx, t, "game_session.ended") != 2 || e.events(ctx, t, "student.game_logged_out") != 2 {
		t.Fatal("end events of the old sessions")
	}
}
