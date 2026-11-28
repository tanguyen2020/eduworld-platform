//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// T028 — F095-AC-23 (TC-CHAT-047, ADR-0078 §3 as changed by the owner 2026-11-28, CHILD_SAFETY §3.6): in a common tenant
// friends are ON by default (sendFriendRequest, createFriendCode, redeemFriendCode work like in a school tenant); the System
// Admin incident switch (chat.common_tenant_social_overrides.friends_enabled = false) turns them off: 403
// SOCIAL_SCOPE_VIOLATION and nothing written; block and mute stay available. The legacy settings_subset key
// common_social_enabled no longer decides anything.
func TestCommonTenantFriendsDefaultOn(t *testing.T) {
	e := setup(t, envOpts{})
	common := uuid.New()
	e.tenant(common, "common", map[string]any{"common_social_enabled": false})
	school := uuid.New()
	a, b, c, d := e.student(common, school), e.student(common, school), e.student(common, school), e.student(common, school)
	e.send(a, b, http.StatusCreated)
	if r := e.call(a, http.MethodPost, "/social/friend-codes", newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("create code by default: %d %s", r.status, r.raw)
	}

	// System Admin switches friends off for this common tenant (the row putCommonTenantSocialOverride writes).
	e.exec(`INSERT INTO chat.common_tenant_social_overrides (tenant_id, direct_enabled, world_enabled, area_free_text_enabled,
		friends_enabled, reason) VALUES ($1, true, true, false, false, 'Su co spam ket ban')`, common)
	before := e.count(`SELECT count(*) FROM social.friend_requests`) + e.count(`SELECT count(*) FROM social.friend_codes`) +
		e.count(`SELECT count(*) FROM infra.outbox_events`)
	if r := e.send(c, d, http.StatusForbidden); r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("send switched off: %s", r.raw)
	}
	if r := e.call(c, http.MethodPost, "/social/friend-codes", newKey(), nil); r.status != http.StatusForbidden ||
		r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("create code switched off: %d %s", r.status, r.raw)
	}
	if r := e.call(d, http.MethodPost, "/social/friend-codes:redeem", newKey(), map[string]any{"code": "ABCD1234"}); r.status != http.StatusForbidden ||
		r.code() != "SOCIAL_SCOPE_VIOLATION" {
		t.Fatalf("redeem switched off: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests`) + e.count(`SELECT count(*) FROM social.friend_codes`) +
		e.count(`SELECT count(*) FROM infra.outbox_events`); n != before {
		t.Fatalf("%d rows written while friends are switched off", n-before)
	}
	// Block and mute stay available: they protect the student (CHILD_SAFETY).
	if r := e.call(c, http.MethodPut, "/social/mutes/"+d.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("mute in common tenant: %d %s", r.status, r.raw)
	}

	// A school tenant never reads the common switch.
	other, sch := uuid.New(), uuid.New()
	e.tenant(other, "school", nil)
	e.send(e.student(other, sch), e.student(other, sch), http.StatusCreated)

	// Switching back on restores friends.
	e.exec(`UPDATE chat.common_tenant_social_overrides SET friends_enabled = true WHERE tenant_id = $1`, common)
	e.send(e.student(common, school), e.student(common, school), http.StatusCreated)
}
