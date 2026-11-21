package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Org holds the catalog and organization queries of F010 (one typed method per operation, ADR-0087).
type Org struct {
	Model   read.OrgReadModel
	Cursors *page.Codec
	Clock   func() time.Time // days_since_last_login of listTenantGradeStudents; nil = time.Now
}

// Page is a keyset page of items (SchoolPage, GradeStudentPage).
type Page[T any] struct {
	Items []T       `json:"items"`
	Page  page.Info `json:"page"`
}

// ListInput is the paging of a list query.
type ListInput struct {
	Cursor string
	Limit  int
}

func pageOf[T any](c *page.Codec, rows []T, limit int, key func(T) (string, uuid.UUID)) Page[T] {
	out := Page[T]{Items: rows, Page: page.Info{Limit: limit}}
	if len(rows) > limit {
		out.Items = rows[:limit]
		k, id := key(out.Items[limit-1])
		next := c.Encode(page.Cursor{Key: k, ID: id})
		out.Page.HasMore, out.Page.NextCursor = true, &next
	}
	if out.Items == nil {
		out.Items = []T{}
	}
	return out
}

func authenticated(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, problem.New(problem.Unauthenticated, "")
	}
	return p, nil
}

func (q Org) cursor(s string) (*page.Cursor, error) {
	c, err := q.Cursors.Decode(s)
	if err != nil {
		return nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	return c, nil
}

// tenantRead authorizes perm on the session tenant (optionally one school) and sets its TenantScope.
func tenantRead(ctx context.Context, perm authz.Permission, school *uuid.UUID) (context.Context, authz.Principal, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return ctx, p, err
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, p, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: school}); err != nil {
		return ctx, p, err
	}
	return tenancy.WithScope(ctx, sc), p, nil
}

func schoolKey(s dto.School) (string, uuid.UUID) { return s.Code, s.SchoolID }

// ListCountries is listCountries (any authenticated user).
func (q Org) ListCountries(ctx context.Context) (out []dto.Country, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListCountries")
	defer func() { done(err) }()
	if _, err = authenticated(ctx); err != nil {
		return nil, err
	}
	return q.Model.Countries(ctx)
}

// ListProvinces is listProvinces (any authenticated user), filtered by country and status.
func (q Org) ListProvinces(ctx context.Context, country, status string) (out []dto.Province, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListProvinces")
	defer func() { done(err) }()
	if _, err = authenticated(ctx); err != nil {
		return nil, err
	}
	if status != "" && status != "active" && status != "inactive" {
		return nil, problem.Invalid("status", problem.SubInvalid, "must be active or inactive")
	}
	return q.Model.Provinces(ctx, country, status)
}

// CatalogSchoolsInput filters listCatalogSchools.
type CatalogSchoolsInput struct {
	ListInput
	ProvinceCode, Level, Q string
	Status                 string // school status (schools_status_chk); empty = every status
}

// schoolStatuses are the values of schools_status_chk (f010 migration).
var schoolStatuses = map[string]bool{"": true, "active": true, "pending_review": true, "disabled": true}

// ListCatalogSchools is listCatalogSchools: portal staff see the schools of the scope set by the request —
// tenant users their session tenant (tenant_ids ignored); a platform principal without X-Tenant-Id only through
// the CrossTenantScope httpx builds from tenant_ids (platform.cross_tenant.read, audited
// cross_tenant.accessed). A plain PlatformScope never lists the schools of every tenant (F010-AC-10, OQ F057-4).
func (q Org) ListCatalogSchools(ctx context.Context, in CatalogSchoolsInput) (out Page[dto.School], err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListCatalogSchools")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType != authz.UserTypeStaff {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if p.TenantID != nil { // tenant users (and a System Admin with X-Tenant-Id): that tenant only
		sc, err := authz.ScopeFor(p)
		if err != nil {
			return out, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		ctx = tenancy.WithScope(ctx, sc)
	} else { // no session tenant: platform staff only, through tenant_ids (a tenant user without X-Tenant-Id → 403)
		if err := authz.Require(ctx, p, authz.PermPlatformCrossTenantRead, authz.Resource{}); err != nil {
			return out, err
		}
		sc, _ := tenancy.ScopeFrom(ctx)
		if _, cross := sc.(tenancy.CrossTenantScope); !cross {
			authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"operation": "listCatalogSchools", "why": "tenant_ids_required"})
			return out, problem.New(problem.PermissionDenied, "tenant_ids is required without X-Tenant-Id")
		}
	}
	if !schoolStatuses[in.Status] {
		return out, problem.Invalid("status", problem.SubInvalid, "must be pending_review, active or disabled")
	}
	after, err := q.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	rows, err := q.Model.Schools(ctx, read.SchoolFilter{ProvinceCode: in.ProvinceCode, Level: in.Level, Q: in.Q, Status: in.Status,
		After: after, Limit: limit})
	if err != nil {
		return out, err
	}
	return pageOf(q.Cursors, rows, limit, schoolKey), nil
}

// ListTenantSchools is listTenantSchools (tenant.school.read): a grant without a school limit (scope tenant or
// subject) lists every school of the session tenant; school-limited grants (School Admin, Academic Manager of a
// school) list their schools only (F012 Clarifications 2026-11-21).
func (q Org) ListTenantSchools(ctx context.Context, status string, in ListInput) (out Page[dto.School], err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantSchools")
	defer func() { done(err) }()
	ctx, only, err := schoolRead(ctx, nil)
	if err != nil {
		return out, err
	}
	return q.schools(ctx, nil, only, status, in)
}

// schoolRead authorizes tenant.school.read on the session tenant (one school when id is set) and sets its
// TenantScope. It returns the schools a school-limited principal may see (nil: every school of the tenant).
// Without the permission, or for a school outside every grant, authz.Require answers 403 and records
// authz.denied; another tenant's school stays invisible to the scoped query (404).
func schoolRead(ctx context.Context, id *uuid.UUID) (context.Context, []uuid.UUID, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return ctx, nil, err
	}
	all, schools := schoolsOf(p, authz.PermTenantSchoolRead)
	if !all && (len(schools) == 0 || (id != nil && !slices.Contains(schools, *id))) {
		// no grant, or a school outside every grant: authz.Require denies (403) and records authz.denied
		ctx, _, err = tenantRead(ctx, authz.PermTenantSchoolRead, id)
		if err == nil {
			err = problem.New(problem.PermissionDenied, "")
		}
		return ctx, nil, err
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	if all { // a grant without a school limit (scope tenant or subject): every school of the scope
		schools = nil
	}
	return tenancy.WithScope(ctx, sc), schools, nil
}

func (q Org) schools(ctx context.Context, tenantID *uuid.UUID, only []uuid.UUID, status string, in ListInput) (out Page[dto.School], err error) {
	after, err := q.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	rows, err := q.Model.Schools(ctx, read.SchoolFilter{TenantID: tenantID, SchoolIDs: only, Status: status, After: after, Limit: limit})
	if err != nil {
		return out, err
	}
	return pageOf(q.Cursors, rows, limit, schoolKey), nil
}

// GetTenantSchool is getTenantSchool (tenant.school.read; other tenant → 404).
func (q Org) GetTenantSchool(ctx context.Context, id uuid.UUID) (out dto.School, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetTenantSchool")
	defer func() { done(err) }()
	if ctx, _, err = schoolRead(ctx, &id); err != nil {
		return out, err
	}
	return q.Model.School(ctx, id)
}

// ListAdminTenantSchools is listAdminTenantSchools (System Admin, one target tenant, cross-tenant read).
func (q Org) ListAdminTenantSchools(ctx context.Context, tenantID uuid.UUID, status string, in ListInput) (out Page[dto.School], err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListAdminTenantSchools")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if err = authz.Require(ctx, p, authz.PermPlatformTenantManage, authz.Resource{}); err != nil {
		return out, err
	}
	sc, err := appscope.TargetTenant(p, tenantID)
	if err != nil {
		return out, err
	}
	return q.schools(tenancy.WithScope(ctx, sc), &tenantID, nil, status, in)
}

// GetMyStudentProfile is getMyStudentProfile: a student reads their own school and grade (F010-AC-19, ADR-0095).
func (q Org) GetMyStudentProfile(ctx context.Context) (out dto.StudentSummary, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetMyStudentProfile")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType != authz.UserTypeStudent {
		return out, problem.New(problem.PermissionDenied, "")
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return q.Model.StudentSummary(tenancy.WithScope(ctx, sc), p.UserID)
}

// GetStudentProfile is getStudentProfile (school.people.read on the student's school; other tenant → 404,
// other school → 403).
func (q Org) GetStudentProfile(ctx context.Context, studentID uuid.UUID) (out dto.StudentSummary, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetStudentProfile")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	if !p.Has(authz.PermSchoolPeopleRead) { // decide 403 before revealing whether the student exists
		return out, authz.Require(ctx, p, authz.PermSchoolPeopleRead, authz.Resource{TenantID: p.TenantID})
	}
	s, err := q.Model.StudentSummary(tenancy.WithScope(ctx, sc), studentID)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermSchoolPeopleRead, authz.Resource{TenantID: p.TenantID, SchoolID: &s.SchoolID}); err != nil {
		return out, err
	}
	return s, nil
}
