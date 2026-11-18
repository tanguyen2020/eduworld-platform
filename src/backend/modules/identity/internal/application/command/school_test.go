package command_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

var (
	hanoi  = catalog.Province{ID: uuid.New(), Code: "vn.ha-noi", CountryCode: "VN", Status: catalog.StatusActive, Name: map[string]string{"vi": "Hà Nội", "en": "Ha Noi"}}
	oldHaG = catalog.Province{ID: uuid.New(), Code: "vn.old.ha-giang", CountryCode: "VN", Status: catalog.StatusInactive}
)

type fakeCatalog struct{}

func (fakeCatalog) ProvinceByCode(_ context.Context, code string) (catalog.Province, error) {
	switch code {
	case hanoi.Code:
		return hanoi, nil
	case oldHaG.Code:
		return oldHaG, nil
	}
	return catalog.Province{}, problem.New(problem.NotFound, "")
}
func (fakeCatalog) Successor(context.Context, uuid.UUID) (*catalog.Province, error) {
	return &hanoi, nil
}

// fakeSchools filters by the tenants of the scope like the ent interceptor + RLS.
type fakeSchools struct {
	rows  map[uuid.UUID]*school.School
	scope tenancy.Scope
}

func visible(ctx context.Context, tenantID uuid.UUID) bool {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return false
	}
	for _, t := range s.TenantIDs() {
		if t == tenantID {
			return true
		}
	}
	return false
}

func (f *fakeSchools) Create(ctx context.Context, s *school.School) error {
	f.scope, _ = tenancy.ScopeFrom(ctx)
	if !visible(ctx, s.TenantID) {
		return problem.New(problem.PermissionDenied, "out of scope")
	}
	c := *s
	f.rows[s.ID] = &c
	return nil
}
func (f *fakeSchools) Get(ctx context.Context, id uuid.UUID) (*school.School, error) {
	r, ok := f.rows[id]
	if !ok || !visible(ctx, r.TenantID) {
		return nil, problem.New(problem.NotFound, "")
	}
	c := *r
	return &c, nil
}
func (f *fakeSchools) Save(_ context.Context, s *school.School) error {
	s.Version++
	c := *s
	f.rows[s.ID] = &c
	return nil
}
func (f *fakeSchools) CountInTenant(_ context.Context, t uuid.UUID) (int, error) {
	n := 0
	for _, r := range f.rows {
		if r.TenantID == t {
			n++
		}
	}
	return n, nil
}

func schoolDeps() (command.SchoolDeps, *fakeTenants, *fakeSchools, *fakeEvents) {
	tn, sc, ev := newTenants(), &fakeSchools{rows: map[uuid.UUID]*school.School{}}, &fakeEvents{}
	return command.SchoolDeps{UoW: &fakeUoW{}, Tenants: tn, Schools: sc, Catalog: fakeCatalog{}, Events: ev, Audit: &fakeAudit{}}, tn, sc, ev
}

func addTenant(r *fakeTenants, id uuid.UUID, typ tenant.Type) {
	r.rows[id] = &tenant.Tenant{ID: id, Code: id.String()[:8], Type: typ, Status: tenant.StatusActive, Version: 1}
}

// F010-AC-07: active province only; school tenant has one school; System Admin writes with a
// CrossTenantScope of the target tenant (ADR-0089 §3).
func TestCreateAdminTenantSchool(t *testing.T) {
	d, tenants, schools, ev := schoolDeps()
	addTenant(tenants, authztest.TenantA, tenant.TypeSchool)
	h := command.CreateAdminTenantSchool{SchoolDeps: d}
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	in := dto.CreateSchool{TenantID: authztest.TenantA, Code: "th-a", Name: dto.LocalizedName{Vi: "TH A"}, ProvinceCode: "vn.ha-noi",
		SchoolLevel: "primary", GradeMin: 1, GradeMax: 5}
	out, err := h.Handle(sa, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "active" || out.Province.Code != "vn.ha-noi" || out.TenantID != authztest.TenantA {
		t.Fatalf("out = %+v", out)
	}
	if cs, ok := schools.scope.(tenancy.CrossTenantScope); !ok || cs.TenantIDs()[0] != authztest.TenantA || cs.Reason() != tenancy.ReasonSystemAdminCrossTenant {
		t.Fatalf("scope = %#v", schools.scope)
	}
	if ev.events[0].Type != "school.updated" || *ev.events[0].TenantID != authztest.TenantA {
		t.Fatalf("event = %+v", ev.events[0])
	}
	in.Code = "th-a2"
	if _, err := h.Handle(sa, in); code(err) != problem.ValidationFailed {
		t.Fatalf("second school in a school tenant: %v", err)
	}
	addTenant(tenants, authztest.TenantB, tenant.TypeUnit)
	in.TenantID, in.ProvinceCode = authztest.TenantB, "vn.old.ha-giang"
	if _, err := h.Handle(sa, in); code(err) != problem.ValidationFailed {
		t.Fatalf("inactive province: %v", err)
	}
	in.ProvinceCode = "vn.unknown"
	if _, err := h.Handle(sa, in); code(err) != problem.ValidationFailed {
		t.Fatalf("unknown province: %v", err)
	}
	authztest.ExpectDenied(t, func() error {
		_, err := h.Handle(authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantB, "tenant_admin")), in)
		return err
	}())
}

// F010-AC-08/09: pending school approved after choosing a province; F010-AC-17/18: other tenant → 404,
// school-limited grant on another school → 403.
func TestTenantSchoolUpdateAndApprove(t *testing.T) {
	d, _, schools, ev := schoolDeps()
	pending, _ := school.NewImported(uuid.New(), authztest.TenantA, "th-p", school.Name{Vi: "TH P"}, "VN", nil, school.LevelPrimary, 1, 5, nil, nil)
	schools.rows[pending.ID] = pending
	other, _ := school.New(uuid.New(), authztest.TenantB, "th-b", school.Name{Vi: "TH B"}, hanoi, school.LevelPrimary, 1, 5, nil, nil)
	schools.rows[other.ID] = other
	admin := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin"))

	approve := command.ApproveTenantSchool{SchoolDeps: d}
	if _, err := approve.Handle(admin, pending.ID, 1); code(err) != problem.ValidationFailed {
		t.Fatalf("approve without province: %v", err)
	}
	upd := command.UpdateTenantSchool{SchoolDeps: d}
	prov := "vn.ha-noi"
	s1, err := upd.Handle(admin, dto.UpdateSchool{SchoolID: pending.ID, IfMatch: 1, ProvinceCode: &prov})
	if err != nil || s1.Version != 2 || s1.Status != "pending_review" {
		t.Fatalf("set province = %+v %v", s1, err)
	}
	if _, err := approve.Handle(admin, pending.ID, 1); code(err) != problem.VersionMismatch {
		t.Fatalf("stale If-Match: %v", err)
	}
	s2, err := approve.Handle(admin, pending.ID, 2)
	if err != nil || s2.Status != "active" {
		t.Fatalf("approve = %+v %v", s2, err)
	}
	last := ev.events[len(ev.events)-1].Payload.(command.SchoolPayload)
	if last.Status != "active" || last.ChangedFields[0] != "status" || last.SettingsSubset == nil {
		t.Fatalf("school.updated = %+v", last)
	}
	if _, err := approve.Handle(admin, pending.ID, 3); code(err) != problem.InvalidStateTransition {
		t.Fatalf("approve active school: %v", err)
	}
	disabled := "disabled"
	if _, err := upd.Handle(admin, dto.UpdateSchool{SchoolID: pending.ID, IfMatch: 3, Status: &disabled}); code(err) != problem.ValidationFailed {
		t.Fatalf("tenant changing status: %v", err)
	}
	name := dto.LocalizedName{Vi: "x"}
	if _, err := upd.Handle(admin, dto.UpdateSchool{SchoolID: other.ID, IfMatch: 1, Name: &name}); code(err) != problem.NotFound {
		t.Fatalf("school of tenant B from A: %v (want 404)", err)
	}
	otherSchool := uuid.New()
	limited := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin", authztest.AtSchool(otherSchool)))
	authztest.ExpectDenied(t, func() error {
		_, err := upd.Handle(limited, dto.UpdateSchool{SchoolID: pending.ID, IfMatch: 3, Name: &name})
		return err
	}())
}

func TestUpdateAdminTenantSchoolStatus(t *testing.T) {
	d, _, schools, _ := schoolDeps()
	s, _ := school.New(uuid.New(), authztest.TenantA, "th-a", school.Name{Vi: "TH A"}, hanoi, school.LevelPrimary, 1, 5, nil, nil)
	schools.rows[s.ID] = s
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	h := command.UpdateAdminTenantSchool{SchoolDeps: d}
	disabled := "disabled"
	out, err := h.Handle(sa, dto.UpdateSchool{TenantID: authztest.TenantA, SchoolID: s.ID, IfMatch: 1, Status: &disabled, Reason: "closed"})
	if err != nil || out.Status != "disabled" {
		t.Fatalf("disable = %+v %v", out, err)
	}
	if _, err := h.Handle(sa, dto.UpdateSchool{TenantID: authztest.TenantB, SchoolID: s.ID, IfMatch: 2, Status: &disabled}); code(err) != problem.NotFound {
		t.Fatalf("school through the wrong tenant path: %v", err)
	}
}
