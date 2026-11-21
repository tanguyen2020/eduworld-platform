//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
)

// T029 — F101-AC-14 (TC-PTY-008): the job party-presence-sweeper evaluates the tracked parties: a member whose presence expired
// becomes disconnected within one cycle, every member receives party.member.status_changed, its Ready resets in the lobby, it
// stays a member; back online → online again with the full snapshot; one replica per cycle (job lock).
func TestPresenceSweeper(t *testing.T) {
	store, _ := valkeyLobby(t)
	e := setupParty(t, popts{lobby: store})
	e.parties.Locks = store
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	for _, k := range []kid{a, b, c} {
		e.online.set(k.id, true)
	}
	if r := e.ready(b, id, true); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	ctx := context.Background()
	sw := command.NewSweeper(e.parties, 5*time.Second)
	if n, err := sw.Run(ctx); err != nil || n != 1 {
		t.Fatalf("first cycle: %d %v", n, err)
	}
	if n, err := sw.Run(ctx); err != nil || n != 0 {
		t.Fatalf("second cycle within the lease of the job lock: %d %v", n, err)
	}

	e.online.set(b.id, false)
	if err := e.parties.SweepParty(ctx, partyRef(e, id)); err != nil {
		t.Fatal(err)
	}
	for _, k := range []kid{a, b, c} {
		got := e.push.of(k.id, "party.member.status_changed")
		if len(got) != 1 || got[0]["user_id"] != b.id.String() || got[0]["status"] != "disconnected" {
			t.Fatalf("status_changed to %s: %v", k.nickname, got)
		}
	}
	rc := e.push.of(a.id, "party.member.ready_changed")
	if last := rc[len(rc)-1]; last["user_id"] != b.id.String() || last["ready"] != false {
		t.Fatalf("Ready of B reset: %v", rc)
	}
	if l, _ := store.Load(ctx, uuid.MustParse(id)); l.Member(b.id).Ready || l.Member(b.id).Status != "disconnected" {
		t.Fatalf("lobby state: %+v", l.Member(b.id))
	}
	if r := e.call(a, http.MethodGet, "/multiplayer/parties/"+id, "", nil); len(r.members()) != 3 {
		t.Fatalf("B still a member: %s", r.raw)
	}
	// No change, no push.
	if err := e.parties.SweepParty(ctx, partyRef(e, id)); err != nil {
		t.Fatal(err)
	}
	if got := e.push.of(a.id, "party.member.status_changed"); len(got) != 1 {
		t.Fatalf("pushes without change: %v", got)
	}
	e.online.set(b.id, true)
	if err := e.parties.SweepParty(ctx, partyRef(e, id)); err != nil {
		t.Fatal(err)
	}
	if got := e.push.of(c.id, "party.member.status_changed"); len(got) != 2 || got[1]["status"] != "online" {
		t.Fatalf("back online: %v", got)
	}
	if got := e.push.of(b.id, "party.state.snapshot"); len(got) == 0 {
		t.Fatal("no snapshot for B")
	}
}
