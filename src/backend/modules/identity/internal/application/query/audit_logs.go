package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// AuditLogs implements listTenantAuditLogs and listPlatformAuditLogs (F015 US2).
type AuditLogs struct {
	Model    read.AuditReadModel
	Cursors  *page.Codec
	MaxRange time.Duration // audit_query_max_range [Proposed] 93 days
}

// AuditLogsInput are the filters of both operations.
type AuditLogsInput struct {
	SchoolID       *uuid.UUID
	ActorID        *uuid.UUID
	Action         string
	ActionPrefixes []string // action_prefix (F059-3)
	ResourceType   string
	ResourceID     string
	From, To       time.Time
	Limit          int
	Cursor         string
}

// AuditPage is AuditLogPage.
type AuditPage struct {
	Data []dto.AuditLogEntry `json:"data"`
	Page page.Info           `json:"page"`
}

func (q AuditLogs) filter(in AuditLogsInput) (read.AuditFilter, error) {
	if err := audit.CheckActionPrefixes(in.ActionPrefixes); err != nil {
		return read.AuditFilter{}, problem.Invalid("action_prefix", problem.SubInvalid, "at most 10 dotted lower-case prefixes of 2 to 64 characters")
	}
	if err := audit.CheckRange(in.From, in.To, q.MaxRange); err != nil {
		return read.AuditFilter{}, problem.Invalid("from", problem.SubInvalid, "from and to are required, to after from, at most the maximum range")
	}
	c, err := q.Cursors.Decode(in.Cursor)
	if err != nil {
		return read.AuditFilter{}, problem.New(problem.BadRequest, "invalid cursor")
	}
	return read.AuditFilter{SchoolID: in.SchoolID, ActorID: in.ActorID, Action: in.Action, ActionPrefixes: in.ActionPrefixes,
		TargetType: in.ResourceType, TargetID: in.ResourceID, From: in.From, To: in.To, After: c, Limit: page.Limit(in.Limit)}, nil
}

func (q AuditLogs) page(ctx context.Context, f read.AuditFilter) (AuditPage, error) {
	rows, err := q.Model.AuditLogs(ctx, f)
	if err != nil {
		return AuditPage{}, err
	}
	out := AuditPage{Data: rows, Page: page.Info{Limit: f.Limit}}
	if len(rows) > f.Limit {
		out.Data = rows[:f.Limit]
		last := out.Data[f.Limit-1]
		next := q.Cursors.Encode(page.Cursor{Key: last.OccurredAt.UTC().Format(time.RFC3339Nano), ID: last.AuditID})
		out.Page.HasMore, out.Page.NextCursor = true, &next
	}
	if out.Data == nil {
		out.Data = []dto.AuditLogEntry{}
	}
	return out, nil
}

// schoolsOf returns the schools a principal holds perm at; all is true for a tenant-wide grant.
func schoolsOf(p authz.Principal, perm authz.Permission) (all bool, schools []uuid.UUID) {
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, perm) {
			continue
		}
		if g.SchoolID == nil {
			return true, nil
		}
		schools = append(schools, *g.SchoolID)
	}
	return false, schools
}

// ListTenant is listTenantAuditLogs (tenant.audit.read): the session tenant only; a School Admin sees the
// entries of the schools of their grants; the moderation journal is never included (F015-AC-07, AC-09).
func (q AuditLogs) ListTenant(ctx context.Context, in AuditLogsInput) (out AuditPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantAuditLogs")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID == nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if in.SchoolID != nil || !p.Has(authz.PermTenantAuditRead) {
		// A school filter must be covered by a grant; without one, school-limited grants are applied below.
		if err := authz.Require(ctx, p, authz.PermTenantAuditRead, authz.Resource{TenantID: p.TenantID, SchoolID: in.SchoolID}); err != nil {
			return out, err
		}
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	f, err := q.filter(in)
	if err != nil {
		return out, err
	}
	f.TenantIDs, f.NoModeration = []uuid.UUID{*p.TenantID}, true
	if all, schools := schoolsOf(p, authz.PermTenantAuditRead); !all {
		f.SchoolIDs = schools
		if f.SchoolIDs == nil {
			f.SchoolIDs = []uuid.UUID{}
		}
	}
	return q.page(tenancy.WithScope(ctx, sc), f)
}

// ListPlatform is listPlatformAuditLogs (platform.audit.read, platform staff): with tenant_ids the
// CrossTenantScope set by httpx (audited cross_tenant.accessed), otherwise the platform entries only.
func (q AuditLogs) ListPlatform(ctx context.Context, in AuditLogsInput) (out AuditPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListPlatformAuditLogs")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID != nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformAuditRead, authz.Resource{}); err != nil {
		return out, err
	}
	f, err := q.filter(in)
	if err != nil {
		return out, err
	}
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if cs, ok := s.(tenancy.CrossTenantScope); ok {
			f.TenantIDs = cs.TenantIDs()
		}
	}
	if len(f.TenantIDs) == 0 {
		f.PlatformOnly = true
		sc, err := authz.SystemPlatformScope("platform-audit-read")
		if err != nil {
			return out, err
		}
		if _, ok := tenancy.ScopeFrom(ctx); !ok {
			ctx = tenancy.WithScope(ctx, sc)
		}
	}
	return q.page(ctx, f)
}
