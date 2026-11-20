package facade

import (
	"context"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Student lifecycle operations of the identity sagas (F016 T024, ADR-0032). Module learning keeps no
// student-owned row yet: attempts, results and mastery arrive with F030+, and the ref.* projections follow the
// identity events (student.cleaned_up / student.restored) on their own. Both operations therefore answer
// nothing_to_do; the feature that adds student data extends them (and may answer blocked for an open exam).

// CleanupStudentData implements learningapi.StrictServerInterface (x-allowed-callers: identity).
func (f Facade) CleanupStudentData(ctx context.Context, req learningapi.CleanupStudentDataRequestObject) (learningapi.CleanupStudentDataResponseObject, error) {
	_, done := observability.UseCase(ctx, "learning.facade.CleanupStudentData")
	done(nil)
	return learningapi.CleanupStudentData200JSONResponse{StudentId: req.StudentId,
		Status: learningapi.CommonInternalCommonV1StudentLifecycleResultStatus("nothing_to_do")}, nil
}

// RestoreStudentData implements learningapi.StrictServerInterface (x-allowed-callers: identity).
func (f Facade) RestoreStudentData(ctx context.Context, req learningapi.RestoreStudentDataRequestObject) (learningapi.RestoreStudentDataResponseObject, error) {
	_, done := observability.UseCase(ctx, "learning.facade.RestoreStudentData")
	done(nil)
	return learningapi.RestoreStudentData200JSONResponse{StudentId: req.StudentId,
		Status: learningapi.CommonInternalCommonV1StudentLifecycleResultStatus("nothing_to_do")}, nil
}

// AnonymizeUserData implements learningapi.StrictServerInterface (x-allowed-callers: identity, F016 US5):
// no attempt, answer or mastery row exists yet (F030+), the ref.* projections follow user.anonymized.
func (f Facade) AnonymizeUserData(ctx context.Context, req learningapi.AnonymizeUserDataRequestObject) (learningapi.AnonymizeUserDataResponseObject, error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.AnonymizeUserData")
	if f.Efficacy == nil {
		done(nil)
		return learningapi.AnonymizeUserData200JSONResponse{UserId: req.UserId,
			Status: learningapi.CommonInternalCommonV1UserLifecycleResultStatus("nothing_to_do")}, nil
	}
	// F035-AC-19: the student leaves the efficacy studies of the tenant; the derived study data is soft-deleted.
	n, err := f.Efficacy.AnonymizeStudent(ctx, req.UserId)
	done(err)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return learningapi.AnonymizeUserData200JSONResponse{UserId: req.UserId,
			Status: learningapi.CommonInternalCommonV1UserLifecycleResultStatus("nothing_to_do")}, nil
	}
	affected := []struct {
		Count        int    `json:"count"`
		ResourceType string `json:"resource_type"`
	}{{Count: n, ResourceType: "efficacy_participant"}}
	return learningapi.AnonymizeUserData200JSONResponse{UserId: req.UserId, Affected: &affected,
		Status: learningapi.CommonInternalCommonV1UserLifecycleResultStatus("done")}, nil
}

// EfficacyAnonymizer is the efficacy step of anonymizeUserData (command.Efficacy, F035-AC-19).
type EfficacyAnonymizer interface {
	AnonymizeStudent(ctx context.Context, studentID uuid.UUID) (int, error)
}

// MergeUserDataHandler is the learning step of the account merge saga (command.MergeUserData).
type MergeUserDataHandler interface {
	Handle(ctx context.Context, in command.MergeUserDataInput) (command.MergeUserDataResult, error)
}

// MergeUserData implements learningapi.StrictServerInterface (x-allowed-callers: identity; OQ F016-2, F045-3): the
// original student_id keeps its learning data and is linked to the target user, idempotent by merge_id
// (Idempotency-Key). Without a handler (binary without the merge step) it answers 503 UNAVAILABLE, never a silent success.
func (f Facade) MergeUserData(ctx context.Context, req learningapi.MergeUserDataRequestObject) (learningapi.MergeUserDataResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.Merge == nil {
		return nil, problem.New(problem.Unavailable, "").Retry()
	}
	b := *req.Body
	in := command.MergeUserDataInput{MergeID: b.MergeId, From: req.UserId, Into: b.TargetUserId, Reason: b.Reason}
	r, err := idempotent(ctx, f.Idempotency, "mergeUserData", in, func(ctx context.Context) (command.MergeUserDataResult, error) {
		return f.Merge.Handle(ctx, in)
	})
	if err != nil {
		return nil, err
	}
	out := learningapi.MergeUserData200JSONResponse{UserId: req.UserId, Status: learningapi.CommonInternalCommonV1UserLifecycleResultStatus(r.Status)}
	if r.BlockedReason != "" {
		reason := r.BlockedReason
		out.BlockedReason = &reason
	}
	return out, nil
}
