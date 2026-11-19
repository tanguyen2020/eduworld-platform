package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LastLoginWriter records the last game login of a user (identity.users.last_login_at, F014-AC-10): written by the
// consumer of student.game_logged_in only (DATABASE_DESIGN §3.12, ADR-0031, ADR-0032 cleanup input).
type LastLoginWriter interface {
	// RecordGameLogin moves last_login_at forward to at (never back); false when it already was at or after at.
	RecordGameLogin(ctx context.Context, user uuid.UUID, at time.Time) (bool, error)
}
