package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PortalAccount is the EduWorld side of a portal login (ADR-0091): identity.users plus whether the user
// holds a valid UserRole in an active tenant or a platform role (F012-AC-27).
type PortalAccount struct {
	UserID        uuid.UUID
	UserType      string
	Status        string
	Email         string // empty when unknown
	Locale        string
	HasPortalRole bool
}

// PortalAccountRepository reads accounts for the portal login, setup and password reset. It reads across
// tenants (user roles of every tenant and platform roles) and must run in a platform scope; every statement
// carries its own deleted_at and validity conditions (AGENTS R27).
type PortalAccountRepository interface {
	// BySubject returns the account of the active external identity (issuer, subject); nil when none.
	BySubject(ctx context.Context, issuer, subject string) (*PortalAccount, error)
	// ByKeycloakID is BySubject over the trusted issuers (EW_OIDC_ISSUERS); nil when none.
	ByKeycloakID(ctx context.Context, issuers []string, subject string) (*PortalAccount, error)
	// ByID returns a live account; nil when none.
	ByID(ctx context.Context, userID uuid.UUID) (*PortalAccount, error)
	// ActiveStaffByLogin returns the active staff account whose username or email equals login (normalized,
	// case-insensitive); nil when none or ambiguous. Students never match (F012-AC-36).
	ActiveStaffByLogin(ctx context.Context, login string) (*PortalAccount, error)
	// KeycloakSubject returns the subject of the user's active identity at one of issuers ("" when none).
	KeycloakSubject(ctx context.Context, userID uuid.UUID, issuers []string) (string, error)
}

// PortalResetToken is a password reset token of identity.portal_password_reset_tokens (hash only).
type PortalResetToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  string // SHA-256 hex
	Locale     string
	ExpiresAt  time.Time
	DeviceHash string
}

// PortalResetTokenRepository persists reset tokens (G table, soft-delete only).
type PortalResetTokenRepository interface {
	// Issue soft-deletes the unused tokens of the user (delete_reason superseded) and inserts t; it returns
	// how many were superseded.
	Issue(ctx context.Context, t PortalResetToken) (superseded int, err error)
	// Use marks the token used atomically (used_at IS NULL, expires_at > now, live) and returns its user;
	// ok=false when the token is unknown, used, expired or superseded (F012-AC-37).
	Use(ctx context.Context, tokenHash string, now time.Time) (userID uuid.UUID, ok bool, err error)
}
