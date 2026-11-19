package query

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ListTrashInput is the query of facade listTrash.
type ListTrashInput struct {
	TenantIDs    []uuid.UUID
	ResourceType string // empty = every registered type
	Cursor       string
	Limit        int
}

// TrashPage is a page of the trash, newest deletion first.
type TrashPage struct {
	Data       []trash.Entry
	NextCursor *string
	HasMore    bool
}

// ListTrashHandler is the use case behind facade listTrash (F020-AC-24).
type ListTrashHandler interface {
	Handle(ctx context.Context, in ListTrashInput) (TrashPage, error)
}

// ListTrash implements ListTrashHandler: System Admin (platform.trash.read) lists deleted items of the requested
// tenants (none = every tenant and platform items); a tenant user with tenant.trash.read [Proposed] only its
// session tenant (tenant_ids ignored). It reads through tenancy.Bypass(trash): audit cross_tenant.accessed and
// eduworld_tenant_filter_bypass_total{module,reason}.
type ListTrash struct {
	Session   read.Session
	Resources *port.Registry
	Cursors   *page.Codec
}

var _ ListTrashHandler = ListTrash{}

// Handle runs the query.
func (h ListTrash) Handle(ctx context.Context, in ListTrashInput) (out TrashPage, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListTrash")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	f := port.TrashFilter{Limit: limitOf(in.Limit)}
	if p.TenantID != nil {
		if err := authz.Require(ctx, p, authz.PermTenantTrashRead, authz.Resource{}); err != nil {
			return out, err
		}
		f.TenantIDs = []uuid.UUID{*p.TenantID}
	} else {
		if _, err := platformOp(ctx, authz.PermPlatformTrashRead); err != nil {
			return out, err
		}
		f.TenantIDs = in.TenantIDs
		f.PlatformToo = len(in.TenantIDs) == 0
	}
	types := h.Resources.Types()
	if in.ResourceType != "" {
		rt, err := contentmodel.ParseResourceType(in.ResourceType)
		if err != nil {
			return out, problem.Invalid("resource_type", problem.SubInvalid, "is not a resource type of module learning")
		}
		types = nil
		if _, ok := h.Resources.Get(rt); ok {
			types = []contentmodel.ResourceType{rt}
		}
	}
	after, err := h.Cursors.Decode(in.Cursor)
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
	if len(types) == 0 {
		return out, nil
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonTrash, tenancy.WithTenantIDs(f.TenantIDs...), tenancy.WithMode(tenancy.ModeRead))
	if err != nil {
		return out, err
	}
	var all []trash.Entry
	err = h.Session.Read(bctx, func(ctx context.Context) error {
		for _, rt := range types {
			hd, _ := h.Resources.Get(rt)
			rows, err := hd.ListDeleted(ctx, f)
			if err != nil {
				return err
			}
			all = append(all, rows...)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	// One keyset over every type: deleted_at DESC, resource_id DESC.
	sort.Slice(all, func(i, j int) bool {
		if !all[i].DeletedAt.Equal(all[j].DeletedAt) {
			return all[i].DeletedAt.After(all[j].DeletedAt)
		}
		return string(all[i].ResourceID[:]) > string(all[j].ResourceID[:])
	})
	if len(all) > f.Limit {
		all = all[:f.Limit]
		last := all[len(all)-1]
		c := h.Cursors.Encode(page.Cursor{Key: last.DeletedAt.UTC().Format(time.RFC3339Nano), ID: last.ResourceID})
		out.NextCursor, out.HasMore = &c, true
	}
	out.Data = all
	return out, nil
}
