package wiring_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

type fakeIdentity struct {
	resp  identityapi.ResolvePrincipalResponseObject
	err   error
	delay time.Duration
	got   identityapi.ResolvePrincipalRequestObject
}

func (f *fakeIdentity) ResolvePrincipal(ctx context.Context, req identityapi.ResolvePrincipalRequestObject) (identityapi.ResolvePrincipalResponseObject, error) {
	f.got = req
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.resp, f.err
}

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F003-AC-28: the adapter calls resolvePrincipal through modcall and maps the facade answer.
func TestPrincipalResolver(t *testing.T) {
	tenant, school := uuid.New(), uuid.New()
	tid, sid := tenant, school
	tt := identityapi.TenantType("school")
	grants := []identityapi.PrincipalGrant{{RoleCode: "school_admin", RoleScopeKind: "tenant", SchoolId: &sid,
		Permissions: []string{"lms.student.read", "not.a.permission"}}}
	ok := identityapi.ResolvePrincipal200JSONResponse{UserId: uuid.New(), TenantId: &tid, TenantType: &tt, UserType: "staff",
		Status: identityapi.PrincipalStatusActive, Grants: &grants, Roles: []string{"school_admin"}, Permissions: []string{"lms.student.read"}}
	fake := &fakeIdentity{resp: ok}
	r := wiring.PrincipalResolver{Identity: fake}
	ctx := tenancy.WithModule(context.Background(), "learning")
	req := authz.ResolveRequest{Subject: authz.SubjectRef{Issuer: "https://kc/realms/eduworld", Subject: "kc-1"}, TenantID: &tenant, ClientID: "eduworld-portal"}

	p, err := r.Resolve(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if *p.TenantID != tenant || len(p.Grants) != 1 || *p.Grants[0].SchoolID != school || len(p.Grants[0].Permissions) != 1 || !p.Has(authz.PermLmsStudentRead) {
		t.Fatalf("principal: %+v", p.PrincipalData)
	}
	if *fake.got.Body.Issuer != req.Subject.Issuer || *fake.got.Body.Subject != "kc-1" || *fake.got.Body.TenantId != tenant {
		t.Fatalf("request: %+v", fake.got.Body)
	}
	if n := modcall.Requests().WithLabelValues("learning", "identity", "resolvePrincipal", "ok"); n == nil {
		t.Fatal("call not made through modcall")
	}

	fake.resp = identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse{Code: "NOT_FOUND"}
	if _, err := r.Resolve(ctx, req); !errors.Is(err, authz.ErrNotMember) {
		t.Fatalf("no membership: %v", err)
	}

	noGrants := ok
	noGrants.Grants = nil
	fake.resp = noGrants
	if _, err := r.Resolve(ctx, req); !errors.Is(err, wiring.ErrNoGrants) || codeOf(err) != problem.Unavailable {
		t.Fatalf("principal without grants must fail closed: %v", err)
	}

	locked := ok
	locked.Status = identityapi.PrincipalStatusLocked
	fake.resp = locked
	if _, err := r.Resolve(ctx, req); codeOf(err) != problem.AccountLocked {
		t.Fatalf("locked: %v", err)
	}

	fake.resp, fake.delay = ok, time.Second
	if _, err := r.Resolve(ctx, req); codeOf(err) != problem.Unavailable {
		t.Fatalf("deadline: %v", err)
	}
	fake.delay = 0

	// A module outside x-allowed-callers of resolvePrincipal is refused.
	if _, err := r.Resolve(tenancy.WithModule(context.Background(), "realtime"), req); codeOf(err) != problem.CallerNotAllowed {
		t.Fatalf("realtime caller: %v", err)
	}
}

// The generated identityapi.Fake serves caller-side facade tests (make test-contract).
func TestGeneratedFake(t *testing.T) {
	var fake identityapi.Fake
	r := wiring.PrincipalResolver{Identity: &fake}
	ctx := tenancy.WithModule(context.Background(), "game")
	req := authz.ResolveRequest{Subject: authz.SubjectRef{Issuer: "iss", Subject: "s"}}
	if _, err := r.Resolve(ctx, req); codeOf(err) != problem.Internal || !errors.Is(err, identityapi.ErrNotFaked) {
		t.Fatalf("unset fake: %v", err)
	}
	fake.ResolvePrincipalFunc = func(context.Context, identityapi.ResolvePrincipalRequestObject) (identityapi.ResolvePrincipalResponseObject, error) {
		return identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse{Code: "NOT_FOUND"}, nil
	}
	if _, err := r.Resolve(ctx, req); !errors.Is(err, authz.ErrNotMember) {
		t.Fatalf("faked 404: %v", err)
	}
}
