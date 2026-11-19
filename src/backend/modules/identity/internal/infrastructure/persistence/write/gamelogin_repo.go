package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// GameLoginRepo implements write.GameLoginRepository with explicit SQL (F013). users and
// external_identities are global (G); login_events is TA* (RLS: identity_app writes rows of its tenant,
// identity_platform rows without tenant).
type GameLoginRepo struct{}

var _ write.GameLoginRepository = GameLoginRepo{}

const gameAccountSelect = `SELECT u.user_id, u.user_type, u.status, u.primary_tenant_id, e.external_identity_id
	FROM identity.users u
	JOIN identity.external_identities e ON e.user_id = u.user_id AND e.status = 'active' AND e.deleted_at IS NULL`

func oneGameAccount(ctx context.Context, q string, args ...any) (*write.GameAccount, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("game account: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.GameAccount
	for rows.Next() {
		var a write.GameAccount
		var tenant uuid.NullUUID
		if err := rows.Scan(&a.UserID, &a.UserType, &a.Status, &tenant, &a.ExternalIdentityID); err != nil {
			return nil, fmt.Errorf("game account: %w", err)
		}
		if tenant.Valid {
			t := tenant.UUID
			a.PrimaryTenantID = &t
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("game account: %w", err)
	}
	if len(out) != 1 {
		return nil, nil
	}
	return &out[0], nil
}

// AccountBySubject implements write.GameLoginRepository.
func (GameLoginRepo) AccountBySubject(ctx context.Context, issuer, subject string) (*write.GameAccount, error) {
	// tenancyguard:ok users/external_identities are global (G); deleted_at filtered
	return oneGameAccount(ctx, gameAccountSelect+` WHERE e.issuer = $1 AND e.subject = $2 AND u.deleted_at IS NULL`, issuer, subject)
}

// CleanedUserBySubject implements write.GameLoginRepository.
func (GameLoginRepo) CleanedUserBySubject(ctx context.Context, issuer, subject string) (*uuid.UUID, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	// tenancyguard:ok users/external_identities are global (G); only a user removed by the inactive cleanup
	err = tx.QueryRowContext(ctx, `SELECT u.user_id FROM identity.external_identities e JOIN identity.users u ON u.user_id = e.user_id
		WHERE e.issuer = $1 AND e.subject = $2 AND e.deleted_at IS NULL AND u.deleted_at IS NOT NULL AND u.delete_reason = 'inactive_cleanup'
		LIMIT 1`, issuer, subject).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cleaned user by subject: %w", err)
	}
	return &id, nil
}

// AccountByID implements write.GameLoginRepository (the identity of the most recent login when several).
func (GameLoginRepo) AccountByID(ctx context.Context, userID uuid.UUID) (*write.GameAccount, error) {
	// tenancyguard:ok users/external_identities are global (G); deleted_at filtered
	return oneGameAccount(ctx, gameAccountSelect+` WHERE u.user_id = $1 AND u.deleted_at IS NULL
		ORDER BY e.last_login_at DESC NULLS LAST, e.external_identity_id LIMIT 1`, userID)
}

// MarkLogin implements write.GameLoginRepository.
func (GameLoginRepo) MarkLogin(ctx context.Context, a write.GameAccount, at time.Time) (bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok identity.external_identities is global (G); deleted_at filtered
	if _, err := tx.ExecContext(ctx, `UPDATE identity.external_identities SET last_login_at = $2, updated_at = $2
		WHERE external_identity_id = $1 AND deleted_at IS NULL`, a.ExternalIdentityID, at); err != nil {
		return false, fmt.Errorf("mark login: %w", err)
	}
	// tenancyguard:ok identity.users is global (G); deleted_at filtered
	res, err := tx.ExecContext(ctx, `UPDATE identity.users SET status = 'active', updated_at = $2, version = version + 1
		WHERE user_id = $1 AND status = 'pending' AND deleted_at IS NULL`, a.UserID, at)
	if err != nil {
		return false, fmt.Errorf("activate user: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// RecordLoginEvent implements write.GameLoginRepository.
func (GameLoginRepo) RecordLoginEvent(ctx context.Context, e write.LoginEvent) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.Must(uuid.NewV7())
	}
	var resolution *string
	if e.TenantResolution != "" {
		resolution = &e.TenantResolution
	}
	// tenancyguard:ok append-only journal; RLS admits identity_app rows of its tenant, identity_platform rows without tenant
	_, err = tx.ExecContext(ctx, `INSERT INTO identity.login_events (login_event_id, tenant_id, created_at, occurred_at, client,
			ip_hash, sis_connection_id, device_id, user_id, result, tenant_resolution)
		VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, $9, $10)`,
		e.ID, e.TenantID, e.OccurredAt, e.Client, e.IPHash, e.SisConnectionID, e.DeviceID, e.UserID, e.Result, resolution)
	if err != nil {
		return fmt.Errorf("login event: %w", err)
	}
	return nil
}

// GameAuthSessionRepo implements write.GameAuthSessionRepository with explicit SQL in the platform scope
// "game-auth" (identity_platform): the presented token hash selects the row, the use case compares its
// tenant with X-Tenant-Id. Every statement filters deleted_at (AGENTS R27); revocation is an UPDATE of
// revoked_at (never a delete, R21).
type GameAuthSessionRepo struct{}

var _ write.GameAuthSessionRepository = GameAuthSessionRepo{}

const sessionColumns = `game_auth_session_id, tenant_id, user_id, keycloak_sid, device_id, platform, refresh_token_hash,
	previous_token_hashes, created_at, last_refreshed_at, revoked_at, coalesce(revoke_reason, ''), version`

func scanSessions(rows *sql.Rows) ([]*gamelogin.Session, error) {
	defer func() { _ = rows.Close() }()
	var out []*gamelogin.Session
	for rows.Next() {
		var s gamelogin.Session
		var prev []byte
		var revoked sql.NullTime
		if err := rows.Scan(&s.ID, &s.TenantID, &s.UserID, &s.KeycloakSID, &s.DeviceID, &s.Platform, &s.CurrentHash, &prev,
			&s.CreatedAt, &s.LastRefreshedAt, &revoked, &s.RevokeReason, &s.Version); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(prev, &s.PreviousHashes); err != nil {
			return nil, err
		}
		if revoked.Valid {
			t := revoked.Time
			s.RevokedAt = &t
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

func querySessions(ctx context.Context, q string, args ...any) ([]*gamelogin.Session, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("game auth session: %w", err)
	}
	list, err := scanSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("game auth session: %w", err)
	}
	return list, nil
}

func firstSession(list []*gamelogin.Session) *gamelogin.Session {
	if len(list) == 0 {
		return nil
	}
	return list[0]
}

// Create implements write.GameAuthSessionRepository.
func (GameAuthSessionRepo) Create(ctx context.Context, s *gamelogin.Session) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	prev, err := json.Marshal(nonNil(s.PreviousHashes))
	if err != nil {
		return err
	}
	// tenancyguard:ok platform scope game-auth; tenant_id is the tenant chosen at login
	err = tx.QueryRowContext(ctx, `INSERT INTO identity.game_auth_sessions (game_auth_session_id, tenant_id, created_at, created_by,
			updated_at, user_id, keycloak_sid, device_id, platform, refresh_token_hash, previous_token_hashes, last_refreshed_at)
		VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $7, $8, $9::jsonb, $3) RETURNING version`,
		s.ID, s.TenantID, s.CreatedAt, s.UserID, s.KeycloakSID, s.DeviceID, s.Platform, s.CurrentHash, string(prev)).Scan(&s.Version)
	if err != nil {
		return fmt.Errorf("create game auth session: %w", err)
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ByTokenHash implements write.GameAuthSessionRepository (current hash first, then the rotated hashes
// through the GIN index; revoked sessions are returned so the use case answers REFRESH_TOKEN_INVALID).
func (GameAuthSessionRepo) ByTokenHash(ctx context.Context, hash string) (*gamelogin.Session, error) {
	// tenancyguard:ok platform scope game-auth: lookup by the hash of a presented token; deleted_at filtered
	list, err := querySessions(ctx, `SELECT `+sessionColumns+` FROM identity.game_auth_sessions
		WHERE (refresh_token_hash = $1 OR previous_token_hashes @> jsonb_build_array($1::text)) AND deleted_at IS NULL
		ORDER BY (revoked_at IS NULL) DESC, created_at DESC LIMIT 1 FOR UPDATE`, hash)
	if err != nil {
		return nil, err
	}
	return firstSession(list), nil
}

// BySID implements write.GameAuthSessionRepository.
func (GameAuthSessionRepo) BySID(ctx context.Context, sid string) (*gamelogin.Session, error) {
	// tenancyguard:ok platform scope game-auth: lookup by Keycloak session id; deleted_at filtered
	list, err := querySessions(ctx, `SELECT `+sessionColumns+` FROM identity.game_auth_sessions
		WHERE keycloak_sid = $1 AND deleted_at IS NULL FOR UPDATE`, sid)
	if err != nil {
		return nil, err
	}
	return firstSession(list), nil
}

// Live implements write.GameAuthSessionRepository.
func (GameAuthSessionRepo) Live(ctx context.Context, f write.SessionFilter) ([]*gamelogin.Session, error) {
	if f.UserID == uuid.Nil {
		return nil, problem.New(problem.Internal, "session filter needs a user")
	}
	var sid *string
	if f.KeycloakSID != "" {
		sid = &f.KeycloakSID
	}
	// tenancyguard:ok platform scope game-auth: the sessions of one user, optionally narrowed by tenant/device/sid; deleted_at filtered
	return querySessions(ctx, `SELECT `+sessionColumns+` FROM identity.game_auth_sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND deleted_at IS NULL
			AND ($2::uuid IS NULL OR tenant_id = $2) AND ($3::uuid IS NULL OR device_id = $3)
			AND ($4::varchar IS NULL OR keycloak_sid = $4)
		ORDER BY created_at FOR UPDATE`, f.UserID, f.TenantID, f.DeviceID, sid)
}

// Save implements write.GameAuthSessionRepository.
func (GameAuthSessionRepo) Save(ctx context.Context, s *gamelogin.Session) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	prev, err := json.Marshal(nonNil(s.PreviousHashes))
	if err != nil {
		return err
	}
	var reason *string
	if s.RevokeReason != "" {
		reason = &s.RevokeReason
	}
	// tenancyguard:ok platform scope game-auth: one session by id and version; deleted_at filtered
	err = tx.QueryRowContext(ctx, `UPDATE identity.game_auth_sessions SET refresh_token_hash = $3, previous_token_hashes = $4::jsonb,
			last_refreshed_at = $5, revoked_at = $6, revoke_reason = $7, updated_at = now(), version = version + 1
		WHERE game_auth_session_id = $1 AND version = $2 AND deleted_at IS NULL RETURNING version`,
		s.ID, s.Version, s.CurrentHash, string(prev), s.LastRefreshedAt, s.RevokedAt, reason).Scan(&s.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return problem.New(problem.Conflict, "game auth session changed")
	}
	if err != nil {
		return fmt.Errorf("save game auth session: %w", err)
	}
	return nil
}
