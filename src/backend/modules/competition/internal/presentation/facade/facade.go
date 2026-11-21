// Package facade implements the in-process facade of module competition (api, generated from
// specs/openapi/modules/competition.internal.v1.yaml): module identity calls the student lifecycle and user merge
// operations of its sagas through pkg/modcall, never over HTTP (ADR-0074, AGENTS R29); no route of this package is
// mounted.
package facade

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Facade serves the lifecycle operations of module competition (F100 T042, OQ F016-2, F045-3).
type Facade struct {
	Lifecycle command.PartyLifecycle
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
