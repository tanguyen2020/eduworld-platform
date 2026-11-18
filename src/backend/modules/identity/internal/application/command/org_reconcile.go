package command

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ReconcileJobName is the job of the nightly Organization membership reconciliation (profile jobs).
const ReconcileJobName = "org-membership-reconcile"

// ReconcileReport summarizes one run.
type ReconcileReport struct {
	Tenants int
	Removed int
}

// ReconcileOrganizationsHandler removes Organization members without an EduWorld membership (F010-AC-22).
type ReconcileOrganizationsHandler interface {
	Handle(ctx context.Context) (ReconcileReport, error)
}

// ReconcileOrganizations implements ReconcileOrganizationsHandler. Each removal is audited as
// org_member.removed and raises an unexpected_org_member issue (audit reconciliation_issue.raised; F011
// persists issues in sis.reconciliation_issues).
type ReconcileOrganizations struct {
	Reader read.OrgMembershipReader
	KC     port.KeycloakAdmin
	UoW    write.UnitOfWork
	Audit  write.Auditor
}

// Handle runs one reconciliation over every tenant with an Organization.
func (h ReconcileOrganizations) Handle(ctx context.Context) (rep ReconcileReport, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ReconcileOrganizations")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope(ReconcileJobName)
	if err != nil {
		return rep, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	tenants, err := h.Reader.TenantOrgs(ctx)
	if err != nil {
		return rep, err
	}
	for _, t := range tenants {
		if t.OrgID == nil {
			continue
		}
		rep.Tenants++
		expected, err := h.Reader.ExpectedMembers(ctx, t.TenantID)
		if err != nil {
			return rep, err
		}
		members, err := h.KC.ListOrganizationMembers(ctx, t.OrgID.String())
		if err != nil {
			return rep, err
		}
		for _, m := range members {
			if expected[m.ID] {
				continue
			}
			if err := h.KC.RemoveOrganizationMember(ctx, t.OrgID.String(), m.ID); err != nil {
				return rep, err
			}
			rep.Removed++
			meta := map[string]any{"tenant_id": t.TenantID.String(), "organization_id": t.OrgID.String(), "keycloak_user_id": m.ID,
				"issue_type": "unexpected_org_member"}
			if err := h.UoW.Do(ctx, func(ctx context.Context) error {
				if err := h.Audit.Record(ctx, audit.Event{Action: "org_member.removed", TargetType: "keycloak_user", TargetID: m.ID,
					Result: audit.ResultSuccess, Reason: "unexpected_org_member", Metadata: meta}); err != nil {
					return err
				}
				return h.Audit.Record(ctx, audit.Event{Action: "reconciliation_issue.raised", TargetType: "keycloak_user", TargetID: m.ID,
					Result: audit.ResultSuccess, Metadata: meta})
			}); err != nil {
				return rep, err
			}
		}
	}
	return rep, nil
}
