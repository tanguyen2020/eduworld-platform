package query

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ResolvePrincipalInput identifies the user by (issuer, subject) of the original token or by user id.
type ResolvePrincipalInput struct {
	Issuer, Subject string
	UserID          *uuid.UUID
	TenantID        *uuid.UUID // nil: platform scope
}

// ResolvePrincipal builds the Principal of resolvePrincipal/getPrincipal (F012-AC-11, AC-12). It is a
// system read called by the principal middleware of every module through the identity facade; the caller
// is authorized by modcall (x-allowed-callers), not by a user permission.
//
//   - tenant given: the user must be a member of the active tenant (effective student — ADR-0095 — or valid tenant
//     UserRole, identity.tenant_memberships_v); a System Admin may act in any active tenant with its
//     platform grants. Otherwise 404 (the caller maps it to TENANT_MISMATCH and records the denial).
//   - no tenant: the platform grants of the user (none for tenant users: they can only call the routes
//     exempt from X-Tenant-Id, such as listMyTenants).
//
// Version is users.version, bumped by every role or status change of the user.
type ResolvePrincipal struct {
	Model read.PrincipalReadModel
}

// Handle resolves the Principal.
func (q ResolvePrincipal) Handle(ctx context.Context, in ResolvePrincipalInput) (out dto.Principal, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ResolvePrincipal")
	defer func() { done(err) }()
	bySubject := in.Issuer != "" && in.Subject != ""
	if bySubject == (in.UserID != nil) {
		return out, problem.Invalid("subject", problem.SubInvalid, "exactly one of issuer+subject or user_id is required")
	}
	var u *read.PrincipalUser
	if bySubject {
		u, err = q.Model.UserBySubject(ctx, in.Issuer, in.Subject)
	} else {
		u, err = q.Model.UserByID(ctx, *in.UserID)
	}
	if err != nil {
		return out, err
	}
	if u == nil {
		return out, problem.New(problem.NotFound, "")
	}
	out = dto.Principal{UserID: u.UserID, UserType: u.UserType, Status: u.Status, Version: int(u.Version)}
	platform, err := q.Model.Grants(ctx, u.UserID, nil)
	if err != nil {
		return out, err
	}
	sysAdmin := false
	for _, g := range platform {
		sysAdmin = sysAdmin || (g.RoleCode == "system_admin" && g.RoleScopeKind == "platform")
	}
	out.IsSystemAdmin = sysAdmin
	if in.TenantID == nil {
		fill(&out, platform)
		return out, nil
	}
	mem, err := q.Model.Membership(ctx, u.UserID, *in.TenantID)
	if err != nil {
		return out, err
	}
	t := *in.TenantID
	out.TenantID = &t // scopes of tenant grants refer to it
	switch {
	case mem != nil:
		grants, err := q.Model.Grants(ctx, u.UserID, in.TenantID)
		if err != nil {
			return out, err
		}
		fill(&out, grants)
		out.TenantType = &mem.TenantType
		out.SchoolIDs, out.GradeLevel = mem.SchoolIDs, mem.GradeLevel
		out.SisConnectionID, out.StudentExternalID = mem.SisConnectionID, mem.StudentExternalID
	case sysAdmin:
		tt, err := q.Model.ActiveTenantType(ctx, t)
		if err != nil {
			return out, err
		}
		if tt == nil {
			return dto.Principal{}, problem.New(problem.NotFound, "")
		}
		fill(&out, platform)
		out.TenantType = tt
	default:
		return dto.Principal{}, problem.New(problem.NotFound, "")
	}
	return out, nil
}

// fill sets grants, roles, the union of permissions and the scopes.
func fill(p *dto.Principal, rows []read.PrincipalGrantRow) {
	roles, perms, scopes := map[string]bool{}, map[string]bool{}, map[string]dto.PrincipalScope{}
	p.Grants = make([]dto.PrincipalGrant, 0, len(rows))
	for _, r := range rows {
		g := dto.PrincipalGrant{RoleCode: r.RoleCode, RoleScopeKind: r.RoleScopeKind, Permissions: r.Permissions}
		sc := dto.PrincipalScope{Type: r.ScopeType}
		switch r.ScopeType {
		case "school":
			g.SchoolID = r.SchoolID
			if g.SchoolID == nil {
				g.SchoolID = r.ScopeID
			}
			sc.ID = g.SchoolID
		case "subject":
			g.SubjectID, g.SchoolID, sc.ID = r.ScopeID, r.SchoolID, r.ScopeID
		case "tenant":
			sc.ID = p.TenantID
		}
		p.Grants = append(p.Grants, g)
		roles[r.RoleCode] = true
		for _, x := range r.Permissions {
			perms[x] = true
		}
		key := sc.Type
		if sc.ID != nil {
			key += ":" + sc.ID.String()
		}
		scopes[key] = sc
	}
	p.Roles, p.Permissions = sortedKeys(roles), sortedKeys(perms)
	p.Scopes = make([]dto.PrincipalScope, 0, len(scopes))
	for _, k := range sortedKeys(func() map[string]bool {
		m := map[string]bool{}
		for k := range scopes {
			m[k] = true
		}
		return m
	}()) {
		p.Scopes = append(p.Scopes, scopes[k])
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// CheckMembershipsInput is checkMemberships (the tenant is the TenantScope of the call).
type CheckMembershipsInput struct {
	UserIDs         []uuid.UUID
	SchoolID        *uuid.UUID
	RequireUserType string // "" | student | staff
}

// CheckMemberships checks up to 100 users against the tenant of the calling scope (party, friends,
// assignments…). Nothing but the check result is returned.
type CheckMemberships struct {
	Model read.MembershipChecker
}

// Handle runs the check.
func (q CheckMemberships) Handle(ctx context.Context, in CheckMembershipsInput) (tenant uuid.UUID, out []read.MemberRow, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.CheckMemberships")
	defer func() { done(err) }()
	if len(in.UserIDs) == 0 || len(in.UserIDs) > 100 {
		return tenant, nil, problem.Invalid("user_ids", problem.SubInvalid, "1 to 100 users")
	}
	sc, ok := tenancy.ScopeFrom(ctx)
	id, single := uuid.Nil, false
	if ok {
		id, single = sc.SessionTenantID()
	}
	if !single {
		return tenant, nil, problem.New(problem.PermissionDenied, "checkMemberships needs the TenantScope of the caller")
	}
	if out, err = q.Model.CheckMembers(ctx, id, in.UserIDs, in.SchoolID); err != nil {
		return id, nil, err
	}
	if in.RequireUserType != "" {
		for i := range out {
			out[i].IsMember = out[i].IsMember && out[i].UserType == in.RequireUserType
		}
	}
	return id, out, nil
}
