// Package authztest is the authorization test harness (F002 T050, BACKEND_DESIGN §15): principal
// fixtures per role (tenant A/B staff, students, platform staff), contexts carrying principal + scope,
// assertions for 404 (outside tenant) / 403 (role), and Cover — the marker tools/routematrix reads to
// prove every implemented operation has tenant-isolation and role-denied tests (F002-AC-28).
package authztest

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Case is a required authorization test case of an operation.
type Case string

const (
	// CrossTenant: a resource of tenant B requested in tenant A answers 404 NOT_FOUND.
	CrossTenant Case = "cross_tenant"
	// RoleDenied: a principal of the right tenant without the permission answers 403.
	RoleDenied Case = "role_denied"
)

// Fixed tenants make failures reproducible.
var (
	TenantA = uuid.MustParse("0193f0a2-7c1e-7000-8000-00000000000a")
	TenantB = uuid.MustParse("0193f0a2-7c1e-7000-8000-00000000000b")
)

func role(name string) authz.RoleDef {
	for _, r := range authz.SystemRoles() {
		if r.Name == name {
			return r
		}
	}
	panic("authztest: unknown system role " + name)
}

// Option adjusts a fixture.
type Option func(*authz.PrincipalData)

// AtSchool limits every grant to one school.
func AtSchool(id uuid.UUID) Option {
	return func(d *authz.PrincipalData) {
		for i := range d.Grants {
			d.Grants[i].SchoolID = &id
		}
	}
}

// ForSubject limits every grant to one subject.
func ForSubject(id uuid.UUID) Option {
	return func(d *authz.PrincipalData) {
		for i := range d.Grants {
			d.Grants[i].SubjectID = &id
		}
	}
}

func build(d authz.PrincipalData, opts []Option) authz.Principal {
	for _, o := range opts {
		o(&d)
	}
	return authz.NewPrincipal(d)
}

func grant(r authz.RoleDef) authz.Grant {
	return authz.Grant{Role: r.Name, ScopeKind: r.ScopeKind, Permissions: append([]authz.Permission(nil), r.Permissions...)}
}

// Staff returns portal staff of tenant holding a tenant system role (e.g. "school_admin").
func Staff(tenant uuid.UUID, roleName string, opts ...Option) authz.Principal {
	r := role(roleName)
	if r.ScopeKind != authz.RoleScopeTenant {
		panic("authztest: " + roleName + " is not a tenant role; use Platform")
	}
	t := tenant
	return build(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &t, TenantType: authz.TenantTypeSchool,
		Grants: []authz.Grant{grant(r)}, Channel: authz.ChannelPortal}, opts)
}

// NoRole returns portal staff of tenant without any permission (role-denied cases).
func NoRole(tenant uuid.UUID) authz.Principal {
	t := tenant
	return authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &t,
		Channel: authz.ChannelPortal})
}

// Student returns a student of tenant using the game client (students hold no portal permission).
func Student(tenant uuid.UUID) authz.Principal {
	t := tenant
	return authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStudent, TenantID: &t,
		TenantType: authz.TenantTypeSchool, Channel: authz.ChannelGame})
}

// Platform returns platform staff holding a platform system role, signed in on the portal (no MFA, ADR-0094).
func Platform(roleName string) authz.Principal {
	r := role(roleName)
	if r.ScopeKind != authz.RoleScopePlatform {
		panic("authztest: " + roleName + " is not a platform role; use Staff")
	}
	return authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, Grants: []authz.Grant{grant(r)},
		IsSystemAdmin: roleName == "system_admin", Channel: authz.ChannelPortal})
}

// Ctx returns ctx carrying p and the scope the principal middleware would set.
func Ctx(ctx context.Context, p authz.Principal) context.Context {
	ctx = authz.WithPrincipal(ctx, p)
	if s, err := authz.ScopeFor(p); err == nil {
		return tenancy.WithScope(ctx, s)
	}
	if s, err := authz.PlatformScope(p); err == nil {
		return tenancy.WithScope(ctx, s)
	}
	return ctx
}

// Cover declares that the calling test covers the given cases of operationID. tools/routematrix finds
// these calls statically; arguments must be a string literal and Case constants.
func Cover(t testing.TB, operationID string, cases ...Case) {
	t.Helper()
	if operationID == "" || len(cases) == 0 {
		t.Fatal("authztest.Cover needs an operation id and at least one case")
	}
}

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// ExpectNotFound fails unless err is 404 NOT_FOUND (resource outside the session tenant).
func ExpectNotFound(t testing.TB, err error) {
	t.Helper()
	if c := code(err); c != problem.NotFound {
		t.Fatalf("want 404 NOT_FOUND (outside tenant), got %q (%v)", c, err)
	}
}

// ExpectDenied fails unless err is 403 PERMISSION_DENIED.
func ExpectDenied(t testing.TB, err error) {
	t.Helper()
	if c := code(err); c != problem.PermissionDenied {
		t.Fatalf("want 403 PERMISSION_DENIED, got %q (%v)", c, err)
	}
}
