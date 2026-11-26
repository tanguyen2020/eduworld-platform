package facade

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Assignments are the use cases behind applyTenantAssignments, revokeTenantAssignments and listResourceAssignments
// (command.ChatAssignments, F082 T040).
type Assignments interface {
	Apply(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID) ([]command.AssignmentView, error)
	Revoke(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID, reason string) ([]command.AssignmentView, error)
	List(ctx context.Context, resourceType string, id uuid.UUID, cursor string, limit int) (command.AssignmentPage, error)
}

func assignmentList(views []command.AssignmentView) api.CommonInternalCommonV1TenantAssignmentList {
	out := api.CommonInternalCommonV1TenantAssignmentList{Data: make([]api.CommonInternalCommonV1TenantAssignment, 0, len(views))}
	for _, v := range views {
		out.Data = append(out.Data, api.CommonInternalCommonV1TenantAssignment{AssignmentId: v.AssignmentID, AssignedAt: v.AssignedAt,
			AssignedBy: v.AssignedBy, ResourceId: v.ResourceID, ResourceType: api.CommonInternalCommonV1ResourceType(v.ResourceType),
			TenantId: v.TenantID, Status: api.CommonInternalCommonV1TenantAssignmentStatus(v.Status), RevokedAt: v.RevokedAt})
	}
	return out
}

func (f Facade) assignments() (Assignments, error) {
	if f.Assignments == nil {
		return nil, problem.New(problem.Unavailable, "sticker pack assignments are not served by this binary").Retry()
	}
	return f.Assignments, nil
}

// ApplyTenantAssignments implements chatapi.StrictServerInterface (x-allowed-callers: identity, F082 T040).
func (f Facade) ApplyTenantAssignments(ctx context.Context, req api.ApplyTenantAssignmentsRequestObject) (api.ApplyTenantAssignmentsResponseObject, error) {
	a, err := f.assignments()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "assignment request required")
	}
	views, err := a.Apply(ctx, string(req.Body.ResourceType), req.Body.ResourceId, req.Body.TenantIds)
	if err != nil {
		return nil, err
	}
	return api.ApplyTenantAssignments200JSONResponse(assignmentList(views)), nil
}

// RevokeTenantAssignments implements chatapi.StrictServerInterface (x-allowed-callers: identity, F082 T040).
func (f Facade) RevokeTenantAssignments(ctx context.Context, req api.RevokeTenantAssignmentsRequestObject) (api.RevokeTenantAssignmentsResponseObject, error) {
	a, err := f.assignments()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "revoke request required")
	}
	views, err := a.Revoke(ctx, string(req.Body.ResourceType), req.Body.ResourceId, req.Body.TenantIds, req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return api.RevokeTenantAssignments200JSONResponse(assignmentList(views)), nil
}

// ListResourceAssignments implements chatapi.StrictServerInterface (x-allowed-callers: identity, F082 T040).
func (f Facade) ListResourceAssignments(ctx context.Context, req api.ListResourceAssignmentsRequestObject) (api.ListResourceAssignmentsResponseObject, error) {
	a, err := f.assignments()
	if err != nil {
		return nil, err
	}
	cursor, limit := "", 0
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	page, err := a.List(ctx, string(req.Params.ResourceType), req.Params.ResourceId, cursor, limit)
	if err != nil {
		return nil, err
	}
	out := assignmentList(page.Data)
	out.Page = api.CommonInternalCommonV1PageInfo{HasMore: page.HasMore, NextCursor: page.NextCursor}
	return api.ListResourceAssignments200JSONResponse(out), nil
}
