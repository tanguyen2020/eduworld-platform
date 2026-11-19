// Package write implements the write ports of module game inside entx.UnitOfWork: repositories (explicit SQL on
// the transaction of ctx, with tenant and deleted_at conditions — AGENTS R27) and the outbox adapters (events,
// audit) of the same transaction.
package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok || !entx.InWriteTx(ctx) {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// SessionRepo implements write.SessionRepository on game.game_sessions.
type SessionRepo struct{}

var _ write.SessionRepository = SessionRepo{}

const sessionColumns = `game_session_id, tenant_id, student_id, school_id, device_id, device_model, client_platform, client_version,
	coalesce(keycloak_sid, ''), started_at, last_heartbeat_at, ended_at, coalesce(end_reason, ''), replaced_by_session_id, version`

func scanSession(row interface{ Scan(...any) error }) (*gamesession.Session, error) {
	var s gamesession.Session
	var school, replaced uuid.NullUUID
	var model sql.NullString
	var ended sql.NullTime
	if err := row.Scan(&s.ID, &s.TenantID, &s.StudentID, &school, &s.DeviceID, &model, &s.Platform, &s.ClientVersion,
		&s.KeycloakSID, &s.StartedAt, &s.LastHeartbeatAt, &ended, &s.EndReason, &replaced, &s.Version); err != nil {
		return nil, err
	}
	if school.Valid {
		s.SchoolID = &school.UUID
	}
	if replaced.Valid {
		s.ReplacedBy = &replaced.UUID
	}
	if model.Valid {
		s.DeviceModel = &model.String
	}
	if ended.Valid {
		t := ended.Time.UTC()
		s.EndedAt = &t
	}
	s.StartedAt, s.LastHeartbeatAt = s.StartedAt.UTC(), s.LastHeartbeatAt.UTC()
	return &s, nil
}

func querySessions(ctx context.Context, query string, args ...any) ([]*gamesession.Session, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*gamesession.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func one(list []*gamesession.Session, err error) (*gamesession.Session, error) {
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return list[0], nil
}

// LockStudent implements write.SessionRepository (DATABASE_DESIGN §3.12: hashtextextended(student_id::text, 0)).
func (SessionRepo) LockStudent(ctx context.Context, student uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, student.String()); err != nil {
		return fmt.Errorf("game sessions: lock student: %w", err)
	}
	return nil
}

// Active implements write.SessionRepository.
func (SessionRepo) Active(ctx context.Context, tenant, student uuid.UUID) (*gamesession.Session, error) {
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered
	return one(querySessions(ctx, `SELECT `+sessionColumns+` FROM game.game_sessions
		WHERE tenant_id = $1 AND student_id = $2 AND ended_at IS NULL AND deleted_at IS NULL FOR UPDATE`, tenant, student))
}

// ByID implements write.SessionRepository.
func (SessionRepo) ByID(ctx context.Context, tenant, id uuid.UUID) (*gamesession.Session, error) {
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered
	return one(querySessions(ctx, `SELECT `+sessionColumns+` FROM game.game_sessions
		WHERE tenant_id = $1 AND game_session_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, id))
}

// ActiveOfStudent implements write.SessionRepository.
func (SessionRepo) ActiveOfStudent(ctx context.Context, student uuid.UUID, tenant *uuid.UUID) ([]*gamesession.Session, error) {
	// tenancyguard:ok the event scope: one tenant (RLS + explicit tenant_id) or the platform scope of a platform event
	// (game_platform, every tenant of this one student); deleted_at filtered
	return querySessions(ctx, `SELECT `+sessionColumns+` FROM game.game_sessions
		WHERE student_id = $1 AND ($2::uuid IS NULL OR tenant_id = $2) AND ended_at IS NULL AND deleted_at IS NULL
		ORDER BY started_at FOR UPDATE`, student, tenant)
}

func uniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Create implements write.SessionRepository; a concurrent active session of the student (unique partial index,
// e.g. in another tenant of the same student) is 409 ACTIVE_SESSION_ON_OTHER_DEVICE.
func (SessionRepo) Create(ctx context.Context, s *gamesession.Session) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	var actor any
	if p, ok := authz.PrincipalFrom(ctx); ok {
		actor = p.UserID
	}
	s.Version = 1
	_, err = t.ExecContext(ctx, `INSERT INTO game.game_sessions (game_session_id, tenant_id, created_at, created_by, updated_at,
			updated_by, version, student_id, school_id, device_id, device_model, client_platform, client_version, keycloak_sid,
			started_at, last_heartbeat_at)
		VALUES ($1, $2, $3, $4, $3, $4, 1, $5, $6, $7, $8, $9, $10, $11, $3, $3)`,
		s.ID, s.TenantID, s.StartedAt, actor, s.StudentID, s.SchoolID, s.DeviceID, s.DeviceModel, s.Platform, s.ClientVersion,
		nullString(s.KeycloakSID))
	if uniqueViolation(err) {
		return problem.New(problem.ActiveSessionOnOtherDevice, "").Wrap(err)
	}
	if err != nil {
		return fmt.Errorf("game sessions: create: %w", err)
	}
	return nil
}

// Save implements write.SessionRepository.
func (SessionRepo) Save(ctx context.Context, s *gamesession.Session) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	var reason any
	if s.EndReason != "" {
		reason = s.EndReason
	}
	// tenancyguard:ok one session by id, tenant and version under its TenantScope; deleted_at filtered
	err = t.QueryRowContext(ctx, `UPDATE game.game_sessions SET last_heartbeat_at = $4, ended_at = $5, end_reason = $6,
			replaced_by_session_id = $7, updated_at = now(), version = version + 1
		WHERE game_session_id = $1 AND tenant_id = $2 AND version = $3 AND deleted_at IS NULL RETURNING version`,
		s.ID, s.TenantID, s.Version, s.LastHeartbeatAt, s.EndedAt, reason, s.ReplacedBy).Scan(&s.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return problem.New(problem.Conflict, "game session changed")
	}
	if err != nil {
		return fmt.Errorf("game sessions: save: %w", err)
	}
	return nil
}

func queryRefs(ctx context.Context, query string, args ...any) ([]write.SessionRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("game sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.SessionRef
	for rows.Next() {
		var r write.SessionRef
		var school uuid.NullUUID
		if err := rows.Scan(&r.ID, &r.TenantID, &r.StudentID, &school); err != nil {
			return nil, err
		}
		if school.Valid {
			r.SchoolID = &school.UUID
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Stale implements write.SessionRepository.
func (SessionRepo) Stale(ctx context.Context, before time.Time, limit int) ([]write.SessionRef, error) {
	// tenancyguard:ok job session-timeout under SystemPlatformScope (game_platform): ids of every tenant only; each
	// session is then ended in its own TenantScope; deleted_at filtered
	return queryRefs(ctx, `SELECT game_session_id, tenant_id, student_id, school_id FROM game.game_sessions
		WHERE ended_at IS NULL AND deleted_at IS NULL AND last_heartbeat_at < $1
		ORDER BY last_heartbeat_at LIMIT $2`, before.UTC(), limit)
}

// ActivePage implements write.SessionRepository.
func (SessionRepo) ActivePage(ctx context.Context, after uuid.UUID, limit int) ([]write.SessionRef, error) {
	// tenancyguard:ok job curfew-session-closer under SystemPlatformScope (game_platform): ids of every tenant only;
	// each session is then ended in its own TenantScope; deleted_at filtered
	return queryRefs(ctx, `SELECT game_session_id, tenant_id, student_id, school_id FROM game.game_sessions
		WHERE ended_at IS NULL AND deleted_at IS NULL AND game_session_id > $1
		ORDER BY game_session_id LIMIT $2`, after, limit)
}
