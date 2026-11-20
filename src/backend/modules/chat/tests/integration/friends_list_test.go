//go:build integration

package integration

import (
	"net/http"
	"slices"
	"testing"
)

// T017 — F095-AC-11 (TC-TEN-001, R16): listFriends returns only the caller's friends as PlayerCards (user_id,
// nickname, avatar_config, level, presence) without real name, school, class or scores; cursor paging; a cleaned-up
// friend is no longer listed.
func TestListFriends(t *testing.T) {
	e := setup(t, envOpts{})
	an := e.student(e.tenantA, e.schoolA)
	friends := make([]student, 3)
	for i := range friends {
		friends[i] = e.student(e.tenantA, e.schoolA)
		e.befriend(an, friends[i])
		e.clock.Add(1)
	}
	stranger, chi := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	e.befriend(stranger, chi) // a friendship the caller is not part of
	r := e.call(an, http.MethodGet, "/social/friends?limit=2", "", nil)
	if r.status != http.StatusOK || len(r.data()) != 2 {
		t.Fatalf("page 1: %d %s", r.status, r.raw)
	}
	pii(t, r)
	page, _ := r.body["page"].(map[string]any)
	next, _ := page["next_cursor"].(string)
	if page["has_more"] != true || next == "" {
		t.Fatalf("page info %v", page)
	}
	r2 := e.call(an, http.MethodGet, "/social/friends?limit=2&cursor="+next, "", nil)
	if r2.status != http.StatusOK || len(r2.data()) != 1 {
		t.Fatalf("page 2: %d %s", r2.status, r2.raw)
	}
	var got []string
	for _, c := range append(r.data(), r2.data()...) {
		got = append(got, c["user_id"].(string))
		if c["presence"] != "offline" || c["level"] != float64(4) || c["avatar_config"] == nil || c["nickname"] == "" {
			t.Fatalf("card %v", c)
		}
	}
	for _, f := range friends {
		if !slices.Contains(got, f.id.String()) {
			t.Fatalf("friend %s missing in %v", f.nickname, got)
		}
	}
	if slices.Contains(got, chi.id.String()) || slices.Contains(got, stranger.id.String()) {
		t.Fatalf("someone else's friendship listed: %v", got)
	}
	// Both sides see each other (symmetric friendship).
	if r := e.call(friends[0], http.MethodGet, "/social/friends", "", nil); len(r.data()) != 1 || r.data()[0]["user_id"] != an.id.String() {
		t.Fatalf("friends of the other side: %s", r.raw)
	}
	// A cleaned-up friend (student.cleaned_up) disappears from the list.
	e.exec(`UPDATE ref.students SET deleted_at = now() WHERE student_id = $1`, friends[1].id)
	if r := e.call(an, http.MethodGet, "/social/friends", "", nil); len(r.data()) != 2 {
		t.Fatalf("cleaned-up friend still listed: %s", r.raw)
	}
}

// T017 — F095-AC-12: removeFriend soft-deletes the friendship (row kept with deleted_at), publishes friend.removed
// (reason removed) and nothing addressed to the other student; both sides no longer see each other; not a friend →
// 404; befriending again creates a new row.
func TestRemoveFriend(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh, chi := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	e.befriend(an, binh)
	if r := e.call(an, http.MethodDelete, "/social/friends/"+chi.id.String(), "", nil); r.status != http.StatusNotFound {
		t.Fatalf("not a friend: %d", r.status)
	}
	if r := e.call(an, http.MethodDelete, "/social/friends/"+binh.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("remove: %d %s", r.status, r.raw)
	}
	if r := e.call(an, http.MethodDelete, "/social/friends/"+binh.id.String(), "", nil); r.status != http.StatusNotFound {
		t.Fatalf("remove twice: %d", r.status)
	}
	if n := e.count(`SELECT count(*) FROM social.friendships WHERE deleted_at IS NOT NULL AND delete_reason = 'removed' AND deleted_by = $1`, an.id); n != 1 {
		t.Fatalf("soft-deleted rows: %d", n)
	}
	removed := e.events("friend.removed")
	if len(removed) != 1 || removed[0]["reason"] != "removed" {
		t.Fatalf("friend.removed %v", removed)
	}
	for _, s := range []student{an, binh} {
		if r := e.call(s, http.MethodGet, "/social/friends", "", nil); len(r.data()) != 0 {
			t.Fatalf("still friends for %s: %s", s.nickname, r.raw)
		}
	}
	// No event addressed to binh other than the relation event (no notification, F123 consumes friend_request.sent only).
	if n := e.count(`SELECT count(*) FROM infra.outbox_events WHERE event_type NOT IN ('friend_request.sent', 'friend.added', 'friend.removed',
		'audit.recorded')`); n != 0 {
		t.Fatalf("%d other events", n)
	}
	e.befriend(binh, an)
	if n := e.count(`SELECT count(*) FROM social.friendships WHERE (user_low_id = $1 OR user_high_id = $1)`, an.id); n != 2 {
		t.Fatalf("rows after befriending again: %d", n)
	}
	if n := e.checkEventSchemas(); n["friend.removed"] != 1 || n["friend.added"] != 2 {
		t.Fatalf("events checked %v", n)
	}
}

// T017 — F095-AC-13 (NFR-SEC-006): listFriendRequests returns the caller's incoming (live pending) or outgoing
// requests with the PlayerCard of the other side; requests between other students never appear.
func TestListFriendRequests(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh, chi, dung := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA),
		e.student(e.tenantA, e.schoolA)
	in := requestID(e.send(binh, an, http.StatusCreated))
	out := requestID(e.send(an, chi, http.StatusCreated))
	other := requestID(e.send(chi, dung, http.StatusCreated))
	r := e.call(an, http.MethodGet, "/social/friend-requests", "", nil) // default incoming
	if r.status != http.StatusOK || len(r.data()) != 1 || r.data()[0]["request_id"] != in {
		t.Fatalf("incoming: %d %s", r.status, r.raw)
	}
	if req, _ := r.data()[0]["requester"].(map[string]any); req["nickname"] != binh.nickname {
		t.Fatalf("requester card %v", r.data()[0])
	}
	pii(t, r)
	o := e.call(an, http.MethodGet, "/social/friend-requests?direction=outgoing", "", nil)
	if len(o.data()) != 1 || o.data()[0]["request_id"] != out || o.data()[0]["status"] != "pending" {
		t.Fatalf("outgoing: %s", o.raw)
	}
	for _, rr := range []resp{r, o} {
		for _, d := range rr.data() {
			if d["request_id"] == other {
				t.Fatalf("someone else's request listed: %s", rr.raw)
			}
		}
	}
	if bad := e.call(an, http.MethodGet, "/social/friend-requests?direction=sideways", "", nil); bad.status < 400 || bad.status >= 500 {
		t.Fatalf("direction: %d", bad.status)
	}
}
