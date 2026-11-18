package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Sis holds the SIS read use cases (F011).
type Sis struct {
	Issues  read.SisIssueReadModel
	Cursors *page.Codec
}

// SisIssuesInput is the query of listSisReconciliationIssues.
type SisIssuesInput struct {
	ListInput
	SchoolID, ConnectionID *uuid.UUID
	Type, Status           string
}

// SisIssuePage is SisReconciliationIssuePage.
type SisIssuePage struct {
	Data []dto.SisIssue `json:"data"`
	Page page.Info      `json:"page"`
}

var issueStatuses = map[string]bool{"": true, "open": true, "resolved": true, "ignored": true}

// SchoolsWith returns the schools where the principal holds perm; all = a tenant-wide grant.
func SchoolsWith(p authz.Principal, perm authz.Permission) (schools []uuid.UUID, all bool) {
	for _, g := range p.Grants {
		for _, gp := range g.Permissions {
			if gp != perm {
				continue
			}
			if g.SchoolID == nil {
				return nil, true
			}
			schools = append(schools, *g.SchoolID)
		}
	}
	return schools, false
}

// ListSisIssues is listSisReconciliationIssues: tenant staff with tenant.sis.read (or tenant.sis.import,
// the School Admin running imports) see the issues of their schools; platform staff with
// platform.sis_connection.manage read the scope set from tenant_ids (CrossTenantScope, audited).
// The account holding a duplicated identifier is shown to platform staff only.
func (q Sis) ListSisIssues(ctx context.Context, in SisIssuesInput) (out SisIssuePage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListSisReconciliationIssues")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if !issueStatuses[in.Status] {
		return out, problem.Invalid("status", problem.SubInvalid, "must be open, resolved or ignored")
	}
	f := read.SisIssueFilter{SchoolID: in.SchoolID, ConnectionID: in.ConnectionID, Type: in.Type, Status: in.Status}
	platform := p.TenantID == nil
	if platform {
		if err := authz.Require(ctx, p, authz.PermPlatformSisConnectionManage, authz.Resource{}); err != nil {
			return out, err
		}
		if _, ok := tenancy.ScopeFrom(ctx); !ok {
			return out, problem.New(problem.PermissionDenied, "")
		}
	} else {
		perm := authz.PermTenantSisRead
		if !p.Has(perm) {
			perm = authz.PermTenantSisImport
		}
		schools, all := SchoolsWith(p, perm)
		school := in.SchoolID
		if school == nil && !all && len(schools) > 0 {
			school = &schools[0] // school-scoped grant: authorize on a granted school, filter on all of them
		}
		if ctx, _, err = tenantRead(ctx, perm, school); err != nil {
			return out, err
		}
		if !all {
			f.SchoolIDs = append([]uuid.UUID{}, schools...)
		}
	}
	if f.After, err = q.cursor(in.Cursor); err != nil {
		return out, err
	}
	f.Limit = page.Limit(in.Limit)
	rows, err := q.Issues.Issues(ctx, f)
	if err != nil {
		return out, err
	}
	if !platform {
		for i := range rows {
			rows[i].HideConflictingUser()
		}
	}
	pg := pageOf(q.Cursors, rows, f.Limit, func(i dto.SisIssue) (string, uuid.UUID) { return "", i.IssueID })
	return SisIssuePage{Data: pg.Items, Page: pg.Page}, nil
}

func (q Sis) cursor(s string) (*page.Cursor, error) { return Org{Cursors: q.Cursors}.cursor(s) }

// SisImports holds getSisImport.
type SisImports struct{ Model read.SisImportReadModel }

// maxImportErrors is the cap of errors returned with an import (spec maxItems).
const maxImportErrors = 1000

// Get is getSisImport (F011-AC-07): tenant.sis.import on the school of the path; an import of another
// school or tenant is 404. Returns the diff and up to 1000 contract violations by row.
func (q SisImports) Get(ctx context.Context, schoolID, importID uuid.UUID) (out dto.SisImport, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetSisImport")
	defer func() { done(err) }()
	if ctx, _, err = tenantRead(ctx, authz.PermTenantSisImport, &schoolID); err != nil {
		return out, err
	}
	if out, err = q.Model.Import(ctx, importID); err != nil {
		return out, err
	}
	if out.SchoolID != schoolID {
		return dto.SisImport{}, problem.New(problem.NotFound, "")
	}
	if out.SyncRunID != nil {
		if out.Errors, err = q.Model.RunErrors(ctx, *out.SyncRunID, maxImportErrors); err != nil {
			return dto.SisImport{}, err
		}
	}
	return out, nil
}
