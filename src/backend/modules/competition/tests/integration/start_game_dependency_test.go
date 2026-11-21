//go:build integration

package integration

import (
	"net/http"
	"testing"
)

// T021 — F101-AC-10, AC-20 (TC-PTY-006, module game MOCK — R17): quest and boss sessions come from module game, called with the
// TenantScope of the party and idempotent on party_activity_id; module game down → 503 UNAVAILABLE with Retry-After and the
// party unchanged; a retry with the same Idempotency-Key after a failed commit asks for the same session (same room); quest
// gets a private_instance of its area, boss the room of the encounter.
func TestStartGameDependency(t *testing.T) {
	store, _ := valkeyLobby(t)
	game := &fakeGame{}
	e := setupParty(t, popts{lobby: store, game: game})
	e.area("forest", []string{"g6_7"}, false)
	a, b := e.kidA(), e.kidA()
	id := e.party(a, b)
	e.lobbyReady(id, "quest", "q-bridge", a, b)

	game.mu.Lock()
	game.down = true
	game.mu.Unlock()
	r := e.startSession(a, id, key())
	if r.status != http.StatusServiceUnavailable || r.code() != "UNAVAILABLE" || r.header.Get("Retry-After") == "" {
		t.Fatalf("module game down: %d %s %v", r.status, r.raw, r.header)
	}
	if s := e.one(`SELECT status FROM multiplayer.parties WHERE party_id = $1`, id); s != "open" {
		t.Fatalf("party changed: %s", s)
	}
	game.mu.Lock()
	game.down = false
	game.mu.Unlock()

	// The party changes between the start of the session in module game and the commit: 412; the retry with the same key
	// gets the same party_activity_id, so the same session of module game.
	k := key()
	game.onStart = func() {
		e.exec(`UPDATE multiplayer.parties SET state_version = state_version + 1 WHERE party_id = $1`, id)
	}
	if r := e.startSession(a, id, k); r.status != http.StatusPreconditionFailed {
		t.Fatalf("party changed before the commit: %d %s", r.status, r.raw)
	}
	game.onStart = nil
	e.exec(`UPDATE multiplayer.parties SET state_version = state_version - 1 WHERE party_id = $1`, id)
	r = e.startSession(a, id, k)
	if r.status != http.StatusCreated {
		t.Fatalf("retry: %d %s", r.status, r.raw)
	}
	if len(game.activity) != 2 || game.activity[0] != game.activity[1] || r.str("party_activity_id") != game.activity[0].String() {
		t.Fatalf("party_activity_id of the retry: %v %s", game.activity, r.raw)
	}
	if r.str("room_type") != "private_instance" || r.str("scene_ref") != "quest/bridge" {
		t.Fatalf("quest room: %s", r.raw)
	}
	if s := e.one(`SELECT area_id || '/' || room_type FROM multiplayer.rooms WHERE room_id = $1`, r.str("room_id")); s != "forest/private_instance" {
		t.Fatalf("quest private_instance: %s", s)
	}
	if s := e.one(`SELECT external_ref::text FROM multiplayer.party_activities WHERE party_id = $1`, id); s != game.starts[game.activity[0]].String() {
		t.Fatalf("external_ref: %s", s)
	}
	for _, sc := range game.scopes {
		if sc != e.tenantA {
			t.Fatalf("facade game called with another tenant scope: %v", game.scopes)
		}
	}

	// Boss: the room is the encounter of module game.
	c, d := e.kidA(), e.kidA()
	id2 := e.party(c, d)
	e.lobbyReady(id2, "boss", "b-slime", c, d)
	r = e.startSession(c, id2, key())
	if r.status != http.StatusCreated || r.str("room_type") != "boss" {
		t.Fatalf("boss: %d %s", r.status, r.raw)
	}
	enc := game.starts[game.activity[len(game.activity)-1]].String()
	if r.str("room_id") != enc {
		t.Fatalf("boss room = encounter %s: %s", enc, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE party_id = $1 AND encounter_id = $2 AND room_type = 'boss'`,
		id2, enc); n != 2 {
		t.Fatalf("boss memberships: %d", n)
	}
}
