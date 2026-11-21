//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// T014 — F101-AC-06 (TC-PTY-004): getParty gives the full lobby (members, presence, Ready, selected activity, state_version,
// leader grace); a member back online receives party.state.snapshot from the presence sweeper; when Valkey lost the lobby
// state it is rebuilt from PostgreSQL with Ready = false and the selected activity kept.
func TestLobbySnapshotRebuild(t *testing.T) {
	store, vk := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store})
	e.area("meadow", []string{"g6_7"}, false)
	a, b := e.kidA(), e.kidA()
	id := e.party(a, b)
	e.lobbyReady(id, "explore", "meadow", a, b)

	r := e.call(b, http.MethodGet, "/multiplayer/parties/"+id, "", nil)
	if r.status != http.StatusOK || r.body["leader_reconnecting"] != false {
		t.Fatalf("getParty: %d %s", r.status, r.raw)
	}
	if act, _ := r.body["current_activity"].(map[string]any); act["activity_ref"] != "meadow" {
		t.Fatalf("current_activity: %s", r.raw)
	}
	for _, m := range r.members() {
		if m["ready"] != true || m["presence"] != "online" {
			t.Fatalf("member: %v", m)
		}
	}
	v := r.body["state_version"]

	// B disconnects, comes back: the sweeper pushes the full snapshot to B.
	e.online.set(b.id, false)
	pid := uuid.MustParse(id)
	ref := partyRef(e, id)
	if err := e.parties.SweepParty(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	e.online.set(b.id, true)
	if err := e.parties.SweepParty(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	snaps := e.push.of(b.id, "party.state.snapshot")
	if len(snaps) == 0 {
		t.Fatal("no party.state.snapshot on reconnection")
	}
	s := snaps[len(snaps)-1]
	if s["state_version"] != v || s["current_activity"].(map[string]any)["id"] != "meadow" || len(s["members"].([]any)) != 2 {
		t.Fatalf("snapshot: %v", s)
	}

	// Valkey loses the lobby state: Ready false, selection kept (PostgreSQL), state_version unchanged.
	if err := vk.Do(context.Background(), vk.B().Flushall().Build()).Error(); err != nil {
		t.Fatal(err)
	}
	r = e.call(a, http.MethodGet, "/multiplayer/parties/"+id, "", nil)
	if r.status != http.StatusOK || r.body["state_version"] != v {
		t.Fatalf("rebuilt: %d %s", r.status, r.raw)
	}
	if act, _ := r.body["current_activity"].(map[string]any); act["activity_ref"] != "meadow" {
		t.Fatalf("selection lost: %s", r.raw)
	}
	for _, m := range r.members() {
		if m["ready"] != false {
			t.Fatalf("Ready after the loss: %v", m)
		}
	}
	if l, _ := store.Load(context.Background(), pid); len(l.Ready()) != 0 {
		t.Fatalf("lobby state after the loss: %+v", l)
	}
}
