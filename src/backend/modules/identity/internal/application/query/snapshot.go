package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SnapshotInput is listProjectionSnapshot: entity type, optional tenant (absent = platform scope for a
// rebuild job of every tenant), cursor and limit (≤ 1000).
type SnapshotInput struct {
	EntityType string
	TenantID   *uuid.UUID
	Cursor     string
	Limit      int
}

// SnapshotPage is a page of items with the paging info.
type SnapshotPage struct {
	Items      []read.SnapshotItem
	NextCursor *string
	HasMore    bool
	Limit      int
}

// MaxSnapshotLimit is the largest page of listProjectionSnapshot (F010-AC-27).
const MaxSnapshotLimit = 1000

// ProjectionSnapshot implements listProjectionSnapshot (facade, read context): the current state of the
// projection sources with the event payload fields, source_version and a deleted flag (F010-AC-27).
type ProjectionSnapshot struct {
	Model   read.SnapshotReader
	Cursors *page.Codec
}

// Handle runs the query. Callers are modules in x-allowed-callers (checked by pkg/modcall and arch-check).
func (h ProjectionSnapshot) Handle(ctx context.Context, in SnapshotInput) (out SnapshotPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListProjectionSnapshot")
	defer func() { done(err) }()
	if !read.SnapshotEntityTypes[in.EntityType] {
		return out, problem.Invalid("entity_type", problem.SubInvalid, "tenant, school or student")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = page.DefaultLimit
	}
	if limit > MaxSnapshotLimit {
		return out, problem.Invalid("limit", problem.SubInvalid, "at most 1000")
	}
	after, err := h.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	var afterID *uuid.UUID
	if after != nil {
		afterID = &after.ID
	}
	if in.TenantID != nil {
		sc, err := authz.EventScope(*in.TenantID)
		if err != nil {
			return out, problem.Invalid("tenant_id", problem.SubInvalid, "invalid tenant").Wrap(err)
		}
		ctx = tenancy.WithScope(ctx, sc)
	} else if ctx, err = tenancy.Bypass(ctx, tenancy.ReasonPlatformJob); err != nil {
		return out, err
	}
	items, err := h.Model.Snapshot(ctx, in.EntityType, in.TenantID, afterID, limit+1)
	if err != nil {
		return out, err
	}
	out.Limit, out.Items = limit, items
	if len(items) > limit {
		out.Items = items[:limit]
		next := h.Cursors.Encode(page.Cursor{ID: out.Items[limit-1].EntityID})
		out.NextCursor, out.HasMore = &next, true
	}
	if out.Items == nil {
		out.Items = []read.SnapshotItem{}
	}
	return out, nil
}
