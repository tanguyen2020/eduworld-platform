//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// T013 — F095-AC-10 (TC-AUTH-014, AGENTS R23): every social route × {school_admin, academic_manager, tenant_admin,
// safety_moderator, system_admin} with portal tokens → 403 GAME_ACCESS_STUDENT_ONLY; nothing is written.
func TestSocialRoutesStaffDenied(t *testing.T) {
	e := setup(t, envOpts{})
	an := e.student(e.tenantA, e.schoolA)
	tokens := map[string]string{
		"school_admin":     e.staff(e.tenantA, "school_admin"),
		"academic_manager": e.staff(e.tenantA, "academic_manager"),
		"tenant_admin":     e.staff(e.tenantA, "tenant_admin"),
		"safety_moderator": e.platformStaff("safety_moderator"),
		"system_admin":     e.platformStaff("system_admin"),
	}
	for _, id := range chathttp.PlayerOperations {
		o := chathttp.Operations[id]
		path := strings.NewReplacer("{user_id}", an.id.String(), "{request_id}", uuid.NewString()).Replace(o.Pattern)
		var body any
		switch id {
		case "sendFriendRequest":
			body = map[string]any{"addressee_id": an.id}
		case "redeemFriendCode":
			body = map[string]any{"code": "ABCD1234"}
		}
		for role, tok := range tokens {
			r := e.do(o.Method, path, tok, map[string]string{"X-Tenant-Id": e.tenantA.String(), "Idempotency-Key": newKey()}, body)
			want := "GAME_ACCESS_STUDENT_ONLY"
			if role == "safety_moderator" && r.code() == "TENANT_MISMATCH" {
				want = r.code() // platform staff of no tenant: X-Tenant-Id is refused before the guard, still 403
			}
			if r.status != http.StatusForbidden || r.code() != want {
				t.Fatalf("%s by %s: %d %s", id, role, r.status, r.raw)
			}
		}
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests`) + e.count(`SELECT count(*) FROM social.blocks`) +
		e.count(`SELECT count(*) FROM social.mutes`) + e.count(`SELECT count(*) FROM social.friend_codes`); n != 0 {
		t.Fatalf("rows written by staff: %d", n)
	}
	coverRoleDenied(t)
}

// T013 — TC-TEN-001, F095-AC-02, AC-11, AC-13, AC-18: a student of tenant A never reaches a relation of tenant B: ids of
// tenant B answer 404, lists of tenant A never contain tenant B rows, and the token of A with X-Tenant-Id of B is
// refused (403 TENANT_MISMATCH).
func TestSocialTenantIsolation(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	dung, em := e.student(e.tenantB, e.schoolB), e.student(e.tenantB, e.schoolB)
	e.befriend(dung, em)
	e.send(em, dung, http.StatusConflict) // already friends
	x := e.student(e.tenantB, e.schoolB)
	reqB := requestID(e.send(x, dung, http.StatusCreated))
	for _, p := range []string{"/social/blocks/", "/social/mutes/"} {
		if r := e.call(dung, http.MethodPut, p+em.id.String(), "", nil); r.status != http.StatusNoContent {
			t.Fatalf("%s in B: %d", p, r.status)
		}
	}
	codeB, _ := e.call(dung, http.MethodPost, "/social/friend-codes", newKey(), nil).body["code"].(string)

	// Ids of tenant B from tenant A → 404.
	b := dung.id.String()
	for id, call := range map[string]func() resp{
		"sendFriendRequest": func() resp {
			return e.call(an, http.MethodPost, "/social/friend-requests", newKey(), map[string]any{"addressee_id": b})
		},
		"acceptFriendRequest": func() resp {
			return e.call(an, http.MethodPost, "/social/friend-requests/"+reqB+":accept", newKey(), nil)
		},
		"rejectFriendRequest": func() resp { return e.call(an, http.MethodPost, "/social/friend-requests/"+reqB+":reject", "", nil) },
		"removeFriend":        func() resp { return e.call(an, http.MethodDelete, "/social/friends/"+b, "", nil) },
		"blockUser":           func() resp { return e.call(an, http.MethodPut, "/social/blocks/"+b, "", nil) },
		"unblockUser":         func() resp { return e.call(an, http.MethodDelete, "/social/blocks/"+b, "", nil) },
		"muteUser":            func() resp { return e.call(an, http.MethodPut, "/social/mutes/"+b, "", nil) },
		"unmuteUser":          func() resp { return e.call(an, http.MethodDelete, "/social/mutes/"+b, "", nil) },
		"redeemFriendCode": func() resp {
			return e.call(an, http.MethodPost, "/social/friend-codes:redeem", newKey(), map[string]any{"code": codeB})
		},
	} {
		if r := call(); r.status != http.StatusNotFound {
			t.Errorf("%s with an id of tenant B: %d %s", id, r.status, r.raw)
		}
	}
	// Lists of tenant A never show tenant B rows (tenant B has friends, requests, blocks and mutes).
	e.befriend(an, binh)
	for id, path := range map[string]string{"listFriends": "/social/friends", "listBlocks": "/social/blocks", "listMutes": "/social/mutes",
		"listFriendRequests": "/social/friend-requests?direction=incoming"} {
		r := e.call(an, http.MethodGet, path, "", nil)
		if r.status != http.StatusOK || strings.Contains(r.raw, dung.id.String()) || strings.Contains(r.raw, em.id.String()) ||
			strings.Contains(r.raw, x.id.String()) {
			t.Errorf("%s of tenant A: %d %s", id, r.status, r.raw)
		}
	}
	if r := e.call(an, http.MethodGet, "/social/friends", "", nil); len(r.data()) != 1 || r.data()[0]["user_id"] != binh.id.String() {
		t.Fatalf("friends of A: %s", r.raw)
	}
	// The token of A with X-Tenant-Id of tenant B → 403, nothing created in B.
	r := e.do(http.MethodPost, "/social/friend-codes", an.token, map[string]string{"X-Tenant-Id": e.tenantB.String(), "Idempotency-Key": newKey()}, nil)
	if r.status != http.StatusForbidden {
		t.Fatalf("A in tenant B: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_codes WHERE owner_id = $1`, an.id); n != 0 {
		t.Fatalf("%d codes of A", n)
	}
	coverCrossTenant(t)
}

// coverRoleDenied declares the role_denied case of the 14 social operations, verified by TestSocialRoutesStaffDenied
// (tools/routematrix reads the literals).
func coverRoleDenied(t *testing.T) {
	authztest.Cover(t, "listFriends", authztest.RoleDenied)
	authztest.Cover(t, "removeFriend", authztest.RoleDenied)
	authztest.Cover(t, "listFriendRequests", authztest.RoleDenied)
	authztest.Cover(t, "sendFriendRequest", authztest.RoleDenied)
	authztest.Cover(t, "acceptFriendRequest", authztest.RoleDenied)
	authztest.Cover(t, "rejectFriendRequest", authztest.RoleDenied)
	authztest.Cover(t, "createFriendCode", authztest.RoleDenied)
	authztest.Cover(t, "redeemFriendCode", authztest.RoleDenied)
	authztest.Cover(t, "listBlocks", authztest.RoleDenied)
	authztest.Cover(t, "blockUser", authztest.RoleDenied)
	authztest.Cover(t, "unblockUser", authztest.RoleDenied)
	authztest.Cover(t, "listMutes", authztest.RoleDenied)
	authztest.Cover(t, "muteUser", authztest.RoleDenied)
	authztest.Cover(t, "unmuteUser", authztest.RoleDenied)
}

// coverCrossTenant declares the cross_tenant case of the 14 social operations, verified by TestSocialTenantIsolation.
func coverCrossTenant(t *testing.T) {
	authztest.Cover(t, "listFriends", authztest.CrossTenant)
	authztest.Cover(t, "removeFriend", authztest.CrossTenant)
	authztest.Cover(t, "listFriendRequests", authztest.CrossTenant)
	authztest.Cover(t, "sendFriendRequest", authztest.CrossTenant)
	authztest.Cover(t, "acceptFriendRequest", authztest.CrossTenant)
	authztest.Cover(t, "rejectFriendRequest", authztest.CrossTenant)
	authztest.Cover(t, "createFriendCode", authztest.CrossTenant)
	authztest.Cover(t, "redeemFriendCode", authztest.CrossTenant)
	authztest.Cover(t, "listBlocks", authztest.CrossTenant)
	authztest.Cover(t, "blockUser", authztest.CrossTenant)
	authztest.Cover(t, "unblockUser", authztest.CrossTenant)
	authztest.Cover(t, "listMutes", authztest.CrossTenant)
	authztest.Cover(t, "muteUser", authztest.CrossTenant)
	authztest.Cover(t, "unmuteUser", authztest.CrossTenant)
}
