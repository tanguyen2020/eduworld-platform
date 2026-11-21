//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

func partyBody(id string, target uuid.UUID) any {
	switch id {
	case "createParty":
		return map[string]any{"name": "Nhóm", "party_type": "friend"}
	case "updatePartySettings":
		return map[string]any{"name": "Nhóm"}
	case "inviteToParty":
		return map[string]any{"invitee_id": target}
	case "transferPartyLeader":
		return map[string]any{"new_leader_id": target}
	case "setPartyReady":
		return map[string]any{"ready": true}
	case "selectPartyActivity":
		return map[string]any{"activity_type": "explore", "activity_ref": "meadow", "expected_version": 0}
	case "startPartySession":
		return map[string]any{"expected_version": 0}
	}
	return nil
}

func (e *penv) opPath(id, party, invitation string, student uuid.UUID) string {
	o := competitionhttp.Operations[id]
	return strings.NewReplacer("{party_id}", party, "{invitation_id}", invitation, "{student_id}", student.String()).Replace(o.Pattern)
}

// T035 — F100-AC-23 (TC-AUTH-014, AGENTS R23): every party operation × {school_admin, academic_manager, tenant_admin,
// safety_moderator, system_admin} with portal tokens → 403 GAME_ACCESS_STUDENT_ONLY; nothing is written.
func TestPartyRoutesStaffDenied(t *testing.T) {
	e := setupParty(t, popts{})
	an := e.kidA()
	tokens := map[string]string{
		"school_admin":     e.staffToken(e.tenantA, "school_admin"),
		"academic_manager": e.staffToken(e.tenantA, "academic_manager"),
		"tenant_admin":     e.staffToken(e.tenantA, "tenant_admin"),
		"safety_moderator": e.platformToken("safety_moderator"),
		"system_admin":     e.platformToken("system_admin"),
	}
	for _, id := range competitionhttp.PlayerOperations {
		o := competitionhttp.Operations[id]
		path := e.opPath(id, uuid.NewString(), uuid.NewString(), an.id)
		for role, tok := range tokens {
			ct := ""
			if o.Method == http.MethodPatch {
				ct = "application/merge-patch+json"
			}
			r := e.do(o.Method, path, tok, map[string]string{"X-Tenant-Id": e.tenantA.String(), "Idempotency-Key": key()}, partyBody(id, an.id), ct)
			want := "GAME_ACCESS_STUDENT_ONLY"
			if role == "safety_moderator" && r.code() == "TENANT_MISMATCH" {
				want = r.code() // platform staff of no tenant: X-Tenant-Id is refused before the guard, still 403
			}
			if r.status != http.StatusForbidden || r.code() != want {
				t.Fatalf("%s by %s: %d %s", id, role, r.status, r.raw)
			}
		}
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.parties`) + e.count(`SELECT count(*) FROM multiplayer.party_invitations`); n != 0 {
		t.Fatalf("rows written by staff: %d", n)
	}
	coverPartyRoleDenied(t)
}

// T035 — F100-AC-24 (TC-TEN-011, TC-TEN-001, ADR-0065): X-Tenant-Id other than the session tenant → 403
// TENANT_MISMATCH; ids of tenant B from tenant A → 404 on every operation; lists of A never show rows of B.
func TestPartyTenantIsolation(t *testing.T) {
	e := setupParty(t, popts{})
	an := e.kidA()
	dung, em := e.kid(e.tenantB, e.schoolB, uuid.Nil, 6), e.kid(e.tenantB, e.schoolB, uuid.Nil, 6)
	pb := e.party(dung, em)
	x := e.kid(e.tenantB, e.schoolB, uuid.Nil, 6)
	e.friends(dung, x)
	invB := e.invite(dung, pb, x, http.StatusCreated).str("invitation_id")
	// Wrong X-Tenant-Id.
	for _, id := range competitionhttp.PlayerOperations {
		o := competitionhttp.Operations[id]
		ct := ""
		if o.Method == http.MethodPatch {
			ct = "application/merge-patch+json"
		}
		r := e.do(o.Method, e.opPath(id, pb, invB, em.id), an.token, map[string]string{"X-Tenant-Id": e.tenantB.String(),
			"Idempotency-Key": key()}, partyBody(id, em.id), ct)
		if r.status != http.StatusForbidden || r.code() != "TENANT_MISMATCH" {
			t.Fatalf("%s with X-Tenant-Id of B: %d %s", id, r.status, r.raw)
		}
	}
	// Ids of tenant B from a student of A (who has a party of his own).
	pa := e.create(an, http.StatusCreated, nil).str("party_id")
	for _, id := range []string{"getParty", "updatePartySettings", "leaveParty", "disbandParty", "transferPartyLeader", "kickPartyMember",
		"inviteToParty", "acceptPartyInvitation", "rejectPartyInvitation", "cancelPartyInvitation", "joinParty"} {
		o := competitionhttp.Operations[id]
		idem := ""
		if o.Idempotency != 0 {
			idem = key()
		}
		r := e.call(an, o.Method, e.opPath(id, pb, invB, em.id), idem, partyBody(id, em.id))
		if r.status != http.StatusNotFound {
			t.Fatalf("%s on tenant B ids: %d %s", id, r.status, r.raw)
		}
	}
	// Inviting a student of tenant B into the party of A: unknown in the tenant → policy violation, never a 2xx.
	if r := e.invite(an, pa, em, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("invite of B: %s", r.raw)
	}
	// Lists and getMyParty of A never show B.
	if r := e.call(an, http.MethodGet, "/multiplayer/parties/me", "", nil); r.str("party_id") != pa {
		t.Fatalf("my party: %s", r.raw)
	}
	if r := e.call(an, http.MethodGet, "/multiplayer/invitations", "", nil); len(r.data()) != 0 {
		t.Fatalf("invitations of A: %s", r.raw)
	}
	// Nothing of B changed.
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE party_id = $1 AND left_at IS NULL`, pb); n != 2 {
		t.Fatalf("party of B: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_invitations WHERE invitation_id = $1 AND status = 'pending'`, invB); n != 1 {
		t.Fatalf("invitation of B: %d", n)
	}
	coverPartyCrossTenant(t)
}

// coverPartyRoleDenied declares the role_denied case of the 14 party operations, verified by TestPartyRoutesStaffDenied.
func coverPartyRoleDenied(t *testing.T) {
	authztest.Cover(t, "createParty", authztest.RoleDenied)
	authztest.Cover(t, "getMyParty", authztest.RoleDenied)
	authztest.Cover(t, "getParty", authztest.RoleDenied)
	authztest.Cover(t, "updatePartySettings", authztest.RoleDenied)
	authztest.Cover(t, "leaveParty", authztest.RoleDenied)
	authztest.Cover(t, "disbandParty", authztest.RoleDenied)
	authztest.Cover(t, "transferPartyLeader", authztest.RoleDenied)
	authztest.Cover(t, "kickPartyMember", authztest.RoleDenied)
	authztest.Cover(t, "inviteToParty", authztest.RoleDenied)
	authztest.Cover(t, "listMyInvitations", authztest.RoleDenied)
	authztest.Cover(t, "acceptPartyInvitation", authztest.RoleDenied)
	authztest.Cover(t, "rejectPartyInvitation", authztest.RoleDenied)
	authztest.Cover(t, "cancelPartyInvitation", authztest.RoleDenied)
	authztest.Cover(t, "joinParty", authztest.RoleDenied)
}

// coverPartyCrossTenant declares the cross_tenant case of the 14 party operations, verified by TestPartyTenantIsolation.
func coverPartyCrossTenant(t *testing.T) {
	authztest.Cover(t, "createParty", authztest.CrossTenant)
	authztest.Cover(t, "getMyParty", authztest.CrossTenant)
	authztest.Cover(t, "getParty", authztest.CrossTenant)
	authztest.Cover(t, "updatePartySettings", authztest.CrossTenant)
	authztest.Cover(t, "leaveParty", authztest.CrossTenant)
	authztest.Cover(t, "disbandParty", authztest.CrossTenant)
	authztest.Cover(t, "transferPartyLeader", authztest.CrossTenant)
	authztest.Cover(t, "kickPartyMember", authztest.CrossTenant)
	authztest.Cover(t, "inviteToParty", authztest.CrossTenant)
	authztest.Cover(t, "listMyInvitations", authztest.CrossTenant)
	authztest.Cover(t, "acceptPartyInvitation", authztest.CrossTenant)
	authztest.Cover(t, "rejectPartyInvitation", authztest.CrossTenant)
	authztest.Cover(t, "cancelPartyInvitation", authztest.CrossTenant)
	authztest.Cover(t, "joinParty", authztest.CrossTenant)
}
