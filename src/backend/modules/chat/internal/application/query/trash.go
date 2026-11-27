package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// TrashInput is the query of facade listTrash (module identity merges the owners, F045/F057).
type TrashInput struct {
	TenantIDs    []uuid.UUID
	ResourceType string // empty = sticker_pack, the only trash type of module chat
	Cursor       string
	Limit        int
}

// TrashEntry is TrashItem of the internal contract (platform library: tenant_id is always null).
type TrashEntry struct {
	ResourceType string
	ResourceID   uuid.UUID
	Title        string
	DeletedAt    time.Time
	DeletedBy    *uuid.UUID
	Restorable   bool
}

// TrashPage is a page of the trash, newest deletion first.
type TrashPage struct {
	Data       []TrashEntry
	HasMore    bool
	NextCursor *string
}

// Trash is the use case behind facade listTrash of module chat (ADR-0028, ADMIN_PORTAL §3.8): System Admin
// (platform.trash.read) lists the soft-deleted sticker packs; they are platform content, so a tenant filter (tenant_ids)
// or a tenant user (tenant.trash.read, session tenant) gets an empty page. Deleted rows are read under
// tenancy.Bypass(trash) (audit cross_tenant.accessed, eduworld_tenant_filter_bypass_total), keyset (deleted_at, pack_id)
// DESC in SQL. Never the stickers or a message: the pack name only.
type Trash struct {
	Read    ReadSession
	Reader  read.TrashReader
	Cursors *page.Codec
}

// List runs the query.
func (q Trash) List(ctx context.Context, in TrashInput) (out TrashPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListTrash")
	defer func() { done(err) }()
	out.Data = []TrashEntry{}
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID != nil {
		// A tenant trash holds the data of its tenant only: sticker packs are platform content.
		return out, authz.Require(ctx, p, authz.PermTenantTrashRead, authz.Resource{})
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTrashRead, authz.Resource{}); err != nil {
		return out, err
	}
	if !p.IsPlatform() {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if in.ResourceType != "" && in.ResourceType != chatasset.ResourceStickerPack {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "is not a resource type of module chat")
	}
	n := page.Limit(in.Limit)
	f := read.TrashFilter{Limit: n + 1}
	after, err := q.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	if after != nil {
		at, err := time.Parse(time.RFC3339Nano, after.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		f.AfterAt, f.AfterID = &at, after.ID
	}
	if len(in.TenantIDs) > 0 {
		return out, nil // tenant_ids selects tenant data only; platform content is listed without a tenant filter
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "chat"), tenancy.ReasonTrash, tenancy.WithMode(tenancy.ModeRead))
	if err != nil {
		return out, err
	}
	var rows []read.TrashRow
	if err := q.Read.Read(bctx, func(ctx context.Context) error {
		var err error
		rows, err = q.Reader.DeletedPacks(ctx, f)
		return err
	}); err != nil {
		return out, err
	}
	if len(rows) > n {
		rows = rows[:n]
		last := rows[n-1]
		c := q.Cursors.Encode(page.Cursor{Key: last.DeletedAt.UTC().Format(time.RFC3339Nano), ID: last.PackID})
		out.HasMore, out.NextCursor = true, &c
	}
	for _, r := range rows {
		out.Data = append(out.Data, TrashEntry{ResourceType: chatasset.ResourceStickerPack, ResourceID: r.PackID, Title: r.Title,
			DeletedAt: r.DeletedAt.UTC(), DeletedBy: r.DeletedBy, Restorable: !r.Conflict})
	}
	return out, nil
}
