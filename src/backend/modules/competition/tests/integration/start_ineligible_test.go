//go:build integration

package integration

import (
	"net/http"
	"testing"
)

// T020 — F101-AC-08 (TC-PTY-006): a member not eligible (area locked; boss for grade 4 only, module game MOCK) → 409
// PARTY_ACTIVITY_NOT_ELIGIBLE with student_id + reason code, nothing written (no activity, room, membership), nobody in a
// session, the refusal audited (ids and reason codes only).
func TestStartIneligible(t *testing.T) {
	store, _ := valkeyLobby(t)
	game := &fakeGame{}
	e := setupParty(t, popts{lobby: store, game: game})
	e.area("forest", []string{"g6_7"}, true)
	a, b := e.kidA(), e.kidA()
	e.unlock(a, "forest")
	id := e.party(a, b)
	e.lobbyReady(id, "explore", "forest", a, b)

	r := e.startSession(a, id, key())
	if r.status != http.StatusConflict || r.code() != "PARTY_ACTIVITY_NOT_ELIGIBLE" {
		t.Fatalf("locked area: %d %s", r.status, r.raw)
	}
	bad, _ := r.body["ineligible_members"].([]any)
	if len(bad) != 1 || bad[0].(map[string]any)["student_id"] != b.id.String() || bad[0].(map[string]any)["reason_code"] != "area_locked" {
		t.Fatalf("ineligible_members: %s", r.raw)
	}
	noPII(t, r)

	// Boss for grade 4 only: both members refused, module game never asked to start.
	e.lobbyReady(id, "boss", "b-golem", a, b)
	r = e.startSession(a, id, key())
	if r.status != http.StatusConflict || r.code() != "PARTY_ACTIVITY_NOT_ELIGIBLE" || len(r.body["ineligible_members"].([]any)) != 2 {
		t.Fatalf("boss: %d %s", r.status, r.raw)
	}
	if game.startN != 0 {
		t.Fatalf("startPartyActivity called %d times", game.startN)
	}
	for q, want := range map[string]int{
		`SELECT count(*) FROM multiplayer.party_activities WHERE party_id = $1`:                                        0,
		`SELECT count(*) FROM multiplayer.session_memberships WHERE party_id = $1`:                                     0,
		`SELECT count(*) FROM multiplayer.parties WHERE party_id = $1 AND status = 'open'`:                             1,
		`SELECT count(*) FROM multiplayer.rooms r WHERE r.room_type IN ('private_instance','boss') AND $1::text <> ''`: 0,
		`SELECT count(*) FROM infra.outbox_events WHERE event_type = 'party.activity_started' AND $1::text <> ''`:      0,
	} {
		if n := e.count(q, id); n != want {
			t.Fatalf("%s: %d", q, n)
		}
	}
	for _, k := range []kid{a, b} {
		if got := e.push.of(k.id, "party.session.started"); len(got) != 0 {
			t.Fatalf("%s entered a session", k.nickname)
		}
	}
	if au := e.audits("party.session_start_denied"); len(au) != 2 || au[0]["reason"] != "not_eligible" {
		t.Fatalf("audit: %v", au)
	}
}
