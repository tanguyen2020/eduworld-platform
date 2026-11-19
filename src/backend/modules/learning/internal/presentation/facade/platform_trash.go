package facade

import (
	"context"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ListTrash implements listTrash (F020-AC-24).
func (f Facade) ListTrash(ctx context.Context, req learningapi.ListTrashRequestObject) (learningapi.ListTrashResponseObject, error) {
	in := query.ListTrashInput{}
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
	p, err := f.Trash.Handle(ctx, in)
	if err != nil {
		return nil, err
	}
	out := learningapi.ListTrash200JSONResponse{Data: make([]learningapi.CommonInternalCommonV1TrashItem, 0, len(p.Data)),
		Page: learningapi.CommonInternalCommonV1PageInfo{HasMore: p.HasMore, NextCursor: p.NextCursor}}
	for _, e := range p.Data {
		it := learningapi.CommonInternalCommonV1TrashItem{ResourceType: learningapi.CommonInternalCommonV1ResourceType(e.ResourceType),
			ResourceId: e.ResourceID, TenantId: e.TenantID, DeletedAt: e.DeletedAt, DeletedBy: e.DeletedBy}
		restorable := e.Restorable()
		it.Restorable = &restorable
		if e.Title != "" {
			title := e.Title
			it.Title = &title
		}
		if e.DeleteReason != "" {
			r := e.DeleteReason
			it.DeleteReason = &r
		}
		out.Data = append(out.Data, it)
	}
	return out, nil
}

// RestoreTrashItem implements restoreTrashItem (F020-AC-25, AC-26).
func (f Facade) RestoreTrashItem(ctx context.Context, req learningapi.RestoreTrashItemRequestObject) (learningapi.RestoreTrashItemResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	in := command.RestoreInput{ResourceType: string(req.ResourceType), ID: req.ResourceId, Reason: req.Body.Reason}
	v, err := idempotent(ctx, f.Idempotency, "restoreTrashItem", in, func(ctx context.Context) (command.RestoreView, error) {
		return f.Restore.Handle(ctx, in)
	})
	if err != nil {
		return nil, err
	}
	return learningapi.RestoreTrashItem200JSONResponse{ResourceType: learningapi.CommonInternalCommonV1ResourceType(v.ResourceType),
		ResourceId: v.ResourceID, RestoredAt: v.RestoredAt}, nil
}
