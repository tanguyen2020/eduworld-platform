package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// TrashInput is the query of facade listTrash (module identity merges the owners, F045/F057).
type TrashInput struct {
	TenantIDs    []uuid.UUID
	ResourceType string // empty = every trash type of module competition
	Cursor       string
	Limit        int
}

// TrashEntry is TrashItem of the internal contract (platform configuration: tenant_id is always null).
type TrashEntry struct {
	ResourceType string
	ResourceID   uuid.UUID
	Title        string
	DeletedAt    time.Time
	DeletedBy    *uuid.UUID
	DeleteReason string
	Restorable   bool
}

// TrashPage is a page of the trash, newest deletion first.
type TrashPage struct {
	Data       []TrashEntry
	HasMore    bool
	NextCursor *string
}

// Trash is the use case behind facade listTrash of module competition (ADR-0028, ADMIN_PORTAL §3.8): System Admin
// (platform.trash.read) lists the soft-deleted tournament and challenge templates; they are platform configurations, so a
// tenant filter (tenant_ids) or a tenant user (tenant.trash.read, session tenant) gets an empty page. Deleted rows are
// read under tenancy.Bypass(trash) (audit cross_tenant.accessed, eduworld_tenant_filter_bypass_total), keyset
// (deleted_at, config_id) DESC in SQL.
type Trash struct {
	Read   read.ReadSession
	Reader read.TrashReader
}

// List runs the query.
func (q Trash) List(ctx context.Context, in TrashInput) (out TrashPage, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListTrash")
	defer func() { done(err) }()
	out.Data = []TrashEntry{}
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID != nil {
		// A tenant trash holds the data of its tenant only: the templates are platform configurations.
		return out, authz.Require(ctx, p, authz.PermTenantTrashRead, authz.Resource{})
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTrashRead, authz.Resource{}); err != nil {
		return out, err
	}
	if !p.IsPlatform() {
		return out, problem.New(problem.PermissionDenied, "")
	}
	types := challenge.TrashTypes()
	if in.ResourceType != "" {
		if !slices.Contains(types, in.ResourceType) {
			return out, problem.Invalid("resource_type", problem.SubInvalid, "is not a resource type of module competition")
		}
		types = []string{in.ResourceType}
	}
	configTypes := make([]string, 0, len(types))
	for _, t := range types {
		configTypes = append(configTypes, challenge.ConfigType(t))
	}
	after, err := decodeKeyset(in.Cursor)
	if err != nil {
		return out, err
	}
	if len(in.TenantIDs) > 0 {
		return out, nil // tenant_ids selects tenant data only; platform configurations are listed without a tenant filter
	}
	n := page.Limit(in.Limit)
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "competition"), tenancy.ReasonTrash, tenancy.WithMode(tenancy.ModeRead))
	if err != nil {
		return out, err
	}
	var rows []read.TrashRow
	if err := q.Read.Read(bctx, func(ctx context.Context) error {
		var err error
		rows, err = q.Reader.DeletedTemplates(ctx, read.TrashFilter{ConfigTypes: configTypes, After: after, Limit: n + 1})
		return err
	}); err != nil {
		return out, err
	}
	if len(rows) > n {
		rows = rows[:n]
		last := rows[n-1]
		c := encodeKeyset(read.Keyset{At: last.DeletedAt, ID: last.ConfigID})
		out.HasMore, out.NextCursor = true, &c
	}
	for _, r := range rows {
		out.Data = append(out.Data, TrashEntry{ResourceType: challenge.ResourceType(r.ConfigType), ResourceID: r.ConfigID, Title: r.Title,
			DeletedAt: r.DeletedAt.UTC(), DeletedBy: r.DeletedBy, DeleteReason: r.DeleteReason, Restorable: !r.Conflict})
	}
	return out, nil
}
