//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// staffUser inserts a staff user (home tenant or platform) with its local identity (platform role).
func staffUser(ctx context.Context, t *testing.T, e *env, username string, home *string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, provisioned_at, status, user_type, username, email,
		display_name, primary_tenant_id) VALUES ($1, now(), now(), now(), 'active', 'staff', $2, $4, $2, $3::uuid)`, id, username, home, username+"@test.vn"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
		VALUES ($1, now(), now(), 'http://kc/realms/eduworld', $2, $3)`, uuid.New(), "kc-"+username, id); err != nil {
		t.Fatal(err)
	}
	return id
}

// grantRole inserts a user role (platform role; the use case comes with US5).
func grantRole(ctx context.Context, t *testing.T, e *env, user uuid.UUID, role string, tenant *string, scope string, school *string, validTo string) {
	t.Helper()
	q := `INSERT INTO identity.user_roles (user_role_id, tenant_id, created_at, updated_at, user_id, role_id, scope_type, scope_id, school_id,
		valid_from, valid_to, granted_by, source) VALUES ($1, $2::uuid, now(), now(), $3, $4, $5, $6::uuid, $6::uuid, now() - interval '1 day', ` + validTo + `, $3, 'manual')`
	if _, err := e.platform.ExecContext(ctx, q, uuid.New(), tenant, user, seed.RoleID(role), scope, school); err != nil {
		t.Fatalf("grant %s: %v", role, err)
	}
}

// F012-AC-11, AC-12 (TC-TEN-002, TC-TEN-011) through the identity facade.
func TestResolvePrincipal(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-prin-a")
	b := seedTenant(ctx, t, e.platform, "pgd-prin-b")
	model := read.PrincipalModel{Session: e.read}
	f := facade.Principals{Resolve: query.ResolvePrincipal{Model: model}, Memberships: query.CheckMemberships{Model: model}}
	resolve := func(issuer, subject string, user *uuid.UUID, tenant *string) identityapi.ResolvePrincipalResponseObject {
		t.Helper()
		body := identityapi.PrincipalResolveRequest{UserId: user}
		if issuer != "" {
			body.Issuer, body.Subject = &issuer, &subject
		}
		if tenant != nil {
			id := uuid.MustParse(*tenant)
			body.TenantId = &id
		}
		resp, err := f.ResolvePrincipal(ctx, identityapi.ResolvePrincipalRequestObject{Body: &body})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	principal := func(r identityapi.ResolvePrincipalResponseObject) identityapi.Principal {
		t.Helper()
		p, ok := r.(identityapi.ResolvePrincipal200JSONResponse)
		if !ok {
			t.Fatalf("response = %T", r)
		}
		return identityapi.Principal(p)
	}

	// Tenant staff, tenant-wide role, resolved by (issuer, subject).
	adm := staffUser(ctx, t, e, "ad.prin.a", &a.tenant)
	grantRole(ctx, t, e, adm, "school_admin", &a.tenant, "tenant", nil, "NULL")
	grantRole(ctx, t, e, adm, "academic_manager", &a.tenant, "tenant", nil, "now() - interval '1 hour'") // expired
	p := principal(resolve("http://kc/realms/eduworld", "kc-ad.prin.a", nil, &a.tenant))
	if p.UserId != adm || p.UserType != "staff" || p.Status != "active" || !slices.Equal(p.Roles, []string{"school_admin"}) ||
		!slices.Contains(p.Permissions, "tenant.sis.import") || slices.Contains(p.Permissions, "curriculum.write") || p.Grants == nil ||
		len(*p.Grants) != 1 || p.TenantType == nil || *p.TenantType != "unit" || p.IsSystemAdmin {
		t.Fatalf("tenant staff principal = %+v", p)
	}
	if len(p.Scopes) != 1 || p.Scopes[0].ScopeType != "tenant" || p.Scopes[0].ScopeId == nil || p.Scopes[0].ScopeId.String() != a.tenant {
		t.Fatalf("scopes = %+v", p.Scopes)
	}
	// AC-12: another tenant → typed 404 (the caller maps it to TENANT_MISMATCH).
	if _, ok := resolve("http://kc/realms/eduworld", "kc-ad.prin.a", nil, &b.tenant).(identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse); !ok {
		t.Fatal("other tenant must be 404")
	}
	if _, ok := resolve("http://kc/realms/eduworld", "kc-unknown", nil, &a.tenant).(identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse); !ok {
		t.Fatal("unknown subject must be 404")
	}
	// Without tenant a tenant user has no platform grant (only routes exempt from X-Tenant-Id).
	if np := principal(resolve("", "", &adm, nil)); len(*np.Grants) != 0 || np.TenantId != nil {
		t.Fatalf("no-tenant principal = %+v", np)
	}

	// School-scoped role: the grant carries the school.
	sch := staffUser(ctx, t, e, "ad.prin.school", &a.tenant)
	grantRole(ctx, t, e, sch, "school_admin", &a.tenant, "school", &a.school, "NULL")
	sp := principal(resolve("", "", &sch, &a.tenant))
	if g := (*sp.Grants)[0]; g.SchoolId == nil || g.SchoolId.String() != a.school || sp.Scopes[0].ScopeType != "school" {
		t.Fatalf("school grant = %+v scopes = %+v", g, sp.Scopes)
	}
	// Revoked (soft-deleted) roles are not part of the Principal.
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.user_roles SET deleted_at = now() WHERE user_id = $1`, sch); err != nil {
		t.Fatal(err)
	}
	if _, ok := resolve("", "", &sch, &a.tenant).(identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse); !ok {
		t.Fatal("a staff without valid role is no longer a member")
	}

	// System Admin: platform scope, and any active tenant with the platform grants.
	sa := staffUser(ctx, t, e, "sa.prin", nil)
	grantRole(ctx, t, e, sa, "system_admin", nil, "platform", nil, "NULL")
	pp := principal(resolve("", "", &sa, nil))
	if !pp.IsSystemAdmin || pp.TenantId != nil || !slices.Contains(pp.Permissions, "platform.tenant.manage") {
		t.Fatalf("system admin platform = %+v", pp)
	}
	if tp := principal(resolve("", "", &sa, &b.tenant)); tp.TenantId == nil || tp.TenantId.String() != b.tenant || !tp.IsSystemAdmin {
		t.Fatalf("system admin in tenant = %+v", tp)
	}

	// Student (ADR-0095 mục 2): the school and grade of the student row, no enrollment and no class.
	st := enrolled(ctx, t, e.platform, a, "hs.prin")
	sp2 := principal(resolve("", "", &st, &a.tenant))
	if sp2.UserType != "student" || sp2.SchoolIds == nil || len(*sp2.SchoolIds) != 1 || (*sp2.SchoolIds)[0].String() != a.school ||
		sp2.GradeLevel == nil || *sp2.GradeLevel != 3 || len(sp2.Permissions) != 0 {
		t.Fatalf("student principal = %+v", sp2)
	}

	// checkMemberships in the TenantScope of the caller, limited to the school of the student.
	scope, _ := authz.EventScope(uuid.MustParse(a.tenant))
	check := func(school string, users ...uuid.UUID) map[uuid.UUID]bool {
		t.Helper()
		sid := uuid.MustParse(school)
		resp, err := f.CheckMemberships(tenancy.WithScope(ctx, scope), identityapi.CheckMembershipsRequestObject{Body: &identityapi.MembershipCheckRequest{
			UserIds: users, SchoolId: &sid}})
		if err != nil {
			t.Fatal(err)
		}
		res := resp.(identityapi.CheckMemberships200JSONResponse)
		if res.TenantId.String() != a.tenant || len(res.Results) != len(users) {
			t.Fatalf("checkMemberships = %+v", res)
		}
		got := map[uuid.UUID]bool{}
		for _, r := range res.Results {
			got[r.UserId] = r.IsMember
		}
		return got
	}
	if got := check(a.school, st, sa, uuid.New()); !got[st] || got[sa] {
		t.Fatalf("checkMemberships school = %v", got)
	}
	if got := check(a.otherSchool, st); got[st] {
		t.Fatal("a student is a member of its own school only")
	}
	// A student that is no longer effective (SIS inactive) is not a member and has no Principal in the tenant.
	if _, err := e.platform.ExecContext(ctx, `UPDATE org.students SET status = 'inactive' WHERE student_id = $1`, st); err != nil {
		t.Fatal(err)
	}
	if got := check(a.school, st); got[st] {
		t.Fatal("an inactive student is not a member")
	}
	if _, ok := resolve("", "", &st, &a.tenant).(identityapi.ResolvePrincipal404ApplicationProblemPlusJSONResponse); !ok {
		t.Fatal("an inactive student has no membership in the tenant")
	}
	if _, err := f.CheckMemberships(ctx, identityapi.CheckMembershipsRequestObject{Body: &identityapi.MembershipCheckRequest{UserIds: []uuid.UUID{st}}}); err == nil {
		t.Fatal("checkMemberships without a tenant scope must fail")
	}
}
