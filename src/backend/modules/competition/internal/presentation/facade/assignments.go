package facade

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// facadeMethod is the method column of the idempotency records written by the facade.
const facadeMethod = "FACADE"

func assignmentList(views []command.AssignmentView) api.CommonInternalCommonV1TenantAssignmentList {
	out := api.CommonInternalCommonV1TenantAssignmentList{Data: make([]api.CommonInternalCommonV1TenantAssignment, 0, len(views))}
	for _, v := range views {
		status := api.CommonInternalCommonV1TenantAssignmentStatus("active")
		if v.RevokedAt != nil {
			status = "revoked"
		}
		out.Data = append(out.Data, api.CommonInternalCommonV1TenantAssignment{AssignmentId: v.AssignmentID,
			ResourceType: api.CommonInternalCommonV1ResourceType(v.ResourceType), ResourceId: v.ResourceID, TenantId: v.TenantID,
			AssignedBy: v.AssignedBy, AssignedAt: v.AssignedAt, Status: status, RevokedAt: v.RevokedAt})
	}
	return out
}

// idempotent replays the result of a facade write called again with the same Idempotency-Key of the saga (stored with
// the result in the transaction of the use case, idempotency.SetResult).
func (f Facade) idempotent(ctx context.Context, op string, req any, fn func(ctx context.Context) ([]command.AssignmentView, error)) ([]command.AssignmentView, error) {
	return replayOr(ctx, f.Idempotency, op, req, fn)
}

// replayOr runs fn once per Idempotency-Key of the caller and replays its stored result afterwards (another body with
// the key is 409 IDEMPOTENCY_KEY_REUSED); a nil store is 503 UNAVAILABLE.
func replayOr[T any](ctx context.Context, store *idempotency.Store, op string, req any, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	raw, ok := modcall.IdempotencyKeyFrom(ctx)
	if !ok {
		return zero, problem.New(problem.IdempotencyKeyRequired, "")
	}
	if store == nil {
		return zero, problem.New(problem.Unavailable, "idempotency store not loaded in this binary").Retry()
	}
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return zero, problem.New(problem.Unauthenticated, "")
	}
	key, issued, err := idempotency.ParseKey(raw, store.Clock().Now(), store.TTL())
	if err != nil {
		return zero, err
	}
	k := idempotency.Key{UserID: p.UserID, Method: facadeMethod, Route: "competition." + op, Key: key, IssuedOn: issued}
	body, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}
	hash := idempotency.Hash(body)
	replay := func() (T, bool, error) {
		rec, err := store.Lookup(ctx, k)
		if err != nil || rec == nil {
			return zero, false, err
		}
		if rec.Hash != hash {
			return zero, false, problem.New(problem.IdempotencyKeyReused, "")
		}
		var v T
		if err := json.Unmarshal(rec.Body, &v); err != nil {
			return zero, false, err
		}
		return v, true, nil
	}
	if v, found, err := replay(); err != nil || found {
		return v, err
	}
	v, err := fn(idempotency.WithPending(ctx, store.NewPending(k, hash, 200)))
	if errors.Is(err, idempotency.ErrDuplicate) {
		if v, found, rerr := replay(); rerr == nil && found {
			return v, nil
		}
	}
	return v, err
}

// ApplyTenantAssignments implements applyTenantAssignments for the tournament and challenge templates (F107-AC-29,
// x-allowed-callers: identity).
func (f Facade) ApplyTenantAssignments(ctx context.Context, req api.ApplyTenantAssignmentsRequestObject) (api.ApplyTenantAssignmentsResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.Assign == nil {
		return nil, problem.New(problem.Unavailable, "assignments not loaded in this binary").Retry()
	}
	b := *req.Body
	views, err := f.idempotent(ctx, "applyTenantAssignments", b, func(ctx context.Context) ([]command.AssignmentView, error) {
		return f.Assign.Apply(ctx, command.AssignInput{ResourceType: string(b.ResourceType), ResourceID: b.ResourceId, TenantIDs: b.TenantIds})
	})
	if err != nil {
		return nil, err
	}
	return api.ApplyTenantAssignments200JSONResponse(assignmentList(views)), nil
}

// RevokeTenantAssignments implements revokeTenantAssignments (F107-AC-29, x-allowed-callers: identity).
func (f Facade) RevokeTenantAssignments(ctx context.Context, req api.RevokeTenantAssignmentsRequestObject) (api.RevokeTenantAssignmentsResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.Assign == nil {
		return nil, problem.New(problem.Unavailable, "assignments not loaded in this binary").Retry()
	}
	b := *req.Body
	views, err := f.idempotent(ctx, "revokeTenantAssignments", b, func(ctx context.Context) ([]command.AssignmentView, error) {
		return f.Assign.Revoke(ctx, command.AssignInput{ResourceType: string(b.ResourceType), ResourceID: b.ResourceId, TenantIDs: b.TenantIds,
			Reason: b.Reason})
	})
	if err != nil {
		return nil, err
	}
	return api.RevokeTenantAssignments200JSONResponse(assignmentList(views)), nil
}

// ListResourceAssignments implements listResourceAssignments (F107-AC-29, x-allowed-callers: identity).
func (f Facade) ListResourceAssignments(ctx context.Context, req api.ListResourceAssignmentsRequestObject) (api.ListResourceAssignmentsResponseObject, error) {
	if f.AssignList == nil {
		return nil, problem.New(problem.Unavailable, "assignments not loaded in this binary").Retry()
	}
	cursor, limit := "", 0
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	p, err := f.AssignList.List(ctx, string(req.Params.ResourceType), req.Params.ResourceId, cursor, limit)
	if err != nil {
		return nil, err
	}
	views := make([]command.AssignmentView, len(p.Data))
	for i, a := range p.Data {
		views[i] = assignmentView(a)
	}
	out := assignmentList(views)
	out.Page = api.CommonInternalCommonV1PageInfo{HasMore: p.HasMore, NextCursor: p.NextCursor}
	return api.ListResourceAssignments200JSONResponse(out), nil
}

func assignmentView(a write.AssignmentRow) command.AssignmentView {
	return command.AssignmentView{AssignmentID: a.ID, ResourceType: a.ResourceType, ResourceID: a.ResourceID, TenantID: a.Tenant,
		AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt}
}
