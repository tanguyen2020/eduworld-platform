//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// deliver runs a consumer handler for one event like pkg/outbox.Consumer does: one transaction with the event scope
// (tenant scope, or the platform scope of a platform event).
func (e *penv) deliver(handle func(context.Context, outbox.Envelope) error, typ string, tenant *uuid.UUID, payload map[string]any) {
	e.t.Helper()
	b, _ := json.Marshal(payload)
	env := outbox.Envelope{EventID: uuid.New(), EventType: typ, OccurredAt: time.Now(), TenantID: tenant, Payload: b}
	var scope tenancy.Scope
	var err error
	if tenant == nil {
		scope, err = authz.SystemPlatformScope("consumer-competition-party-safety-identity")
	} else {
		scope, err = authz.EventScope(*tenant)
	}
	if err != nil {
		e.t.Fatal(err)
	}
	ctx := tenancy.WithModule(context.Background(), "competition")
	if err := e.uow.InTenantTx(ctx, scope, func(ctx context.Context) error { return handle(ctx, env) }); err != nil {
		e.t.Fatalf("%s: %v", typ, err)
	}
}

func (e *penv) safety() (consumer.PartySafetyChat, consumer.PartySafetyIdentity) {
	s := command.PartySafety{Parties: e.parties, Sanctions: persistence.Sanctions{}}
	return consumer.PartySafetyChat{S: s}, consumer.PartySafetyIdentity{S: s}
}

func (e *penv) memberOf(k kid) string {
	e.t.Helper()
	r := e.call(k, http.MethodGet, "/multiplayer/parties/me", "", nil)
	if r.status == http.StatusNotFound {
		return ""
	}
	return r.str("party_id")
}

// T036 — F100-AC-25 (TC-CHAT-005): user.blocked cancels the pending invitations between the two students (either
// direction) and the block projection refuses the next invitation; sanction.applied restrict_multiplayer takes the
// member out of its party (policy, the leader passes on), cancels its invitations and blocks its next party actions
// until sanction.lifted.
func TestSafetyChanges(t *testing.T) {
	e := setupParty(t, popts{})
	chat, _ := e.safety()
	a, b, c := e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c)
	x := e.kidA()
	e.friends(a, x)
	inv := e.invite(a, id, x, http.StatusCreated).str("invitation_id")
	tenant := e.tenantA
	e.deliver(chat.Handle, "user.blocked", &tenant, map[string]any{"tenant_id": tenant, "blocker_id": x.id, "blocked_id": a.id,
		"source_version": 1})
	var status, reason string
	if err := e.su.QueryRow(`SELECT status, end_reason FROM multiplayer.party_invitations WHERE invitation_id = $1`, inv).
		Scan(&status, &reason); err != nil || status != "cancelled" || reason != "blocked" {
		t.Fatalf("blocked invitation: %s %s %v", status, reason, err)
	}
	if got := e.push.of(x.id, "party.invitation.updated"); len(got) != 1 || got[0]["status"] != "cancelled" {
		t.Fatalf("invitee told: %v", got)
	}
	e.block(x, a) // projection competition.projection_social
	if r := e.invite(a, id, x, http.StatusForbidden); r.code() != "PARTY_INVITE_NOT_ALLOWED" {
		t.Fatalf("after block: %s", r.raw)
	}
	// The leader a gets restrict_multiplayer: out of the party (policy), b (joined first) leads; a's invitations end.
	other := e.kidA()
	e.friends(c, other)
	sanction := uuid.New()
	until := e.clock.Now().Add(7 * 24 * time.Hour)
	e.deliver(chat.Handle, "sanction.applied", &tenant, map[string]any{"tenant_id": tenant, "sanction_id": sanction, "user_id": a.id,
		"case_id": uuid.New(), "sanction_type": "restrict_multiplayer", "until": until})
	if e.memberOf(a) != "" {
		t.Fatal("sanctioned student still in a party")
	}
	r := e.call(b, http.MethodGet, "/multiplayer/parties/me", "", nil)
	if r.str("leader_id") != b.id.String() || len(r.members()) != 2 {
		t.Fatalf("party after sanction: %s", r.raw)
	}
	if got := e.push.of(b.id, "party.member.left"); len(got) != 1 || got[0]["reason"] != "account_disabled" {
		t.Fatalf("member.left: %v", got)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE student_id = $1 AND left_reason = 'policy'`, a.id); n != 1 {
		t.Fatalf("left_reason policy: %d", n)
	}
	if au := e.audits("party.member_removed_by_policy"); len(au) != 1 {
		t.Fatalf("audit: %v", au)
	}
	// Next actions of a refused while the sanction is in force; the same event again changes nothing.
	if r := e.call(a, http.MethodPost, "/multiplayer/parties", key(), map[string]any{"name": "Nhóm", "party_type": "friend"}); r.status !=
		http.StatusForbidden || r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("create while sanctioned: %d %s", r.status, r.raw)
	}
	if r := e.invite(b, id, a, http.StatusForbidden); r.code() != "PARTY_POLICY_VIOLATION" {
		t.Fatalf("invite a sanctioned student: %s", r.raw)
	}
	e.deliver(chat.Handle, "sanction.lifted", &tenant, map[string]any{"tenant_id": tenant, "sanction_id": sanction, "user_id": a.id,
		"sanction_type": "restrict_multiplayer"})
	e.create(a, http.StatusCreated, nil)
	// A sanction outside multiplayer (mute) changes nothing.
	e.deliver(chat.Handle, "sanction.applied", &tenant, map[string]any{"tenant_id": tenant, "sanction_id": uuid.New(), "user_id": c.id,
		"sanction_type": "chat_mute"})
	if e.memberOf(c) != id {
		t.Fatal("chat sanction removed a member")
	}
	e.checkEventSchemas()
}

// F100 edge cases: student.synced to another school or deactivated, user.status_changed locked (platform event) and
// student.cleaned_up take the student out of its party (policy).
func TestSafetyIdentityChanges(t *testing.T) {
	e := setupParty(t, popts{})
	_, ident := e.safety()
	a, b, c, d, f := e.kidA(), e.kidA(), e.kidA(), e.kidA(), e.kidA()
	id := e.party(a, b, c, d, f)
	tenant := e.tenantA
	e.deliver(ident.Handle, "student.synced", &tenant, map[string]any{"student_id": b.id, "school_id": e.schoolA2, "action": "update"})
	if e.memberOf(b) != "" {
		t.Fatal("moved student still in the party")
	}
	if got := e.push.of(a.id, "party.member.left"); len(got) != 1 || got[0]["reason"] != "transferred_school" {
		t.Fatalf("member.left: %v", got)
	}
	e.deliver(ident.Handle, "student.synced", &tenant, map[string]any{"student_id": c.id, "school_id": e.schoolA, "action": "update"})
	if e.memberOf(c) != id {
		t.Fatal("same school removed")
	}
	e.deliver(ident.Handle, "student.synced", &tenant, map[string]any{"student_id": c.id, "school_id": e.schoolA, "action": "deactivate"})
	e.deliver(ident.Handle, "user.status_changed", nil, map[string]any{"user_id": d.id, "from": "active", "to": "locked"})
	e.deliver(ident.Handle, "student.cleaned_up", &tenant, map[string]any{"student_id": f.id, "tenant_id": tenant})
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE party_id = $1 AND left_reason = 'policy'`, id); n != 4 {
		t.Fatalf("removed by policy: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.party_members WHERE party_id = $1 AND left_at IS NULL`, id); n != 1 {
		t.Fatalf("left in party: %d", n)
	}
	e.checkEventSchemas()
}
