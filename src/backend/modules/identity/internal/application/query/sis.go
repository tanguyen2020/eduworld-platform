package query

import (
	"context"

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

// SisAdmin holds the platform reads of SIS connections and runs (gateway nội bộ, platform.sis_connection.manage).
type SisAdmin struct {
	Model   read.SisAdminReadModel
	Cursors *page.Codec
}

// SisConnectionPage is SisConnectionPage of the spec.
type SisConnectionPage struct {
	Data []dto.SisConnection `json:"data"`
	Page page.Info           `json:"page"`
}

// SyncRunPage is SyncRunPage of the spec.
type SyncRunPage struct {
	Data []dto.SyncRun `json:"data"`
	Page page.Info     `json:"page"`
}

// SyncRecordErrorPage is SyncRecordErrorPage of the spec.
type SyncRecordErrorPage struct {
	Data []dto.SisRecordError `json:"data"`
	Page page.Info            `json:"page"`
}

func (q SisAdmin) platform(ctx context.Context) (authz.Principal, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return p, err
	}
	return p, authz.Require(ctx, p, authz.PermPlatformSisConnectionManage, authz.Resource{})
}

// scoped reads the tenant of a resource under the audited System Admin bypass, then scopes the read to it
// (X-Tenant-Id resolved principal: its own tenant only).
func (q SisAdmin) scoped(ctx context.Context, p authz.Principal, tenantOf func(context.Context) (uuid.UUID, error)) (context.Context, error) {
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return ctx, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	t, err := tenantOf(bctx)
	if err != nil {
		return ctx, err
	}
	sc, err := appscope.TargetTenant(p, t)
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(ctx, sc), nil
}

func idKey[T any](id func(T) uuid.UUID) func(T) (string, uuid.UUID) {
	return func(v T) (string, uuid.UUID) { return "", id(v) }
}

// ListConnections is listSisConnections over the scope set from tenant_ids (CrossTenantScope, audited).
func (q SisAdmin) ListConnections(ctx context.Context, kind, status string, in ListInput) (out SisConnectionPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListSisConnections")
	defer func() { done(err) }()
	if _, err = q.platform(ctx); err != nil {
		return out, err
	}
	if _, ok := tenancy.ScopeFrom(ctx); !ok {
		return out, problem.New(problem.PermissionDenied, "")
	}
	after, err := Org{Cursors: q.Cursors}.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	rows, err := q.Model.Connections(ctx, kind, status, after, limit)
	if err != nil {
		return out, err
	}
	pg := pageOf(q.Cursors, rows, limit, idKey(func(c dto.SisConnection) uuid.UUID { return c.ConnectionID }))
	return SisConnectionPage{Data: pg.Items, Page: pg.Page}, nil
}

// ListRuns is listSisSyncRuns.
func (q SisAdmin) ListRuns(ctx context.Context, connectionID uuid.UUID, status string, in ListInput) (out SyncRunPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListSisSyncRuns")
	defer func() { done(err) }()
	p, err := q.platform(ctx)
	if err != nil {
		return out, err
	}
	if ctx, err = q.scoped(ctx, p, func(c context.Context) (uuid.UUID, error) { return q.Model.ConnectionTenant(c, connectionID) }); err != nil {
		return out, err
	}
	after, err := Org{Cursors: q.Cursors}.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	rows, err := q.Model.Runs(ctx, connectionID, status, after, limit)
	if err != nil {
		return out, err
	}
	pg := pageOf(q.Cursors, rows, limit, idKey(func(r dto.SyncRun) uuid.UUID { return r.SyncRunID }))
	return SyncRunPage{Data: pg.Items, Page: pg.Page}, nil
}

// GetRun is getSisSyncRun.
func (q SisAdmin) GetRun(ctx context.Context, runID uuid.UUID) (out dto.SyncRun, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetSisSyncRun")
	defer func() { done(err) }()
	p, err := q.platform(ctx)
	if err != nil {
		return out, err
	}
	if ctx, err = q.scoped(ctx, p, func(c context.Context) (uuid.UUID, error) { return q.Model.RunTenant(c, runID) }); err != nil {
		return out, err
	}
	return q.Model.Run(ctx, runID)
}

// ListRunErrors is listSisSyncRunErrors.
func (q SisAdmin) ListRunErrors(ctx context.Context, runID uuid.UUID, in ListInput) (out SyncRecordErrorPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListSisSyncRunErrors")
	defer func() { done(err) }()
	p, err := q.platform(ctx)
	if err != nil {
		return out, err
	}
	if ctx, err = q.scoped(ctx, p, func(c context.Context) (uuid.UUID, error) { return q.Model.RunTenant(c, runID) }); err != nil {
		return out, err
	}
	if _, err = q.Model.Run(ctx, runID); err != nil { // the run must be visible in the scope
		return out, err
	}
	after, err := Org{Cursors: q.Cursors}.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	rows, err := q.Model.RunErrorPage(ctx, runID, after, limit)
	if err != nil {
		return out, err
	}
	pg := pageOf(q.Cursors, rows, limit, idKey(func(e dto.SisRecordError) uuid.UUID { return e.ID }))
	return SyncRecordErrorPage{Data: pg.Items, Page: pg.Page}, nil
}
