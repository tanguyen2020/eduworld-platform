//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
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
// social), another tenant, a restrict_multiplayer sanction, another grade band, a non-friend in a friend party, a school
// policy with same_grade_only → 403 PARTY_POLICY_VIOLATION with an audit (rule in the reason).
func TestInvitePolicyViolations(t *testing.T) {
	e := setupParty(t, popts{})
	a := e.kidA()
	id := e.create(a, http.StatusCreated, map[string]any{"name": "Nhóm", "party_type": "friend", "max_members": 8}).str("party_id")
	sanctioned := e.kidA()
	e.exec(`INSERT INTO ref.sanctions (sanction_id, tenant_id, user_id, sanction_type, until, active, source_version)
		VALUES ($1, $2, $3, 'restrict_multiplayer', now() + interval '7 days', true, 1)`, uuid.New(), e.tenantA, sanctioned.id)
	cases := map[string]kid{
		"school":      e.kid(e.tenantA, e.schoolA2, 6),
		"tenant":      e.kid(e.tenantB, e.schoolB, 6),
		"sanction":    sanctioned,
		"grade_band":  e.kid(e.tenantA, e.schoolA, 8),
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
	// A unit tenant with cross_school_social_enabled: another school of the unit is fine.
	u1, u2 := e.kid(e.tenantU, e.schoolU1, 6), e.kid(e.tenantU, e.schoolU2, 6)
	e.friends(u1, u2)
	up := e.create(u1, http.StatusCreated, nil).str("party_id")
	e.invite(u1, up, u2, http.StatusCreated)
	// A school policy turning same_grade_only on for grade 6 (a legacy cross_class key is ignored, ADR-0095).
	e.exec(`INSERT INTO multiplayer.party_policies (policy_id, tenant_id, created_at, updated_at, scope_type, scope_id, school_id, rules)
		VALUES (gen_random_uuid(), $1, now(), now(), 'school', $2, $2, '{"same_grade_only":true,"cross_class":true}')`, e.tenantA, e.schoolA)
	other := e.kid(e.tenantA, e.schoolA, 7)
	e.friends(a, other)
	if r := e.invite(a, id, other, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("school policy same_grade_only: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_invitations WHERE party_id = $1`, id); n != 0 {
		t.Fatalf("invitations written: %d", n)
	}
}

// ADR-0095 D2 — F100-AC-09 (TC-PTY-011, PARTY_SYSTEM §8 R12): a grade 1–3 party takes friends of the exact grade of
// the inviter; another school of the same tenant only when the tenant has cross-school social (ADR-0054); never another
// tenant. Every refusal is 403 PARTY_POLICY_VIOLATION with the rule in the audit, never in the response.
func TestGrades13SameGradeParty(t *testing.T) {
	e := setupParty(t, popts{})
	small := e.kid(e.tenantA, e.schoolA, 3)
	sp := e.create(small, http.StatusCreated, map[string]any{"name": "Nhóm khối 3", "party_type": "friend"}).str("party_id")
	sameGrade := e.kid(e.tenantA, e.schoolA, 3)
	e.friends(small, sameGrade)
	e.invite(small, sp, sameGrade, http.StatusCreated)
	deny := map[string]kid{
		"grade":  e.kid(e.tenantA, e.schoolA, 2),  // same band, other grade
		"school": e.kid(e.tenantA, e.schoolA2, 3), // same grade, other school, tenant without cross-school social
		"tenant": e.kid(e.tenantB, e.schoolB, 3),  // same grade, other tenant: never projected in tenant A
	}
	for rule, k := range deny {
		if rule != "tenant" {
			e.friends(small, k)
		}
		r := e.invite(small, sp, k, http.StatusForbidden)
		if r.code() != "PARTY_POLICY_VIOLATION" || strings.Contains(r.raw, rule) {
			t.Fatalf("%s: %s", rule, r.raw)
		}
	}
	reasons := map[string]bool{}
	for _, p := range e.audits("party.invitation_policy_denied") {
		reasons[p["reason"].(string)] = true
	}
	for rule := range deny {
		if !reasons[rule] {
			t.Fatalf("audit reason %s missing: %v", rule, reasons)
		}
	}
	// A unit tenant with cross-school social: same grade at another school joins, another grade does not.
	u1, u2, u3 := e.kid(e.tenantU, e.schoolU1, 2), e.kid(e.tenantU, e.schoolU2, 2), e.kid(e.tenantU, e.schoolU2, 1)
	e.friends(u1, u2)
	e.friends(u1, u3)
	up := e.create(u1, http.StatusCreated, map[string]any{"name": "Nhóm khối 2", "party_type": "quest"}).str("party_id")
	e.invite(u1, up, u2, http.StatusCreated)
	if r := e.invite(u1, up, u3, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("unit tenant, other grade: %s", r.raw)
	}
	// study_party is not allowed for grades 1–3 and study_group is no longer a party type (D7).
	for _, typ := range []string{"study_party", "study_group"} {
		if r := e.create(e.kid(e.tenantA, e.schoolA, 3), http.StatusUnprocessableEntity, map[string]any{"name": "Nhóm",
			"party_type": typ}); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s at grade 3: %d %s", typ, r.status, r.raw)
		}
	}
}

// ADR-0095: student.placement_changed (owner F010) keeps the grade level and school of ref.students that the grade
// rule of a grade 1–3 party reads; an older source_version is ignored; an inactive placement makes the student unknown
// to the policies (RuleTenant), an active one brings it back.
func TestPlacementChangedFeedsGradeRule(t *testing.T) {
	e := setupParty(t, popts{})
	h := consumer.IdentityProjection{P: command.Projection{Refs: persistence.Refs{}}}
	place := func(k kid, grade int, status string, version int64) {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"student_id": k.id, "tenant_id": k.tenant, "school_id": k.school, "grade_level": grade,
			"status": status, "source": "sis", "source_version": version})
		scope, err := authz.EventScope(k.tenant)
		if err != nil {
			t.Fatal(err)
		}
		tt := k.tenant
		env := outbox.Envelope{EventID: uuid.New(), EventType: "student.placement_changed", OccurredAt: time.Now(), TenantID: &tt, Payload: b}
		if err := e.uow.InTenantTx(context.Background(), scope, func(ctx context.Context) error { return h.Handle(ctx, env) }); err != nil {
			t.Fatal(err)
		}
	}
	small := e.kid(e.tenantA, e.schoolA, 3)
	peer := e.kid(e.tenantA, e.schoolA, 2)
	e.friends(small, peer)
	sp := e.create(small, http.StatusCreated, map[string]any{"name": "Nhóm khối 3", "party_type": "friend"}).str("party_id")
	if r := e.invite(small, sp, peer, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("grade 2: %s", r.raw)
	}
	place(peer, 3, "active", 5) // moved up to grade 3
	place(peer, 2, "active", 4) // older: ignored
	if n := e.count(`SELECT count(*) FROM ref.students WHERE student_id = $1 AND grade_level = 3 AND source_version = 5`, peer.id); n != 1 {
		t.Fatalf("ref.students not updated: %d", n)
	}
	inv := e.invite(small, sp, peer, http.StatusCreated).str("invitation_id")
	e.call(small, http.MethodPost, "/multiplayer/invitations/"+inv+":cancel", "", nil)
	other := e.kid(e.tenantA, e.schoolA, 3)
	e.friends(small, other)
	place(other, 3, "inactive", 6)
	if r := e.invite(small, sp, other, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("inactive placement: %s", r.raw)
	}
	place(other, 3, "active", 7)
	e.invite(small, sp, other, http.StatusCreated)
}
