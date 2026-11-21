//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// T022 — F100-AC-08 (TC-PTY-011): a member who is not allowed by invitation_policy, or a block in either direction
// between the invitee and a member → 403 PARTY_INVITE_NOT_ALLOWED that never says why; with invitation_policy members a
// member invites.
func TestInviteNotAllowed(t *testing.T) {
	e := setupParty(t, popts{})
	a, b := e.kidA(), e.kidA()
	id := e.party(a, b)
	c := e.kidA()
	e.friends(b, c)
	e.friends(a, c)
	if r := e.invite(b, id, c, http.StatusForbidden); r.code() != "PARTY_INVITE_NOT_ALLOWED" {
		t.Fatalf("member under leader_only: %s", r.raw)
	}
	if r := e.call(a, http.MethodPatch, "/multiplayer/parties/"+id, "", map[string]any{"invitation_policy": "members"}); r.status != http.StatusOK {
		t.Fatalf("settings: %d %s", r.status, r.raw)
	}
	e.invite(b, id, c, http.StatusCreated)
	for _, dir := range []string{"invitee blocks", "member blocks"} {
		x := e.kidA()
		e.friends(a, x)
		if dir == "invitee blocks" {
			e.block(x, b) // x blocked a member (not the inviter)
		} else {
			e.block(a, x)
		}
		r := e.invite(a, id, x, http.StatusForbidden)
		if r.code() != "PARTY_INVITE_NOT_ALLOWED" || strings.Contains(strings.ToLower(r.raw), "block") {
			t.Fatalf("%s: %s", dir, r.raw)
		}
	}
	if a := e.audits("party.invitation_policy_denied"); len(a) != 3 {
		t.Fatalf("audits: %d", len(a))
	}
}

// T022 — F100-AC-09 (TC-PTY-011, ADR-0054, PARTY_SYSTEM §8): another school (unless a unit tenant with cross-school
// social), another tenant, a restrict_multiplayer sanction, grades 1–3 outside the class, another grade band, a
// non-friend in a friend party → 403 PARTY_POLICY_VIOLATION with an audit (rule in the reason).
func TestInvitePolicyViolations(t *testing.T) {
	e := setupParty(t, popts{})
	a := e.kidA()
	id := e.create(a, http.StatusCreated, map[string]any{"name": "Nhóm", "party_type": "friend", "max_members": 8}).str("party_id")
	sanctioned := e.kidA()
	e.exec(`INSERT INTO ref.sanctions (sanction_id, tenant_id, user_id, sanction_type, until, active, source_version)
		VALUES ($1, $2, $3, 'restrict_multiplayer', now() + interval '7 days', true, 1)`, uuid.New(), e.tenantA, sanctioned.id)
	cases := map[string]kid{
		"school":      e.kid(e.tenantA, e.schoolA2, uuid.Nil, 6),
		"tenant":      e.kid(e.tenantB, e.schoolB, uuid.Nil, 6),
		"sanction":    sanctioned,
		"grade_band":  e.kid(e.tenantA, e.schoolA, e.classA, 8),
		"friend_only": e.kidA(), // not a friend
	}
	for rule, k := range cases {
		if rule != "friend_only" && rule != "tenant" {
			e.friends(a, k)
		}
		r := e.invite(a, id, k, http.StatusForbidden)
		if r.code() != "PARTY_POLICY_VIOLATION" || strings.Contains(r.raw, rule) {
			t.Fatalf("%s: %s", rule, r.raw)
		}
	}
	reasons := map[string]bool{}
	for _, p := range e.audits("party.invitation_policy_denied") {
		reasons[p["reason"].(string)] = true
	}
	for rule := range cases {
		want := rule
		if rule == "tenant" {
			want = "tenant" // unknown in the session tenant: never reveals the other tenant
		}
		if !reasons[want] {
			t.Fatalf("audit reason %s missing: %v", want, reasons)
		}
	}
	// Grades 1–3: friends of the same class only (PARTY_SYSTEM §8 R12).
	small := e.kid(e.tenantA, e.schoolA, e.classA, 3)
	sameClass, otherClass := e.kid(e.tenantA, e.schoolA, e.classA, 3), e.kid(e.tenantA, e.schoolA, e.classA2, 3)
	e.friends(small, sameClass)
	e.friends(small, otherClass)
	sp := e.create(small, http.StatusCreated, map[string]any{"name": "Nhóm 3A", "party_type": "friend"}).str("party_id")
	e.invite(small, sp, sameClass, http.StatusCreated)
	if r := e.invite(small, sp, otherClass, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("grade 3 other class: %s", r.raw)
	}
	// A unit tenant with cross_school_social_enabled: another school of the unit is fine.
	u1, u2 := e.kid(e.tenantU, e.schoolU1, uuid.Nil, 6), e.kid(e.tenantU, e.schoolU2, uuid.Nil, 6)
	e.friends(u1, u2)
	up := e.create(u1, http.StatusCreated, nil).str("party_id")
	e.invite(u1, up, u2, http.StatusCreated)
	// A school policy without cross_grade_band… and a class policy turning cross_class off for grade 6.
	e.exec(`INSERT INTO multiplayer.party_policies (policy_id, tenant_id, created_at, updated_at, scope_type, scope_id, school_id, rules)
		VALUES (gen_random_uuid(), $1, now(), now(), 'class', $2, $3, '{"cross_class":false}')`, e.tenantA, e.classA, e.schoolA)
	other := e.kid(e.tenantA, e.schoolA, e.classA2, 6)
	e.friends(a, other)
	if r := e.invite(a, id, other, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("class policy: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_invitations WHERE party_id = $1`, id); n != 0 {
		t.Fatalf("invitations written: %d", n)
	}
}
