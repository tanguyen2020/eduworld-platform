//go:build integration

package integration

import (
	"net/http"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// T010 — F095-AC-01, AC-04 (TC-SOC-001): a request to a classmate of the same school is pending, expires after
// friend_request_ttl, returns 201 FriendRequest with public cards, writes friend_request.sent and the audit in the same
// transaction; the same Idempotency-Key replays the same response without a second request; already requested,
// already friends → 409 CONFLICT; to oneself → 422.
func TestSendFriendRequest(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	key := newKey()
	r := e.call(an, http.MethodPost, "/social/friend-requests", key, map[string]any{"addressee_id": binh.id})
	if r.status != http.StatusCreated || r.body["status"] != "pending" || requestID(r) == "" {
		t.Fatalf("send: %d %s", r.status, r.raw)
	}
	pii(t, r)
	exp, err := time.Parse(time.RFC3339Nano, r.body["expires_at"].(string))
	if err != nil || !exp.Equal(e.clock.Now().Add(7*24*time.Hour)) {
		t.Fatalf("expires_at %v (%v)", r.body["expires_at"], err)
	}
	if card, _ := r.body["addressee"].(map[string]any); card["nickname"] != binh.nickname || card["user_id"] != binh.id.String() {
		t.Fatalf("addressee card %v", r.body["addressee"])
	}
	again := e.call(an, http.MethodPost, "/social/friend-requests", key, map[string]any{"addressee_id": binh.id})
	if again.status != http.StatusCreated || requestID(again) != requestID(r) || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s %v", again.status, again.raw, again.header)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests WHERE requester_id = $1`, an.id); n != 1 {
		t.Fatalf("%d requests after a replay", n)
	}
	sent := e.events("friend_request.sent")
	if len(sent) != 1 || sent[0]["requester_id"] != an.id.String() || sent[0]["addressee_id"] != binh.id.String() ||
		sent[0]["source"] != "direct" {
		t.Fatalf("friend_request.sent %v", sent)
	}
	if !slices.Contains(e.audits(), "social.friend_request_sent") {
		t.Fatalf("audit %v", e.audits())
	}
	// Same pair again (new key) → 409 CONFLICT; to oneself → 422.
	if r := e.send(an, binh, http.StatusConflict); r.code() != "CONFLICT" {
		t.Fatalf("duplicate: %s", r.raw)
	}
	if r := e.send(an, an, http.StatusUnprocessableEntity); r.code() != "VALIDATION_FAILED" {
		t.Fatalf("self: %s", r.raw)
	}
	// Already friends → 409 CONFLICT, both ways.
	if a := e.call(binh, http.MethodPost, "/social/friend-requests/"+requestID(r)+":accept", newKey(), nil); a.status != http.StatusNoContent {
		t.Fatalf("accept: %d %s", a.status, a.raw)
	}
	e.send(an, binh, http.StatusConflict)
	e.send(binh, an, http.StatusConflict)
	if n := e.checkEventSchemas(); n["friend_request.sent"] != 1 || n["friend.added"] != 1 {
		t.Fatalf("events checked %v", n)
	}
}

// T010 — F095-AC-02 (TC-SOC-001, TC-TEN-001, ADR-0054): another tenant or a student who is not active → 404; another
// school of a school tenant, or of a unit tenant without cross_school_social_enabled → 403 SOCIAL_SCOPE_VIOLATION;
// once the unit tenant enables it → 201. Nothing is written for a refused request.
func TestSendFriendRequestScope(t *testing.T) {
	e := setup(t, envOpts{})
	an := e.student(e.tenantA, e.schoolA)
	dung := e.student(e.tenantB, e.schoolB)
	if r := e.send(an, dung, http.StatusNotFound); r.code() != "NOT_FOUND" {
		t.Fatalf("other tenant: %s", r.raw)
	}
	e.send(an, student{id: uuid.New()}, http.StatusNotFound)
	gone := e.student(e.tenantA, e.schoolA)
	e.exec(`UPDATE ref.students SET status = 'deactivated' WHERE student_id = $1`, gone.id)
	e.send(an, gone, http.StatusNotFound)
	cleaned := e.student(e.tenantA, e.schoolA)
	e.exec(`UPDATE ref.students SET deleted_at = now() WHERE student_id = $1`, cleaned.id)
	e.send(an, cleaned, http.StatusNotFound)

	other := e.student(e.tenantA, e.schoolA2)
	if r := e.send(an, other, http.StatusForbidden); r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("other school: %s", r.raw)
	}
	u1, u2 := e.student(e.tenantU, e.schoolU1), e.student(e.tenantU, e.schoolU2)
	if r := e.send(u1, u2, http.StatusForbidden); r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("unit tenant off: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests`); n != 0 {
		t.Fatalf("%d requests written by refused calls", n)
	}
	e.tenant(e.tenantU, "unit", map[string]any{"cross_school_social_enabled": true})
	e.send(u1, u2, http.StatusCreated)
	// A school tenant never goes cross-school, even with the flag (it only applies to unit tenants).
	e.tenant(e.tenantA, "school", map[string]any{"cross_school_social_enabled": true})
	e.send(an, other, http.StatusForbidden)
}

// T010 — F095-AC-03 (TC-SOC-002): the 11th pending request → 409 FRIEND_LIMIT_REACHED; the 21st request of the
// platform day → 429 RATE_LIMITED with Retry-After until the next day; the 101st friend → 409 FRIEND_LIMIT_REACHED.
func TestFriendLimits(t *testing.T) {
	e := setup(t, envOpts{})
	an := e.student(e.tenantA, e.schoolA)
	others := make([]student, 22)
	for i := range others {
		others[i] = e.student(e.tenantA, e.schoolA)
	}
	for i := 0; i < 10; i++ {
		e.send(an, others[i], http.StatusCreated)
	}
	if r := e.send(an, others[10], http.StatusConflict); r.code() != "FRIEND_LIMIT_REACHED" {
		t.Fatalf("11th pending: %s", r.raw)
	}
	// The addressees reject: no more pending (rejected ones are not pending), but the day quota counts every request.
	e.exec(`UPDATE social.friend_requests SET status = 'rejected', responded_at = now() WHERE requester_id = $1`, an.id)
	for i := 10; i < 20; i++ {
		e.send(an, others[i], http.StatusCreated)
	}
	e.exec(`UPDATE social.friend_requests SET status = 'rejected', responded_at = now() WHERE requester_id = $1 AND status = 'pending'`, an.id)
	r := e.send(an, others[20], http.StatusTooManyRequests)
	if r.code() != "RATE_LIMITED" {
		t.Fatalf("21st of the day: %s", r.raw)
	}
	// 03:00 UTC = 10:00 ICT: the day ends in 14 h.
	if s, _ := strconv.Atoi(r.header.Get("Retry-After")); s != 14*3600 {
		t.Fatalf("Retry-After %q", r.header.Get("Retry-After"))
	}
	e.clock.Add(14 * time.Hour)
	e.send(an, others[20], http.StatusCreated)

	// 100 friends: no new request, and nobody can make a 101st friend accept.
	chi := e.student(e.tenantA, e.schoolA)
	for i := 0; i < 100; i++ {
		f := uuid.New()
		e.exec(`INSERT INTO ref.students (student_id, tenant_id, school_id, status, source_version) VALUES ($1, $2, $3, 'active', 1)`,
			f, e.tenantA, e.schoolA)
		lo, hi := chi.id, f
		if lo.String() > hi.String() {
			lo, hi = hi, lo
		}
		e.exec(`INSERT INTO social.friendships (friendship_id, tenant_id, user_low_id, user_high_id) VALUES ($1, $2, $3, $4)`,
			uuid.New(), e.tenantA, lo, hi)
	}
	if r := e.send(chi, others[21], http.StatusConflict); r.code() != "FRIEND_LIMIT_REACHED" {
		t.Fatalf("101st friend: %s", r.raw)
	}
	id := requestID(e.send(others[21], chi, http.StatusCreated))
	if r := e.call(chi, http.MethodPost, "/social/friend-requests/"+id+":accept", newKey(), nil); r.status != http.StatusConflict ||
		r.code() != "FRIEND_LIMIT_REACHED" {
		t.Fatalf("accept as 101st friend: %d %s", r.status, r.raw)
	}
}

// T010 — F095-AC-05: only the addressee accepts a live pending request: exactly one friendship (normalized pair),
// request accepted, friend.added + audit; someone else, the requester, an expired or handled request → 404. Two
// requests crossing each other and accepted at the same time still give exactly one friendship, the other request
// cancelled (edge case).
func TestAcceptFriendRequest(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh, chi := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	id := requestID(e.send(an, binh, http.StatusCreated))
	accept := func(s student, id string) resp {
		return e.call(s, http.MethodPost, "/social/friend-requests/"+id+":accept", newKey(), nil)
	}
	for _, s := range []student{chi, an} {
		if r := accept(s, id); r.status != http.StatusNotFound {
			t.Fatalf("accept by %s: %d %s", s.nickname, r.status, r.raw)
		}
	}
	if r := accept(binh, uuid.NewString()); r.status != http.StatusNotFound {
		t.Fatalf("unknown request: %d", r.status)
	}
	key := newKey()
	if r := e.call(binh, http.MethodPost, "/social/friend-requests/"+id+":accept", key, nil); r.status != http.StatusNoContent {
		t.Fatalf("accept: %d %s", r.status, r.raw)
	}
	if r := e.call(binh, http.MethodPost, "/social/friend-requests/"+id+":accept", key, nil); r.status != http.StatusNoContent ||
		r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replayed accept: %d %v", r.status, r.header)
	}
	if r := accept(binh, id); r.status != http.StatusNotFound {
		t.Fatalf("handled request: %d", r.status)
	}
	if n := e.count(`SELECT count(*) FROM social.friendships WHERE deleted_at IS NULL AND (user_low_id = $1 OR user_high_id = $1)`, an.id); n != 1 {
		t.Fatalf("%d friendships", n)
	}
	if st := e.count(`SELECT count(*) FROM social.friend_requests WHERE request_id = $1 AND status = 'accepted' AND responded_at IS NOT NULL`, id); st != 1 {
		t.Fatal("request not accepted")
	}
	added := e.events("friend.added")
	if len(added) != 1 || added[0]["user_low_id"].(string) >= added[0]["user_high_id"].(string) {
		t.Fatalf("friend.added %v", added)
	}
	if !slices.Contains(e.audits(), "social.friend_added") {
		t.Fatalf("audit %v", e.audits())
	}
	// Expired request → 404.
	dung := e.student(e.tenantA, e.schoolA)
	late := requestID(e.send(dung, chi, http.StatusCreated))
	e.clock.Add(7*24*time.Hour + time.Second)
	if r := accept(chi, late); r.status != http.StatusNotFound {
		t.Fatalf("expired: %d", r.status)
	}

	// Crossing requests accepted at once (TC race): exactly one friendship, the other request cancelled.
	for round := 0; round < 5; round++ {
		x, y := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
		xy, yx := requestID(e.send(x, y, http.StatusCreated)), requestID(e.send(y, x, http.StatusCreated))
		var wg sync.WaitGroup
		codes := make([]int, 2)
		wg.Add(2)
		go func() { defer wg.Done(); codes[0] = accept(y, xy).status }()
		go func() { defer wg.Done(); codes[1] = accept(x, yx).status }()
		wg.Wait()
		slices.Sort(codes)
		if codes[0] != http.StatusNoContent || codes[1] != http.StatusNotFound {
			t.Fatalf("round %d: %v", round, codes)
		}
		if n := e.count(`SELECT count(*) FROM social.friendships WHERE deleted_at IS NULL AND (user_low_id = $1 OR user_high_id = $1)`, x.id); n != 1 {
			t.Fatalf("round %d: %d friendships", round, n)
		}
		if n := e.count(`SELECT count(*) FROM social.friend_requests WHERE status = 'cancelled' AND request_id IN ($1, $2)`, xy, yx); n != 1 {
			t.Fatalf("round %d: %d cancelled", round, n)
		}
	}
}
