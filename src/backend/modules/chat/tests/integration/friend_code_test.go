//go:build integration

package integration

import (
	"net/http"
	"regexp"
	"testing"
	"time"
)

var codeRe = regexp.MustCompile(`^[A-Z0-9]{6,12}$`)

// T012 — F095-AC-08 (TC-SOC-001): createFriendCode returns a code ^[A-Z0-9]{6,12}$ expiring after friend_code_ttl; a
// new code replaces the previous one; redeemFriendCode by another student of the tenant creates a request (source
// friend_code) with the scope checks of sendFriendRequest; an unknown, replaced or expired code → 404
// FRIEND_CODE_INVALID; a malformed code → 422; the own code → 422; a code of another tenant is unknown (404).
func TestFriendCodes(t *testing.T) {
	e := setup(t, envOpts{})
	an, chi := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	key := newKey()
	r := e.call(an, http.MethodPost, "/social/friend-codes", key, nil)
	code, _ := r.body["code"].(string)
	if r.status != http.StatusCreated || !codeRe.MatchString(code) {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	if exp, err := time.Parse(time.RFC3339Nano, r.body["expires_at"].(string)); err != nil || !exp.Equal(e.clock.Now().Add(10*time.Minute)) {
		t.Fatalf("expires_at %v", r.body["expires_at"])
	}
	if again := e.call(an, http.MethodPost, "/social/friend-codes", key, nil); again.body["code"] != code {
		t.Fatalf("replay gave another code: %s", again.raw)
	}
	second := e.call(an, http.MethodPost, "/social/friend-codes", newKey(), nil)
	code2, _ := second.body["code"].(string)
	if second.status != http.StatusCreated || code2 == code {
		t.Fatalf("new code: %d %s", second.status, second.raw)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_codes WHERE owner_id = $1 AND deleted_at IS NULL`, an.id); n != 1 {
		t.Fatalf("%d live codes", n)
	}
	redeem := func(s student, c string) resp {
		return e.call(s, http.MethodPost, "/social/friend-codes:redeem", newKey(), map[string]any{"code": c})
	}
	if r := redeem(chi, code); r.status != http.StatusNotFound || r.code() != "FRIEND_CODE_INVALID" {
		t.Fatalf("replaced code: %d %s", r.status, r.raw)
	}
	if r := redeem(chi, "ZZZZ9999"); r.status != http.StatusNotFound || r.code() != "FRIEND_CODE_INVALID" {
		t.Fatalf("unknown code: %d %s", r.status, r.raw)
	}
	for _, bad := range []string{"abc", "abcdefgh", "TOO-LONG-CODE"} {
		if r := redeem(chi, bad); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("malformed %q: %d %s", bad, r.status, r.raw)
		}
	}
	if r := redeem(an, code2); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("own code: %d %s", r.status, r.raw)
	}
	dung := e.student(e.tenantB, e.schoolB)
	if r := redeem(dung, code2); r.status != http.StatusNotFound || r.code() != "FRIEND_CODE_INVALID" {
		t.Fatalf("code of another tenant: %d %s", r.status, r.raw)
	}
	ok := redeem(chi, code2)
	if ok.status != http.StatusCreated || ok.body["status"] != "pending" {
		t.Fatalf("redeem: %d %s", ok.status, ok.raw)
	}
	if a, _ := ok.body["addressee"].(map[string]any); a["user_id"] != an.id.String() {
		t.Fatalf("addressee %v", ok.body["addressee"])
	}
	sent := e.events("friend_request.sent")
	if len(sent) != 1 || sent[0]["source"] != "friend_code" {
		t.Fatalf("friend_request.sent %v", sent)
	}
	// Same scope as a direct request: another school → 403; a block → 403.
	other := e.student(e.tenantA, e.schoolA2)
	if r := redeem(other, code2); r.status != http.StatusForbidden || r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("other school: %d %s", r.status, r.raw)
	}
	binh := e.student(e.tenantA, e.schoolA)
	if r := e.call(an, http.MethodPut, "/social/blocks/"+binh.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("block: %d", r.status)
	}
	if r := redeem(binh, code2); r.status != http.StatusForbidden || r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("blocked: %d %s", r.status, r.raw)
	}
	// Expired code → 404 FRIEND_CODE_INVALID.
	e.clock.Add(10*time.Minute + time.Second)
	if r := redeem(e.student(e.tenantA, e.schoolA), code2); r.status != http.StatusNotFound || r.code() != "FRIEND_CODE_INVALID" {
		t.Fatalf("expired code: %d %s", r.status, r.raw)
	}
}
