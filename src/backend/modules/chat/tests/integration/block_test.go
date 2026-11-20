//go:build integration

package integration

import (
	"net/http"
	"slices"
	"testing"
)

// T019 — F095-AC-14, AC-15, AC-16 (TC-CHAT-005, SC-002): blocking ends the friendship (friend.removed reason blocked),
// cancels the pending requests of both directions and publishes user.blocked exactly once with the audit, all in one
// transaction; blocking again is 204 without a second event; afterwards requests between the two are refused with 403
// SOCIAL_SCOPE_VIOLATION in both directions (no hint of the direction); unblocking publishes user.unblocked and does
// not restore the friendship; unblocking someone not blocked → 404. The blocked student is never notified.
func TestBlockUser(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh, chi := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	e.befriend(an, binh)
	pendingIn := requestID(e.send(chi, an, http.StatusCreated))
	pendingOut := requestID(e.send(an, chi, http.StatusCreated))

	// an blocks binh (friend) and chi (two pending requests).
	for _, s := range []student{binh, chi} {
		if r := e.call(an, http.MethodPut, "/social/blocks/"+s.id.String(), "", nil); r.status != http.StatusNoContent {
			t.Fatalf("block %s: %d %s", s.nickname, r.status, r.raw)
		}
	}
	if r := e.call(an, http.MethodPut, "/social/blocks/"+binh.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("block again: %d", r.status)
	}
	if blocked := e.events("user.blocked"); len(blocked) != 2 || blocked[0]["blocker_id"] != an.id.String() ||
		blocked[0]["blocked_id"] != binh.id.String() {
		t.Fatalf("user.blocked %v", blocked)
	}
	removed := e.events("friend.removed")
	if len(removed) != 1 || removed[0]["reason"] != "blocked" {
		t.Fatalf("friend.removed %v", removed)
	}
	if n := e.count(`SELECT count(*) FROM social.friendships WHERE deleted_at IS NULL`); n != 0 {
		t.Fatalf("friendship still live: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests WHERE request_id IN ($1, $2) AND status = 'cancelled'`, pendingIn, pendingOut); n != 2 {
		t.Fatalf("cancelled requests: %d", n)
	}
	audits := e.audits()
	if c := slices.IndexFunc(audits, func(a string) bool { return a == "social.user_blocked" }); c < 0 {
		t.Fatalf("audit %v", audits)
	}
	// Requests in both directions → 403 SOCIAL_SCOPE_VIOLATION, same answer whoever blocked.
	for _, pair := range [][2]student{{an, binh}, {binh, an}, {chi, an}, {an, chi}} {
		if r := e.send(pair[0], pair[1], http.StatusForbidden); r.code() != "SOCIAL_SCOPE_VIOLATION" {
			t.Fatalf("blocked pair: %s", r.raw)
		}
	}
	// The blocked student sees nothing: no friend, no block listed, no request.
	for path, want := range map[string]int{"/social/friends": 0, "/social/blocks": 0, "/social/friend-requests?direction=outgoing": 0} {
		if r := e.call(binh, http.MethodGet, path, "", nil); len(r.data()) != want {
			t.Fatalf("%s of the blocked student: %s", path, r.raw)
		}
	}
	// Unblock: user.unblocked, friendship not restored; unblocking again → 404.
	if r := e.call(an, http.MethodDelete, "/social/blocks/"+binh.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("unblock: %d %s", r.status, r.raw)
	}
	if r := e.call(an, http.MethodDelete, "/social/blocks/"+binh.id.String(), "", nil); r.status != http.StatusNotFound {
		t.Fatalf("unblock twice: %d", r.status)
	}
	if r := e.call(binh, http.MethodDelete, "/social/blocks/"+an.id.String(), "", nil); r.status != http.StatusNotFound {
		t.Fatalf("unblock by the blocked student: %d", r.status)
	}
	if un := e.events("user.unblocked"); len(un) != 1 {
		t.Fatalf("user.unblocked %v", un)
	}
	if r := e.call(an, http.MethodGet, "/social/friends", "", nil); len(r.data()) != 0 {
		t.Fatalf("friendship restored: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM social.blocks WHERE blocker_id = $1 AND deleted_at IS NOT NULL`, an.id); n != 1 {
		t.Fatalf("soft-deleted blocks: %d", n)
	}
	e.send(binh, an, http.StatusCreated) // allowed again after the unblock
	if n := e.checkEventSchemas(); n["user.blocked"] != 2 || n["user.unblocked"] != 1 || n["friend.removed"] != 1 {
		t.Fatalf("events checked %v", n)
	}
}
