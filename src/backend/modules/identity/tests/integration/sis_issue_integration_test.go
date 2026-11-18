//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F011-AC-11, AC-17: list and resolve reconciliation issues with school scope, tenant isolation and audit.
func TestSisIssues(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-iss-a")
	b := seedTenant(ctx, t, e.platform, "pgd-iss-b")
	ta, school := uuid.MustParse(a.tenant), uuid.MustParse(a.school)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	conn, err := conns.Create(sa, ta, "csv_import", "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, conn.ID, []uuid.UUID{school}, "pilot"); err != nil {
		t.Fatal(err)
	}
	dup := enrolled(ctx, t, e.platform, a, "hs.dup") // the same child, created by hand before the SIS import
	run := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))
	files := map[string]string{
		"classes.csv": "class_external_id,school_external_id,academic_year,grade_level,name\nC-3B,TH-A,2026-2027,3,3B\n",
		"students.csv": "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n" +
			"HS-1,TH-A,001,An,3,active,hs.i.an\nHS-4,TH-A,004,Dung,3,active,hs.dup\n",
		"enrollments.csv": "student_external_id,class_external_id,start_date,status\nHS-1,C-3B,2026-09-05,active\nHS-9,C-3B,2026-09-05,active\n",
	}
	res, err := run.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: sisFiles(t, files)})
	if err != nil || res.Issues != 2 {
		t.Fatalf("import = %+v %v", res, err)
	}

	codec, err := page.NewCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	q := query.Sis{Issues: read.SisIssueModel{Session: e.read}, Cursors: codec}
	open, err := q.ListSisIssues(admin, query.SisIssuesInput{Status: "open"})
	if err != nil || len(open.Data) != 2 {
		t.Fatalf("list = %+v %v", open, err)
	}
	var dupIssue, orphan uuid.UUID
	for _, i := range open.Data {
		switch i.IssueType {
		case "duplicate_identifier":
			dupIssue = i.IssueID
			if _, ok := i.EntityRefs["user_id"]; ok {
				t.Fatal("the conflicting account is shown to tenant staff")
			}
		case "orphan_enrollment":
			orphan = i.IssueID
		}
	}
	// Paging: one per page, newest first.
	p1, err := q.ListSisIssues(admin, query.SisIssuesInput{ListInput: query.ListInput{Limit: 1}})
	if err != nil || len(p1.Data) != 1 || !p1.Page.HasMore {
		t.Fatalf("page 1 = %+v %v", p1, err)
	}
	p2, err := q.ListSisIssues(admin, query.SisIssuesInput{ListInput: query.ListInput{Limit: 1, Cursor: *p1.Page.NextCursor}})
	if err != nil || len(p2.Data) != 1 || p2.Data[0].IssueID == p1.Data[0].IssueID {
		t.Fatalf("page 2 = %+v %v", p2, err)
	}
	// A School Admin of another school sees nothing; the System Admin sees the conflicting account.
	other := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin", authztest.AtSchool(uuid.MustParse(a.otherSchool))))
	if got, err := q.ListSisIssues(other, query.SisIssuesInput{}); err != nil || len(got.Data) != 0 {
		t.Fatalf("other school = %+v %v", got, err)
	}
	sysP := authztest.Platform("system_admin")
	cross, err := authz.CrossTenantScope(sysP, []uuid.UUID{ta}, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		t.Fatal(err)
	}
	all, err := q.ListSisIssues(tenancy.WithScope(authz.WithPrincipal(ctx, sysP), cross), query.SisIssuesInput{Type: "duplicate_identifier"})
	if err != nil || len(all.Data) != 1 || all.Data[0].EntityRefs["user_id"] != dup.String() {
		t.Fatalf("system admin = %+v %v", all, err)
	}
	if got, err := q.ListSisIssues(authztest.Ctx(ctx, authztest.Staff(uuid.MustParse(b.tenant), "tenant_admin")), query.SisIssuesInput{}); err != nil || len(got.Data) != 0 {
		t.Fatalf("other tenant = %+v %v", got, err)
	}

	// AC-17: outside the school scope or the tenant → 404.
	h := command.ResolveSisIssue{UoW: e.uow, Issues: write.SisSyncRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}
	if _, err := h.Handle(other, command.ResolveSisIssueInput{IssueID: orphan, Resolution: "ignore"}); code(err) != problem.NotFound {
		t.Fatalf("other school: %v", err)
	}
	if _, err := h.Handle(authztest.Ctx(ctx, authztest.Staff(uuid.MustParse(b.tenant), "tenant_admin")), command.ResolveSisIssueInput{IssueID: orphan, Resolution: "ignore"}); code(err) != problem.NotFound {
		t.Fatalf("other tenant: %v", err)
	}
	if _, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: orphan, Resolution: "same_person"}); code(err) != problem.ValidationFailed {
		t.Fatalf("same_person on orphan: %v", err)
	}
	ign, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: orphan, Resolution: "ignore", Note: "test row"})
	if err != nil || ign.Status != "ignored" || ign.ResolvedBy == nil || ign.ResolvedAt == nil {
		t.Fatalf("ignore = %+v %v", ign, err)
	}
	if _, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: orphan, Resolution: "fixed_in_source"}); code(err) != problem.Conflict {
		t.Fatalf("resolve twice: %v", err)
	}

	// AC-11: same person → the existing account is linked, no second account; the next import updates it.
	same, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: dupIssue, Resolution: "same_person"})
	if err != nil || same.Status != "resolved" {
		t.Fatalf("same_person = %+v %v", same, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.sis_identity_links WHERE user_id = $1 AND student_external_id = 'HS-4'
		AND status = 'active' AND tenant_id = $2`, dup, a.tenant); n != 1 {
		t.Fatalf("link of the existing account = %d", n)
	}
	again, err := run.Handle(admin, command.RunImportInput{ConnectionID: conn.ID, SchoolID: school, Files: sisFiles(t, files)})
	if err != nil || again.Diff["student"]["update"] != 1 || again.Issues != 1 { // only the orphan enrollment again
		t.Fatalf("re-import = %+v %v", again, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username IN ('hs.dup', 'hs.i.an')`); n != 2 {
		t.Fatalf("accounts = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND full_name = 'Dung' AND tenant_id = $2`, dup, a.tenant); n != 1 {
		t.Fatal("the linked student is not updated by the next import")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'sis.reconciliation_issue_resolved'`); n != 2 {
		t.Fatalf("resolution audits = %d", n)
	}
}
