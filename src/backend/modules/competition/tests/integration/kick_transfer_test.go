//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"
)

// T031 — F100-AC-18 (TC-PTY-014, AC-PTY-011): the leader kicks a member (kicked), the invitations the member sent are
// cancelled, the member may not be invited or join again for party_kick_reinvite_block, party.member_kicked + audit;
// a member who is not the leader → 403 PARTY_NOT_LEADER.
func TestKickPartyMember(t *testing.T) {
	e := setupParty(t, popts{})
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	if r := e.call(a, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"invitation_policy": "members"}); r.status != http.StatusOK {
		t.Fatal(r.raw)
	}
	x := e.kidA()
	e.friends(b, x)
	invX := e.invite(b, id, x, http.StatusCreated).str("invitation_id")
	if r := e.call(c, http.MethodPost, "/multiplayer/parties/"+id+"/members/"+b.id.String()+":kick", "", nil); r.status != http.StatusForbidden ||
		r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("kick by member: %d %s", r.status, r.raw)
	}
	if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+"/members/"+b.id.String()+":kick", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("kick: %d %s", r.status, r.raw)
	}
	if got := e.push.of(b.id, "party.member.left"); len(got) != 1 || got[0]["reason"] != "kicked" {
		t.Fatalf("kicked told: %v", got)
	}
	var status, reason string
	if err := e.su.QueryRow(`SELECT status, end_reason FROM multiplayer.party_invitations WHERE invitation_id = $1`, invX).
		Scan(&status, &reason); err != nil || status != "cancelled" || reason != "inviter_kicked" {
		t.Fatalf("invitation of the kicked member: %s %s %v", status, reason, err)
	}
	if ev := e.events("party.member_kicked"); len(ev) != 1 || ev[0]["kicked_by"] != a.id.String() {
		t.Fatalf("member_kicked: %v", ev)
	}
	if au := e.audits("party.member_kicked"); len(au) != 1 {
		t.Fatalf("audit: %v", au)
	}
	// Re-invite and join refused within party_kick_reinvite_block, allowed after.
	if r := e.invite(a, id, b, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("re-invite: %s", r.raw)
	}
	if u := e.call(a, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"visibility": "class"}); u.status != http.StatusOK {
		t.Fatal(u.raw)
	}
	if r := e.call(b, http.MethodPost, "/multiplayer/parties/"+id+":join", key(), nil); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("rejoin: %s", r.raw)
	}
	e.clock.Add(10*time.Minute + time.Second)
	if r := e.call(b, http.MethodPost, "/multiplayer/parties/"+id+":join", key(), nil); r.status != http.StatusOK {
		t.Fatalf("rejoin after the block: %d %s", r.status, r.raw)
	}
	// The leader cannot kick itself; a non-member → 404.
	if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+"/members/"+a.id.String()+":kick", "", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("self kick: %d %s", r.status, r.raw)
	}
	if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+"/members/"+x.id.String()+":kick", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("kick non-member: %d %s", r.status, r.raw)
	}
	if h := e.history(id); !contains(h, "member_kicked") {
		t.Fatalf("history: %v", h)
	}
}

// party_kick_rate (10 per hour per leader, scaled down): the next kick → 429 RATE_LIMITED.
func TestKickRate(t *testing.T) {
	cfg := partyConfig
	cfg.KickPerHour = 1
	e := setupParty(t, popts{config: &cfg})
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+"/members/"+b.id.String()+":kick", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+"/members/"+c.id.String()+":kick", "", nil)
	if r.status != http.StatusTooManyRequests || r.code() != "RATE_LIMITED" || r.header.Get("Retry-After") == "" {
		t.Fatalf("kick rate: %d %s", r.status, r.raw)
	}
}

// T031 — F100-AC-19 (TC-PTY-014): the leader transfers to an online member in one transaction (exactly one active
// leader, party.leader_transferred manual, audit); to an offline member → 409 PARTY_MEMBER_NOT_ONLINE; a member who is
// not the leader → 403 PARTY_NOT_LEADER.
func TestTransferPartyLeader(t *testing.T) {
	e := setupParty(t, popts{})
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	path := "/multiplayer/parties/" + id + ":transfer-leader"
	if r := e.call(a, http.MethodPost, path, key(), map[string]any{"new_leader_id": b.id}); r.status != http.StatusConflict ||
		r.code() != "PARTY_MEMBER_NOT_ONLINE" {
		t.Fatalf("offline: %d %s", r.status, r.raw)
	}
	if r := e.call(c, http.MethodPost, path, key(), map[string]any{"new_leader_id": b.id}); r.status != http.StatusForbidden ||
		r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("not leader: %d %s", r.status, r.raw)
	}
	if r := e.call(a, http.MethodPost, path, key(), map[string]any{"new_leader_id": e.kidA().id}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("not a member: %d %s", r.status, r.raw)
	}
	e.online.set(b.id, true)
	r := e.call(a, http.MethodPost, path, key(), map[string]any{"new_leader_id": b.id})
	if r.status != http.StatusOK || r.str("leader_id") != b.id.String() {
		t.Fatalf("transfer: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE party_id = $1 AND left_at IS NULL AND role = 'leader'`, id); n != 1 {
		t.Fatalf("leaders: %d", n)
	}
	if ev := e.events("party.leader_transferred"); len(ev) != 1 || ev[0]["reason"] != "manual" || ev[0]["from"] != a.id.String() {
		t.Fatalf("event: %v", ev)
	}
	if au := e.audits("party.leader_transferred"); len(au) != 1 {
		t.Fatalf("audit: %v", au)
	}
	for _, k := range []kid{a, b, c} {
		if got := e.push.of(k.id, "party.leader.transferred"); len(got) != 1 || got[0]["to_user_id"] != b.id.String() {
			t.Fatalf("push to %s: %v", k.nickname, got)
		}
	}
	// The former leader is a member now.
	if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+"/members/"+c.id.String()+":kick", "", nil); r.code() != "PARTY_NOT_LEADER" {
		t.Fatalf("former leader kicks: %s", r.raw)
	}
	e.checkEventSchemas()
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
