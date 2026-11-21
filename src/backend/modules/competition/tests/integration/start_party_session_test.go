//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
)

// T019 — F101-AC-07, AC-09 (TC-PTY-005, TC-PTY-006): the leader starts an explore session when every online member is Ready:
// one transaction with the running activity, its private_instance room, one session membership per participant, the party
// in_activity and party.activity_started; 201 and party.session.started with the same room_id to every participant; an
// offline member stays out; a member → 403 PARTY_NOT_LEADER; a member not Ready → 409 PARTY_NOT_READY; no
// Idempotency-Key → 428; the same key again → the same result and one activity.
func TestStartPartySession(t *testing.T) {
	store, _ := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store})
	e.area("meadow", []string{"g6_7"}, false)
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	e.lobbyReady(id, "explore", "meadow", a, b)
	e.online.set(c.id, false)

	if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+":start", "", map[string]any{"expected_version": e.version(a, id)}); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no Idempotency-Key: %d %s", r.status, r.raw)
	}
	if r := e.startSession(b, id, key()); r.status != http.StatusForbidden || r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("member starts: %d %s", r.status, r.raw)
	}
	if r := e.ready(b, id, false); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	if r := e.startSession(a, id, key()); r.status != http.StatusConflict || r.code() != "PARTY_NOT_READY" {
		t.Fatalf("B not ready: %d %s", r.status, r.raw)
	}
	if r := e.ready(b, id, true); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	stale := e.version(a, id) - 1
	if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+":start", key(), map[string]any{"expected_version": stale}); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale version: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_activities WHERE party_id = $1`, id); n != 0 {
		t.Fatalf("refused starts wrote %d activities", n)
	}

	k, body := key(), map[string]any{"expected_version": e.version(a, id)}
	r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+":start", k, body)
	if r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	room, act := r.str("room_id"), r.str("party_activity_id")
	if r.str("room_type") != "private_instance" || r.str("game_session_ref") != act || len(r.body["members"].([]any)) != 2 {
		t.Fatalf("PartySessionStart: %s", r.raw)
	}
	if ex, _ := r.body["excluded_members"].([]any); len(ex) != 1 || ex[0].(map[string]any)["student_id"] != c.id.String() {
		t.Fatalf("offline C excluded: %s", r.raw)
	}
	again := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+":start", k, body)
	if again.status != http.StatusCreated || again.str("room_id") != room || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s %v", again.status, again.raw, again.header)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_activities WHERE party_id = $1`, id); n != 1 {
		t.Fatalf("activities: %d", n)
	}
	if s := e.one(`SELECT status || '/' || room_type || '/' || area_id FROM multiplayer.rooms WHERE room_id = $1`, room); s != "open/private_instance/meadow" {
		t.Fatalf("room: %s", s)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE party_activity_id = $1 AND room_id = $2 AND left_at IS NULL
		AND student_id IN ($3, $4)`, act, room, a.id, b.id); n != 2 {
		t.Fatalf("memberships: %d", n)
	}
	if s := e.one(`SELECT status || '/' || current_activity_id::text FROM multiplayer.parties WHERE party_id = $1`, id); s != "in_activity/"+act {
		t.Fatalf("party: %s", s)
	}
	ev := e.events("party.activity_started")
	if len(ev) != 1 || ev[0]["room_id"] != room || len(ev[0]["member_ids"].([]any)) != 2 {
		t.Fatalf("party.activity_started: %v", ev)
	}
	for _, kk := range []kid{a, b} {
		got := e.push.of(kk.id, "party.session.started")
		if len(got) != 1 || got[0]["room_id"] != room || got[0]["activity"].(map[string]any)["id"] != "meadow" {
			t.Fatalf("session.started to %s: %v", kk.nickname, got)
		}
	}
	if got := e.push.of(c.id, "party.session.started"); len(got) != 0 {
		t.Fatalf("offline C entered the session: %v", got)
	}
	if g := e.call(b, http.MethodGet, "/multiplayer/parties/"+id, "", nil); g.str("status") != "in_activity" || g.str("current_room_id") != room {
		t.Fatalf("getParty in session: %s", g.raw)
	}
	if r := e.startSession(a, id, key()); r.status != http.StatusUnprocessableEntity || r.code() != "INVALID_STATE_TRANSITION" {
		t.Fatalf("second start: %d %s", r.status, r.raw)
	}
	if seen := checkLobbyEventSchemas(t, e); seen["party.activity_started"] != 1 {
		t.Fatalf("schemas: %v", seen)
	}

	// Realtime command: the game session of the caller is recorded on its membership.
	d, f := e.kidA(), e.kidA()
	id2 := e.party(d, f)
	e.lobbyReady(id2, "explore", "meadow", d, f)
	caller := d.caller()
	rep := natscmd.SessionStart(e.parties)(context.Background(), caller, wsPayload(t, map[string]any{"party_id": id2,
		"expected_version": e.version(d, id2)}))
	if !rep.OK {
		t.Fatalf("ws start: %+v", rep.Problem)
	}
	if s := e.one(`SELECT game_session_id::text FROM multiplayer.session_memberships WHERE party_id = $1 AND student_id = $2`, id2, d.id); s != caller.GameSessionID.String() {
		t.Fatalf("game session of the leader: %s", s)
	}
	if s := e.one(`SELECT coalesce(game_session_id::text, 'unknown') FROM multiplayer.session_memberships WHERE party_id = $1 AND student_id = $2`,
		id2, f.id); s != "unknown" {
		t.Fatalf("game session of F is set when F enters the room: %s", s)
	}
	_ = uuid.Nil
}
