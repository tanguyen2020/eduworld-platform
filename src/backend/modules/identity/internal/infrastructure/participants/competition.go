package participants

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	competitionapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// CompetitionFacade is the subset of the competition facade used by the student lifecycle sagas (F100 T042, OQ
// F016-2): the student leaves its party; nothing to restore (parties are temporary).
type CompetitionFacade interface {
	CleanupStudentData(ctx context.Context, req competitionapi.CleanupStudentDataRequestObject) (competitionapi.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req competitionapi.RestoreStudentDataRequestObject) (competitionapi.RestoreStudentDataResponseObject, error)
}

// CompetitionAnonymizer is the subset of the competition facade used by the user_anonymize saga.
type CompetitionAnonymizer interface {
	AnonymizeUserData(ctx context.Context, req competitionapi.AnonymizeUserDataRequestObject) (competitionapi.AnonymizeUserDataResponseObject, error)
}

// Competition adapts the competition facade.
type Competition struct{ F CompetitionFacade }

var _ port.StudentDataParticipant = Competition{}

// Name implements port.StudentDataParticipant.
func (Competition) Name() string { return "competition" }

// Cleanup implements port.StudentDataParticipant.
func (c Competition) Cleanup(ctx context.Context, student, run uuid.UUID, lastLogin *time.Time) (port.StudentDataResult, error) {
	body := competitionapi.CleanupStudentDataJSONRequestBody{CleanupRunId: run, Reason: "inactive_cleanup", LastLoginAt: lastLogin}
	resp, err := modcall.Call(ctx, op("competition", "cleanupStudentData", competitionapi.AllowedCallers, competitionapi.ContextRequired),
		func(ctx context.Context) (competitionapi.CleanupStudentDataResponseObject, error) {
			return c.F.CleanupStudentData(ctx, competitionapi.CleanupStudentDataRequestObject{StudentId: student, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(competitionapi.CleanupStudentData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("competition cleanupStudentData: unexpected response %T", resp)
	}
	var reason *string
	if v.BlockedReason != nil {
		s := string(*v.BlockedReason)
		reason = &s
	}
	return result(string(v.Status), reason), nil
}

// Restore implements port.StudentDataParticipant.
func (c Competition) Restore(ctx context.Context, student uuid.UUID, viaLogin bool) (port.StudentDataResult, error) {
	body := competitionapi.RestoreStudentDataJSONRequestBody{Reason: competitionapi.CommonInternalCommonV1StudentRestoreRequestReason(restoreReason(viaLogin))}
	resp, err := modcall.Call(ctx, op("competition", "restoreStudentData", competitionapi.AllowedCallers, competitionapi.ContextRequired),
		func(ctx context.Context) (competitionapi.RestoreStudentDataResponseObject, error) {
			return c.F.RestoreStudentData(ctx, competitionapi.RestoreStudentDataRequestObject{StudentId: student, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(competitionapi.RestoreStudentData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("competition restoreStudentData: unexpected response %T", resp)
	}
	return result(string(v.Status), nil), nil
}

// CompetitionAnonymize adapts anonymizeUserData of the competition facade (F016 US5, F100 T042).
type CompetitionAnonymize struct{ F CompetitionAnonymizer }

var _ port.UserDataAnonymizer = CompetitionAnonymize{}

// Name implements port.UserDataAnonymizer.
func (CompetitionAnonymize) Name() string { return "competition" }

// Anonymize implements port.UserDataAnonymizer.
func (c CompetitionAnonymize) Anonymize(ctx context.Context, user, erasure uuid.UUID, reason string) (port.StudentDataResult, error) {
	body := competitionapi.AnonymizeUserDataJSONRequestBody{ErasureRequestId: erasure,
		Reason: competitionapi.CommonInternalCommonV1UserAnonymizeRequestReason(reason)}
	resp, err := modcall.Call(ctx, op("competition", "anonymizeUserData", competitionapi.AllowedCallers, competitionapi.ContextRequired),
		func(ctx context.Context) (competitionapi.AnonymizeUserDataResponseObject, error) {
			return c.F.AnonymizeUserData(ctx, competitionapi.AnonymizeUserDataRequestObject{UserId: user, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(competitionapi.AnonymizeUserData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("competition anonymizeUserData: unexpected response %T", resp)
	}
	return result(string(v.Status), v.BlockedReason), nil
}
