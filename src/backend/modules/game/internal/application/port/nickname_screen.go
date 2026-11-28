package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNicknameScreenUnavailable: the moderation (module chat) or the personal-data screen (module identity) could not
// decide — error, 503 or nickname_moderation_deadline exceeded. The typed nickname is refused with 503 UNAVAILABLE and
// nothing is stored: never treated as allowed (fail-closed, F136-AC-14).
var ErrNicknameScreenUnavailable = errors.New("game: nickname screening unavailable")

// NicknameModeration is moderateDisplayText of the chat facade with context nickname (F136-AC-11): the same
// normalisation, catalogue (platform + tenant), switches and always-on PII step as chat. Only allowed/blocked — never the
// matched term, its position or category (R15). Any failure is ErrNicknameScreenUnavailable.
type NicknameModeration interface {
	CheckNickname(ctx context.Context, tenant, student uuid.UUID, text string) (allowed bool, err error)
}

// NicknamePersonalDataScreen is screenNicknamePersonalData of the identity facade (F136-AC-12): blocked when the nickname
// holds the real name or the school name of the student. Only allowed/blocked — module game never receives the name
// (R13, R16). Any failure is ErrNicknameScreenUnavailable.
type NicknamePersonalDataScreen interface {
	ScreenNickname(ctx context.Context, tenant, student uuid.UUID, text string) (allowed bool, err error)
}

// NicknameAttemptCounter counts the typed nickname submissions of a student over nickname_attempt_rate (Valkey
// ew:game:nick:attempts:<tenant_id>:<student_id>; numbers only, never the typed text — R14). A secondary protection
// against probing the catalogue (R9): no sanction, no warning (R19, F136-AC-15).
type NicknameAttemptCounter interface {
	// Hit counts one submission at now and reports whether it is within the limit; when it is not, retryAfter is the
	// wait until the oldest counted submission leaves the window.
	Hit(ctx context.Context, tenant, student uuid.UUID, now time.Time) (allowed bool, retryAfter time.Duration, err error)
}

// Results of eduworld_nickname_checks_total{result} (plan F136 "Quan sát"); no user or nickname label (R14).
const (
	NicknameAllowed     = "allowed"
	NicknameFormat      = "format"
	NicknameModerated   = "moderation"
	NicknamePersonal    = "personal"
	NicknameTaken       = "taken"
	NicknameRateLimited = "rate_limited"
	NicknameUnavailable = "unavailable"
)

// NicknameMetrics records the typed nickname checks (eduworld_nickname_checks_total{result}) and the facade latency.
type NicknameMetrics interface {
	NicknameCheck(result string)
	NicknameScreenLatency(facade string, d time.Duration)
}
