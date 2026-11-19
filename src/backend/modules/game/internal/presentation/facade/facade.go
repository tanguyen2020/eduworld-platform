// Package facade implements the in-process facade of module game (gameapi, generated from
// specs/openapi/modules/game.internal.v1.yaml): module identity calls the student lifecycle operations of its
// sagas through pkg/modcall, never over HTTP (ADR-0074, AGENTS R29); no route of this package is mounted.
package facade

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// Facade serves the student lifecycle operations of module game (F016).
type Facade struct {
	Lifecycle command.StudentLifecycle
}

// CleanupStudentData implements gameapi.StrictServerInterface (x-allowed-callers: identity).
func (f Facade) CleanupStudentData(ctx context.Context, req api.CleanupStudentDataRequestObject) (api.CleanupStudentDataResponseObject, error) {
	status, reason, err := f.Lifecycle.Cleanup(ctx, req.StudentId)
	if err != nil {
		return nil, err
	}
	out := api.CleanupStudentData200JSONResponse{StudentId: req.StudentId, Status: api.CommonInternalCommonV1StudentLifecycleResultStatus(status)}
	if reason != "" {
		r := api.CommonInternalCommonV1StudentLifecycleResultBlockedReason(reason)
		out.BlockedReason = &r
	}
	return out, nil
}

// RestoreStudentData implements gameapi.StrictServerInterface (x-allowed-callers: identity).
func (f Facade) RestoreStudentData(ctx context.Context, req api.RestoreStudentDataRequestObject) (api.RestoreStudentDataResponseObject, error) {
	status, err := f.Lifecycle.Restore(ctx, req.StudentId)
	if err != nil {
		return nil, err
	}
	return api.RestoreStudentData200JSONResponse{StudentId: req.StudentId, Status: api.CommonInternalCommonV1StudentLifecycleResultStatus(status)}, nil
}

// AnonymizeUserData implements gameapi.StrictServerInterface (x-allowed-callers: identity, F016 US5). Module
// game keeps no personal data of a student yet (nickname, avatar and inventory arrive with F070+; the login
// events are anonymized by their own retention, DATA_PRIVACY §2), so the step has nothing to do.
func (f Facade) AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error) {
	status, err := f.Lifecycle.Anonymize(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return api.AnonymizeUserData200JSONResponse{UserId: req.UserId, Status: api.CommonInternalCommonV1UserLifecycleResultStatus(status)}, nil
}
