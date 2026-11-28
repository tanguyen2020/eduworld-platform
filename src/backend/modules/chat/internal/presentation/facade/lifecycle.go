package facade

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Lifecycle is the chat part of the identity sagas (command.UserLifecycle, F115 T042, OQ F016-2, F045-3).
type Lifecycle interface {
	Cleanup(ctx context.Context, student uuid.UUID) (status, reason string, err error)
	Restore(ctx context.Context, student uuid.UUID) (string, error)
	Anonymize(ctx context.Context, user uuid.UUID) (string, error)
	Merge(ctx context.Context, in command.MergeInput) (status, reason string, err error)
}

func (f Facade) lifecycle() (Lifecycle, error) {
	if f.Lifecycle == nil {
		return nil, problem.New(problem.Unavailable, "chat lifecycle not wired")
	}
	return f.Lifecycle, nil
}

// CleanupStudentData implements api.StrictServerInterface (x-allowed-callers: identity, student_cleanup saga, ADR-0032):
// blocked open_moderation_case while a moderation case of the student is open.
func (f Facade) CleanupStudentData(ctx context.Context, req api.CleanupStudentDataRequestObject) (api.CleanupStudentDataResponseObject, error) {
	l, err := f.lifecycle()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "cleanup request required")
	}
	status, reason, err := l.Cleanup(ctx, req.StudentId)
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

// RestoreStudentData implements api.StrictServerInterface (x-allowed-callers: identity, student_restore saga and the
// compensation of a blocked cleanup).
func (f Facade) RestoreStudentData(ctx context.Context, req api.RestoreStudentDataRequestObject) (api.RestoreStudentDataResponseObject, error) {
	l, err := f.lifecycle()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "restore request required")
	}
	status, err := l.Restore(ctx, req.StudentId)
	if err != nil {
		return nil, err
	}
	return api.RestoreStudentData200JSONResponse{StudentId: req.StudentId, Status: api.CommonInternalCommonV1StudentLifecycleResultStatus(status)}, nil
}

// AnonymizeUserData implements api.StrictServerInterface (x-allowed-callers: identity, user_anonymize saga, F016 US5).
func (f Facade) AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error) {
	l, err := f.lifecycle()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "anonymize request required")
	}
	status, err := l.Anonymize(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return api.AnonymizeUserData200JSONResponse{UserId: req.UserId, Status: api.CommonInternalCommonV1UserLifecycleResultStatus(status)}, nil
}

// MergeUserData implements api.StrictServerInterface (x-allowed-callers: identity, user_merge saga F045): idempotent per
// merge_id (Idempotency-Key of the saga step) — a repeated call has nothing left to move.
func (f Facade) MergeUserData(ctx context.Context, req api.MergeUserDataRequestObject) (api.MergeUserDataResponseObject, error) {
	l, err := f.lifecycle()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problem.Invalid("body", problem.SubRequired, "merge request required")
	}
	status, reason, err := l.Merge(ctx, command.MergeInput{MergeID: req.Body.MergeId, From: req.UserId, Into: req.Body.TargetUserId})
	if err != nil {
		return nil, err
	}
	out := api.MergeUserData200JSONResponse{UserId: req.UserId, Status: api.CommonInternalCommonV1UserLifecycleResultStatus(status)}
	if reason != "" {
		out.BlockedReason = &reason
	}
	return out, nil
}
