package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Admin Portal login through the backend (ADR-0091, F012 US7/US8). Passwords, OTP codes, refresh tokens,
// setup tickets and reset tokens cross these ports as []byte or opaque strings and are never logged,
// traced or audited (AGENTS R14, F012-AC-31).

// Errors of the portal identity provider (Keycloak direct grant of the confidential client eduworld-portal).
// Any other error is transient or a misconfiguration (timeout, 5xx, open circuit, invalid_client) and is
// answered 503 UNAVAILABLE — never as a credential error.
var (
	// ErrPortalInvalidCredentials: unknown username, wrong password, wrong or missing OTP, account disabled or
	// temporarily locked by brute-force detection (Keycloak answers all of them alike).
	ErrPortalInvalidCredentials = errors.New("portal idp: invalid credentials")
	// ErrPortalAccessDenied: the direct grant flow refused the account after the password step ("Allow access
	// only for role": student accounts) — F004 research R5 (d).
	ErrPortalAccessDenied = errors.New("portal idp: access denied by the flow")
	// ErrPortalSetupRequired: correct password but required actions remain ("Account is not fully set up").
	ErrPortalSetupRequired = errors.New("portal idp: account not fully set up")
	// ErrPortalRefreshInvalid: the refresh token is expired, revoked, rotated already or malformed.
	ErrPortalRefreshInvalid = errors.New("portal idp: refresh token invalid")
	// ErrKCPasswordPolicy: the Admin API refused a password because of the realm password policy.
	ErrKCPasswordPolicy = errors.New("keycloak: password policy violation")
)

// PortalTokens is the result of a direct grant or a refresh. The refresh token only ever travels to the
// HttpOnly cookie; the access token only to the response body (ADR-0091 §4).
type PortalTokens struct {
	AccessToken      string
	RefreshToken     string
	ExpiresIn        int // seconds
	RefreshExpiresIn int // seconds (idle of the client session, capped by its maximum)
	// Claims of the access token received from the token endpoint (server-to-server over TLS).
	Issuer    string
	Subject   string // Keycloak user id
	SessionID string // sid
	AMR       []string
	IssuedAt  time.Time
}

// PortalIdP is the token client of eduworld-portal (Keycloak token/logout endpoints). Every call has a
// timeout and a circuit breaker; only Logout (idempotent) is retried (AGENTS R29).
type PortalIdP interface {
	// PasswordGrant runs the direct grant with the password and, when otp is not empty, the OTP code.
	PasswordGrant(ctx context.Context, username string, password, otp []byte) (PortalTokens, error)
	// Refresh rotates the refresh token.
	Refresh(ctx context.Context, refreshToken string) (PortalTokens, error)
	// Logout ends the Keycloak session of the refresh token (sid); an already ended session is success.
	Logout(ctx context.Context, refreshToken string) error
	// SessionKey returns a stable, non-secret key of the session of a refresh token (hash of its sid, or of
	// the token when it cannot be read) for rate limiting; it never validates the token.
	SessionKey(refreshToken string) string
}

// PortalAccountAdmin is the subset of the Keycloak Admin API (client svc-identity) used by the portal
// account setup and password reset (ADR-0091 §3).
type PortalAccountAdmin interface {
	// FindUserByLogin finds the Keycloak user by exact username, or by exact email when login contains "@".
	FindUserByLogin(ctx context.Context, login string) (KCUser, error)
	// RequiredActions returns the pending required actions of the user (UPDATE_PASSWORD, CONFIGURE_TOTP…).
	RequiredActions(ctx context.Context, userID string) ([]string, error)
	// ResetPassword sets a permanent password (realm policy applies → ErrKCPasswordPolicy).
	ResetPassword(ctx context.Context, userID string, password []byte) error
	// RemoveRequiredAction removes one required action (no-op when absent).
	RemoveRequiredAction(ctx context.Context, userID, action string) error
	// ExecuteActionsEmail sends the Keycloak action-token email for the actions (CONFIGURE_TOTP).
	ExecuteActionsEmail(ctx context.Context, userID string, actions ...string) error
	// LogoutUser revokes every Keycloak session of the user.
	LogoutUser(ctx context.Context, userID string) error
}

// Keycloak required actions used by the portal setup.
const (
	KCActionUpdatePassword = "UPDATE_PASSWORD"
	KCActionConfigureTOTP  = "CONFIGURE_TOTP"
)

// ErrSetupTicketInvalid: the setup ticket is unknown, used or expired.
var ErrSetupTicketInvalid = errors.New("portal: setup ticket invalid")

// SetupTicket is the one-time state behind a setup_ticket (Valkey, TTL portal_setup_ticket_ttl). It never
// holds a password.
type SetupTicket struct {
	UserID       uuid.UUID `json:"user_id"`
	KCUserID     string    `json:"kc_user_id"`
	Steps        []string  `json:"required_steps"`
	OTPEmailSent bool      `json:"otp_email_sent"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SetupTicketStore keeps setup tickets by the SHA-256 of the ticket (ew:identity:portal_setup:<hash>).
type SetupTicketStore interface {
	// Save stores t under the ticket hash until t.ExpiresAt.
	Save(ctx context.Context, ticketHash string, t SetupTicket) error
	// Take reads and deletes the ticket atomically (GETDEL); ErrSetupTicketInvalid when absent.
	Take(ctx context.Context, ticketHash string) (SetupTicket, error)
	// MarkOTPEmail reserves the CONFIGURE_TOTP email of the user for window; false when already sent in it.
	MarkOTPEmail(ctx context.Context, userID uuid.UUID, window time.Duration) (bool, error)
	// UnmarkOTPEmail releases the reservation after a failed send.
	UnmarkOTPEmail(ctx context.Context, userID uuid.UUID) error
}

// PortalSession is what module identity remembers of a portal session (no token): the time the password
// was entered, so refreshes of platform staff stop after EW_OIDC_PLATFORM_MAX_AUTH_AGE (direct grant
// tokens carry no auth_time — F004 research R5).
type PortalSession struct {
	UserID   uuid.UUID `json:"user_id"`
	AuthTime time.Time `json:"auth_time"`
	MFA      bool      `json:"mfa"`
}

// PortalSessionStore keeps PortalSession by sid (ew:identity:portal_session:<hash(sid)>).
type PortalSessionStore interface {
	Save(ctx context.Context, sid string, s PortalSession, ttl time.Duration) error
	// Get returns found=false when absent (expired or lost: treated as unknown auth_time).
	Get(ctx context.Context, sid string) (s PortalSession, found bool, err error)
	Delete(ctx context.Context, sid string) error
}

// RateLimiter is a fixed-window counter (Valkey). retryAfter is set when !allowed.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}

// PasswordResetMail is the only content of the reset email: the link (ADR-0091 §3).
type PasswordResetMail struct {
	To     string
	Locale string
	Link   string
}

// Mailer sends the business emails of module identity (EW_SMTP_*).
type Mailer interface {
	SendPasswordReset(ctx context.Context, m PasswordResetMail) error
}

// Results of eduworld_portal_login_total.
const (
	LoginSuccess            = "success"
	LoginInvalidCredentials = "invalid_credentials"
	LoginStaffOnly          = "staff_only"
	LoginDeactivated        = "deactivated"
	LoginSetupRequired      = "setup_required"
	LoginRateLimited        = "rate_limited"
	LoginUnavailable        = "unavailable"
)

// PortalAuthMetrics records eduworld_portal_login_total{result} and eduworld_portal_csrf_rejected_total.
type PortalAuthMetrics interface {
	Login(result string)
	CSRFRejected(operation string)
}
