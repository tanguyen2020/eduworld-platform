//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// caller is the identity the realtime gateway puts in the Ew-* headers of a command of k.
func (k kid) caller() rt.Caller {
	return rt.Caller{TenantID: k.tenant, UserID: k.id, GameSessionID: uuid.New(), DeviceID: uuid.New(), ConnID: "c-" + k.id.String()[:8], NodeID: "n1"}
}

func wsPayload(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// T011 — F101-AC-01, AC-02 (TC-PTY-004): Ready over REST and over the realtime command updates the lobby state in Valkey and
// pushes party.member.ready_changed (all_ready) to every member, without any domain event; a party that is not open →
// 422 INVALID_STATE_TRANSITION; a non-member or another tenant → 404 PARTY_NOT_FOUND (PERMISSION_DENIED on WS); more
// than party_lobby_cmd_rate → 429 RATE_LIMITED.
func TestLobbyReady(t *testing.T) {
	store, _ := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store})
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	for _, k := range []kid{a, b, c} {
		e.online.set(k.id, true)
	}
	events := e.count(`SELECT count(*) FROM infra.outbox_events`)

	if r := e.ready(b, id, true); r.status != http.StatusNoContent {
		t.Fatalf("ready B: %d %s", r.status, r.raw)
	}
	for _, k := range []kid{a, b, c} {
		got := e.push.of(k.id, "party.member.ready_changed")
		if len(got) != 1 || got[0]["user_id"] != b.id.String() || got[0]["ready"] != true || got[0]["all_ready"] != false {
			t.Fatalf("ready_changed to %s: %v", k.nickname, got)
		}
	}
	if l, err := store.Load(context.Background(), uuid.MustParse(id)); err != nil || !l.Member(b.id).Ready || l.Member(a.id).Ready {
		t.Fatalf("lobby state: %+v %v", l, err)
	}
	// The realtime command uses the same use case (identity from the Ew-* headers only).
	h := natscmd.ReadySet(e.parties)
	if rep := h(context.Background(), a.caller(), wsPayload(t, map[string]any{"party_id": id, "ready": true})); !rep.OK {
		t.Fatalf("ws ready A: %+v", rep.Problem)
	}
	if rep := h(context.Background(), c.caller(), wsPayload(t, map[string]any{"party_id": id, "ready": true})); !rep.OK {
		t.Fatalf("ws ready C: %+v", rep.Problem)
	}
	last := e.push.of(b.id, "party.member.ready_changed")
	if len(last) != 3 || last[2]["all_ready"] != true {
		t.Fatalf("all_ready after the three: %v", last)
	}
	if n := e.count(`SELECT count(*) FROM infra.outbox_events`); n != events {
		t.Fatalf("Ready wrote %d domain events", n-events)
	}
	// getParty shows Ready (lobby snapshot).
	for _, m := range e.call(b, http.MethodGet, "/multiplayer/parties/"+id, "", nil).members() {
		if m["ready"] != true {
			t.Fatalf("member ready in getParty: %v", m)
		}
	}

	// F101-AC-02: outsiders, other tenant, not open, rate.
	out, other := e.kidA(), e.kid(e.tenantB, e.schoolB, uuid.Nil, 6)
	for _, k := range []kid{out, other} {
		if r := e.ready(k, id, true); r.status != http.StatusNotFound || r.code() != "PARTY_NOT_FOUND" {
			t.Fatalf("outsider %s: %d %s", k.nickname, r.status, r.raw)
		}
		if rep := h(context.Background(), k.caller(), wsPayload(t, map[string]any{"party_id": id, "ready": true})); rep.OK ||
			rep.Problem.Code != "PERMISSION_DENIED" {
			t.Fatalf("ws outsider: %+v", rep)
		}
	}
	e.exec(`UPDATE multiplayer.parties SET status = 'in_activity' WHERE party_id = $1`, id)
	if r := e.ready(b, id, false); r.status != http.StatusUnprocessableEntity || r.code() != "INVALID_STATE_TRANSITION" {
		t.Fatalf("not open: %d %s", r.status, r.raw)
	}
	e.exec(`UPDATE multiplayer.parties SET status = 'open' WHERE party_id = $1`, id)
	limited := false
	for i := 0; i < 8; i++ {
		r := e.ready(c, id, i%2 == 0)
		if r.status == http.StatusTooManyRequests {
			if r.code() != "RATE_LIMITED" || r.header.Get("Retry-After") == "" {
				t.Fatalf("429: %s %v", r.raw, r.header)
			}
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("party_lobby_cmd_rate 5/s never reached")
	}
	if r := e.call(b, http.MethodPut, "/multiplayer/parties/"+id+"/members/me/ready", "", map[string]any{}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("missing ready: %d %s", r.status, r.raw)
	}
}
