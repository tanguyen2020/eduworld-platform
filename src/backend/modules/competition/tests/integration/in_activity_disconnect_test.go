//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// T031 — F101-AC-17, AC-18 (TC-PTY-008, Clock giả): during a session a disconnected member keeps its membership for
// disconnect_grace (the others go on); back within the grace nothing closes; past the grace the membership closes with
// disconnected; an activity that cannot go on without the member, or whose online participants stay below min_members
// for party_activity_min_members_timeout, ends with timeout and the party returns to its lobby.
func TestInActivityDisconnect(t *testing.T) {
	store, _ := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store, game: &fakeGame{}})
	e.area("meadow", []string{"g6_7"}, false)
	e.area("forest", []string{"g6_7"}, false)
	ctx := context.Background()
	sweep := func(id string) {
		t.Helper()
		if err := e.parties.SweepParty(ctx, partyRef(e, id)); err != nil {
			t.Fatal(err)
		}
	}
	started := func(typ, ref string, a, b kid) (string, string) {
		t.Helper()
		id := e.party(a, b)
		e.lobbyReady(id, typ, ref, a, b)
		r := e.startSession(a, id, key())
		if r.status != http.StatusCreated {
			t.Fatalf("start: %d %s", r.status, r.raw)
		}
		sweep(id)
		return id, r.str("party_activity_id")
	}
	open := func(act string, k kid) int {
		return e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE party_activity_id = $1 AND student_id = $2 AND left_at IS NULL`,
			act, k.id)
	}
	ended := func(act string) string {
		return e.one(`SELECT status || '/' || coalesce(end_reason, '') FROM multiplayer.party_activities WHERE party_activity_id = $1`, act)
	}

	// Explore (min 1, goes on): B away past the grace leaves the session only.
	a, b := e.kidA(), e.kidA()
	id1, act1 := started("explore", "meadow", a, b)
	e.online.set(b.id, false)
	sweep(id1)
	if got := e.push.of(a.id, "party.member.status_changed"); len(got) != 1 || got[0]["status"] != "disconnected" {
		t.Fatalf("A sees B disconnected: %v", got)
	}
	e.clock.Add(119 * time.Second)
	sweep(id1)
	if open(act1, b) != 1 {
		t.Fatal("membership closed within disconnect_grace")
	}
	e.online.set(b.id, true) // back within the grace (another device included: presence is per student)
	sweep(id1)
	e.online.set(b.id, false)
	sweep(id1)
	e.clock.Add(121 * time.Second)
	sweep(id1)
	if open(act1, b) != 0 || e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE party_activity_id = $1 AND student_id = $2
		AND leave_reason = 'disconnected'`, act1, b.id) != 1 {
		t.Fatal("membership of B not closed with disconnected past the grace")
	}
	if ended(act1) != "running/" || open(act1, a) != 1 {
		t.Fatalf("explore goes on with A: %s", ended(act1))
	}

	// Quest (min 2): below min_members past party_activity_min_members_timeout ends with timeout.
	c, d := e.kidA(), e.kidA()
	id2, act2 := started("quest", "q-bridge", c, d)
	e.online.set(d.id, false)
	sweep(id2)
	e.clock.Add(60 * time.Second)
	sweep(id2)
	if ended(act2) != "running/" {
		t.Fatalf("ended before the threshold: %s", ended(act2))
	}
	e.clock.Add(61 * time.Second)
	sweep(id2)
	if ended(act2) != "ended/timeout" {
		t.Fatalf("below min_members: %s", ended(act2))
	}
	if s := e.one(`SELECT status FROM multiplayer.parties WHERE party_id = $1`, id2); s != "open" {
		t.Fatalf("party: %s", s)
	}
	if got := e.push.of(c.id, "party.session.ended"); len(got) != 1 || got[0]["reason"] != "timeout" || got[0]["outcome"] != "abandoned" {
		t.Fatalf("session.ended: %v", got)
	}

	// An activity that does not go on without a member: past the grace → timeout.
	f, g := e.kidA(), e.kidA()
	id3, act3 := started("explore", "forest", f, g)
	e.exec(`UPDATE multiplayer.party_activities SET allow_continue_on_disconnect = false WHERE party_activity_id = $1`, act3)
	e.online.set(g.id, false)
	sweep(id3)
	e.clock.Add(121 * time.Second)
	sweep(id3)
	if ended(act3) != "ended/timeout" {
		t.Fatalf("no continue without G: %s", ended(act3))
	}
	checkLobbyEventSchemas(t, e)
}
