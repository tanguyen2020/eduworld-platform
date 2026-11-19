package query_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// fakeRoster is a TEST DOUBLE of the class roster read model (only ClassStudents is used).
type fakeRoster struct {
	read.OrgReadModel
	rows  []dto.ClassStudent
	calls int
}

func (f *fakeRoster) ClassStudents(context.Context, read.ClassStudentFilter) ([]dto.ClassStudent, error) {
	f.calls++
	return append([]dto.ClassStudent(nil), f.rows...), nil
}

// fakeIssues is a TEST DOUBLE of the issue read model.
type fakeIssues struct {
	visible bool
	got     read.SisIssueFilter
	rows    []dto.SisIssue
}

func (f *fakeIssues) Issues(_ context.Context, fl read.SisIssueFilter) ([]dto.SisIssue, error) {
	f.got = fl
	return append([]dto.SisIssue(nil), f.rows...), nil
}
func (f *fakeIssues) SchoolVisible(context.Context, uuid.UUID) (bool, error) { return f.visible, nil }

func codeOf(err error) problem.Code {
	var p *problem.Error
	if errors.As(err, &p) {
		return p.Code
	}
	return ""
}

// F052-1: listTenantClassStudents authorizes on the school of the path (no grant → 403, another school → 404)
// and computes days_since_last_login on the server.
func TestListTenantClassStudentsScope(t *testing.T) {
	ctx := context.Background()
	tenant, school, other := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	last := now.Add(-73 * time.Hour)
	m := &fakeRoster{rows: []dto.ClassStudent{{StudentID: uuid.New(), DisplayName: "An", LastLoginAt: &last}, {StudentID: uuid.New(), DisplayName: "Binh"}}}
	codec, _ := page.NewCodec([]byte("0123456789abcdef0123456789abcdef"))
	q := query.Org{Model: m, Cursors: codec, Clock: func() time.Time { return now }}
	in := query.ClassStudentsInput{SchoolID: school, ClassID: uuid.New()}

	out, err := q.ListTenantClassStudents(authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin", authztest.AtSchool(school))), in)
	if err != nil || len(out.Items) != 2 || out.Items[0].DaysSinceLastLogin == nil || *out.Items[0].DaysSinceLastLogin != 3 ||
		out.Items[1].DaysSinceLastLogin != nil {
		t.Fatalf("school admin = %+v %v", out, err)
	}
	if _, err := q.ListTenantClassStudents(authztest.Ctx(ctx, authztest.Staff(tenant, "tenant_admin")), in); err != nil {
		t.Fatalf("tenant admin: %v", err)
	}
	calls := m.calls
	if _, err := q.ListTenantClassStudents(authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin", authztest.AtSchool(other))), in); codeOf(err) != problem.NotFound {
		t.Fatalf("another school: %v (want 404)", err)
	}
	authztest.ExpectDenied(t, func() error {
		_, err := q.ListTenantClassStudents(authztest.Ctx(ctx, authztest.NoRole(tenant)), in)
		return err
	}())
	if m.calls != calls {
		t.Fatal("the roster is read before the authorization")
	}
	in.Q = "a"
	if _, err := q.ListTenantClassStudents(authztest.Ctx(ctx, authztest.Staff(tenant, "tenant_admin")), in); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("q of one character: %v", err)
	}
}

// F052-1: listSchoolReconciliationIssues filters on the school of the path, answers 404 for an unknown school
// and never returns the conflicting account.
func TestListSchoolSisIssuesScope(t *testing.T) {
	ctx := context.Background()
	tenant, school, imp := uuid.New(), uuid.New(), uuid.New()
	m := &fakeIssues{visible: true, rows: []dto.SisIssue{{IssueID: uuid.New(), IssueType: "duplicate_identifier", EntityRefs: map[string]string{"user_id": "x", "field": "username"}}}}
	codec, _ := page.NewCodec([]byte("0123456789abcdef0123456789abcdef"))
	q := query.Sis{Issues: m, Cursors: codec}
	in := query.SchoolSisIssuesInput{SchoolID: school, ImportID: &imp, Status: "open"}
	out, err := q.ListSchoolSisIssues(authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin", authztest.AtSchool(school))), in)
	if err != nil || len(out.Data) != 1 || m.got.SchoolID == nil || *m.got.SchoolID != school || m.got.ImportID != &imp {
		t.Fatalf("list = %+v %+v %v", out, m.got, err)
	}
	if _, ok := out.Data[0].EntityRefs["user_id"]; ok {
		t.Fatal("conflicting account returned")
	}
	if _, err := q.ListSchoolSisIssues(authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin", authztest.AtSchool(uuid.New()))), in); codeOf(err) != problem.NotFound {
		t.Fatalf("another school: %v", err)
	}
	authztest.ExpectDenied(t, func() error {
		_, err := q.ListSchoolSisIssues(authztest.Ctx(ctx, authztest.NoRole(tenant)), in)
		return err
	}())
	m.visible = false
	if _, err := q.ListSchoolSisIssues(authztest.Ctx(ctx, authztest.Staff(tenant, "tenant_admin")), in); codeOf(err) != problem.NotFound {
		t.Fatalf("unknown school: %v", err)
	}
	m.visible = true
	in.Status = "closed"
	if _, err := q.ListSchoolSisIssues(authztest.Ctx(ctx, authztest.Staff(tenant, "tenant_admin")), in); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("bad status: %v", err)
	}
}
