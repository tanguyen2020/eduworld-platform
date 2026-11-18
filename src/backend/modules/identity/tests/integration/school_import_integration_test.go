//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F010-AC-08: former province → successor + province_outdated; no province → pending_review; unknown
// province and duplicates are row errors; a dry run persists nothing.
func TestImportSchools(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	tid := uuid.New()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO platform.tenants (tenant_id, created_at, updated_at, code, name, tenant_type, keycloak_org_alias, settings)
		VALUES ($1, now(), now(), 'pgd-imp', 'PGD IMP', 'unit', 'pgd-imp', '{}')`, tid); err != nil {
		t.Fatal(err)
	}
	var successor string
	if err := e.platform.QueryRowContext(ctx, `SELECT n.code FROM catalog.province_successions s
		JOIN catalog.provinces o ON o.province_id = s.old_province_id JOIN catalog.provinces n ON n.province_id = s.new_province_id
		WHERE o.code = 'vn.old.ha-giang'`).Scan(&successor); err != nil {
		t.Fatal(err)
	}
	h := command.ImportSchools{SchoolDeps: command.SchoolDeps{UoW: e.uow, Tenants: write.TenantRepo{}, Schools: write.SchoolRepo{},
		Catalog: write.CatalogRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}, Savepoints: e.uow}
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	rows := []command.SchoolImportRow{
		{Code: "imp-1", NameVi: "TH Cũ", ProvinceCode: "vn.old.ha-giang", SchoolLevel: "primary", GradeMin: 1, GradeMax: 5},
		{Code: "imp-2", NameVi: "TH Thiếu tỉnh", SchoolLevel: "primary", GradeMin: 1, GradeMax: 5},
		{Code: "imp-3", NameVi: "TH Sai tỉnh", ProvinceCode: "vn.nowhere", SchoolLevel: "primary", GradeMin: 1, GradeMax: 5},
		{Code: "imp-1", NameVi: "TH Trùng mã", ProvinceCode: "vn.ha-noi", SchoolLevel: "primary", GradeMin: 1, GradeMax: 5},
	}
	dry, err := h.Handle(sa, tid, rows, true)
	if err != nil || !dry.DryRun || dry.CreatedRows != 2 {
		t.Fatalf("dry run = %+v %v", dry, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.schools WHERE tenant_id = $1 AND deleted_at IS NULL`, tid); n != 0 {
		t.Fatalf("dry run persisted %d schools", n)
	}
	res, err := h.Handle(sa, tid, rows, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalRows != 4 || res.CreatedRows != 2 || res.ErrorRows != 2 {
		t.Fatalf("result = %+v", res)
	}
	codes := map[int]string{}
	for _, m := range res.Errors {
		codes[m.Row] = m.Code
	}
	if codes[3] != "province_unknown" || codes[4] != "school_duplicate" {
		t.Fatalf("errors = %+v", res.Errors)
	}
	warn := map[int]string{}
	for _, m := range res.Warnings {
		warn[m.Row] = m.Code
	}
	if warn[1] != "province_outdated" || warn[2] != "province_missing" {
		t.Fatalf("warnings = %+v", res.Warnings)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.schools s JOIN catalog.provinces p ON p.province_id = s.province_id
		WHERE s.tenant_id = $1 AND s.code = 'imp-1' AND p.code = $2 AND s.status = 'active' AND s.deleted_at IS NULL`, tid, successor); n != 1 {
		t.Fatal("former province not mapped to its successor")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.schools WHERE tenant_id = $1 AND code = 'imp-2' AND status = 'pending_review'
		AND province_id IS NULL AND deleted_at IS NULL`, tid); n != 1 {
		t.Fatal("school without province not pending_review")
	}
	authztest.ExpectDenied(t, func() error {
		_, err := h.Handle(authztest.Ctx(ctx, authztest.Staff(tid, "tenant_admin")), tid, rows, true)
		return err
	}())
}
