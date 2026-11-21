package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// LessonRepo implements write.LessonStore with explicit SQL in the transaction of ctx (TenantScope; RLS T).
type LessonRepo struct{}

var _ write.LessonStore = LessonRepo{}

const sessionColumns = `lesson_session_id, tenant_id, school_id, student_id, lesson_id, lesson_version_id, lesson_version, status, context_mode,
	assignment_id, recommendation_id, array_to_string(completed_activity_ids, ','), array_to_string(completed_attempt_ids, ','),
	active_ms, started_at, ended_at, version`

func splitIDs(s string) []uuid.UUID {
	var out []uuid.UUID
	for _, p := range strings.Split(s, ",") {
		if id, err := uuid.Parse(p); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func scanSession(row interface{ Scan(...any) error }) (*write.LessonSession, error) {
	var s write.LessonSession
	var asg, rec uuid.NullUUID
	var acts, atts string
	var ended sql.NullTime
	err := row.Scan(&s.ID, &s.TenantID, &s.SchoolID, &s.StudentID, &s.LessonID, &s.LessonVersionID, &s.LessonVersion, &s.Status, &s.ContextMode,
		&asg, &rec, &acts, &atts, &s.ActiveMS, &s.StartedAt, &ended, &s.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lesson session: %w", err)
	}
	s.AssignmentID, s.RecommendationID = nullUUID(asg), nullUUID(rec)
	s.CompletedActivityIDs, s.CompletedAttemptIDs, s.EndedAt = splitIDs(acts), splitIDs(atts), nullTime(ended)
	s.StartedAt = s.StartedAt.UTC()
	return &s, nil
}

func (LessonRepo) one(ctx context.Context, query string, args ...any) (*write.LessonSession, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	return scanSession(tx.QueryRowContext(ctx, query, args...))
}

// OpenSession implements write.LessonStore.
func (r LessonRepo) OpenSession(ctx context.Context, studentID, lessonID uuid.UUID) (*write.LessonSession, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return r.one(ctx, `SELECT `+sessionColumns+` FROM learning.lesson_sessions WHERE tenant_id = infra.current_tenant_id()
		AND student_id = $1 AND lesson_id = $2 AND status = 'in_progress' AND deleted_at IS NULL`, studentID, lessonID)
}

// LatestSession implements write.LessonStore.
func (r LessonRepo) LatestSession(ctx context.Context, studentID, lessonID uuid.UUID) (*write.LessonSession, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return r.one(ctx, `SELECT `+sessionColumns+` FROM learning.lesson_sessions WHERE tenant_id = infra.current_tenant_id()
		AND student_id = $1 AND lesson_id = $2 AND deleted_at IS NULL
		ORDER BY (status = 'in_progress') DESC, started_at DESC LIMIT 1`, studentID, lessonID)
}

// Session implements write.LessonStore.
func (r LessonRepo) Session(ctx context.Context, id uuid.UUID) (*write.LessonSession, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return r.one(ctx, `SELECT `+sessionColumns+` FROM learning.lesson_sessions WHERE tenant_id = infra.current_tenant_id()
		AND lesson_session_id = $1 AND deleted_at IS NULL`, id)
}

// CompletedVersion implements write.LessonStore.
func (LessonRepo) CompletedVersion(ctx context.Context, studentID, lessonVersionID uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var n int
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM learning.lesson_sessions WHERE tenant_id = infra.current_tenant_id()
		AND student_id = $1 AND lesson_version_id = $2 AND status = 'completed' AND deleted_at IS NULL`, studentID, lessonVersionID).Scan(&n); err != nil {
		return false, fmt.Errorf("completed lesson version: %w", err)
	}
	return n > 0, nil
}

// InsertSession implements write.LessonStore.
func (LessonRepo) InsertSession(ctx context.Context, s write.LessonSession) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy); conflict on the open-session unique
	res, err := tx.ExecContext(ctx, `INSERT INTO learning.lesson_sessions (lesson_session_id, tenant_id, school_id, student_id, lesson_id,
		lesson_version_id, lesson_version, status, context_mode, assignment_id, recommendation_id, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'in_progress', $8, $9, $10, $11)
		ON CONFLICT (tenant_id, student_id, lesson_id) WHERE status = 'in_progress' AND deleted_at IS NULL DO NOTHING`,
		s.ID, s.TenantID, s.SchoolID, s.StudentID, s.LessonID, s.LessonVersionID, s.LessonVersion, s.ContextMode, s.AssignmentID,
		s.RecommendationID, s.StartedAt.UTC())
	if err != nil {
		return false, fmt.Errorf("insert lesson session: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// UpdateSession implements write.LessonStore.
func (LessonRepo) UpdateSession(ctx context.Context, s write.LessonSession) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.lesson_sessions SET status = $2, completed_activity_ids = $3::uuid[],
		completed_attempt_ids = $4::uuid[], active_ms = $5, ended_at = $6, version = version + 1, updated_at = now()
		WHERE lesson_session_id = $1 AND version = $7 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		s.ID, s.Status, uuidArray(s.CompletedActivityIDs), uuidArray(s.CompletedAttemptIDs), s.ActiveMS, s.EndedAt, s.Version)
	if err != nil {
		return false, fmt.Errorf("update lesson session: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// Attempt implements write.LessonStore.
func (LessonRepo) Attempt(ctx context.Context, attemptID uuid.UUID) (*write.LessonAttempt, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var a write.LessonAttempt
	var act uuid.NullUUID
	var dur sql.NullInt64
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT attempt_id, student_id, activity_id, context_type, context_id, status, duration_ms
		FROM assessment.attempts WHERE attempt_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, attemptID).
		Scan(&a.AttemptID, &a.StudentID, &act, &a.ContextType, &a.ContextID, &a.Status, &dur)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lesson attempt: %w", err)
	}
	a.ActivityID, a.DurationMS = nullUUID(act), dur.Int64
	return &a, nil
}
