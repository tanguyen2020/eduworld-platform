package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
)

// Game login through the backend (F013, ADR-0068, ADR-0086). The token client of the confidential client
// eduworld-game-backend has the PortalIdP shape (password grant without OTP, refresh with rotation, logout);
// passwords cross it as []byte and are never logged, traced or audited (F013-AC-16).

// KeycloakSessionAdmin ends one Keycloak user session by its id (Admin API DELETE /sessions/{sid}, client
// svc-identity): revocation of a game session for reuse, takeover, password change, lock or cleanup.
// An already ended session is success.
type KeycloakSessionAdmin interface {
	DeleteSession(ctx context.Context, sid string) error
}

// ErrLoginTicketInvalid: the login ticket is unknown, used or expired.
var ErrLoginTicketInvalid = errors.New("game login: login ticket invalid")

// LoginTicket is the one-time state behind a login_ticket (Valkey, TTL game_login_ticket_ttl, research R1):
// the authenticated user, the device it is bound to, the tenants offered and the encrypted token set.
type LoginTicket struct {
	UserID             uuid.UUID   `json:"user_id"`
	ExternalIdentityID uuid.UUID   `json:"external_identity_id"`
	DeviceID           uuid.UUID   `json:"device_id"`
	Platform           string      `json:"platform"`
	ClientVersion      string      `json:"client_version"`
	Choices            []uuid.UUID `json:"choices"`
	Tokens             PortalTokens
	ExpiresAt          time.Time `json:"expires_at"`
}

// LoginTicketStore keeps tickets by the SHA-256 of the ticket (ew:identity:login_ticket:<hash>). The token
// set is encrypted (AES-GCM) before it reaches Valkey; Take is GETDEL (one use).
type LoginTicketStore interface {
	Save(ctx context.Context, ticketHash string, t LoginTicket) error
	Take(ctx context.Context, ticketHash string) (LoginTicket, error)
}

// CurfewSource returns the effective curfew of a student (platform_curfew united with the curfew window of
// the school, ADR-0078). F015 owns the school settings; until then the platform value applies.
type CurfewSource interface {
	EffectiveCurfew(ctx context.Context, tenantID uuid.UUID, schoolID *uuid.UUID) (gamelogin.Curfew, error)
}

// Results of eduworld_auth_game_login_total and identity.login_events.result.
const (
	GameLoginSuccess              = "success"
	GameLoginInvalidCredentials   = "invalid_credentials"
	GameLoginNotProvisioned       = "account_not_provisioned"
	GameLoginStudentOnly          = "game_access_student_only"
	GameLoginLocked               = "account_locked"
	GameLoginDeactivated          = "account_deactivated"
	GameLoginNoActiveEnrollment   = "no_active_enrollment"
	GameLoginTenantNotSelected    = "tenant_not_selected"
	GameLoginTenantMismatch       = "tenant_mismatch"
	GameLoginRateLimited          = "rate_limited"
	GameLoginQueueBusy            = "queue_busy"
	GameLoginVersionUnsupported   = "client_version_unsupported"
	GameLoginCurfewActive         = "curfew_active"
	GameLoginUnavailable          = "unavailable"
	GameLoginTicketInvalidOutcome = "login_ticket_invalid" // metric only (not a journal result)
)

// GameAuthMetrics records the metrics of the game login (MONITORING, plan F013 "Quan sát").
type GameAuthMetrics interface {
	Login(result string, d time.Duration)
	Refresh(mode, result string)
	Revoked(reason string)
	QueueRejected()
	RateLimited(key string)
}
