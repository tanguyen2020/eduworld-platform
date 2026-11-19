package facade

import (
	"context"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
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
