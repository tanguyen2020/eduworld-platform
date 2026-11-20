//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// T028 — F095-AC-23 (TC-CHAT-047, ADR-0078, CHILD_SAFETY §3.6): in a common tenant, sendFriendRequest, createFriendCode
// and redeemFriendCode answer 403 SOCIAL_SCOPE_VIOLATION and write nothing (no request, no code, no event); once a
// System Admin re-enabled friends for that tenant (settings_subset.common_social_enabled, projected from the tenant
// settings) they work.
func TestCommonTenantFriendsOff(t *testing.T) {
	e := setup(t, envOpts{})
	common := uuid.New()
	e.tenant(common, "common", nil)
	school := uuid.New()
	a, b := e.student(common, school), e.student(common, school)
	if r := e.send(a, b, http.StatusForbidden); r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("send: %s", r.raw)
	}
	if r := e.call(a, http.MethodPost, "/social/friend-codes", newKey(), nil); r.status != http.StatusForbidden ||
		r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("create code: %d %s", r.status, r.raw)
	}
	if r := e.call(b, http.MethodPost, "/social/friend-codes:redeem", newKey(), map[string]any{"code": "ABCD1234"}); r.status != http.StatusForbidden ||
		r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("redeem: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests`) + e.count(`SELECT count(*) FROM social.friend_codes`) +
		e.count(`SELECT count(*) FROM infra.outbox_events`); n != 0 {
		t.Fatalf("%d rows written in a common tenant", n)
	}
	// Block and mute stay available: they protect the student (CHILD_SAFETY).
	if r := e.call(a, http.MethodPut, "/social/mutes/"+b.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("mute in common tenant: %d %s", r.status, r.raw)
	}

	e.tenant(common, "common", map[string]any{"common_social_enabled": true})
	e.send(a, b, http.StatusCreated)
	r := e.call(a, http.MethodPost, "/social/friend-codes", newKey(), nil)
	if r.status != http.StatusCreated {
		t.Fatalf("create code after re-enable: %d %s", r.status, r.raw)
	}
}
