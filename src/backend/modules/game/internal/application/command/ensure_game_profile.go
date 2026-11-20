package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
)

// EnsureGameProfile is the step of createGameSession (F014) that creates the game profile of a student on the first
// session — no avatar, no nickname, so getGameProfile answers setup_required = true (F070-AC-01) — and records
// last_game_login_at on later sessions. It runs inside the transaction of its caller (never its own), so a new
// student has a profile before the client asks for it.
func EnsureGameProfile(ctx context.Context, repo write.ProfileRepository, tenant, student, school uuid.UUID, now time.Time) error {
	return repo.Ensure(ctx, profile.New(tenant, student, school, now), &now)
}
