//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
)

// T025 — F100-AC-13 (TC-PTY-014): the invitee rejects (inviter told), the inviter or the leader cancels (invitee told);
// anyone else → 403 PERMISSION_DENIED.
func TestRejectAndCancelInvitation(t *testing.T) {
	e := setupParty(t, popts{})
	a, b := e.kidA(), e.kidA()
	id := e.party(a, b)
	if r := e.call(a, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"invitation_policy": "members"}); r.status != http.StatusOK {
		t.Fatal(r.raw)
	}
	c, d, x := e.kidA(), e.kidA(), e.kidA()
	for _, k := range []kid{c, d} {
		e.friends(b, k)
	}
	invC := e.invite(b, id, c, http.StatusCreated).str("invitation_id") // sent by member b
	invD := e.invite(b, id, d, http.StatusCreated).str("invitation_id")
	if r := e.call(x, http.MethodPost, "/multiplayer/invitations/"+invC+":reject", "", nil); r.status != http.StatusNotFound &&
		r.status != http.StatusForbidden {
		t.Fatalf("reject by another student: %d %s", r.status, r.raw)
	}
	if r := e.call(c, http.MethodPost, "/multiplayer/invitations/"+invC+":reject", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("reject: %d %s", r.status, r.raw)
	}
	if got := e.push.of(b.id, "party.invitation.updated"); len(got) != 1 || got[0]["status"] != "rejected" {
		t.Fatalf("inviter told: %v", got)
	}
	if r := e.call(c, http.MethodPost, "/multiplayer/invitations/"+invC+":reject", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("reject twice: %d", r.status)
	}
	// d (the invitee) and x (outsider) may not cancel; the leader a (not the inviter) may.
	for _, k := range []kid{d, x} {
		r := e.call(k, http.MethodPost, "/multiplayer/invitations/"+invD+":cancel", "", nil)
		if k == x && r.status == http.StatusNotFound {
			continue // x is not in the party: the invitation of another party is still not his to see
		}
		if r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
			t.Fatalf("cancel by %s: %d %s", k.nickname, r.status, r.raw)
		}
	}
	if r := e.call(a, http.MethodPost, "/multiplayer/invitations/"+invD+":cancel", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("cancel by leader: %d %s", r.status, r.raw)
	}
	if got := e.push.of(d.id, "party.invitation.updated"); len(got) != 1 || got[0]["status"] != "cancelled" {
		t.Fatalf("invitee told: %v", got)
	}
	if ev := e.events("party.invitation_rejected"); len(ev) != 1 {
		t.Fatalf("rejected events: %v", ev)
	}
	if ev := e.events("party.invitation_cancelled"); len(ev) != 1 || ev[0]["cancelled_by"] != a.id.String() || ev[0]["reason"] != "cancelled" {
		t.Fatalf("cancelled events: %v", ev)
	}
	if seen := e.checkEventSchemas(); seen["party.invitation_cancelled"] != 1 || seen["party.invitation_rejected"] != 1 {
		t.Fatalf("schemas: %v", seen)
	}
}

// T025 — F100-AC-14: a pending invitation expires past expires_at (job party-invitation-expiry), when its party is
// disbanded, when the invitee enters another party, and the oldest one when the invitee holds more than
// party_pending_invites_max; each with party.invitation_expired and its reason.
func TestInvitationExpiry(t *testing.T) {
	cfg := partyConfig
	cfg.PendingMax = 2
	e := setupParty(t, popts{config: &cfg})
	target := e.kidA()
	quest := func() (kid, string) {
		l := e.kidA()
		return l, e.create(l, http.StatusCreated, map[string]any{"name": "Quest", "party_type": "quest"}).str("party_id")
	}
	// ttl: the job expires it past expires_at, and only then.
	l1, p1 := quest()
	inv1 := e.invite(l1, p1, target, http.StatusCreated).str("invitation_id")
	run := func() int {
		n, err := e.parties.ExpireInvitations(context.Background(), persistence.Expiry{}, 100)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := run(); n != 0 {
		t.Fatalf("expired too early: %d", n)
	}
	e.clock.Add(5*time.Minute + time.Second)
	if n := run(); n != 1 {
		t.Fatalf("ttl: %d", n)
	}
	status := func(id string) (string, string) {
		var s, r string
		if err := e.su.QueryRow(`SELECT status, coalesce(end_reason, '') FROM multiplayer.party_invitations WHERE invitation_id = $1`, id).
			Scan(&s, &r); err != nil {
			t.Fatal(err)
		}
		return s, r
	}
	if s, r := status(inv1); s != "expired" || r != "ttl" {
		t.Fatalf("ttl: %s %s", s, r)
	}
	// party disbanded.
	l2, p2 := quest()
	inv2 := e.invite(l2, p2, target, http.StatusCreated).str("invitation_id")
	if r := e.call(l2, http.MethodPost, "/multiplayer/parties/"+p2+":disband", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	if s, r := status(inv2); s != "expired" || r != "party_disbanded" {
		t.Fatalf("disbanded: %s %s", s, r)
	}
	// overflow: party_pending_invites_max = 2 → the oldest of three expires.
	l3, p3 := quest()
	l4, p4 := quest()
	l5, p5 := quest()
	inv3 := e.invite(l3, p3, target, http.StatusCreated).str("invitation_id")
	e.clock.Add(time.Second)
	inv4 := e.invite(l4, p4, target, http.StatusCreated).str("invitation_id")
	e.clock.Add(time.Second)
	inv5 := e.invite(l5, p5, target, http.StatusCreated).str("invitation_id")
	if s, r := status(inv3); s != "expired" || r != "pending_overflow" {
		t.Fatalf("overflow: %s %s", s, r)
	}
	// joined another party: accepting inv5 expires inv4.
	e.accept(target, inv5, http.StatusOK)
	if s, r := status(inv4); s != "expired" || r != "joined_other_party" {
		t.Fatalf("joined other: %s %s", s, r)
	}
	reasons := map[any]int{}
	for _, p := range e.events("party.invitation_expired") {
		reasons[p["reason"]]++
	}
	for _, r := range []string{"ttl", "party_disbanded", "pending_overflow", "joined_other_party"} {
		if reasons[r] != 1 {
			t.Fatalf("expired events %v", reasons)
		}
	}
	e.checkEventSchemas()
}

// T025 — F100-AC-15 (TC-TEN-001, R16): listMyInvitations?status=pending lists the live invitations addressed to the
// caller only; the inviter is a public card (nickname, avatar).
func TestListMyInvitations(t *testing.T) {
	e := setupParty(t, popts{})
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	e.friends(a, b)
	e.friends(a, c)
	id := e.create(a, http.StatusCreated, nil).str("party_id")
	invB := e.invite(a, id, b, http.StatusCreated).str("invitation_id")
	e.invite(a, id, c, http.StatusCreated)
	r := e.call(b, http.MethodGet, "/multiplayer/invitations?status=pending", "", nil)
	if r.status != http.StatusOK {
		t.Fatalf("list: %d %s", r.status, r.raw)
	}
	noPII(t, r)
	data := r.data()
	if len(data) != 1 || data[0]["invitation_id"] != invB || data[0]["invitee_id"] != b.id.String() {
		t.Fatalf("only mine: %s", r.raw)
	}
	inviter, _ := data[0]["inviter"].(map[string]any)
	for k := range inviter {
		switch k {
		case "user_id", "nickname", "level", "avatar_config":
		default:
			t.Fatalf("inviter field %s: %s", k, r.raw)
		}
	}
	if inviter["nickname"] != a.nickname {
		t.Fatalf("inviter: %v", inviter)
	}
	// The inviter sees no invitation addressed to him; expired ones leave the pending list.
	if r := e.call(a, http.MethodGet, "/multiplayer/invitations?status=pending", "", nil); len(r.data()) != 0 {
		t.Fatalf("inviter list: %s", r.raw)
	}
	e.clock.Add(6 * time.Minute)
	if r := e.call(b, http.MethodGet, "/multiplayer/invitations?status=pending", "", nil); len(r.data()) != 0 {
		t.Fatalf("expired still pending: %s", r.raw)
	}
	if r := e.call(b, http.MethodGet, "/multiplayer/invitations", "", nil); len(r.data()) != 1 {
		t.Fatalf("all statuses: %s", r.raw)
	}
	_ = uuid.Nil
}
