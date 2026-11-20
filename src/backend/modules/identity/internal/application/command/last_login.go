package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
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

// RecordPlayTime applies game_session.ended of module game to identity.daily_play_time (OQ-17, F013-AC-33): the
// session duration is split over the days (Location, ICT) it covers, so the game login can refuse a student who
// reached the daily play limit of the school without calling module game (R30). It runs inside the transaction of
// the consumer identity.daily_play_time (TenantScope of the event; idempotency per event_id by pkg/outbox).
type RecordPlayTime struct {
	PlayTime write.PlayTimeWriter
	Location *time.Location
}

// Handle adds a session of user in tenant that lasted duration and ended at endedAt.
func (h RecordPlayTime) Handle(ctx context.Context, tenant, user uuid.UUID, endedAt time.Time, duration time.Duration) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RecordPlayTime")
	defer func() { done(err) }()
	if tenant == uuid.Nil || user == uuid.Nil || endedAt.IsZero() || duration < 0 {
		return problem.New(problem.Internal, "game session end without tenant, user, time or duration")
	}
	loc := h.Location
	if loc == nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	for day, played := range settings.SplitByDay(endedAt, duration, loc) {
		if err := h.PlayTime.AddPlayTime(ctx, tenant, user, day, played); err != nil {
			return err
		}
	}
	return nil
}
