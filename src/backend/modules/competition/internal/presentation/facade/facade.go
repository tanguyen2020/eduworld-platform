// Package facade implements the in-process facade of module competition (api, generated from
// specs/openapi/modules/competition.internal.v1.yaml): module identity calls the student lifecycle and user merge
// operations of its sagas and the template assignments (F107) through pkg/modcall, never over HTTP (ADR-0074, AGENTS R29); no route of this package is
// mounted.
package facade

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
)

// Facade serves the lifecycle operations of module competition (F100 T042, OQ F016-2, F045-3).
type Facade struct {
	// Ops serves listOutboxEventIds and listProcessedEventIds for `worker reconcile` (F134); nil = 503 UNAVAILABLE.
	Ops       *reconcile.Source
	Lifecycle command.PartyLifecycle
	// Hub answers getHubPosition (F071 T049); nil in a binary without the room host registry → 503 UNAVAILABLE.
	Hub *query.HubPosition
	// Assign and AssignList serve the template assignments of the identity saga (F107-AC-29); nil → 503 UNAVAILABLE.
	Assign      *command.TemplateAssignments
	AssignList  *query.Assignments
	Idempotency *idempotency.Store
	// TrashList and TrashRestore serve listTrash and restoreTrashItem of the templates (ADR-0028, F057); nil → 503 UNAVAILABLE.
	TrashList    *query.Trash
	TrashRestore *command.RestoreTrash
}

// CleanupStudentData implements api.StrictServerInterface (x-allowed-callers: identity).
func (f Facade) CleanupStudentData(ctx context.Context, req api.CleanupStudentDataRequestObject) (api.CleanupStudentDataResponseObject, error) {
	status, err := f.Lifecycle.Cleanup(ctx, req.StudentId)
	if err != nil {
		return nil, err
	}
	return api.CleanupStudentData200JSONResponse{StudentId: req.StudentId, Status: api.CommonInternalCommonV1StudentLifecycleResultStatus(status)}, nil
}

// RestoreStudentData implements api.StrictServerInterface (x-allowed-callers: identity).
func (f Facade) RestoreStudentData(ctx context.Context, req api.RestoreStudentDataRequestObject) (api.RestoreStudentDataResponseObject, error) {
	status, err := f.Lifecycle.Restore(ctx, req.StudentId)
	if err != nil {
		return nil, err
	}
	return api.RestoreStudentData200JSONResponse{StudentId: req.StudentId, Status: api.CommonInternalCommonV1StudentLifecycleResultStatus(status)}, nil
}

// AnonymizeUserData implements api.StrictServerInterface (x-allowed-callers: identity, F016 US5).
func (f Facade) AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error) {
	status, err := f.Lifecycle.Anonymize(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return api.AnonymizeUserData200JSONResponse{UserId: req.UserId, Status: api.CommonInternalCommonV1UserLifecycleResultStatus(status)}, nil
}

// MergeUserData implements api.StrictServerInterface (x-allowed-callers: identity, F045 user merge saga): idempotent per
// merge_id (Idempotency-Key of the saga step) — a repeated call has nothing left to move.
func (f Facade) MergeUserData(ctx context.Context, req api.MergeUserDataRequestObject) (api.MergeUserDataResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "merge request required")
	}
	status, err := f.Lifecycle.Merge(ctx, req.UserId, req.Body.TargetUserId)
	if err != nil {
		return nil, err
	}
	return api.MergeUserData200JSONResponse{UserId: req.UserId, Status: api.CommonInternalCommonV1UserLifecycleResultStatus(status)}, nil
}

// GetHubPosition implements api.StrictServerInterface (x-allowed-callers: game, F071 T049): the last position the room
// host stored for a student seated in a hub instance of the tenant of the scope; 404 NOT_FOUND otherwise.
func (f Facade) GetHubPosition(ctx context.Context, req api.GetHubPositionRequestObject) (api.GetHubPositionResponseObject, error) {
	if f.Hub == nil {
		return nil, problem.New(problem.Unavailable, "hub registry not loaded in this binary").Retry()
	}
	p, err := f.Hub.Get(ctx, req.StudentId)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, problem.New(problem.NotFound, "student not in a hub instance of this tenant")
	}
	return api.GetHubPosition200JSONResponse{StudentId: req.StudentId, AreaId: p.AreaID, RoomId: p.RoomID,
		Position: []float64{p.Position[0], p.Position[1], p.Position[2]}}, nil
}
