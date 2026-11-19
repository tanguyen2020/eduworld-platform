//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
)

// F014-AC-24 (TC-AUTH-006, fixtures of F013 T045): user.access_revoked (locked) → account_locked; user.access_revoked
// (password_changed) → access_revoked; user.status_changed → locked → account_locked; student.cleaned_up → cleanup.
// The login event records logout_at/end_reason, game_session.ended is published once, the client is told
// (session.session.ended), a redelivered event (same event_id) is not processed twice, a malformed payload goes to
// the DLQ. The end-to-end lock/cleanup is F016-AC-24.
func TestSessionEndOnRevoke(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	b := newBus(ctx, t)
	b.run(ctx, t, consumer.SessionEndOnRevokeOptions(consumer.SessionEndOnRevoke{Sessions: e.sessions}, e.uow, e.app, b.js))
	cases := []struct {
		fixture, reason, push string
	}{
		{"user.access_revoked.locked.json", "account_locked", "account_locked"},
		{"user.access_revoked.password_changed.json", "access_revoked", "password_changed"},
		{"user.status_changed.locked.json", "account_locked", "account_locked"},
		{"student.cleaned_up.json", "cleanup", "cleanup"},
	}
	for _, tc := range cases {
		s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
		r := e.start(t, s, e.device(s), newKey(), nil)
		if r.status != http.StatusCreated {
			t.Fatalf("%s start: %d %s", tc.fixture, r.status, r.raw)
		}
		id := r.body["game_session_id"].(string)
		before := e.events(ctx, t, "game_session.ended")
		env := fixture(t, tc.fixture, s.id, e.tenantA)
		b.publishEnvelope(ctx, t, env)
		b.publishEnvelope(ctx, t, env) // redelivery of the same event_id
		e.processed(ctx, t, messaging.SessionRevokeName, env)
		waitFor(t, 10*time.Second, tc.fixture+" ended", func() bool {
			return count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND end_reason = $2`, id, tc.reason) == 1
		})
		if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events WHERE game_session_id = $1 AND end_reason = $2
			AND logout_at IS NOT NULL`, id, tc.reason); n != 1 {
			t.Fatalf("%s: login event not closed", tc.fixture)
		}
		time.Sleep(300 * time.Millisecond) // let a duplicate delivery run, if any
		if got := e.events(ctx, t, "game_session.ended") - before; got != 1 {
			t.Fatalf("%s: game_session.ended published %d times", tc.fixture, got)
		}
		want := "ended:" + s.id.String() + ":" + id + ":" + tc.push
		found := false
		for _, m := range e.pushes.all() {
			found = found || m == want
		}
		if !found {
			t.Fatalf("%s: no realtime push %s in %v", tc.fixture, want, e.pushes.all())
		}
	}
	// Activation ends nothing; a malformed payload is refused at once (DLQ).
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	id := e.start(t, s, e.device(s), newKey(), nil).body["game_session_id"].(string)
	act := fixture(t, "user.status_changed.activated.json", s.id, e.tenantA)
	b.publishEnvelope(ctx, t, act)
	e.processed(ctx, t, messaging.SessionRevokeName, act)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND ended_at IS NULL`, id); n != 1 {
		t.Fatal("activation ended the session")
	}
	bad := b.publish(ctx, t, "student.cleaned_up", &e.tenantA, map[string]any{"student_id": "x", "tenant_id": e.tenantA.String()})
	e.deadLettered(ctx, t, messaging.SessionRevokeName, bad)
	// A platform user.access_revoked (no envelope tenant) ends the sessions in every tenant of the student.
	p := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	pid := e.start(t, p, e.device(p), newKey(), nil).body["game_session_id"].(string)
	plat := b.publish(ctx, t, "user.access_revoked", nil, map[string]any{"user_id": p.id.String(), "reason": "deactivated", "tenant_id": nil})
	e.processed(ctx, t, messaging.SessionRevokeName, plat)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND end_reason = 'access_revoked'`, pid); n != 1 {
		t.Fatal("platform revocation did not end the session")
	}
}
