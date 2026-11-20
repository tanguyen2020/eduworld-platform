//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// T029 (F090-AC-24; TC-AUTH-006, TC-AUTH-036, TC-TEN-009): a control access_revoked on ew:rt:user:<id> (each reason,
// password_changed included) → session.session.ended with that reason, close 4403 within 5 s, the resume buffer is
// deleted and a later resume fails.
func TestRevocation(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", nil)
	for _, reason := range []string{"account_locked", "access_revoked", "cleanup", "admin", "password_changed"} {
		an := newStudent()
		c := dial(t, n, e.ticket(an))
		token := c.next("system.session.welcome").field("resume_token").(string)
		e.publish(rt.UserChannel(an.user), e.message("notification.item.created", map[string]any{"n": 1}))
		c.next("notification.item.created")
		ctl, err := rt.Revoke(reason)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		e.publish(rt.UserChannel(an.user), ctl)
		m := c.next("session.session.ended")
		if m.field("reason") != reason || m.field("game_session_id") != an.session.String() {
			t.Fatalf("%s: ended %s", reason, m.Payload)
		}
		if code := c.closed(5 * time.Second); code != 4403 || time.Since(start) > 5*time.Second {
			t.Fatalf("%s: close %d after %v", reason, code, time.Since(start))
		}
		d := dial(t, n, e.ticket(an))
		d.next("system.session.welcome")
		d.send("system.session.resume", newID(), map[string]any{"resume_token": token, "last_seq": 0})
		if f := d.next("system.session.resume_failed"); f.field("reason") != "unknown_token" {
			t.Fatalf("%s: resume after revocation %s", reason, f.Payload)
		}
	}
}

// T042 (F090-AC-32, TC-AUTH-092) and T030 gateway side (F090-AC-25, AC-26, TC-AUTH-015): session.session.ended curfew /
// daily_limit for the held session → delivered then 4410, resume invalidated; session.session.replaced for the held
// session → delivered then 4409; the same messages for another session of the student keep the connection open.
func TestSessionEndedAndReplaced(t *testing.T) {
	e := newEnv(t)
	n := e.node("gw-1", nil)
	cases := []struct {
		typ, reason string
		code        int
	}{{"session.session.ended", "curfew", 4410}, {"session.session.ended", "daily_limit", 4410}, {"session.session.replaced", "", 4409}}
	for _, tc := range cases {
		an := newStudent()
		c := dial(t, n, e.ticket(an))
		token := c.next("system.session.welcome").field("resume_token").(string)
		payload := func(session uuid.UUID) map[string]any {
			p := map[string]any{"game_session_id": session.String()}
			if tc.reason != "" {
				p["reason"], p["ended_at"], p["reopen_at"] = tc.reason, time.Now().UnixMilli(), time.Now().Add(time.Hour).UnixMilli()
			} else {
				p["replaced_at"], p["new_device_platform"] = time.Now().UnixMilli(), "android"
			}
			return p
		}
		// Another session (the new device after a takeover, or a stale one): nothing happens to this connection.
		other := uuid.New()
		e.publish(rt.UserChannel(an.user), e.message(tc.typ, payload(other)).ForSession(other))
		c.send("system.heartbeat.ping", newID(), map[string]any{"client_time": 1})
		c.next("system.heartbeat.pong")
		c.none(tc.typ, 200*time.Millisecond)
		// The held session.
		e.publish(rt.UserChannel(an.user), e.message(tc.typ, payload(an.session)).ForSession(an.session))
		m := c.next(tc.typ)
		if tc.reason != "" && (m.field("reason") != tc.reason || m.field("reopen_at") == nil) {
			t.Fatalf("%s: %s", tc.reason, m.Payload)
		}
		if code := c.closed(5 * time.Second); int(code) != tc.code {
			t.Fatalf("%s %s: close %d", tc.typ, tc.reason, code)
		}
		d := dial(t, n, e.ticket(an))
		d.next("system.session.welcome")
		d.send("system.session.resume", newID(), map[string]any{"resume_token": token, "last_seq": 0})
		if f := d.next("system.session.resume_failed"); f.field("reason") != "unknown_token" {
			t.Fatalf("%s: resume token still valid: %s", tc.typ, f.Payload)
		}
	}
}
