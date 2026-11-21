package query

import (
	"context"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// schoolScoped authorizes the first of perms whose grants cover the school of the path and sets the session
// TenantScope (school-scoped public routes of the Admin Portal, F052-1):
//   - no grant holds any of perms → 403 PERMISSION_DENIED (audited authz.denied by authz.Require);
//   - the grants holding them are limited to other schools → 404 NOT_FOUND, so a School Admin learns nothing
//     about another school (the denial is still audited as outside_grant_scope).
func schoolScoped(ctx context.Context, school uuid.UUID, perms ...authz.Permission) (context.Context, authz.Principal, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return ctx, p, err
	}
	var held []authz.Permission
	for _, perm := range perms {
		if !p.Has(perm) {
			continue
		}
		held = append(held, perm)
		if schools, all := SchoolsWith(p, perm); all || slices.Contains(schools, school) {
			return tenantRead(ctx, perm, &school)
		}
	}
	if len(held) == 0 {
		return ctx, p, authz.Require(ctx, p, perms[0], authz.Resource{TenantID: p.TenantID})
	}
	_ = authz.Require(ctx, p, held[0], authz.Resource{TenantID: p.TenantID, SchoolID: &school}) // audits the denial
	return ctx, p, problem.New(problem.NotFound, "")
}

// ClassStudentsInput is the query of listTenantClassStudents.
type ClassStudentsInput struct {
	ListInput
	SchoolID, ClassID uuid.UUID
	Q                 string
}

// ListTenantClassStudents is listTenantClassStudents (F052-1, owner F010): the read-only roster of a class
// with the last game login. tenant.class.read (Tenant Admin) or lms.class.read (School Admin, Academic
// Manager: the two permissions of action.school_lms.classes.view) on the school of the path; a school or
// class outside the tenant or the grant scope → 404. Only the ClassStudent fields leave the module (R16).
func (q Org) ListTenantClassStudents(ctx context.Context, in ClassStudentsInput) (out Page[dto.ClassStudent], err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantClassStudents")
	defer func() { done(err) }()
	if ctx, _, err = schoolScoped(ctx, in.SchoolID, authz.PermTenantStudentRead, authz.PermLmsStudentRead); err != nil {
		return out, err
	}
	if n := utf8.RuneCountInString(in.Q); in.Q != "" && (n < 2 || n > 100) {
		return out, problem.Invalid("q", problem.SubInvalid, "must be 2 to 100 characters")
	}
	after, err := q.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	rows, err := q.Model.ClassStudents(ctx, read.ClassStudentFilter{SchoolID: in.SchoolID, ClassID: in.ClassID, Q: in.Q, After: after, Limit: limit})
	if err != nil {
		return out, err
	}
	now := time.Now()
	if q.Clock != nil {
		now = q.Clock()
	}
	for i := range rows {
		rows[i].DaysSinceLastLogin = daysSince(rows[i].LastLoginAt, now)
	}
	return pageOf(q.Cursors, rows, limit, func(s dto.ClassStudent) (string, uuid.UUID) { return s.DisplayName, s.StudentID }), nil
}

// daysSince is days_since_last_login: whole days since the last game login (nil when never logged in).
func daysSince(at *time.Time, now time.Time) *int {
	if at == nil {
		return nil
	}
	d := 0
	if now.After(*at) {
		d = int(now.Sub(*at) / (24 * time.Hour))
	}
	return &d
}

// SchoolSisIssuesInput is the query of listSchoolReconciliationIssues.
type SchoolSisIssuesInput struct {
	ListInput
	SchoolID     uuid.UUID
	ImportID     *uuid.UUID
	Type, Status string
}

// ListSchoolSisIssues is listSchoolReconciliationIssues (F052-1, owner F011): the public, school-scoped
// view of listSisReconciliationIssues for School Admin and Tenant Admin. tenant.sis.read or
// tenant.sis.import on the school of the path; a school outside the tenant or the grant scope → 404. The
// account holding a duplicated identifier is never returned on this route (platform staff only).
func (q Sis) ListSchoolSisIssues(ctx context.Context, in SchoolSisIssuesInput) (out SisIssuePage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListSchoolReconciliationIssues")
	defer func() { done(err) }()
	if ctx, _, err = schoolScoped(ctx, in.SchoolID, authz.PermTenantSisRead, authz.PermTenantSisImport); err != nil {
		return out, err
	}
	if !issueStatuses[in.Status] {
		return out, problem.Invalid("status", problem.SubInvalid, "must be open, resolved or ignored")
	}
	visible, err := q.Issues.SchoolVisible(ctx, in.SchoolID)
	if err != nil {
		return out, err
	}
	if !visible {
		return out, problem.New(problem.NotFound, "")
	}
	school := in.SchoolID
	f := read.SisIssueFilter{SchoolID: &school, ImportID: in.ImportID, Type: in.Type, Status: in.Status}
	if f.After, err = q.cursor(in.Cursor); err != nil {
		return out, err
	}
	f.Limit = page.Limit(in.Limit)
	rows, err := q.Issues.Issues(ctx, f)
	if err != nil {
		return out, err
	}
	for i := range rows {
		rows[i].HideConflictingUser()
	}
	pg := pageOf(q.Cursors, rows, f.Limit, func(i dto.SisIssue) (string, uuid.UUID) { return "", i.IssueID })
	return SisIssuePage{Data: pg.Items, Page: pg.Page}, nil
}
