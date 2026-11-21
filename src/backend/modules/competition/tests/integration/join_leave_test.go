//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// T030 — F100-AC-16 (TC-PTY-011): class parties take students of the leader's class, friends parties the friends of
// the leader; a private party without an accepted invitation → 403 PARTY_POLICY_VIOLATION; a repeated join is
// idempotent.
func TestJoinParty(t *testing.T) {
	e := setupParty(t, popts{})
	a := e.kidA()
	id := e.create(a, http.StatusCreated, map[string]any{"name": "Lớp 6A", "party_type": "quest", "visibility": "class", "max_members": 8}).
		str("party_id")
	mate := e.kidA()
	otherClass := e.kid(e.tenantA, e.schoolA, e.classA2, 6)
	k := key()
	r := e.call(mate, http.MethodPost, "/multiplayer/parties/"+id+":join", k, nil)
	if r.status != http.StatusOK || len(r.members()) != 2 {
		t.Fatalf("join class: %d %s", r.status, r.raw)
	}
	if again := e.call(mate, http.MethodPost, "/multiplayer/parties/"+id+":join", key(), nil); again.status != http.StatusOK ||
		len(again.members()) != 2 {
		t.Fatalf("join again: %d %s", again.status, again.raw)
	}
	if r := e.call(otherClass, http.MethodPost, "/multiplayer/parties/"+id+":join", key(), nil); r.status != http.StatusForbidden ||
		r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("other class: %d %s", r.status, r.raw)
	}
	if got := e.push.of(a.id, "party.member.joined"); len(got) != 1 {
		t.Fatalf("leader told: %v", got)
	}
	if ev := e.events("party.member_joined"); len(ev) != 1 || ev[0]["via"] != "join" {
		t.Fatalf("event: %v", ev)
	}
	// friends: the friend of the leader joins, a stranger does not.
	if u := e.call(a, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"visibility": "friends"}); u.status != http.StatusOK {
		t.Fatal(u.raw)
	}
	friend := e.kid(e.tenantA, e.schoolA, e.classA2, 6)
	e.friends(a, friend)
	if r := e.call(friend, http.MethodPost, "/multiplayer/parties/"+id+":join", key(), nil); r.status != http.StatusOK {
		t.Fatalf("join friends: %d %s", r.status, r.raw)
	}
	if r := e.call(otherClass, http.MethodPost, "/multiplayer/parties/"+id+":join", key(), nil); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("stranger: %s", r.raw)
	}
	// private: no accepted invitation → refused.
	if u := e.call(a, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"visibility": "private"}); u.status != http.StatusOK {
		t.Fatal(u.raw)
	}
	stranger := e.kidA()
	if r := e.call(stranger, http.MethodPost, "/multiplayer/parties/"+id+":join", key(), nil); r.status != http.StatusForbidden ||
		r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("private: %d %s", r.status, r.raw)
	}
	// A party of another tenant is unknown (404).
	dung := e.kid(e.tenantB, e.schoolB, uuid.Nil, 6)
	if r := e.call(dung, http.MethodPost, "/multiplayer/parties/"+id+":join", key(), nil); r.status != http.StatusNotFound {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
}

// T030 — F100-AC-17 (TC-PTY-014, PARTY_SYSTEM §9): a member leaves (left); the leader leaves → the online member who
// joined first leads (leader_left), nobody online → the member who joined first; the last one out disbands (all_left).
func TestLeaveParty(t *testing.T) {
	e := setupParty(t, popts{})
	a, b, c, d := e.kidA(), e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b)
	e.clock.Add(time.Second)
	e.friends(a, c)
	e.accept(c, e.invite(a, id, c, http.StatusCreated).str("invitation_id"), http.StatusOK)
	e.clock.Add(time.Second)
	e.friends(a, d)
	e.accept(d, e.invite(a, id, d, http.StatusCreated).str("invitation_id"), http.StatusOK)
	// d leaves.
	if r := e.call(d, http.MethodPost, "/multiplayer/parties/"+id+":leave", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("leave: %d %s", r.status, r.raw)
	}
	if got := e.push.of(a.id, "party.member.left"); len(got) != 1 || got[0]["reason"] != "left" || got[0]["user_id"] != d.id.String() {
		t.Fatalf("member.left: %v", got)
	}
	if r := e.call(d, http.MethodGet, "/multiplayer/parties/"+id, "", nil); r.status != http.StatusNotFound {
		t.Fatalf("former member reads: %d", r.status)
	}
	// The leader leaves while c (joined after b) is the only online member → c leads.
	e.online.set(c.id, true)
	if r := e.call(a, http.MethodPost, "/multiplayer/parties/"+id+":leave", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("leader leaves: %d %s", r.status, r.raw)
	}
	r := e.call(b, http.MethodGet, "/multiplayer/parties/"+id, "", nil)
	if r.str("leader_id") != c.id.String() {
		t.Fatalf("new leader (online first): %s", r.raw)
	}
	if ev := e.events("party.leader_transferred"); len(ev) != 1 || ev[0]["reason"] != "leader_left" || ev[0]["to"] != c.id.String() {
		t.Fatalf("leader_transferred: %v", ev)
	}
	if got := e.push.of(b.id, "party.leader.transferred"); len(got) != 1 || got[0]["reason"] != "leader_left" {
		t.Fatalf("push: %v", got)
	}
	// c leaves with nobody online → b (joined first) leads.
	e.online.set(c.id, false)
	if r := e.call(c, http.MethodPost, "/multiplayer/parties/"+id+":leave", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	if r := e.call(b, http.MethodGet, "/multiplayer/parties/me", "", nil); r.str("leader_id") != b.id.String() {
		t.Fatalf("fallback leader: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE party_id = $1 AND left_at IS NULL AND role = 'leader'`, id); n != 1 {
		t.Fatalf("active leaders: %d", n)
	}
	// Last one out → disbanded (all_left).
	if r := e.call(b, http.MethodPost, "/multiplayer/parties/"+id+":leave", "", nil); r.status != http.StatusNoContent {
		t.Fatal(r.raw)
	}
	if ev := e.events("party.disbanded"); len(ev) != 1 || ev[0]["reason"] != "all_left" {
		t.Fatalf("disbanded: %v", ev)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.parties WHERE party_id = $1 AND status = 'disbanded' AND disbanded_at IS NOT NULL`, id); n != 1 {
		t.Fatalf("status: %d", n)
	}
	if r := e.call(b, http.MethodPost, "/multiplayer/parties/"+id+":leave", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("leave twice: %d", r.status)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE party_id = $1 AND left_reason = 'left'`, id); n != 4 {
		t.Fatalf("left rows: %d", n)
	}
	e.checkEventSchemas()
}
