//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// kcMembers extends kcUsers with Organization membership per user and a call log.
type kcMembers struct {
	*kcUsers
	orgsOf map[string]map[string]bool // kc user → org ids
	attrs  map[string]map[string][]string
	calls  []string
}

func (k *kcMembers) ListMemberOrganizations(_ context.Context, id string) ([]port.KCOrganization, error) {
	var out []port.KCOrganization
	for o := range k.orgsOf[id] {
		out = append(out, port.KCOrganization{ID: o, Attributes: k.attrs[o]})
	}
	return out, nil
}
func (k *kcMembers) AddOrganizationMember(_ context.Context, org, id string) error {
	k.calls = append(k.calls, "add:"+org)
	if k.orgsOf[id] == nil {
		k.orgsOf[id] = map[string]bool{}
	}
	k.orgsOf[id][org] = true
	return nil
}
func (k *kcMembers) RemoveOrganizationMember(_ context.Context, org, id string) error {
	k.calls = append(k.calls, "remove:"+org)
	delete(k.orgsOf[id], org)
	return nil
}
func (k *kcMembers) LogoutUser(context.Context, string) error {
	k.calls = append(k.calls, "logout")
	return nil
}

// F010-AC-20: after a tenant transfer the worker removes the old Organization, adds the new one, then
// revokes the sessions and emits user.access_revoked; a second run changes nothing.
func TestMembershipSync(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	issuer := "http://localhost:8180/realms/eduworld"
	commonOrg, bOrg := uuid.New(), uuid.New()
	b := seedTenant(ctx, t, e.platform, "pgd-mb")
	for tid, org := range map[string]uuid.UUID{tenant.CommonTenantID.String(): commonOrg, b.tenant: bOrg} {
		if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET keycloak_org_id = $1 WHERE tenant_id = $2 AND deleted_at IS NULL`, org, tid); err != nil {
			t.Fatal(err)
		}
	}
	kc := &kcMembers{kcUsers: newKCUsers(), orgsOf: map[string]map[string]bool{},
		attrs: map[string]map[string][]string{commonOrg.String(): {"tenant_id": {tenant.CommonTenantID.String()}}, bOrg.String(): {"tenant_id": {b.tenant}}}}
	create := command.CreateCommonStudents{UoW: e.uow, Tenants: write.TenantRepo{}, Accounts: write.AccountRepo{}, Students: write.StudentRepo{}, KC: kc,
		Events: e.deps.Events, Audit: e.deps.Audit, LocalIssuer: issuer}
	out, err := create.Handle(authztest.Ctx(ctx, authztest.Platform("system_admin")), command.CreateCommonStudentsInput{TenantID: tenant.CommonTenantID,
		Students: []command.CommonStudentInput{{Username: "hs.chuyen", DisplayName: "Chuyển", GradeLevel: 3}}})
	if err != nil || out.CreatedCount != 1 {
		t.Fatalf("create = %+v %v", out, err)
	}
	userID, kcID := *out.Results[0].UserID, kc.users["hs.chuyen"]

	// Move the student to tenant B (cross-tenant scope of System Admin), as F011 would.
	sa := authztest.Platform("system_admin")
	cs, _ := authz.CrossTenantScope(sa, []uuid.UUID{tenant.CommonTenantID, uuid.MustParse(b.tenant)}, tenancy.ReasonSystemAdminCrossTenant)
	tr := command.TransferStudent{TransferDeps: command.TransferDeps{UoW: e.uow, Schools: write.SchoolRepo{}, Students: write.StudentRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}}
	if _, err := tr.Handle(tenancy.WithScope(authz.WithPrincipal(ctx, sa), cs), command.TransferStudentInput{StudentID: userID,
		TargetSchoolID: uuid.MustParse(b.school), TargetGrade: 3, On: time.Now().UTC(), Source: placement.SourceSIS}); err != nil {
		t.Fatal(err)
	}

	// Consumer transaction of tenant_membership.changed (envelope tenant = B).
	sync := command.SyncUserMembership{Reader: read.MembershipModel{Session: e.read, LocalIssuer: issuer}, KC: kc, Events: e.deps.Events, Audit: e.deps.Audit}
	scopeB, _ := authz.EventScope(uuid.MustParse(b.tenant))
	kc.calls = nil
	if err := e.uow.InTenantTx(ctx, scopeB, func(ctx context.Context) error { return sync.Handle(ctx, userID) }); err != nil {
		t.Fatal(err)
	}
	want := []string{"remove:" + commonOrg.String(), "add:" + bOrg.String(), "logout"}
	if len(kc.calls) != 3 || kc.calls[0] != want[0] || kc.calls[1] != want[1] || kc.calls[2] != want[2] {
		t.Fatalf("keycloak calls = %v, want %v", kc.calls, want)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user.access_revoked'
		AND envelope->'payload'->>'user_id' = $1 AND envelope->'payload'->>'reason' = 'transferred'
		AND envelope->'payload'->>'tenant_id' = $2`, userID.String(), tenant.CommonTenantID.String()); n != 1 {
		t.Fatalf("user.access_revoked = %d", n)
	}
	// Idempotent: a redelivery changes nothing and revokes nothing.
	kc.calls = nil
	if err := e.uow.InTenantTx(ctx, scopeB, func(ctx context.Context) error { return sync.Handle(ctx, userID) }); err != nil || len(kc.calls) != 0 {
		t.Fatalf("second sync: %v calls=%v", err, kc.calls)
	}
	if !kc.orgsOf[kcID][bOrg.String()] || kc.orgsOf[kcID][commonOrg.String()] {
		t.Fatalf("final membership = %v", kc.orgsOf[kcID])
	}
}

func (k *kcMembers) ListOrganizationMembers(_ context.Context, org string) ([]port.KCUser, error) {
	var out []port.KCUser
	for u, orgs := range k.orgsOf {
		if orgs[org] {
			out = append(out, port.KCUser{ID: u})
		}
	}
	return out, nil
}

// F010-AC-22: the nightly job removes an Organization member without a membership in the tenant (added by
// hand in Keycloak), keeps real members and audits the removal.
func TestReconcileOrganizations(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	issuer := "http://localhost:8180/realms/eduworld"
	commonOrg := uuid.New()
	if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET keycloak_org_id = $1 WHERE tenant_id = $2 AND deleted_at IS NULL`, commonOrg, tenant.CommonTenantID); err != nil {
		t.Fatal(err)
	}
	kc := &kcMembers{kcUsers: newKCUsers(), orgsOf: map[string]map[string]bool{}, attrs: map[string]map[string][]string{}}
	create := command.CreateCommonStudents{UoW: e.uow, Tenants: write.TenantRepo{}, Accounts: write.AccountRepo{}, Students: write.StudentRepo{}, KC: kc,
		Events: e.deps.Events, Audit: e.deps.Audit, LocalIssuer: issuer}
	if _, err := create.Handle(authztest.Ctx(ctx, authztest.Platform("system_admin")), command.CreateCommonStudentsInput{TenantID: tenant.CommonTenantID,
		Students: []command.CommonStudentInput{{Username: "hs.that", DisplayName: "Thật", GradeLevel: 2}}}); err != nil {
		t.Fatal(err)
	}
	real := kc.users["hs.that"]
	intruder := uuid.NewString()
	kc.orgsOf[intruder] = map[string]bool{commonOrg.String(): true}

	job := command.ReconcileOrganizations{Reader: read.OrgMembershipModel{Session: e.read, LocalIssuer: issuer}, KC: kc, UoW: e.uow, Audit: e.deps.Audit}
	rep, err := job.Handle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Removed != 1 || kc.orgsOf[intruder][commonOrg.String()] || !kc.orgsOf[real][commonOrg.String()] {
		t.Fatalf("report = %+v, intruder member=%v, real member=%v", rep, kc.orgsOf[intruder][commonOrg.String()], kc.orgsOf[real][commonOrg.String()])
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' IN ('org_member.removed', 'reconciliation_issue.raised')`); n != 2 {
		t.Fatalf("audit events = %d, want 2", n)
	}
	if rep, err := job.Handle(ctx); err != nil || rep.Removed != 0 {
		t.Fatalf("second run = %+v %v", rep, err)
	}
}
