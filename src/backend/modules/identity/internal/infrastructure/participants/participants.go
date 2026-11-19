// Package participants adapts the facades of modules game and learning to port.StudentDataParticipant
// (F016 T001): every call goes through pkg/modcall (span, saga deadline, x-allowed-callers, tenant scope and
// Idempotency-Key from ctx, AGENTS R29); retries belong to the saga engine, never to the adapter.
package participants

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// GameFacade is the subset of the game facade used by the sagas.
type GameFacade interface {
	CleanupStudentData(ctx context.Context, req gameapi.CleanupStudentDataRequestObject) (gameapi.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req gameapi.RestoreStudentDataRequestObject) (gameapi.RestoreStudentDataResponseObject, error)
}

// LearningFacade is the subset of the learning facade used by the sagas.
type LearningFacade interface {
	CleanupStudentData(ctx context.Context, req learningapi.CleanupStudentDataRequestObject) (learningapi.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req learningapi.RestoreStudentDataRequestObject) (learningapi.RestoreStudentDataResponseObject, error)
}

func op(callee, operation string, allowed, ctxReq map[string][]string) modcall.Op {
	return modcall.Op{Caller: "identity", Callee: callee, Operation: operation, Kind: modcall.Saga,
		AllowedCallers: allowed[operation], ContextRequired: ctxReq[operation]}
}

func restoreReason(viaLogin bool) string {
	if viaLogin {
		return "student_logged_in_again"
	}
	return "admin_restore"
}

func result(status string, reason *string) port.StudentDataResult {
	r := port.StudentDataResult{Status: status}
	if reason != nil {
		r.BlockedReason = *reason
	}
	return r
}

// Game adapts the game facade.
type Game struct{ F GameFacade }

var _ port.StudentDataParticipant = Game{}

// Name implements port.StudentDataParticipant.
func (Game) Name() string { return "game" }

// Cleanup implements port.StudentDataParticipant.
func (g Game) Cleanup(ctx context.Context, student, run uuid.UUID, lastLogin *time.Time) (port.StudentDataResult, error) {
	body := gameapi.CleanupStudentDataJSONRequestBody{CleanupRunId: run, Reason: "inactive_cleanup", LastLoginAt: lastLogin}
	resp, err := modcall.Call(ctx, op("game", "cleanupStudentData", gameapi.AllowedCallers, gameapi.ContextRequired),
		func(ctx context.Context) (gameapi.CleanupStudentDataResponseObject, error) {
			return g.F.CleanupStudentData(ctx, gameapi.CleanupStudentDataRequestObject{StudentId: student, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(gameapi.CleanupStudentData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("game cleanupStudentData: unexpected response %T", resp)
	}
	var reason *string
	if v.BlockedReason != nil {
		s := string(*v.BlockedReason)
		reason = &s
	}
	return result(string(v.Status), reason), nil
}

// Restore implements port.StudentDataParticipant.
func (g Game) Restore(ctx context.Context, student uuid.UUID, viaLogin bool) (port.StudentDataResult, error) {
	body := gameapi.RestoreStudentDataJSONRequestBody{Reason: gameapi.CommonInternalCommonV1StudentRestoreRequestReason(restoreReason(viaLogin))}
	resp, err := modcall.Call(ctx, op("game", "restoreStudentData", gameapi.AllowedCallers, gameapi.ContextRequired),
		func(ctx context.Context) (gameapi.RestoreStudentDataResponseObject, error) {
			return g.F.RestoreStudentData(ctx, gameapi.RestoreStudentDataRequestObject{StudentId: student, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(gameapi.RestoreStudentData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("game restoreStudentData: unexpected response %T", resp)
	}
	return result(string(v.Status), nil), nil
}

// Learning adapts the learning facade.
type Learning struct{ F LearningFacade }

var _ port.StudentDataParticipant = Learning{}

// Name implements port.StudentDataParticipant.
func (Learning) Name() string { return "learning" }

// Cleanup implements port.StudentDataParticipant.
func (l Learning) Cleanup(ctx context.Context, student, run uuid.UUID, lastLogin *time.Time) (port.StudentDataResult, error) {
	body := learningapi.CleanupStudentDataJSONRequestBody{CleanupRunId: run, Reason: "inactive_cleanup", LastLoginAt: lastLogin}
	resp, err := modcall.Call(ctx, op("learning", "cleanupStudentData", learningapi.AllowedCallers, learningapi.ContextRequired),
		func(ctx context.Context) (learningapi.CleanupStudentDataResponseObject, error) {
			return l.F.CleanupStudentData(ctx, learningapi.CleanupStudentDataRequestObject{StudentId: student, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(learningapi.CleanupStudentData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("learning cleanupStudentData: unexpected response %T", resp)
	}
	var reason *string
	if v.BlockedReason != nil {
		s := string(*v.BlockedReason)
		reason = &s
	}
	return result(string(v.Status), reason), nil
}

// Restore implements port.StudentDataParticipant.
func (l Learning) Restore(ctx context.Context, student uuid.UUID, viaLogin bool) (port.StudentDataResult, error) {
	body := learningapi.RestoreStudentDataJSONRequestBody{Reason: learningapi.CommonInternalCommonV1StudentRestoreRequestReason(restoreReason(viaLogin))}
	resp, err := modcall.Call(ctx, op("learning", "restoreStudentData", learningapi.AllowedCallers, learningapi.ContextRequired),
		func(ctx context.Context) (learningapi.RestoreStudentDataResponseObject, error) {
			return l.F.RestoreStudentData(ctx, learningapi.RestoreStudentDataRequestObject{StudentId: student, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(learningapi.RestoreStudentData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("learning restoreStudentData: unexpected response %T", resp)
	}
	return result(string(v.Status), nil), nil
}

// GameAnonymizer is the subset of the game facade used by the user_anonymize saga.
type GameAnonymizer interface {
	AnonymizeUserData(ctx context.Context, req gameapi.AnonymizeUserDataRequestObject) (gameapi.AnonymizeUserDataResponseObject, error)
}

// LearningAnonymizer is the subset of the learning facade used by the user_anonymize saga.
type LearningAnonymizer interface {
	AnonymizeUserData(ctx context.Context, req learningapi.AnonymizeUserDataRequestObject) (learningapi.AnonymizeUserDataResponseObject, error)
}

// GameAnonymize adapts anonymizeUserData of the game facade (F016 US5).
type GameAnonymize struct{ F GameAnonymizer }

var _ port.UserDataAnonymizer = GameAnonymize{}

// Name implements port.UserDataAnonymizer.
func (GameAnonymize) Name() string { return "game" }

// Anonymize implements port.UserDataAnonymizer.
func (g GameAnonymize) Anonymize(ctx context.Context, user, erasure uuid.UUID, reason string) (port.StudentDataResult, error) {
	body := gameapi.AnonymizeUserDataJSONRequestBody{ErasureRequestId: erasure, Reason: gameapi.CommonInternalCommonV1UserAnonymizeRequestReason(reason)}
	resp, err := modcall.Call(ctx, op("game", "anonymizeUserData", gameapi.AllowedCallers, gameapi.ContextRequired),
		func(ctx context.Context) (gameapi.AnonymizeUserDataResponseObject, error) {
			return g.F.AnonymizeUserData(ctx, gameapi.AnonymizeUserDataRequestObject{UserId: user, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(gameapi.AnonymizeUserData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("game anonymizeUserData: unexpected response %T", resp)
	}
	return result(string(v.Status), v.BlockedReason), nil
}

// LearningAnonymize adapts anonymizeUserData of the learning facade (F016 US5).
type LearningAnonymize struct{ F LearningAnonymizer }

var _ port.UserDataAnonymizer = LearningAnonymize{}

// Name implements port.UserDataAnonymizer.
func (LearningAnonymize) Name() string { return "learning" }

// Anonymize implements port.UserDataAnonymizer.
func (l LearningAnonymize) Anonymize(ctx context.Context, user, erasure uuid.UUID, reason string) (port.StudentDataResult, error) {
	body := learningapi.AnonymizeUserDataJSONRequestBody{ErasureRequestId: erasure, Reason: learningapi.CommonInternalCommonV1UserAnonymizeRequestReason(reason)}
	resp, err := modcall.Call(ctx, op("learning", "anonymizeUserData", learningapi.AllowedCallers, learningapi.ContextRequired),
		func(ctx context.Context) (learningapi.AnonymizeUserDataResponseObject, error) {
			return l.F.AnonymizeUserData(ctx, learningapi.AnonymizeUserDataRequestObject{UserId: user, Body: &body})
		})
	if err != nil {
		return port.StudentDataResult{}, err
	}
	v, ok := resp.(learningapi.AnonymizeUserData200JSONResponse)
	if !ok {
		return port.StudentDataResult{}, fmt.Errorf("learning anonymizeUserData: unexpected response %T", resp)
	}
	return result(string(v.Status), v.BlockedReason), nil
}
