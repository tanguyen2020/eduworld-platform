package facade

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ListTrash implements listTrash for the tournament and challenge templates (ADR-0028, x-allowed-callers: identity).
func (f Facade) ListTrash(ctx context.Context, req api.ListTrashRequestObject) (api.ListTrashResponseObject, error) {
	if f.TrashList == nil {
		return nil, problem.New(problem.Unavailable, "the trash is not served by this binary").Retry()
	}
	in := query.TrashInput{}
	if req.Params.TenantIds != nil {
		in.TenantIDs = append([]uuid.UUID(nil), *req.Params.TenantIds...)
	}
	if req.Params.ResourceType != nil {
		in.ResourceType = string(*req.Params.ResourceType)
	}
	if req.Params.Cursor != nil {
		in.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		in.Limit = *req.Params.Limit
	}
	p, err := f.TrashList.List(ctx, in)
	if err != nil {
		return nil, err
	}
	out := api.ListTrash200JSONResponse{Data: make([]api.CommonInternalCommonV1TrashItem, 0, len(p.Data)),
		Page: api.CommonInternalCommonV1PageInfo{HasMore: p.HasMore, NextCursor: p.NextCursor}}
	for _, e := range p.Data {
		it := api.CommonInternalCommonV1TrashItem{ResourceType: api.CommonInternalCommonV1ResourceType(e.ResourceType), ResourceId: e.ResourceID,
			DeletedAt: e.DeletedAt, DeletedBy: e.DeletedBy}
		restorable, title := e.Restorable, e.Title
		it.Restorable, it.Title = &restorable, &title
		if e.DeleteReason != "" {
			r := e.DeleteReason
			it.DeleteReason = &r
		}
		out.Data = append(out.Data, it)
	}
	return out, nil
}

// RestoreTrashItem implements restoreTrashItem for the templates (x-allowed-callers: identity, x-context-required:
// tenant_scope, idempotency_key): a call again with the same Idempotency-Key returns the stored result.
func (f Facade) RestoreTrashItem(ctx context.Context, req api.RestoreTrashItemRequestObject) (api.RestoreTrashItemResponseObject, error) {
	if f.TrashRestore == nil {
		return nil, problem.New(problem.Unavailable, "the trash is not served by this binary").Retry()
	}
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	in := command.RestoreTrashInput{ResourceType: string(req.ResourceType), ID: req.ResourceId, Reason: req.Body.Reason}
	v, err := replayOr(ctx, f.Idempotency, "restoreTrashItem", in, func(ctx context.Context) (command.RestoreTrashView, error) {
		return f.TrashRestore.Restore(ctx, in)
	})
	if err != nil {
		return nil, err
	}
	return api.RestoreTrashItem200JSONResponse{ResourceType: api.CommonInternalCommonV1ResourceType(v.ResourceType), ResourceId: v.ResourceID,
		RestoredAt: v.RestoredAt}, nil
}
