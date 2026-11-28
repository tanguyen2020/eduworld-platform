package facade

import (
	"context"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func assignmentList(views []command.AssignmentView) learningapi.CommonInternalCommonV1TenantAssignmentList {
	out := learningapi.CommonInternalCommonV1TenantAssignmentList{Data: make([]learningapi.CommonInternalCommonV1TenantAssignment, 0, len(views))}
	for _, v := range views {
		status := learningapi.CommonInternalCommonV1TenantAssignmentStatus("active")
		if v.RevokedAt != nil {
			status = "revoked"
		}
		out.Data = append(out.Data, learningapi.CommonInternalCommonV1TenantAssignment{AssignmentId: v.AssignmentID,
			ResourceType: learningapi.CommonInternalCommonV1ResourceType(v.ResourceType), ResourceId: v.ResourceID, TenantId: v.TenantID,
			AssignedBy: v.AssignedBy, AssignedAt: v.AssignedAt, Status: status, RevokedAt: v.RevokedAt, Created: v.Created})
	}
	return out
}

// ApplyTenantAssignments implements applyTenantAssignments (F020-AC-15, AC-16, AC-20).
func (f Facade) ApplyTenantAssignments(ctx context.Context, req learningapi.ApplyTenantAssignmentsRequestObject) (learningapi.ApplyTenantAssignmentsResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	b := *req.Body
	views, err := idempotent(ctx, f.Idempotency, "applyTenantAssignments", b, func(ctx context.Context) ([]command.AssignmentView, error) {
		return f.Apply.Handle(ctx, command.AssignmentsInput{ResourceType: string(b.ResourceType), ResourceID: b.ResourceId, TenantIDs: b.TenantIds})
	})
	if err != nil {
		return nil, err
	}
	return learningapi.ApplyTenantAssignments200JSONResponse(assignmentList(views)), nil
}

// RevokeTenantAssignments implements revokeTenantAssignments (F020-AC-17, AC-20).
func (f Facade) RevokeTenantAssignments(ctx context.Context, req learningapi.RevokeTenantAssignmentsRequestObject) (learningapi.RevokeTenantAssignmentsResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	b := *req.Body
	views, err := idempotent(ctx, f.Idempotency, "revokeTenantAssignments", b, func(ctx context.Context) ([]command.AssignmentView, error) {
		return f.Revoke.Handle(ctx, command.AssignmentsInput{ResourceType: string(b.ResourceType), ResourceID: b.ResourceId,
			TenantIDs: b.TenantIds, Reason: b.Reason})
	})
	if err != nil {
		return nil, err
	}
	return learningapi.RevokeTenantAssignments200JSONResponse(assignmentList(views)), nil
}

// ListResourceAssignments implements listResourceAssignments.
func (f Facade) ListResourceAssignments(ctx context.Context, req learningapi.ListResourceAssignmentsRequestObject) (learningapi.ListResourceAssignmentsResponseObject, error) {
	in := query.ListResourceAssignmentsInput{ResourceType: string(req.Params.ResourceType), ResourceID: req.Params.ResourceId}
	if req.Params.Cursor != nil {
		in.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		in.Limit = *req.Params.Limit
	}
	p, err := f.Assignments.Handle(ctx, in)
	if err != nil {
		return nil, err
	}
	views := make([]command.AssignmentView, len(p.Data))
	for i, a := range p.Data {
		views[i] = command.AssignmentView{AssignmentID: a.AssignmentID, ResourceType: a.ResourceType, ResourceID: a.ResourceID,
			TenantID: a.TenantID, AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt}
	}
	out := assignmentList(views)
	out.Page = learningapi.CommonInternalCommonV1PageInfo{HasMore: p.HasMore, NextCursor: p.NextCursor}
	return learningapi.ListResourceAssignments200JSONResponse(out), nil
}
