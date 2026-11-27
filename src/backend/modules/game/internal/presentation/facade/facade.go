// Package facade implements the in-process facade of module game (gameapi, generated from
// specs/openapi/modules/game.internal.v1.yaml): module identity calls the student lifecycle operations of its
// sagas through pkg/modcall, never over HTTP (ADR-0074, AGENTS R29); no route of this package is mounted.
package facade

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
)

// Facade serves the student lifecycle operations of module game (F016).
type Facade struct {
	// Ops serves listOutboxEventIds and listProcessedEventIds for `worker reconcile` (F134); nil = 503 UNAVAILABLE.
	Ops       *reconcile.Source
	Lifecycle command.StudentLifecycle
	// GameAdmin serves the tenant assignments (identity) and the school catalog (learning) of F078.
	GameAdmin
	// PartyActivities serves the party activities of the Party Lobby (competition, F103).
	PartyActivities
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
// game overwrites the nickname and removes the custom avatar of the game profile (F070 T039); the login events are
// anonymized by their own retention (DATA_PRIVACY §2).
func (f Facade) AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error) {
	status, err := f.Lifecycle.Anonymize(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return api.AnonymizeUserData200JSONResponse{UserId: req.UserId, Status: api.CommonInternalCommonV1UserLifecycleResultStatus(status)}, nil
}

// MergeUserData implements gameapi.StrictServerInterface (x-allowed-callers: identity, F045 user merge saga; F070
// T039): the target keeps one game profile, the source profile is soft-deleted (or moved when the target has none).
// Idempotent per merge_id (Idempotency-Key of the saga step): a repeated call has nothing left to move.
func (f Facade) MergeUserData(ctx context.Context, req api.MergeUserDataRequestObject) (api.MergeUserDataResponseObject, error) {
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "merge request required")
	}
	status, reason, err := f.Lifecycle.Merge(ctx, req.UserId, req.Body.TargetUserId, req.Body.MergeId)
	if err != nil {
		return nil, err
	}
	out := api.MergeUserData200JSONResponse{UserId: req.UserId, Status: api.CommonInternalCommonV1UserLifecycleResultStatus(status)}
	if reason != "" {
		out.BlockedReason = &reason
	}
	return out, nil
}
