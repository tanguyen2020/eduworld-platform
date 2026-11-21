//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// T030 — F101-AC-15, AC-16 (TC-PTY-009, Clock giả): a leader back within party_leader_grace keeps the role (the lobby says it is
// reconnecting meanwhile); past the grace the online member who joined first becomes leader (party.leader_transferred
// leader_disconnected, party.leader.transferred, audit); nobody online → dormant, the first member back online becomes leader;
// a friend party nobody comes back to for party_dormant_timeout is disbanded with reason inactivity.
func TestLeaderGraceDormant(t *testing.T) {
	store, _ := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store})
	ctx := context.Background()
	sweep := func(id string) {
		t.Helper()
		if err := e.parties.SweepParty(ctx, partyRef(e, id)); err != nil {
			t.Fatal(err)
		}
	}
	leader := func(k kid, id string) string {
		t.Helper()
		return e.call(k, http.MethodGet, "/multiplayer/parties/"+id, "", nil).str("leader_id")
	}
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	for _, k := range []kid{a, b, c} {
		e.online.set(k.id, true)
	}
	sweep(id)

	// Back within the grace.
	e.online.set(a.id, false)
	sweep(id)
	if r := e.call(b, http.MethodGet, "/multiplayer/parties/"+id, "", nil); r.body["leader_reconnecting"] != true {
		t.Fatalf("leader_reconnecting: %s", r.raw)
	}
	e.clock.Add(50 * time.Second)
	e.online.set(a.id, true)
	sweep(id)
	e.clock.Add(30 * time.Second)
	sweep(id)
	if leader(b, id) != a.id.String() {
		t.Fatal("leader back within the grace lost the role")
	}

	// Past the grace: B (online, joined before C) becomes leader.
	e.online.set(a.id, false)
	sweep(id)
	e.clock.Add(61 * time.Second)
	sweep(id)
	if leader(b, id) != b.id.String() {
		t.Fatalf("leader after the grace: %s", leader(b, id))
	}
	ev := e.events("party.leader_transferred")
	if len(ev) != 1 || ev[0]["reason"] != "leader_disconnected" || ev[0]["from"] != a.id.String() || ev[0]["to"] != b.id.String() {
		t.Fatalf("party.leader_transferred: %v", ev)
	}
	if got := e.push.of(c.id, "party.leader.transferred"); len(got) != 1 || got[0]["reason"] != "leader_disconnected" {
		t.Fatalf("party.leader.transferred: %v", got)
	}
	if au := e.audits("party.leader_transferred"); len(au) != 1 || au[0]["reason"] != "leader_disconnected" {
		t.Fatalf("audit: %v", au)
	}
	if r := e.call(b, http.MethodGet, "/multiplayer/parties/"+id, "", nil); len(r.members()) != 3 || r.body["leader_reconnecting"] != false {
		t.Fatalf("A still a member, no grace: %s", r.raw)
	}

	// Dormant: nobody online past the grace keeps the leader; the first member back becomes leader.
	d, f := e.kidA(), e.kidA()
	id2 := e.party(d, f)
	sweep(id2)
	e.clock.Add(61 * time.Second)
	sweep(id2)
	if leader(d, id2) != d.id.String() {
		t.Fatal("dormant party changed leader")
	}
	e.online.set(f.id, true)
	sweep(id2)
	if leader(f, id2) != f.id.String() {
		t.Fatalf("first member back is leader: %s", leader(f, id2))
	}

	// Dormant past party_dormant_timeout: disbanded (inactivity).
	g, h := e.kidA(), e.kidA()
	id3 := e.party(g, h)
	sweep(id3)
	e.clock.Add(29 * time.Minute)
	sweep(id3)
	if s := e.one(`SELECT status FROM multiplayer.parties WHERE party_id = $1`, id3); s != "open" {
		t.Fatalf("disbanded before the timeout: %s", s)
	}
	e.clock.Add(2 * time.Minute)
	sweep(id3)
	if s := e.one(`SELECT status FROM multiplayer.parties WHERE party_id = $1`, id3); s != "disbanded" {
		t.Fatalf("dormant party: %s", s)
	}
	dis := e.events("party.disbanded")
	if len(dis) != 1 || dis[0]["reason"] != "inactivity" || dis[0]["actor_id"] != nil {
		t.Fatalf("party.disbanded: %v", dis)
	}
	if refs, _ := store.Tracked(ctx, 100); len(refs) != 2 {
		t.Fatalf("disbanded party still tracked: %v", refs)
	}
	checkLobbyEventSchemas(t, e)
}
