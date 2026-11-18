//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F010-AC-07..AC-09, AC-17 with the real repositories, RLS and cross-tenant audit.
func TestSchoolLifecycle(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	mk := func(code string) dto.Tenant {
		out, err := command.CreateTenant{TenantDeps: e.deps}.Handle(sa, dto.CreateTenant{Code: code, Name: code, TenantType: "unit", Alias: code})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	ta, tb := mk("pgd-a"), mk("pgd-b")
	sd := command.SchoolDeps{UoW: e.uow, Tenants: write.TenantRepo{}, Schools: write.SchoolRepo{}, Catalog: write.CatalogRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}

	created, err := command.CreateAdminTenantSchool{SchoolDeps: sd}.Handle(sa, dto.CreateSchool{TenantID: ta.TenantID, Code: "th-a1",
		Name: dto.LocalizedName{Vi: "TH A1", En: "A1 Primary"}, ProvinceCode: "vn.ha-noi", SchoolLevel: "primary", GradeMin: 1, GradeMax: 5})
	if err != nil {
		t.Fatal(err)
	}
	if created.Province == nil || created.Province.Name["vi"] == "" || created.Status != "active" {
		t.Fatalf("created = %+v", created)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'cross_tenant.accessed'`); n == 0 {
		t.Fatal("System Admin write into a tenant must record cross_tenant.accessed (ADR-0089 §3)")
	}
	// F010-AC-07: a former province is refused.
	if _, err := (command.CreateAdminTenantSchool{SchoolDeps: sd}).Handle(sa, dto.CreateSchool{TenantID: ta.TenantID, Code: "th-a2",
		Name: dto.LocalizedName{Vi: "TH A2"}, ProvinceCode: "vn.old.ha-giang", SchoolLevel: "primary", GradeMin: 1, GradeMax: 5}); code(err) != problem.ValidationFailed {
		t.Fatalf("former province: %v", err)
	}

	// An imported school without province waits in pending_review (F010-AC-08), inserted in tenant A.
	pending, _ := school.NewImported(uuid.New(), ta.TenantID, "th-a3", school.Name{Vi: "TH A3"}, "VN", nil, school.LevelPrimary, 1, 5, nil, nil)
	scopeA, _ := authz.EventScope(ta.TenantID)
	if err := e.uow.InTenantTx(ctx, scopeA, func(ctx context.Context) error { return write.SchoolRepo{}.Create(ctx, pending) }); err != nil {
		t.Fatal(err)
	}
	adminA := authztest.Ctx(ctx, authztest.Staff(ta.TenantID, "tenant_admin"))
	prov := "vn.ha-noi"
	upd, err := command.UpdateTenantSchool{SchoolDeps: sd}.Handle(adminA, dto.UpdateSchool{SchoolID: pending.ID, IfMatch: 1, ProvinceCode: &prov})
	if err != nil || upd.Status != "pending_review" || upd.Version != 2 {
		t.Fatalf("set province = %+v %v", upd, err)
	}
	ok, err := command.ApproveTenantSchool{SchoolDeps: sd}.Handle(adminA, pending.ID, 2)
	if err != nil || ok.Status != "active" {
		t.Fatalf("approve (F010-AC-09) = %+v %v", ok, err)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'school.updated' AND tenant_id = $1
		AND envelope->'payload'->>'status' = 'active' AND envelope->'payload'->>'school_id' = $2`, ta.TenantID, pending.ID.String()); n != 1 {
		t.Fatalf("school.updated after approval = %d", n)
	}
	// F010-AC-17: tenant B never sees tenant A's school (404, RLS + filter).
	adminB := authztest.Ctx(ctx, authztest.Staff(tb.TenantID, "tenant_admin"))
	name := dto.LocalizedName{Vi: "hijack"}
	if _, err := (command.UpdateTenantSchool{SchoolDeps: sd}).Handle(adminB, dto.UpdateSchool{SchoolID: pending.ID, IfMatch: 3, Name: &name}); code(err) != problem.NotFound {
		t.Fatalf("tenant B on tenant A's school: %v", err)
	}
	// Merge patch clears nullable fields.
	moet := "79000123"
	withMoet := dto.UpdateSchool{SchoolID: pending.ID, IfMatch: 3}
	withMoet.MoetCode.Set, withMoet.MoetCode.Value = true, &moet
	if s, err := (command.UpdateTenantSchool{SchoolDeps: sd}).Handle(adminA, withMoet); err != nil || s.MoetCode == nil || *s.MoetCode != moet {
		t.Fatalf("set moet = %+v %v", s, err)
	}
	clear := dto.UpdateSchool{SchoolID: pending.ID, IfMatch: 4}
	clear.MoetCode.Set = true
	if s, err := (command.UpdateTenantSchool{SchoolDeps: sd}).Handle(adminA, clear); err != nil || s.MoetCode != nil {
		t.Fatalf("clear moet = %+v %v", s, err)
	}
}
