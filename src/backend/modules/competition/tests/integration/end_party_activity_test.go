//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// gameEvent delivers an event of module game to the consumer competition.party_activity_end in a tenant transaction, like
// pkg/outbox.Consumer (payload per EVENT_DESIGN §4 — fixture, schema owned by F103).
func gameEvent(t *testing.T, e *penv, typ string, tenant uuid.UUID, payload map[string]any) {
	t.Helper()
	b, _ := json.Marshal(payload)
	env := outbox.Envelope{EventID: uuid.New(), EventType: typ, TenantID: &tenant, OccurredAt: time.Now().UTC(), Payload: b}
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	h := consumer.PartyActivityEnd{H: e.parties}
	if err := e.uow.InTenantTx(context.Background(), sc, func(ctx context.Context) error { return h.Handle(ctx, env) }); err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
}

// T023 — F101-AC-12, AC-13 (TC-PTY-006, TC-PTY-010, TC-SVC-003): a party activity ends on quest.party_completed /
// boss.encounter_completed (idempotent), when every participant left, when the party is disbanded, on server failure; the
// memberships and the room close, the party returns to open with Ready reset, party.activity_ended (+ party.activity_completed
// when completed) and party.session.ended; no score, result or mastery of any student in the activity, the events or the
// messages (FR-PTY-013).
func TestEndPartyActivity(t *testing.T) {
	store, _ := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store, game: &fakeGame{}})
	e.area("meadow", []string{"g6_7"}, false)
	e.area("forest", []string{"g6_7"}, false)
	ctx := context.Background()

	start := func(typ, ref string, leader kid, others ...kid) (string, string) {
		t.Helper()
		id := e.party(leader, others...)
		e.lobbyReady(id, typ, ref, leader, others...)
		r := e.startSession(leader, id, key())
		if r.status != http.StatusCreated {
			t.Fatalf("start: %d %s", r.status, r.raw)
		}
		return id, r.str("party_activity_id")
	}

	// completed by module game, twice (idempotent).
	a, b := e.kidA(), e.kidA()
	id, act := start("quest", "q-bridge", a, b)
	v := e.version(a, id)
	payload := map[string]any{"party_quest_session_id": uuid.New(), "party_activity_id": act, "party_id": id, "outcome": "completed",
		"member_ids": []string{a.id.String(), b.id.String()}, "score": 97, "mastery": 0.9}
	gameEvent(t, e, "quest.party_completed", e.tenantA, payload)
	gameEvent(t, e, "quest.party_completed", e.tenantA, payload)
	if s := e.one(`SELECT status || '/' || end_reason || '/' || result_summary::text FROM multiplayer.party_activities WHERE party_activity_id = $1`,
		act); s != `ended/completed/{"outcome": "completed"}` {
		t.Fatalf("activity: %s", s)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.session_memberships WHERE party_activity_id = $1 AND leave_reason = 'room_closed'`, act); n != 2 {
		t.Fatalf("memberships closed: %d", n)
	}
	if s := e.one(`SELECT r.status FROM multiplayer.rooms r JOIN multiplayer.party_activities a ON a.room_id = r.room_id WHERE a.party_activity_id = $1`,
		act); s != "closed" {
		t.Fatalf("room: %s", s)
	}
	g := e.call(a, http.MethodGet, "/multiplayer/parties/"+id, "", nil)
	if g.str("status") != "open" || g.body["state_version"] != float64(v+1) || g.body["current_room_id"] != nil {
		t.Fatalf("party back to the lobby: %s", g.raw)
	}
	if n := len(e.events("party.activity_ended")); n != 1 {
		t.Fatalf("party.activity_ended: %d", n)
	}
	comp := e.events("party.activity_completed")
	if len(comp) != 1 || len(comp[0]["member_ids"].([]any)) != 2 {
		t.Fatalf("party.activity_completed: %v", comp)
	}
	for _, k := range []kid{a, b} {
		got := e.push.of(k.id, "party.session.ended")
		if len(got) != 1 || got[0]["outcome"] != "completed" || got[0]["reason"] != "completed" {
			t.Fatalf("session.ended to %s: %v", k.nickname, got)
		}
		raw, _ := json.Marshal(got[0])
		for _, w := range []string{"score", "mastery", "attempt", "contribution"} {
			if strings.Contains(string(raw), w) {
				t.Fatalf("session.ended leaks %s: %s", w, raw)
			}
		}
	}

	// boss.encounter_completed: only the shared result is kept, never contributions.
	c, d := e.kidA(), e.kidA()
	_, act2 := start("boss", "b-slime", c, d)
	gameEvent(t, e, "boss.encounter_completed", e.tenantA, map[string]any{"encounter_id": uuid.New(), "party_activity_id": act2,
		"result": "defeated", "contributions": []map[string]any{{"student_id": c.id, "eligible": true, "score": 12}}})
	if s := e.one(`SELECT end_reason || '/' || result_summary::text FROM multiplayer.party_activities WHERE party_activity_id = $1`, act2); s != `completed/{"result": "defeated"}` {
		t.Fatalf("boss end: %s", s)
	}
	// A solo encounter (no party_activity_id) is ignored; another tenant's event cannot end this party's activity.
	gameEvent(t, e, "boss.encounter_completed", e.tenantA, map[string]any{"encounter_id": uuid.New(), "result": "defeated"})

	// all_left: both participants leave the party (the third member stays offline in the lobby).
	f, g2, h := e.kidA(), e.kidA(), e.kidA()
	id3, act3 := startWithOffline(t, e, f, g2, h)
	_ = id3
	for _, k := range []kid{f, g2} {
		if r := e.call(k, http.MethodPost, "/multiplayer/parties/"+id3+":leave", "", nil); r.status != http.StatusNoContent {
			t.Fatalf("leave: %d %s", r.status, r.raw)
		}
	}
	if s := e.one(`SELECT end_reason FROM multiplayer.party_activities WHERE party_activity_id = $1`, act3); s != "all_left" {
		t.Fatalf("all_left: %s", s)
	}
	if s := e.one(`SELECT status FROM multiplayer.parties WHERE party_id = $1`, id3); s != "open" {
		t.Fatalf("party of H: %s", s)
	}

	// disbanded: the leader disbands during the session.
	i, j := e.kidA(), e.kidA()
	id4, act4 := start("explore", "meadow", i, j)
	if r := e.call(i, http.MethodPost, "/multiplayer/parties/"+id4+":disband", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	if s := e.one(`SELECT end_reason FROM multiplayer.party_activities WHERE party_activity_id = $1`, act4); s != "disbanded" {
		t.Fatalf("disbanded: %s", s)
	}
	if got := e.push.of(j.id, "party.session.ended"); len(got) != 1 || got[0]["outcome"] != "aborted" {
		t.Fatalf("session.ended on disband: %v", got)
	}

	// server_failure: a failure of the room (command level; no producer in Release 1.0).
	m, n := e.kidA(), e.kidA()
	_, act5 := start("explore", "forest", m, n)
	sc, _ := authz.EventScope(e.tenantA)
	if err := e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error {
		_, err := e.parties.EndPartyActivity(ctx, e.tenantA, uuid.MustParse(act5), "server_failure", nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := e.push.of(m.id, "party.session.ended"); len(got) != 1 || got[0]["outcome"] != "failed" {
		t.Fatalf("server_failure: %v", got)
	}

	if seen := checkLobbyEventSchemas(t, e); seen["party.activity_ended"] != 5 || seen["party.activity_completed"] != 2 {
		t.Fatalf("events: %v", seen)
	}
	var leaked int
	for _, typ := range lobbyEventTypes {
		for _, p := range e.events(typ) {
			raw, _ := json.Marshal(p)
			for _, w := range []string{"score", "mastery", "attempt", "contribution"} {
				if strings.Contains(string(raw), w) {
					leaked++
				}
			}
		}
	}
	if leaked != 0 {
		t.Fatalf("%d party events carry learning results", leaked)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_activities WHERE result_summary::text ~ '(score|mastery|attempt|contribution)'`); n != 0 {
		t.Fatalf("result_summary with learning results: %d", n)
	}
}

// startWithOffline starts an explore session of f and g while h, the third member, is offline.
func startWithOffline(t *testing.T, e *penv, f, g, h kid) (string, string) {
	t.Helper()
	id := e.party(f, g, h)
	e.lobbyReady(id, "explore", "meadow", f, g)
	e.online.set(h.id, false)
	r := e.startSession(f, id, key())
	if r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	return id, r.str("party_activity_id")
}
