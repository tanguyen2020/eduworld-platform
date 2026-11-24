package modules

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// Caller-side facade test of resolvePrincipal for the on-call rotation (F124-AC-12) against the generated fake (TEST
// DOUBLE identityapi.Fake): request by user_id in the platform scope (tenant_id null), caller chat, platform grants
// mapped, 404 → ErrPrincipalUnknown, locked → unknown, failure → error (fail closed), late wiring.
func TestIdentityPrincipalsFacade(t *testing.T) {
	user := uuid.New()
	status := identityapi.PrincipalStatusActive
	var failure error
	notFound := false
	fake := &identityapi.Fake{ResolvePrincipalFunc: func(ctx context.Context, r identityapi.ResolvePrincipalRequestObject) (identityapi.ResolvePrincipalResponseObject, error) {
		if modcall.CallerFrom(ctx) != "chat" || r.Body.UserId == nil || *r.Body.UserId != user || r.Body.TenantId != nil {
			t.Fatalf("request: %s %+v", modcall.CallerFrom(ctx), r.Body)
		}
		if failure != nil {
			return nil, failure
		}
		if notFound {
			return identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse{Code: "NOT_FOUND", Status: 404, Title: "NOT_FOUND", Type: "about:blank"}, nil
		}
		grants := []identityapi.PrincipalGrant{{RoleCode: "safety_moderator", RoleScopeKind: "platform",
			Permissions: []string{string(authz.PermModerationCaseAct), "not.a.permission"}}}
		return identityapi.ResolvePrincipal200JSONResponse{UserId: user, UserType: identityapi.PrincipalUserTypeStaff, Status: status,
			Grants: &grants}, nil
	}}
	p, err := Principals{F: fake}.Platform(context.Background(), user)
	if err != nil || !p.Has(authz.PermModerationCaseAct) || !p.HasPlatformRole() || p.UserType != authz.UserTypeStaff {
		t.Fatalf("principal: %+v %v", p, err)
	}
	notFound = true
	if _, err := (Principals{F: fake}).Platform(context.Background(), user); !errors.Is(err, port.ErrPrincipalUnknown) {
		t.Fatalf("404: %v", err)
	}
	notFound, status = false, identityapi.PrincipalStatusLocked
	if _, err := (Principals{F: fake}).Platform(context.Background(), user); !errors.Is(err, port.ErrPrincipalUnknown) {
		t.Fatalf("locked: %v", err)
	}
	status, failure = identityapi.PrincipalStatusActive, errors.New("TEST DOUBLE: identity unavailable")
	if _, err := (Principals{F: fake}).Platform(context.Background(), user); err == nil || errors.Is(err, port.ErrPrincipalUnknown) {
		t.Fatalf("failure: %v", err)
	}
	var late LatePrincipals
	if _, err := late.Platform(context.Background(), user); err == nil {
		t.Fatal("no facade must fail closed")
	}
	failure = nil
	late.Set(fake)
	if p, err := late.Platform(context.Background(), user); err != nil || !p.Has(authz.PermModerationCaseAct) {
		t.Fatalf("late: %v", err)
	}
}
