//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
)

// T012 — F101-AC-03, AC-04 (TC-PTY-005): the leader selects an activity with expected_version: durable selection, Ready of every
// member reset, state_version + 1, party.activity_selected and party.activity.selected with the ineligible members (reason codes
// only); a member → 403 PARTY_NOT_LEADER; a stale expected_version → 412 VERSION_MISMATCH with the current state_version.
func TestSelectActivity(t *testing.T) {
	store, _ := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store})
	e.area("forest", []string{"g6_7"}, true)
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	e.unlock(a, "forest")
	e.unlock(b, "forest")
	id := e.party(a, b, c)
	for _, k := range []kid{a, b, c} {
		e.online.set(k.id, true)
	}
	if r := e.ready(b, id, true); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	v := e.version(a, id)

	if r := e.pick(b, id, "explore", "forest", v); r.status != http.StatusForbidden || r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("member selects: %d %s", r.status, r.raw)
	}
	if r := e.pick(a, id, "explore", "forest", v-1); r.status != http.StatusPreconditionFailed || r.code() != "VERSION_MISMATCH" ||
		r.body["state_version"] != float64(v) {
		t.Fatalf("stale version: %d %s", r.status, r.raw)
	}
	r := e.pick(a, id, "explore", "forest", v)
	if r.status != http.StatusOK || r.body["state_version"] != float64(v+1) {
		t.Fatalf("select: %d %s", r.status, r.raw)
	}
	if act, _ := r.body["current_activity"].(map[string]any); act["activity_type"] != "explore" || act["activity_ref"] != "forest" {
		t.Fatalf("current_activity: %v", r.body["current_activity"])
	}
	for _, m := range r.members() {
		if m["ready"] != false {
			t.Fatalf("Ready not reset in the response: %v", m)
		}
	}
	if l, _ := store.Load(context.Background(), uuid.MustParse(id)); l.Member(b.id).Ready {
		t.Fatal("Ready of B not reset in the lobby state")
	}
	for _, k := range []kid{a, b, c} {
		got := e.push.of(k.id, "party.activity.selected")
		if len(got) != 1 {
			t.Fatalf("activity.selected to %s: %v", k.nickname, got)
		}
		bad, _ := got[0]["ineligible_members"].([]any)
		if len(bad) != 1 || bad[0].(map[string]any)["user_id"] != c.id.String() || bad[0].(map[string]any)["reason_code"] != "area_locked" {
			t.Fatalf("ineligible_members: %v", got[0])
		}
	}
	ev := e.events("party.activity_selected")
	if len(ev) != 1 || ev[0]["activity_type"] != "explore" || ev[0]["selected_by"] != a.id.String() || ev[0]["state_version"] != float64(v+1) {
		t.Fatalf("party.activity_selected: %v", ev)
	}
	checkLobbyEventSchemas(t, e)
	if s := e.one(`SELECT selected_activity_type || '/' || selected_activity_ref FROM multiplayer.parties WHERE party_id = $1`, id); s != "explore/forest" {
		t.Fatalf("durable selection: %s", s)
	}
	// Unknown area, activity not enabled in M4a, quest without module game.
	if r := e.pick(a, id, "explore", "nowhere", -1); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown area: %d %s", r.status, r.raw)
	}
	if r := e.pick(a, id, "team_battle", "x", -1); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("team_battle: %d %s", r.status, r.raw)
	}
	if r := e.pick(a, id, "quest", "q-bridge", -1); r.status != http.StatusServiceUnavailable || r.code() != "UNAVAILABLE" ||
		r.header.Get("Retry-After") == "" {
		t.Fatalf("quest without module game: %d %s %v", r.status, r.raw, r.header)
	}
	// Realtime command: same rules; a stale version is CONFLICT.
	time.Sleep(1100 * time.Millisecond) // a new window of party_lobby_cmd_rate
	h := natscmd.ActivitySelect(e.parties)
	cur := e.version(a, id)
	if rep := h(context.Background(), a.caller(), wsPayload(t, map[string]any{"party_id": id, "expected_version": cur - 1,
		"activity": map[string]any{"kind": "explore", "id": "forest"}})); rep.OK || rep.Problem.Code != "CONFLICT" {
		t.Fatalf("ws stale: %+v", rep)
	}
	if rep := h(context.Background(), b.caller(), wsPayload(t, map[string]any{"party_id": id, "expected_version": cur,
		"activity": map[string]any{"kind": "explore", "id": "forest"}})); rep.OK || rep.Problem.Code != "PARTY_NOT_LEADER" {
		t.Fatalf("ws member: %+v", rep)
	}
	if rep := h(context.Background(), a.caller(), wsPayload(t, map[string]any{"party_id": id, "expected_version": cur,
		"activity": map[string]any{"kind": "explore", "id": "forest"}})); !rep.OK {
		t.Fatalf("ws select: %+v", rep.Problem)
	}
}
