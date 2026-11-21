//go:build integration

package integration

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

// T021 — F100-AC-07, AC-11 (TC-PTY-002, TC-PTY-003): A invites B, C, D → pending invitations expiring after
// party_invite_ttl (server clock), party.invitation_sent and party.invitation.received to each invitee; B and C accept
// → members, party.member.joined to every member; a repeated accept never adds a second membership; D accepts once
// max_members = 3 is reached → 409 PARTY_FULL.
func TestInvitationFlow(t *testing.T) {
	e := setupParty(t, popts{})
	a, b, c, d := e.kidA(), e.kidA(), e.kidA(), e.kidA()
	for _, k := range []kid{b, c, d} {
		e.friends(a, k)
	}
	id := e.create(a, http.StatusCreated, map[string]any{"name": "Nhóm", "party_type": "friend", "max_members": 3}).str("party_id")
	invs := map[string]string{}
	for _, k := range []kid{b, c, d} {
		r := e.invite(a, id, k, http.StatusCreated)
		noPII(t, r)
		exp, _ := time.Parse(time.RFC3339, r.str("expires_at"))
		if r.str("status") != "pending" || !exp.Equal(e.clock.Now().Add(5*time.Minute)) || r.str("party_name") != "Nhóm" {
			t.Fatalf("invitation: %s", r.raw)
		}
		if inviter, _ := r.body["inviter"].(map[string]any); inviter["nickname"] != a.nickname {
			t.Fatalf("inviter card: %s", r.raw)
		}
		invs[k.nickname] = r.str("invitation_id")
		got := e.push.of(k.id, "party.invitation.received")
		if len(got) != 1 || got[0]["invitation_id"] != invs[k.nickname] || got[0]["member_count"].(float64) != 1 {
			t.Fatalf("push to %s: %v", k.nickname, got)
		}
	}
	if ev := e.events("party.invitation_sent"); len(ev) != 3 {
		t.Fatalf("invitation_sent: %d", len(ev))
	}
	k := key()
	r := e.call(b, http.MethodPost, "/multiplayer/invitations/"+invs[b.nickname]+":accept", k, nil)
	if r.status != http.StatusOK || len(r.members()) != 2 {
		t.Fatalf("accept B: %d %s", r.status, r.raw)
	}
	noPII(t, r)
	if again := e.call(b, http.MethodPost, "/multiplayer/invitations/"+invs[b.nickname]+":accept", k, nil); again.status != http.StatusOK ||
		again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if again := e.accept(b, invs[b.nickname], http.StatusOK); len(again.members()) != 2 { // new key: still one membership
		t.Fatalf("repeat accept: %s", again.raw)
	}
	for _, who := range []kid{a, b} {
		if got := e.push.of(who.id, "party.member.joined"); len(got) != 1 {
			t.Fatalf("member.joined to %s: %v", who.nickname, got)
		}
	}
	if got := e.push.of(a.id, "party.invitation.updated"); len(got) != 1 || got[0]["status"] != "accepted" {
		t.Fatalf("invitation.updated to inviter: %v", got)
	}
	e.accept(c, invs[c.nickname], http.StatusOK)
	if r := e.accept(d, invs[d.nickname], http.StatusConflict); r.code() != "PARTY_FULL" {
		t.Fatalf("full: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE party_id = $1 AND left_at IS NULL`, id); n != 3 {
		t.Fatalf("members: %d", n)
	}
	if ev := e.events("party.member_joined"); len(ev) != 2 || ev[0]["via"] != "invitation" {
		t.Fatalf("member_joined: %v", ev)
	}
	if ev := e.events("party.invitation_accepted"); len(ev) != 2 {
		t.Fatalf("invitation_accepted: %v", ev)
	}
	seen := e.checkEventSchemas()
	for _, typ := range []string{"party.created", "party.invitation_sent", "party.invitation_accepted", "party.member_joined"} {
		if seen[typ] == 0 {
			t.Fatalf("%s not checked: %v", typ, seen)
		}
	}
}

// T024 — F100-AC-12 (TC-PTY-003, AC-PTY-010): several accepts at once beyond max_members → exactly the free seats
// succeed, the rest 409 PARTY_FULL; an expired invitation → 410 PARTY_INVITATION_EXPIRED; another student → 403
// PERMISSION_DENIED.
func TestAcceptInvitationLimits(t *testing.T) {
	e := setupParty(t, popts{})
	a := e.kidA()
	kids := []kid{e.kidA(), e.kidA(), e.kidA(), e.kidA()}
	id := e.create(a, http.StatusCreated, map[string]any{"name": "Nhóm", "party_type": "friend", "max_members": 3}).str("party_id")
	invs := make([]string, len(kids))
	for i, k := range kids {
		e.friends(a, k)
		invs[i] = e.invite(a, id, k, http.StatusCreated).str("invitation_id")
	}
	var wg sync.WaitGroup
	res := make([]presp, len(kids))
	for i, k := range kids {
		wg.Add(1)
		go func(i int, k kid) {
			defer wg.Done()
			res[i] = e.call(k, http.MethodPost, "/multiplayer/invitations/"+invs[i]+":accept", key(), nil)
		}(i, k)
	}
	wg.Wait()
	ok, full := 0, 0
	for _, r := range res {
		switch {
		case r.status == http.StatusOK:
			ok++
		case r.status == http.StatusConflict && r.code() == "PARTY_FULL":
			full++
		default:
			t.Fatalf("accept: %d %s", r.status, r.raw)
		}
	}
	if ok != 2 || full != 2 {
		t.Fatalf("ok %d full %d", ok, full)
	}
	// Expired: a new party of another leader, clock past expires_at.
	l2, x, y := e.kidA(), e.kidA(), e.kidA()
	e.friends(l2, x)
	e.friends(l2, y)
	p2 := e.create(l2, http.StatusCreated, nil).str("party_id")
	invX := e.invite(l2, p2, x, http.StatusCreated).str("invitation_id")
	invY := e.invite(l2, p2, y, http.StatusCreated).str("invitation_id")
	if r := e.accept(x, invY, http.StatusForbidden); r.code() != "PERMISSION_DENIED" {
		t.Fatalf("not the invitee: %s", r.raw)
	}
	e.clock.Add(5*time.Minute + time.Second)
	if r := e.accept(x, invX, http.StatusGone); r.code() != "PARTY_INVITATION_EXPIRED" {
		t.Fatalf("expired: %s", r.raw)
	}
}
