package challengesql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Store implements write.ChallengeStore on game.challenge_attempts (write transaction only).
type Store struct{}

var _ write.ChallengeStore = Store{}

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok || !entx.InWriteTx(ctx) {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

func actor(ctx context.Context) any {
	if p, ok := authz.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return nil
}

// ErrAttemptConflict is a concurrent change of an attempt (optimistic version check).
var ErrAttemptConflict = errors.New("challenge attempt: concurrent change")

// LockStudent implements write.ChallengeStore.
func (Store) LockStudent(ctx context.Context, student uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok advisory lock only (no table read)
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('game.challenge:' || $1::text, 0))`, student); err != nil {
		return fmt.Errorf("challenge: lock student: %w", err)
	}
	return nil
}

func queryAttempts(ctx context.Context, query string, args ...any) ([]*challenge.Attempt, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("challenge attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*challenge.Attempt
	for rows.Next() {
		a, err := scanAttempt(rows)
		if err != nil {
			return nil, fmt.Errorf("challenge attempts: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func first(as []*challenge.Attempt, err error) (*challenge.Attempt, error) {
	if err != nil || len(as) == 0 {
		return nil, err
	}
	return as[0], nil
}

// ActiveOf implements write.ChallengeStore.
func (Store) ActiveOf(ctx context.Context, tenant, student uuid.UUID) (*challenge.Attempt, error) {
	// tenancyguard:ok tenant table game.challenge_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return first(queryAttempts(ctx, `SELECT `+attemptCols+` FROM game.challenge_attempts
		WHERE tenant_id = $1 AND student_id = $2 AND status = 'in_progress' AND deleted_at IS NULL FOR UPDATE`, tenant, student))
}

// ByID implements write.ChallengeStore.
func (Store) ByID(ctx context.Context, tenant, id uuid.UUID) (*challenge.Attempt, error) {
	// tenancyguard:ok tenant table game.challenge_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return first(queryAttempts(ctx, `SELECT `+attemptCols+` FROM game.challenge_attempts
		WHERE tenant_id = $1 AND challenge_attempt_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, id))
}

// ByLearningAttempt implements write.ChallengeStore (GIN index challenge_attempts_learning_gin).
func (Store) ByLearningAttempt(ctx context.Context, tenant, attemptID uuid.UUID) (*challenge.Attempt, error) {
	needle, err := json.Marshal([]string{attemptID.String()})
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.challenge_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return first(queryAttempts(ctx, `SELECT `+attemptCols+` FROM game.challenge_attempts
		WHERE tenant_id = $1 AND learning_attempt_ids @> $2::jsonb AND status = 'in_progress' AND deleted_at IS NULL FOR UPDATE`,
		tenant, string(needle)))
}

func ids(xs []uuid.UUID) (string, error) {
	if xs == nil {
		xs = []uuid.UUID{}
	}
	b, err := json.Marshal(xs)
	return string(b), err
}

// Insert implements write.ChallengeStore.
func (Store) Insert(ctx context.Context, a *challenge.Attempt) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	learning, err := ids(a.LearningAttemptIDs)
	if err != nil {
		return false, err
	}
	processed, err := ids(a.Processed)
	if err != nil {
		return false, err
	}
	var lives any
	if a.LivesLeft != nil {
		lives = *a.LivesLeft
	}
	// tenancyguard:ok tenant table game.challenge_attempts: explicit tenant_id (RLS insert policy); one in progress per student
	res, err := t.ExecContext(ctx, `INSERT INTO game.challenge_attempts (challenge_attempt_id, tenant_id, created_at, created_by, updated_at,
			student_id, school_id, challenge_id, challenge_version_id, challenge_type, difficulty, status, lives_left, period_key, started_at,
			deadline_at, learning_attempt_ids, processed_attempt_ids)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12, $13, $3, $14, $15::jsonb, $16::jsonb)
		ON CONFLICT DO NOTHING`, a.ID, a.TenantID, a.StartedAt, actor(ctx), a.StudentID, a.SchoolID, a.ChallengeID, a.VersionID, a.Type,
		a.Difficulty, a.Status, lives, a.PeriodKey, nullTime(a.DeadlineAt), learning, processed)
	if err != nil {
		return false, fmt.Errorf("challenge attempts: insert: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Save implements write.ChallengeStore.
func (Store) Save(ctx context.Context, a *challenge.Attempt) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	learning, err := ids(a.LearningAttemptIDs)
	if err != nil {
		return err
	}
	processed, err := ids(a.Processed)
	if err != nil {
		return err
	}
	var lives, reason any
	if a.LivesLeft != nil {
		lives = *a.LivesLeft
	}
	if a.EndReason != "" {
		reason = a.EndReason
	}
	// tenancyguard:ok tenant table game.challenge_attempts: explicit tenant_id + deleted_at IS NULL (RLS update policy)
	res, err := t.ExecContext(ctx, `UPDATE game.challenge_attempts SET status = $3, end_reason = $4, score = $5, items_answered = $6,
			correct_streak = $7, lives_left = $8, goal_progress = $9, activities_completed = $10, first_clear = $11, ended_at = $12,
			learning_attempt_ids = $13::jsonb, processed_attempt_ids = $14::jsonb, updated_at = now(), updated_by = $15, version = version + 1
		WHERE tenant_id = $1 AND challenge_attempt_id = $2 AND version = $16 AND deleted_at IS NULL`, a.TenantID, a.ID, a.Status, reason,
		a.Score, a.ItemsAnswered, a.Streak, lives, a.GoalProgress, a.ActivitiesCompleted, a.FirstClear, nullTime(a.EndedAt), learning,
		processed, actor(ctx), a.Version)
	if err != nil {
		return fmt.Errorf("challenge attempts: save: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		if err != nil {
			return err
		}
		return ErrAttemptConflict
	}
	a.Version++
	return nil
}

// Cleared implements write.ChallengeStore.
func (Store) Cleared(ctx context.Context, tenant, student, challengeID, except uuid.UUID) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok tenant table game.challenge_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM game.challenge_attempts WHERE tenant_id = $1 AND student_id = $2
		AND challenge_id = $3 AND status = 'succeeded' AND challenge_attempt_id <> $4 AND deleted_at IS NULL)`, tenant, student, challengeID,
		except).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("challenge cleared: %w", err)
	}
	return ok, nil
}

// Due implements write.ChallengeStore (platform scope of the job; deadline and resume indexes).
func (Store) Due(ctx context.Context, before time.Time, resume time.Duration, limit int) ([]write.AttemptRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform scope of the job challenge-deadline (game_platform, every tenant); deleted_at filtered; ids only
	rows, err := t.QueryContext(ctx, `(SELECT challenge_attempt_id, tenant_id, student_id FROM game.challenge_attempts
			WHERE status = 'in_progress' AND deadline_at IS NOT NULL AND deadline_at < $1 AND deleted_at IS NULL ORDER BY deadline_at LIMIT $3)
		UNION ALL
		(SELECT challenge_attempt_id, tenant_id, student_id FROM game.challenge_attempts
			WHERE status = 'in_progress' AND deadline_at IS NULL AND started_at < $2 AND deleted_at IS NULL ORDER BY started_at LIMIT $3)`,
		before, before.Add(-resume), limit)
	if err != nil {
		return nil, fmt.Errorf("due challenge attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AttemptRef
	for rows.Next() {
		var r write.AttemptRef
		if err := rows.Scan(&r.ID, &r.TenantID, &r.StudentID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ActiveOfStudentAnyTenant implements write.ChallengeStore.
func (Store) ActiveOfStudentAnyTenant(ctx context.Context, student uuid.UUID, tenant *uuid.UUID) ([]*challenge.Attempt, error) {
	// tenancyguard:ok the event scope: one tenant (RLS + explicit tenant_id) or the platform scope of a platform event
	// (game_platform, every tenant of this one student); deleted_at filtered
	return queryAttempts(ctx, `SELECT `+attemptCols+` FROM game.challenge_attempts
		WHERE student_id = $1 AND ($2::uuid IS NULL OR tenant_id = $2) AND status = 'in_progress' AND deleted_at IS NULL
		ORDER BY started_at FOR UPDATE`, student, tenant)
}
