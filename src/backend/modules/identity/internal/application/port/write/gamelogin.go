package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
)

// GameAccount is the EduWorld side of a game login (identity.users + the active external identity).
type GameAccount struct {
	UserID             uuid.UUID
	UserType           string
	Status             string
	PrimaryTenantID    *uuid.UUID
	ExternalIdentityID uuid.UUID
}

// LoginEvent is one row of identity.login_events (no username, password, token or raw IP).
type LoginEvent struct {
	ID               uuid.UUID
	TenantID         *uuid.UUID
	UserID           *uuid.UUID
	OccurredAt       time.Time
	Client           string
	IPHash           []byte
	SisConnectionID  *uuid.UUID
	DeviceID         *uuid.UUID
	Result           string
	TenantResolution string
}

// GameLoginRepository reads and writes the accounts and journal of the game login (F013).
type GameLoginRepository interface {
	// AccountBySubject returns the account of the active external identity (issuer, subject); nil when none.
	// users and external_identities are global (G).
	AccountBySubject(ctx context.Context, issuer, subject string) (*GameAccount, error)
	// AccountByID returns a live account; nil when none.
	AccountByID(ctx context.Context, userID uuid.UUID) (*GameAccount, error)
	// MarkLogin sets external_identities.last_login_at and moves the user pending → active; activated is true
	// on that transition (the caller publishes user.activated in the same transaction, F013-AC-07).
	MarkLogin(ctx context.Context, a GameAccount, at time.Time) (activated bool, err error)
	// RecordLoginEvent appends a journal row: under the TenantScope of e.TenantID (identity_app, RLS), or
	// under a PlatformScope when e.TenantID is nil (identity_platform, ADR-0089 §2).
	RecordLoginEvent(ctx context.Context, e LoginEvent) error
}

// LoginMembershipReader reads the tenants of a user before any TenantScope exists (ADR-0089 §3): it calls
// identity.memberships_for_user inside tenancy.Bypass(ctx, auth_login) with identity_app and joins the
// tenant rows. Only the login paths (/auth/game/*, /auth/session, login-credentials:verify) may use it.
type LoginMembershipReader interface {
	LoginMemberships(ctx context.Context, userID uuid.UUID) ([]gamelogin.Membership, error)
}

// SessionFilter selects live game sessions to revoke (at least UserID).
type SessionFilter struct {
	UserID      uuid.UUID
	TenantID    *uuid.UUID
	DeviceID    *uuid.UUID
	KeycloakSID string
}

// GameAuthSessionRepository persists the GameAuthSession aggregate (identity.game_auth_sessions). Every
// method runs in the platform scope "game-auth" (the presented token selects the row; its tenant is
// compared with X-Tenant-Id by the use case) with explicit deleted_at conditions (AGENTS R27).
type GameAuthSessionRepository interface {
	Create(ctx context.Context, s *gamelogin.Session) error
	// ByTokenHash locks (FOR UPDATE) the session whose current or rotated hashes contain hash; nil when none.
	ByTokenHash(ctx context.Context, hash string) (*gamelogin.Session, error)
	// BySID locks the live session of a Keycloak session id; nil when none.
	BySID(ctx context.Context, sid string) (*gamelogin.Session, error)
	// Live locks the unrevoked sessions matching f.
	Live(ctx context.Context, f SessionFilter) ([]*gamelogin.Session, error)
	// Save writes rotation or revocation (optimistic version check; 409 CONFLICT when stale).
	Save(ctx context.Context, s *gamelogin.Session) error
}
