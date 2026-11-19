package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// RecordGameLogin applies student.game_logged_in of module game to identity.users.last_login_at (F014-AC-10, T025):
// asynchronous by design (two databases, ADR-0037), idempotent and never moving the time back. It runs inside the
// transaction of the consumer identity.last_login_on_game_logged_in.
type RecordGameLogin struct {
	Users write.LastLoginWriter
}

// Handle records the login of user at.
func (h RecordGameLogin) Handle(ctx context.Context, user uuid.UUID, at time.Time) (applied bool, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RecordGameLogin")
	defer func() { done(err) }()
	if user == uuid.Nil || at.IsZero() {
		return false, problem.New(problem.Internal, "game login without user or time")
	}
	return h.Users.RecordGameLogin(ctx, user, at.UTC())
}
