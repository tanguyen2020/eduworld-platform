package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
)

// LastLoginWriter records the last game login of a user (identity.users.last_login_at, F014-AC-10): written by the
// consumer of student.game_logged_in only (DATABASE_DESIGN §3.12, ADR-0031, ADR-0032 cleanup input).
type LastLoginWriter interface {
	// RecordGameLogin moves last_login_at forward to at (never back); false when it already was at or after at.
	RecordGameLogin(ctx context.Context, user uuid.UUID, at time.Time) (bool, error)
}

// PlayTimeWriter adds played time to identity.daily_play_time (OQ-17, F013-AC-33): written by the consumer of
// game_session.ended only, under the TenantScope of the event.
type PlayTimeWriter interface {
	// AddPlayTime adds played to the time of user in tenant on day (capped at one day).
	AddPlayTime(ctx context.Context, tenant, user uuid.UUID, day settings.PlayDate, played time.Duration) error
}

// ChatDirectPolicyWriter stores the projection of the direct_message_enabled of a chat policy level (delta F112-7):
// written by the consumer identity.chat_direct_policy only (platform level under the platform scope of the event,
// the other levels under its TenantScope). A level is replaced only by a newer source version; applied reports a write.
type ChatDirectPolicyWriter interface {
	SetPlatformChatDirect(ctx context.Context, enabled bool, version int64) (applied bool, err error)
	SetChatDirect(ctx context.Context, tenant uuid.UUID, level string, scopeID uuid.UUID, enabled bool, version int64) (applied bool, err error)
}
