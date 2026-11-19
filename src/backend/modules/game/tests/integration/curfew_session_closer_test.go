//go:build integration

package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

// F014-AC-26 (TC-AUTH-092, ADR-0078): at the start of the effective curfew the job curfew-session-closer ends every
// active session of the students under it with end_reason curfew (≤ 1 minute: run each minute), records the login
// event, publishes game_session.ended and session.session.ended (reason curfew with reopen_at). A school whose window
// starts later keeps its sessions until then; two instances running together never end a session twice.
func TestCurfewSessionCloser(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{curfew: "21:30-06:30"})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	e.clock.Set(time.Date(2026, 11, 19, 21, 0, 0, 0, ict))
	ids := map[string]string{}
	for name, s := range map[string]student{"an": an, "dung": dung} {
		r := e.start(t, s, e.device(s), newKey(), nil)
		if r.status != http.StatusCreated {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
		ids[name] = r.body["game_session_id"].(string)
	}
	// Keep the sessions alive (heartbeats) so only the curfew can end them.
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.game_sessions SET last_heartbeat_at = $1`, time.Date(2026, 11, 19, 21, 29, 0, 0, ict)); err != nil {
		t.Fatal(err)
	}
	e.clock.Set(time.Date(2026, 11, 19, 21, 29, 0, 0, ict))
	if n, err := e.sessions.CloseSessionsAtCurfew(ctx, 10); err != nil || n != 0 {
		t.Fatalf("21:29: %d %v", n, err)
	}
	e.clock.Set(time.Date(2026, 11, 19, 21, 30, 5, 0, ict))
	var wg sync.WaitGroup
	results := make([]int, 2)
	for i := range results { // two worker instances at the same tick
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := e.sessions.CloseSessionsAtCurfew(ctx, 1) // batch 1: pages through every session
			if err != nil {
				t.Errorf("instance %d: %v", i, err)
			}
			results[i] = n
		}()
	}
	wg.Wait()
	if results[0]+results[1] != 2 {
		t.Fatalf("sessions ended by the two instances: %v", results)
	}
	for name, id := range ids {
		if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions s JOIN game.login_events l USING (game_session_id)
			WHERE s.game_session_id = $1 AND s.end_reason = 'curfew' AND l.end_reason = 'curfew' AND s.ended_at = $2`, id,
			time.Date(2026, 11, 19, 21, 30, 5, 0, ict)); n != 1 {
			t.Fatalf("%s not ended with curfew", name)
		}
	}
	if e.events(ctx, t, "game_session.ended") != 2 {
		t.Fatalf("game_session.ended = %d", e.events(ctx, t, "game_session.ended"))
	}
	pushed := 0
	for _, m := range e.pushes.all() {
		if m == "ended:"+an.id.String()+":"+ids["an"]+":curfew:reopen" || m == "ended:"+dung.id.String()+":"+ids["dung"]+":curfew:reopen" {
			pushed++
		}
	}
	if pushed != 2 {
		t.Fatalf("session.session.ended pushes: %v", e.pushes.all())
	}
}
