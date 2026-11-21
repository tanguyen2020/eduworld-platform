//go:build integration

package integration

import (
	"net/http"
	"testing"
)

// T032 — F100-AC-20 (TC-PTY-014, AC-PTY-011): the leader disbands → status disbanded with disbanded_at, every member
// leaves (disbanded), pending invitations expire, one party.disbanded (leader_disbanded) + audit, party.party.disbanded
// to every member; a second call → 204 without a second event; a member who is not the leader → 403.
func TestDisbandParty(t *testing.T) {
	e := setupParty(t, popts{})
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b)
	e.friends(a, c)
	inv := e.invite(a, id, c, http.StatusCreated).str("invitation_id")
	if r := e.call(b, http.MethodPost, "/multiplayer/parties/"+id+":disband", "", nil); r.status != http.StatusForbidden ||
		r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("by member: %d %s", r.status, r.raw)
	}
	for i := 0; i < 2; i++ {
		if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+":disband", "", nil); r.status != http.StatusNoContent {
			t.Fatalf("disband %d: %d %s", i, r.status, r.raw)
		}
	}
	if ev := e.events("party.disbanded"); len(ev) != 1 || ev[0]["reason"] != "leader_disbanded" || ev[0]["actor_id"] != a.id.String() {
		t.Fatalf("one event: %v", ev)
	}
	if au := e.audits("party.disbanded"); len(au) != 1 {
		t.Fatalf("audit: %v", au)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE party_id = $1 AND left_reason = 'disbanded'`, id); n != 2 {
		t.Fatalf("members left: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_invitations WHERE invitation_id = $1 AND status = 'expired'
		AND end_reason = 'party_disbanded'`, inv); n != 1 {
		t.Fatalf("invitation: %d", n)
	}
	for _, k := range []kid{a, b} {
		if got := e.push.of(k.id, "party.party.disbanded"); len(got) != 1 {
			t.Fatalf("push to %s: %v", k.nickname, got)
		}
		if r := e.call(k, http.MethodGet, "/multiplayer/parties/me", "", nil); r.status != http.StatusNotFound {
			t.Fatalf("still in a party: %s", r.raw)
		}
	}
	if r := e.accept(c, inv, http.StatusNotFound); r.status != http.StatusNotFound {
		t.Fatalf("accept after disband: %s", r.raw)
	}
	// Members can start a new party at once.
	e.create(b, http.StatusCreated, nil)
	e.checkEventSchemas()
}

// T032 — F100-AC-21, AC-22: the leader updates name/visibility/invitation_policy/max_members (≥ members, ≤ cap),
// party.settings_updated + party.state.snapshot to members; max_members below the members → 422; a member → 403
// PARTY_NOT_LEADER. Every change of the party is in its append-only history in the same transaction.
func TestPartySettingsAndHistory(t *testing.T) {
	e := setupParty(t, popts{})
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	path := "/multiplayer/parties/" + id
	if r := e.call(b, http.MethodPatch, path, "", map[string]any{"name": "Của B"}); r.status != http.StatusForbidden || r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("member: %d %s", r.status, r.raw)
	}
	if r := e.call(a, http.MethodPatch, path, "", map[string]any{"max_members": 2}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("below members: %d %s", r.status, r.raw)
	}
	r := e.call(a, http.MethodPatch, path, "", map[string]any{"name": "Nhóm Mới", "visibility": "friends", "max_members": 5})
	if r.status != http.StatusOK || r.str("name") != "Nhóm Mới" || r.str("visibility") != "friends" || r.body["max_members"].(float64) != 5 {
		t.Fatalf("update: %d %s", r.status, r.raw)
	}
	version := r.body["state_version"].(float64)
	if ev := e.events("party.settings_updated"); len(ev) != 1 || len(ev[0]["changed_fields"].([]any)) != 3 {
		t.Fatalf("event: %v", ev)
	}
	for _, k := range []kid{a, b, c} {
		got := e.push.of(k.id, "party.state.snapshot")
		if len(got) != 1 || got[0]["state_version"].(float64) != version || got[0]["visibility"] != "friends" {
			t.Fatalf("snapshot to %s: %v", k.nickname, got)
		}
	}
	// No change → no event.
	e.call(a, http.MethodPatch, path, "", map[string]any{"visibility": "friends"})
	if ev := e.events("party.settings_updated"); len(ev) != 1 {
		t.Fatalf("no-op event: %d", len(ev))
	}
	// History: created, joined ×2, settings, then kick, leave, transfer (leader leaves), disband.
	if r := e.call(a, http.MethodPost, path+"/members/"+c.id.String()+":kick", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	if r := e.call(a, http.MethodPost, path+":leave", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	if r := e.call(b, http.MethodPost, path+":disband", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	h := e.history(id)
	want := []string{"created", "member_joined", "member_joined", "settings_updated", "member_kicked", "member_left", "leader_transferred",
		"disbanded"}
	if len(h) != len(want) {
		t.Fatalf("history %v, want %v", h, want)
	}
	for i := range want {
		if h[i] != want[i] {
			t.Fatalf("history %v, want %v", h, want)
		}
	}
	// state_version grows by exactly one per change of membership or settings.
	if n := e.count(`SELECT count(*) FROM (SELECT (data->>'state_version')::bigint v FROM multiplayer.party_histories WHERE party_id = $1
		AND event_type <> 'leader_transferred' GROUP BY v HAVING count(*) > 1) d`, id); n != 0 {
		t.Fatalf("state_version reused: %d", n)
	}
	e.checkEventSchemas()
}
