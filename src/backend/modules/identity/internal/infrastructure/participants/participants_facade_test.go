package participants

import (
	"context"
	"testing"

	"github.com/google/uuid"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func sagaCtx(t *testing.T) context.Context {
	t.Helper()
	sc, err := authz.EventScope(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	return modcall.WithIdempotencyKey(tenancy.WithScope(context.Background(), sc), uuid.Must(uuid.NewV7()).String())
}

// Caller-side facade test of the F016 sagas against the generated fakes (TEST DOUBLES gameapi.Fake,
// learningapi.Fake): request mapping, blocked reason, restore reason, required context, caller identity.
func TestParticipantsFacade(t *testing.T) {
	student, run := uuid.New(), uuid.New()
	var gotReason string
	game := &gameapi.Fake{
		CleanupStudentDataFunc: func(ctx context.Context, r gameapi.CleanupStudentDataRequestObject) (gameapi.CleanupStudentDataResponseObject, error) {
			if modcall.CallerFrom(ctx) != "identity" || r.StudentId != student || r.Body.CleanupRunId != run || r.Body.Reason != "inactive_cleanup" {
				t.Fatalf("cleanup request: %s %+v", modcall.CallerFrom(ctx), r.Body)
			}
			b := gameapi.CommonInternalCommonV1StudentLifecycleResultBlockedReason("active_game_session")
			return gameapi.CleanupStudentData200JSONResponse{StudentId: student, Status: "blocked", BlockedReason: &b}, nil
		},
		RestoreStudentDataFunc: func(_ context.Context, r gameapi.RestoreStudentDataRequestObject) (gameapi.RestoreStudentDataResponseObject, error) {
			gotReason = string(r.Body.Reason)
			return gameapi.RestoreStudentData200JSONResponse{StudentId: student, Status: "done"}, nil
		},
	}
	learning := &learningapi.Fake{
		CleanupStudentDataFunc: func(_ context.Context, _ learningapi.CleanupStudentDataRequestObject) (learningapi.CleanupStudentDataResponseObject, error) {
			return learningapi.CleanupStudentData200JSONResponse{StudentId: student, Status: "nothing_to_do"}, nil
		},
		RestoreStudentDataFunc: func(_ context.Context, r learningapi.RestoreStudentDataRequestObject) (learningapi.RestoreStudentDataResponseObject, error) {
			gotReason = string(r.Body.Reason)
			return learningapi.RestoreStudentData200JSONResponse{StudentId: student, Status: "nothing_to_do"}, nil
		},
	}
	ctx := sagaCtx(t)

	r, err := Game{F: game}.Cleanup(ctx, student, run, nil)
	if err != nil || r.Status != "blocked" || r.BlockedReason != "active_game_session" {
		t.Fatalf("game cleanup: %+v %v", r, err)
	}
	if r, err := (Learning{F: learning}).Cleanup(ctx, student, run, nil); err != nil || r.Status != "nothing_to_do" {
		t.Fatalf("learning cleanup: %+v %v", r, err)
	}
	if r, err := (Game{F: game}).Restore(ctx, student, true); err != nil || r.Status != "done" || gotReason != "student_logged_in_again" {
		t.Fatalf("game restore: %+v %v %s", r, err, gotReason)
	}
	if _, err := (Learning{F: learning}).Restore(ctx, student, false); err != nil || gotReason != "admin_restore" {
		t.Fatalf("learning restore: %v %s", err, gotReason)
	}

	// cleanupStudentData requires the tenant scope and the step Idempotency-Key (x-context-required).
	if _, err := (Game{F: game}).Cleanup(context.Background(), student, run, nil); problem.From(err, "").Code != problem.Internal {
		t.Fatalf("missing context: %v", err)
	}
	// A facade error reaches the saga unchanged (the engine decides retry or compensation).
	failing := &gameapi.Fake{CleanupStudentDataFunc: func(context.Context, gameapi.CleanupStudentDataRequestObject) (gameapi.CleanupStudentDataResponseObject, error) {
		return nil, problem.New(problem.Unavailable, "")
	}}
	if _, err := (Game{F: failing}).Cleanup(ctx, student, run, nil); problem.From(err, "").Code != problem.Unavailable {
		t.Fatalf("facade error: %v", err)
	}
	// Only module identity may call the lifecycle operations of game and learning.
	for _, ops := range []map[string][]string{gameapi.AllowedCallers, learningapi.AllowedCallers} {
		for _, o := range []string{"cleanupStudentData", "restoreStudentData"} {
			if len(ops[o]) != 1 || ops[o][0] != "identity" {
				t.Fatalf("x-allowed-callers %s: %v", o, ops[o])
			}
		}
	}
}
