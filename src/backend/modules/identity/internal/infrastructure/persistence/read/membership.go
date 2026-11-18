package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entext "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/externalidentity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// MembershipModel implements read.MembershipReader. It reads one user's memberships in every tenant, so it
// runs under tenancy.Bypass(platform_job) (worker keycloak-membership, audited and counted, ADR-0089 §3).
type MembershipModel struct {
	Session     *entx.ReadSession
	LocalIssuer string // issuer of the local Keycloak realm (first of EW_OIDC_ISSUERS)
}

var _ read.MembershipReader = MembershipModel{}

// MembershipOf returns the Keycloak user id and the tenants (with Organization ids) of the user.
func (m MembershipModel) MembershipOf(ctx context.Context, userID uuid.UUID) (out read.UserMembership, err error) {
	ctx, err = tenancy.Bypass(ctx, tenancy.ReasonPlatformJob)
	if err != nil {
		return out, err
	}
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		ids, err := ent.NewClient(ent.Driver(entx.Driver(ctx))).ExternalIdentity.Query().
			Where(entext.UserID(userID), entext.Issuer(m.LocalIssuer), entext.Status("active")).All(ctx)
		if err != nil {
			return err
		}
		if len(ids) == 1 {
			out.KeycloakUserID = ids[0].Subject
		}
		tx, _ := entx.Tx(ctx)
		rows, err := tx.QueryContext(ctx, `SELECT m.tenant_id, t.keycloak_org_id
			FROM identity.tenant_memberships_v m
			JOIN platform.tenants t ON t.tenant_id = m.tenant_id AND t.deleted_at IS NULL
			WHERE m.user_id = $1`, userID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var to read.TenantOrg
			var org uuid.NullUUID
			if err := rows.Scan(&to.TenantID, &org); err != nil {
				return err
			}
			if org.Valid {
				id := org.UUID
				to.OrgID = &id
			}
			out.Tenants = append(out.Tenants, to)
		}
		return rows.Err()
	})
	return out, err
}

// OrgMembershipModel implements read.OrgMembershipReader under tenancy.Bypass(platform_job).
type OrgMembershipModel struct {
	Session     *entx.ReadSession
	LocalIssuer string
}

var _ read.OrgMembershipReader = OrgMembershipModel{}

// TenantOrgs returns every live tenant with its Organization id.
func (m OrgMembershipModel) TenantOrgs(ctx context.Context) (out []read.TenantOrg, err error) {
	if ctx, err = tenancy.Bypass(ctx, tenancy.ReasonPlatformJob); err != nil {
		return nil, err
	}
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := ent.NewClient(ent.Driver(entx.Driver(ctx))).Tenant.Query().All(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, read.TenantOrg{TenantID: r.ID, OrgID: r.KeycloakOrgID})
		}
		return nil
	})
	return out, err
}

// ExpectedMembers returns the Keycloak subjects (local issuer) of the users with a membership in the tenant.
func (m OrgMembershipModel) ExpectedMembers(ctx context.Context, tenantID uuid.UUID) (out map[string]bool, err error) {
	if ctx, err = tenancy.Bypass(ctx, tenancy.ReasonPlatformJob); err != nil {
		return nil, err
	}
	out = map[string]bool{}
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		rows, err := tx.QueryContext(ctx, `SELECT ei.subject
			FROM identity.tenant_memberships_v m
			JOIN identity.external_identities ei ON ei.user_id = m.user_id AND ei.issuer = $2 AND ei.status = 'active' AND ei.deleted_at IS NULL
			WHERE m.tenant_id = $1`, tenantID, m.LocalIssuer)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				return err
			}
			out[s] = true
		}
		return rows.Err()
	})
	return out, err
}
