// Package wiring holds composition-root adapters shared by the binaries (ADR-0074): they connect ports
// of pkg/ (which never import modules/) to module facades through pkg/modcall.
package wiring

import (
	"context"
	"errors"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// IdentityPrincipals is the part of the identity facade the resolver needs (identityapi.StrictServerInterface).
type IdentityPrincipals interface {
	ResolvePrincipal(ctx context.Context, req identityapi.ResolvePrincipalRequestObject) (identityapi.ResolvePrincipalResponseObject, error)
}

// PrincipalResolver implements authz.PrincipalResolver over the identity facade operation
// resolvePrincipal (F003 T048, AC-28). The calling module is the module of the request
// (tenancy.ModuleFrom, set by httpx from the operation); identity's own routes call the facade directly.
type PrincipalResolver struct {
	Identity IdentityPrincipals
}

// ErrNoGrants: identity returned a Principal without grants; per-scope permissions are unknown, so the
// request is refused (fail closed) instead of widening or narrowing permissions.
var ErrNoGrants = errors.New("wiring: principal without grants")

// Resolve implements authz.PrincipalResolver.
func (r PrincipalResolver) Resolve(ctx context.Context, req authz.ResolveRequest) (authz.Principal, error) {
	iss, sub, client := req.Subject.Issuer, req.Subject.Subject, req.ClientID
	body := identityapi.PrincipalResolveRequest{Issuer: &iss, Subject: &sub, ClientId: &client, TenantId: req.TenantID}
	call := func(ctx context.Context) (identityapi.ResolvePrincipalResponseObject, error) {
		return r.Identity.ResolvePrincipal(ctx, identityapi.ResolvePrincipalRequestObject{Body: &body})
	}
	caller := tenancy.ModuleFrom(ctx)
	var resp identityapi.ResolvePrincipalResponseObject
	var err error
	if caller == "identity" {
		resp, err = call(ctx)
	} else {
		resp, err = modcall.Call(ctx, modcall.Op{Caller: caller, Callee: "identity", Operation: "resolvePrincipal", Kind: modcall.Read,
			AllowedCallers: identityapi.AllowedCallers["resolvePrincipal"], ContextRequired: identityapi.ContextRequired["resolvePrincipal"]}, call)
	}
	if err != nil {
		return authz.Principal{}, err
	}
	switch v := resp.(type) {
	case identityapi.ResolvePrincipal200JSONResponse:
		return toPrincipal(identityapi.Principal(v))
	case identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse:
		return authz.Principal{}, authz.ErrNotMember
	case identityapi.ResolvePrincipal403ApplicationProblemPlusJSONResponse:
		return authz.Principal{}, problem.New(problem.Code(v.Code), "")
	case identityapi.ResolvePrincipal401ApplicationProblemPlusJSONResponse:
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	default:
		return authz.Principal{}, problem.New(problem.Unavailable, "unexpected identity response").Retry()
	}
}

func toPrincipal(p identityapi.Principal) (authz.Principal, error) {
	switch p.Status {
	case identityapi.PrincipalStatusActive:
	case identityapi.PrincipalStatusLocked:
		return authz.Principal{}, problem.New(problem.AccountLocked, "")
	default:
		return authz.Principal{}, problem.New(problem.AccountDeactivated, "")
	}
	if p.Grants == nil {
		return authz.Principal{}, problem.New(problem.Unavailable, "identity principal without grants").Wrap(ErrNoGrants)
	}
	d := authz.PrincipalData{UserID: p.UserId, UserType: authz.UserType(p.UserType), IsSystemAdmin: p.IsSystemAdmin}
	if p.TenantId != nil {
		t := *p.TenantId
		d.TenantID = &t
	}
	if p.TenantType != nil {
		d.TenantType = authz.TenantType(*p.TenantType)
	}
	for _, g := range *p.Grants {
		grant := authz.Grant{Role: g.RoleCode, ScopeKind: authz.RoleScopeKind(g.RoleScopeKind)}
		if g.SchoolId != nil {
			s := *g.SchoolId
			grant.SchoolID = &s
		}
		if g.SubjectId != nil {
			s := *g.SubjectId
			grant.SubjectID = &s
		}
		for _, perm := range g.Permissions {
			if pp := authz.Permission(perm); pp.Known() {
				grant.Permissions = append(grant.Permissions, pp)
			}
		}
		d.Grants = append(d.Grants, grant)
	}
	return authz.NewPrincipal(d), nil
}
