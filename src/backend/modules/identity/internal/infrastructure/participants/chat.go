package participants

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// ChatFacade is the subset of the chat facade used by the student lifecycle sagas (F115 T042, OQ F016-2): the student
// leaves its channels (blocked while a moderation case of the student is open); a restore brings the persistent
// memberships back.
type ChatFacade interface {
	CleanupStudentData(ctx context.Context, req chatapi.CleanupStudentDataRequestObject) (chatapi.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req chatapi.RestoreStudentDataRequestObject) (chatapi.RestoreStudentDataResponseObject, error)
}

// ChatAnonymizer is the subset of the chat facade used by the user_anonymize saga.
type ChatAnonymizer interface {
	AnonymizeUserData(ctx context.Context, req chatapi.AnonymizeUserDataRequestObject) (chatapi.AnonymizeUserDataResponseObject, error)
}

// Chat adapts the chat facade.
type Chat struct{ F ChatFacade }

var _ port.StudentDataParticipant = Chat{}

// Name implements port.StudentDataParticipant.
func (Chat) Name() string { return "chat" }

// Cleanup implements port.StudentDataParticipant.
func (c Chat) Cleanup(ctx context.Context, student, run uuid.UUID, lastLogin *time.Time) (port.StudentDataResult, error) {
	body := chatapi.CleanupStudentDataJSONRequestBody{CleanupRunId: run, Reason: "inactive_cleanup", LastLoginAt: lastLogin}
	resp, err := modcall.Call(ctx, op("chat", "cleanupStudentData", chatapi.AllowedCallers, chatapi.ContextRequired),
		func(ctx context.Context) (chatapi.CleanupStudentDataResponseObject, error) {
			return c.F.CleanupStudentData(ctx, chatapi.CleanupStudentDataRequestObject{StudentId: student, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(chatapi.CleanupStudentData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("chat cleanupStudentData: unexpected response %T", resp)
	}
	var reason *string
	if v.BlockedReason != nil {
		s := string(*v.BlockedReason)
		reason = &s
	}
	return result(string(v.Status), reason), nil
}

// Restore implements port.StudentDataParticipant.
func (c Chat) Restore(ctx context.Context, student uuid.UUID, viaLogin bool) (port.StudentDataResult, error) {
	body := chatapi.RestoreStudentDataJSONRequestBody{Reason: chatapi.CommonInternalCommonV1StudentRestoreRequestReason(restoreReason(viaLogin))}
	resp, err := modcall.Call(ctx, op("chat", "restoreStudentData", chatapi.AllowedCallers, chatapi.ContextRequired),
		func(ctx context.Context) (chatapi.RestoreStudentDataResponseObject, error) {
			return c.F.RestoreStudentData(ctx, chatapi.RestoreStudentDataRequestObject{StudentId: student, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(chatapi.RestoreStudentData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("chat restoreStudentData: unexpected response %T", resp)
	}
	return result(string(v.Status), nil), nil
}

// ChatAnonymize adapts anonymizeUserData of the chat facade (F016 US5, F115 T042).
type ChatAnonymize struct{ F ChatAnonymizer }

var _ port.UserDataAnonymizer = ChatAnonymize{}

// Name implements port.UserDataAnonymizer.
func (ChatAnonymize) Name() string { return "chat" }

// Anonymize implements port.UserDataAnonymizer.
func (c ChatAnonymize) Anonymize(ctx context.Context, user, erasure uuid.UUID, reason string) (port.StudentDataResult, error) {
	body := chatapi.AnonymizeUserDataJSONRequestBody{ErasureRequestId: erasure,
		Reason: chatapi.CommonInternalCommonV1UserAnonymizeRequestReason(reason)}
	resp, err := modcall.Call(ctx, op("chat", "anonymizeUserData", chatapi.AllowedCallers, chatapi.ContextRequired),
		func(ctx context.Context) (chatapi.AnonymizeUserDataResponseObject, error) {
			return c.F.AnonymizeUserData(ctx, chatapi.AnonymizeUserDataRequestObject{UserId: user, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(chatapi.AnonymizeUserData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("chat anonymizeUserData: unexpected response %T", resp)
	}
	return result(string(v.Status), v.BlockedReason), nil
}
