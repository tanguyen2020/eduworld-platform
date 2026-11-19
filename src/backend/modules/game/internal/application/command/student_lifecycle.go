package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Results of the student lifecycle facade operations (StudentLifecycleResult.status).
const (
	LifecycleDone        = "done"
	LifecycleBlocked     = "blocked"
	LifecycleNothingToDo = "nothing_to_do"
)

// StudentLifecycle implements the game part of the identity sagas (F016 T024, ADR-0032): a student with an
// active game session is not cleaned up (blocked: active_game_session). Module game keeps no other
// student-owned data yet (the game profile, inventory and quests arrive with F070+), so a cleanup or a restore
// otherwise has nothing to do; sessions and login events are history and stay.
type StudentLifecycle struct {
	UoW      write.UnitOfWork
	Sessions write.SessionRepository
}

// Cleanup answers cleanupStudentData for the tenant of the caller's scope.
func (h StudentLifecycle) Cleanup(ctx context.Context, studentID uuid.UUID) (status, blockedReason string, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CleanupStudentData")
	defer func() { done(err) }()
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return "", "", problem.New(problem.Internal, "cleanupStudentData needs a tenant scope")
	}
	tenant, single := s.SessionTenantID()
	if !single {
		return "", "", problem.New(problem.Internal, "cleanupStudentData needs one tenant")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		active, err := h.Sessions.Active(ctx, tenant, studentID)
		if err != nil {
			return err
		}
		if active != nil {
			status, blockedReason = LifecycleBlocked, "active_game_session"
			return nil
		}
		status = LifecycleNothingToDo
		return nil
	})
	return status, blockedReason, err
}

// Restore answers restoreStudentData: nothing of module game was removed by the cleanup.
func (h StudentLifecycle) Restore(ctx context.Context, _ uuid.UUID) (string, error) {
	_, done := observability.UseCase(ctx, "game.command.RestoreStudentData")
	done(nil)
	return LifecycleNothingToDo, nil
}
