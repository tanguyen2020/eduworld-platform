// Package modules adapts the facades of other modules for module chat (ADR-0074, AGENTS R29): in-process calls through
// pkg/modcall (span, deadline, scope and Principal of the caller), never internal HTTP.
package modules

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// IdentityPrincipals is resolvePrincipal of the identity facade (x-allowed-callers: chat).
type IdentityPrincipals interface {
	ResolvePrincipal(ctx context.Context, req identityapi.ResolvePrincipalRequestObject) (identityapi.ResolvePrincipalResponseObject, error)
}

// Principals implements port.PrincipalDirectory on resolvePrincipal by user_id in the platform scope (tenant_id null):
// the grants of the platform roles of a staff member (F124-AC-12). 404 → port.ErrPrincipalUnknown; any other failure
// is returned as is (the caller answers 503, fail closed).
type Principals struct{ F IdentityPrincipals }

var _ port.PrincipalDirectory = Principals{}

// Platform implements port.PrincipalDirectory (deadline modcall.Read: the request is waiting).
func (p Principals) Platform(ctx context.Context, user uuid.UUID) (authz.Principal, error) {
	if p.F == nil {
		return authz.Principal{}, errors.New("chat: identity facade not wired")
	}
	op := modcall.Op{Caller: "chat", Callee: "identity", Operation: "resolvePrincipal", Kind: modcall.Read,
		AllowedCallers: identityapi.AllowedCallers["resolvePrincipal"], ContextRequired: identityapi.ContextRequired["resolvePrincipal"]}
	u := user
	body := identityapi.PrincipalResolveRequest{UserId: &u, TenantId: nil}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (identityapi.ResolvePrincipalResponseObject, error) {
		return p.F.ResolvePrincipal(ctx, identityapi.ResolvePrincipalRequestObject{Body: &body})
	})
	if err != nil {
		return authz.Principal{}, err
	}
	switch v := resp.(type) {
	case identityapi.ResolvePrincipal200JSONResponse:
		return toPrincipal(identityapi.Principal(v))
	case identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse:
		return authz.Principal{}, port.ErrPrincipalUnknown
	default:
		return authz.Principal{}, fmt.Errorf("chat: unexpected identity response %T", resp)
	}
}

// toPrincipal keeps the platform grants of an active account (a locked or deactivated one holds no permission).
func toPrincipal(p identityapi.Principal) (authz.Principal, error) {
	if p.Status != identityapi.PrincipalStatusActive {
		return authz.Principal{}, port.ErrPrincipalUnknown
	}
	d := authz.PrincipalData{UserID: p.UserId, UserType: authz.UserType(p.UserType), IsSystemAdmin: p.IsSystemAdmin}
	if p.Grants != nil {
		for _, g := range *p.Grants {
			grant := authz.Grant{Role: g.RoleCode, ScopeKind: authz.RoleScopeKind(g.RoleScopeKind)}
			for _, perm := range g.Permissions {
				if pp := authz.Permission(perm); pp.Known() {
					grant.Permissions = append(grant.Permissions, pp)
				}
			}
			d.Grants = append(d.Grants, grant)
		}
	}
	return authz.NewPrincipal(d), nil
}

// LatePrincipals is the PrincipalDirectory of the commands while module identity is wired after Register (Module.Wire):
// until a facade is set every resolution fails (fail closed: 503).
type LatePrincipals struct {
	mu sync.RWMutex
	f  IdentityPrincipals
}

var _ port.PrincipalDirectory = (*LatePrincipals)(nil)

// Set wires the identity facade (nil: none).
func (l *LatePrincipals) Set(f IdentityPrincipals) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// Platform implements port.PrincipalDirectory.
func (l *LatePrincipals) Platform(ctx context.Context, user uuid.UUID) (authz.Principal, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return Principals{F: f}.Platform(ctx, user)
}
