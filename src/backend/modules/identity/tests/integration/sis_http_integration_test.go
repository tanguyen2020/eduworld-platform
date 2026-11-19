//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F011-AC-26, AC-27 and the F011 routes through the httpx chain (auth, X-Tenant-Id, Idempotency-Key,
// OpenAPI validation): connections, schools, imports, issues, runs; no secret_ref in any response.
func TestSisHTTP(t *testing.T) {
	ctx := context.Background()
	h := newHTTPEnv(ctx, t, newKCUsers())
	now := time.Now()
	a := seedTenant(ctx, t, h.platform, "pgd-http-a")
	b := seedTenant(ctx, t, h.platform, "pgd-http-b")
	ta, tb := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)

	saData := authztest.Platform("system_admin").PrincipalData
	h.res.Set("kc-sa", saData)
	sa := h.idp.Token("kc-sa", httpxtest.PortalClient, true, now)
	adm := authztest.Staff(ta, "school_admin").PrincipalData
	h.res.Set("kc-adm", adm)
	admTok := h.idp.Token("kc-adm", httpxtest.PortalClient, true, now)
	admB := authztest.Staff(tb, "school_admin").PrincipalData
	h.res.Set("kc-adm-b", admB)
	admBTok := h.idp.Token("kc-adm-b", httpxtest.PortalClient, true, now)
	key := func(tenant uuid.UUID) map[string]string {
		m := map[string]string{"Idempotency-Key": ids.New().String()}
		if tenant != uuid.Nil {
			m["X-Tenant-Id"] = tenant.String()
		}
		return m
	}
	noSecret := func(r httpResp) {
		t.Helper()
		if strings.Contains(r.raw, "secret") || strings.Contains(r.raw, "config") {
			t.Fatalf("credential field in response: %s", r.raw)
		}
	}

	// AC-26: createSisConnection (platform permission, X-Tenant-Id, Idempotency-Key).
	if r := h.do(t, "POST", "/sis/connections", sa, map[string]string{"X-Tenant-Id": a.tenant}, map[string]any{"provider_kind": "csv_import"}); r.status != 428 {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/sis/connections", admTok, key(ta), map[string]any{"provider_kind": "csv_import"}); r.status != 403 {
		t.Fatalf("school admin: %d %s", r.status, r.raw)
	}
	r := h.do(t, "POST", "/sis/connections", sa, key(ta), map[string]any{"provider_kind": "csv_import", "reason": "pilot"})
	if r.status != 201 || r.body["tenant_id"] != a.tenant {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	noSecret(r)
	conn := r.body["sis_connection_id"].(string)
	r2 := h.do(t, "POST", "/sis/connections", sa, key(ta), map[string]any{"provider_kind": "csv_import"})
	conn2 := r2.body["sis_connection_id"].(string)

	// setSisConnectionSchools / listSisConnectionSchools; a school of another active connection → 409.
	if r := h.do(t, "PUT", "/admin/sis/connections/"+conn+"/schools", sa, key(uuid.Nil), map[string]any{"school_ids": []string{a.school}, "reason": "pilot"}); r.status != 200 {
		t.Fatalf("set schools: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "PUT", "/admin/sis/connections/"+conn2+"/schools", sa, key(uuid.Nil), map[string]any{"school_ids": []string{a.school}, "reason": "x"}); r.status != 409 ||
		!strings.Contains(r.raw, "SIS_SCHOOL_ALREADY_CONNECTED") {
		t.Fatalf("already connected: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/admin/sis/connections/"+conn+"/schools", sa, nil, nil); r.status != 200 || !strings.Contains(r.raw, a.school) {
		t.Fatalf("list schools: %d %s", r.status, r.raw)
	}
	// listSisConnections over tenant_ids (System Admin cross-tenant read).
	r = h.do(t, "GET", "/sis/connections?tenant_ids="+a.tenant, sa, nil, nil)
	if r.status != 200 || !strings.Contains(r.raw, conn) || !strings.Contains(r.raw, `"school_count":1`) {
		t.Fatalf("list connections: %d %s", r.status, r.raw)
	}
	noSecret(r)
	if r := h.do(t, "GET", "/sis/connections?tenant_ids="+b.tenant, sa, nil, nil); r.status != 200 || strings.Contains(r.raw, conn) {
		t.Fatalf("other tenant connections: %d %s", r.status, r.raw)
	}

	// createSisImport (School Admin) → 202 with a presigned URL; getSisImport; another tenant → 404.
	imp := h.do(t, "POST", "/schools/"+a.school+"/sis-imports", admTok, key(ta),
		map[string]any{"file_name": "students.csv", "size_bytes": 120, "entity_kinds": []string{"students"}})
	if imp.status != 202 || imp.body["upload_url"] == nil || imp.body["status"] != "awaiting_upload" || imp.body["dry_run"] != true {
		t.Fatalf("create import: %d %s", imp.status, imp.raw)
	}
	importID := imp.body["import_id"].(string)
	if r := h.do(t, "POST", "/schools/"+a.school+"/sis-imports", admTok, key(ta),
		map[string]any{"file_name": "students.csv", "size_bytes": 20 << 20, "entity_kinds": []string{"students"}}); r.status != 422 {
		t.Fatalf("too big: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/schools/"+a.school+"/sis-imports/"+importID, admTok, map[string]string{"X-Tenant-Id": a.tenant}, nil); r.status != 200 {
		t.Fatalf("get import: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/schools/"+b.school+"/sis-imports/"+importID, admBTok, map[string]string{"X-Tenant-Id": b.tenant}, nil); r.status != 404 {
		t.Fatalf("other tenant import: %d %s", r.status, r.raw)
	}

	// A run (direct use case), then the run reads and the issues of the school.
	run := command.SisRun{UoW: h.uow, Savepoints: h.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Rules: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{}, Students: write.StudentRepo{},
		Events: h.deps.Events, Audit: h.deps.Audit}
	res, err := run.Handle(authztest.Ctx(ctx, authztest.Staff(ta, "school_admin")), command.RunImportInput{ConnectionID: uuid.MustParse(conn),
		SchoolID: uuid.MustParse(a.school), Files: sisFiles(t, map[string]string{"students.csv": "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n" +
			"HS-1,TH-A,001,An,3,active,hs.http.an\nHS-2,TH-A,002,,x,active,hs.http.b\n"})})
	if err != nil {
		t.Fatal(err)
	}
	runID := res.RunID.String()
	if r := h.do(t, "GET", "/sis/connections/"+conn+"/sync-runs", sa, nil, nil); r.status != 200 || !strings.Contains(r.raw, runID) {
		t.Fatalf("runs: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/sis/runs/"+runID, sa, nil, nil); r.status != 200 || r.body["kind"] != "csv_import" || r.body["status"] != "failed" {
		t.Fatalf("run: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/sis/runs/"+runID+"/errors?limit=1", sa, nil, nil); r.status != 200 || !strings.Contains(r.raw, "SIS_CONTRACT_VIOLATION") ||
		r.body["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("run errors: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/sis/runs/"+runID, admTok, map[string]string{"X-Tenant-Id": a.tenant}, nil); r.status != 403 {
		t.Fatalf("tenant staff on runs: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "GET", "/sis/reconciliation-issues?status=open", admTok, map[string]string{"X-Tenant-Id": a.tenant}, nil); r.status != 200 {
		t.Fatalf("issues: %d %s", r.status, r.raw)
	}
}
