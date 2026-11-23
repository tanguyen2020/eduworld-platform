// Package challengesql reads the published challenges (G tables), their tenant assignment
// (infra.tenant_resource_assignments) and the challenge attempts (T table) on a transaction, and writes the attempts
// (Store, write transaction only) — F076. Explicit SQL: the tenant table carries its tenant_id and deleted_at
// conditions (AGENTS R27, RLS as well), platform content its deleted_at condition.
package challengesql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Querier is the part of *sql.Tx the readers need.
type Querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// Content implements port.ChallengeContent on the transaction of ctx (read session or write transaction).
type Content struct{}

var _ port.ChallengeContent = Content{}

func q(ctx context.Context) (Querier, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

const versionCols = `v.challenge_id, v.challenge_version_id, v.version_no, v.payload`

func scanVersions(rows *sql.Rows) ([]port.ChallengeRow, error) {
	defer func() { _ = rows.Close() }()
	var out []port.ChallengeRow
	for rows.Next() {
		var r port.ChallengeRow
		var raw []byte
		if err := rows.Scan(&r.ChallengeID, &r.VersionID, &r.VersionNo, &raw); err != nil {
			return nil, err
		}
		v, err := challenge.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("challenge payload: %w", err)
		}
		r.Version = v
		out = append(out, r)
	}
	return out, rows.Err()
}

func one(rows []port.ChallengeRow, err error) (*port.ChallengeRow, error) {
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// AvailableChallenges implements port.ChallengeContent: the keyset page (challenge_id) of the current versions assigned
// to the tenant, of the grade, playable solo (solo type, not party_required) and open at now; LIMIT in the database.
func (Content) AvailableChallenges(ctx context.Context, f port.ChallengeFilter) ([]port.ChallengeRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform challenges (G tables) joined with the active assignments of the session tenant ($1, explicit; infra without RLS)
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.challenges d
		JOIN game.challenge_versions v ON v.challenge_version_id = d.current_version_id
		JOIN infra.tenant_resource_assignments a ON a.resource_type = 'challenge' AND a.resource_id = d.challenge_id AND a.tenant_id = $1
			AND a.deleted_at IS NULL
		WHERE d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'
			AND v.grade_levels @> jsonb_build_array($2::int)
			AND v.challenge_type IN ('time','perfect','survival','puzzle','knowledge','weekly','seasonal') AND v.mode <> 'party_required'
			AND ($3::text = '' OR v.challenge_type = $3) AND ($4::text = '' OR v.area_id = $4)
			AND (v.available_from IS NULL OR v.available_from <= $5) AND (v.available_to IS NULL OR v.available_to > $5)
			AND ($6::uuid IS NULL OR d.challenge_id > $6)
		ORDER BY d.challenge_id LIMIT $7`, f.Tenant, f.Grade, f.Type, f.AreaID, f.Now, f.After, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("available challenges: %w", err)
	}
	return scanVersions(rows)
}

// Assigned implements port.ChallengeContent.
func (Content) Assigned(ctx context.Context, tenant, challengeID uuid.UUID) (bool, error) {
	t, err := q(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok infra.tenant_resource_assignments (no RLS, ADR-0089 §4): explicit tenant_id + deleted_at IS NULL
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM infra.tenant_resource_assignments
		WHERE resource_type = 'challenge' AND resource_id = $2 AND tenant_id = $1 AND deleted_at IS NULL)`, tenant, challengeID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("challenge assignment: %w", err)
	}
	return ok, nil
}

// CurrentChallenge implements port.ChallengeContent.
func (Content) CurrentChallenge(ctx context.Context, challengeID uuid.UUID) (*port.ChallengeRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.challenges d
		JOIN game.challenge_versions v ON v.challenge_version_id = d.current_version_id
		WHERE d.challenge_id = $1 AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, challengeID)
	if err != nil {
		return nil, fmt.Errorf("challenge: %w", err)
	}
	return one(scanVersions(rows))
}

// ChallengeVersion implements port.ChallengeContent (a pinned version whatever its status: never deleted, ADR-0043).
func (Content) ChallengeVersion(ctx context.Context, versionID uuid.UUID) (*port.ChallengeRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.challenge_versions v
		WHERE v.challenge_version_id = $1 AND v.deleted_at IS NULL`, versionID)
	if err != nil {
		return nil, fmt.Errorf("challenge version: %w", err)
	}
	return one(scanVersions(rows))
}

// KnownChallenges implements port.ChallengeContent.
func (Content) KnownChallenges(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]string, len(ids))
	for i, id := range ids {
		list[i] = id.String()
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; published pointer only
	rows, err := t.QueryContext(ctx, `SELECT challenge_id FROM game.challenges WHERE challenge_id::text = ANY($1)
		AND current_version_id IS NOT NULL AND deleted_at IS NULL`, list)
	if err != nil {
		return nil, fmt.Errorf("known challenges: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// Usage implements port.ChallengeContent: attempts of the period (every status: an abandoned attempt counts), the last
// failure (any period, for the cooldown) and whether the challenge was ever cleared.
func (Content) Usage(ctx context.Context, tenant, student, challengeID uuid.UUID, period string) (port.ChallengeUsage, error) {
	var u port.ChallengeUsage
	t, err := q(ctx)
	if err != nil {
		return u, err
	}
	var failedAt sql.NullTime
	var failedOn sql.NullString
	// tenancyguard:ok tenant table game.challenge_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT
			count(*) FILTER (WHERE period_key = $4),
			bool_or(status = 'succeeded'),
			(SELECT ended_at FROM game.challenge_attempts f WHERE f.tenant_id = $1 AND f.student_id = $2 AND f.challenge_id = $3
				AND f.status = 'failed' AND f.deleted_at IS NULL ORDER BY f.ended_at DESC LIMIT 1),
			(SELECT difficulty FROM game.challenge_attempts f WHERE f.tenant_id = $1 AND f.student_id = $2 AND f.challenge_id = $3
				AND f.status = 'failed' AND f.deleted_at IS NULL ORDER BY f.ended_at DESC LIMIT 1)
		FROM game.challenge_attempts WHERE tenant_id = $1 AND student_id = $2 AND challenge_id = $3 AND deleted_at IS NULL`,
		tenant, student, challengeID, period).Scan(&u.Attempts, new(sql.NullBool), &failedAt, &failedOn)
	if err != nil {
		return u, fmt.Errorf("challenge usage: %w", err)
	}
	if failedAt.Valid {
		at := failedAt.Time.UTC()
		u.LastFailedAt, u.LastFailedOn = &at, failedOn.String
	}
	return u, nil
}

// Attempt implements port.ChallengeContent.
func (Content) Attempt(ctx context.Context, tenant, id uuid.UUID) (*challenge.Attempt, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.challenge_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return scanOne(t.QueryRowContext(ctx, `SELECT `+attemptCols+` FROM game.challenge_attempts
		WHERE tenant_id = $1 AND challenge_attempt_id = $2 AND deleted_at IS NULL`, tenant, id))
}

// attemptCols are the columns of an attempt (scanAttempt order).
const attemptCols = `challenge_attempt_id, tenant_id, student_id, school_id, challenge_id, challenge_version_id, challenge_type, difficulty,
	status, end_reason, score, items_answered, correct_streak, lives_left, goal_progress, activities_completed, period_key, first_clear,
	started_at, deadline_at, ended_at, learning_attempt_ids, processed_attempt_ids, version`

type scanner interface{ Scan(...any) error }

func scanAttempt(row scanner) (*challenge.Attempt, error) {
	var a challenge.Attempt
	var reason sql.NullString
	var lives sql.NullInt64
	var deadline, ended sql.NullTime
	var learning, processed []byte
	if err := row.Scan(&a.ID, &a.TenantID, &a.StudentID, &a.SchoolID, &a.ChallengeID, &a.VersionID, &a.Type, &a.Difficulty, &a.Status,
		&reason, &a.Score, &a.ItemsAnswered, &a.Streak, &lives, &a.GoalProgress, &a.ActivitiesCompleted, &a.PeriodKey, &a.FirstClear,
		&a.StartedAt, &deadline, &ended, &learning, &processed, &a.Version); err != nil {
		return nil, err
	}
	a.EndReason = reason.String
	if lives.Valid {
		n := int(lives.Int64)
		a.LivesLeft = &n
	}
	a.StartedAt = a.StartedAt.UTC()
	if deadline.Valid {
		d := deadline.Time.UTC()
		a.DeadlineAt = &d
	}
	if ended.Valid {
		e := ended.Time.UTC()
		a.EndedAt = &e
	}
	if err := json.Unmarshal(learning, &a.LearningAttemptIDs); err != nil {
		return nil, fmt.Errorf("challenge attempt: learning ids: %w", err)
	}
	if err := json.Unmarshal(processed, &a.Processed); err != nil {
		return nil, fmt.Errorf("challenge attempt: processed ids: %w", err)
	}
	return &a, nil
}

func scanOne(row *sql.Row) (*challenge.Attempt, error) {
	a, err := scanAttempt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("challenge attempt: %w", err)
	}
	return a, nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
