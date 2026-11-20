package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// ExamModel implements read.ExamReadModel with explicit SQL (tenant of the session through RLS and
// infra.current_tenant_id(), deleted_at IS NULL); single rows reuse the queries of the write repository.
type ExamModel struct{}

// Sessions implements read.ExamReadModel (newest first, keyset on the id).
func (ExamModel) Sessions(ctx context.Context, f read.ExamSessionFilter) ([]write.ExamSessionRow, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	scope := "TRUE"
	var scopeArgs []any
	if f.StudentID == nil {
		scope, scopeArgs = scopeCondition(f.Scopes, 10)
	}
	audience := "assigned"
	if f.Library {
		audience = "library"
	}
	args := append([]any{f.StudentID, audience, f.OpenAt, f.SubjectID, f.GradeLevel, nullString(f.Type), f.After, f.Limit, f.Library}, scopeArgs...)
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on both tables; scope condition built from fixed column names
	rows, err := q.QueryContext(ctx, `SELECT `+examSessionColumnsA+` FROM assessment.exam_sessions a
		WHERE a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL
			AND ($1::uuid IS NULL OR (a.audience = $2::varchar AND a.status <> 'cancelled' AND EXISTS (SELECT 1 FROM assessment.exam_participations p
				WHERE p.exam_session_id = a.exam_session_id AND p.student_id = $1::uuid AND p.deleted_at IS NULL)))
			AND (NOT $9::boolean OR (a.audience = 'library' AND a.status = 'open' AND NOT a.is_official
				AND a.exam_type IN ('practice_test','mock') AND a.window_start <= $3::timestamptz AND a.window_end > $3::timestamptz))
			AND ($4::uuid IS NULL OR a.subject_id = $4::uuid) AND ($5::int IS NULL OR a.grade_level = $5::int)
			AND ($6::varchar IS NULL OR a.exam_type = $6::varchar) AND ($7::uuid IS NULL OR a.exam_session_id < $7::uuid) AND `+scope+`
		ORDER BY a.exam_session_id DESC LIMIT $8`, args...)
	if err != nil {
		return nil, fmt.Errorf("exam sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ExamSessionRow
	for rows.Next() {
		s, err := ScanExamSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// examSessionColumnsA are the columns of ScanExamSession on alias a.
const examSessionColumnsA = `a.exam_session_id, a.tenant_id, a.school_id, a.subject_id, a.grade_level, a.course_id, a.assignment_id, a.title,
	a.exam_type, a.is_official, a.blueprint_id, a.blueprint_version_id, a.window_start, a.window_end, a.duration_sec, a.target, a.sections,
	a.audience, a.variant_strategy, a.variant_count, a.feedback_timing, a.result_release, a.show_correct_answers, a.show_explanations,
	a.late_start_policy, a.accommodations, a.max_attempts, a.score_policy, a.question_count, a.total_points::float8, a.status, a.created_by,
	a.created_by_role, a.opened_at, a.closed_at, a.results_released_at, a.results_released_by, a.cancelled_at, a.cancel_reason, a.version,
	a.created_at, a.efficacy_wave_id`

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

const ExamSessionColumns = `exam_session_id, tenant_id, school_id, subject_id, grade_level, course_id, assignment_id, title, exam_type,
	is_official, blueprint_id, blueprint_version_id, window_start, window_end, duration_sec, target, sections, audience, variant_strategy,
	variant_count, feedback_timing, result_release, show_correct_answers, show_explanations, late_start_policy, accommodations,
	max_attempts, score_policy, question_count, total_points::float8, status, created_by, created_by_role, opened_at, closed_at,
	results_released_at, results_released_by, cancelled_at, cancel_reason, version, created_at, efficacy_wave_id`

type rowScanner interface{ Scan(dest ...any) error }

// ScanExamSession scans examSessionColumns.
func ScanExamSession(r rowScanner) (write.ExamSessionRow, error) {
	var s write.ExamSessionRow
	var grade sql.NullInt64
	var course, asg, by, wave uuid.NullUUID
	err := r.Scan(&s.ID, &s.TenantID, &s.SchoolID, &s.SubjectID, &grade, &course, &asg, &s.Title, &s.Type, &s.IsOfficial, &s.BlueprintID,
		&s.BlueprintVersionID, &s.WindowStart, &s.WindowEnd, &s.DurationSec, &s.Target, &s.Sections, &s.Audience, &s.Strategy, &s.VariantCount,
		&s.FeedbackTiming, &s.ResultRelease, &s.ShowCorrect, &s.ShowExplanations, &s.LateStartPolicy, &s.Accommodations, &s.MaxAttempts,
		&s.ScorePolicy, &s.QuestionCount, &s.TotalPoints, &s.Status, &s.CreatedBy, &s.CreatedByRole, &s.OpenedAt, &s.ClosedAt, &s.ReleasedAt,
		&by, &s.CancelledAt, &s.CancelReason, &s.Version, &s.CreatedAt, &wave)
	if grade.Valid {
		g := int(grade.Int64)
		s.GradeLevel = &g
	}
	s.CourseID, s.AssignmentID, s.ReleasedBy, s.EfficacyWaveID = nullUUID(course), nullUUID(asg), nullUUID(by), nullUUID(wave)
	return s, err
}

const ExamPaperColumns = `exam_paper_id, tenant_id, exam_session_id, variant_no, student_id, attempt_no, seed, items,
	total_points::float8, expected_p::float8, generator_version, balance_warning`

func ScanExamPaper(r rowScanner) (write.ExamPaperRow, error) {
	var p write.ExamPaperRow
	var student uuid.NullUUID
	var attemptNo sql.NullInt64
	var items []byte
	if err := r.Scan(&p.ID, &p.TenantID, &p.SessionID, &p.VariantNo, &student, &attemptNo, &p.Seed, &items, &p.TotalPoints, &p.ExpectedP,
		&p.GeneratorVersion, &p.BalanceWarning); err != nil {
		return p, err
	}
	p.StudentID = nullUUID(student)
	if attemptNo.Valid {
		n := int(attemptNo.Int64)
		p.AttemptNo = &n
	}
	return p, json.Unmarshal(items, &p.Items)
}

// Session implements read.ExamReadModel.
func (ExamModel) Session(ctx context.Context, id uuid.UUID) (*write.ExamSessionRow, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	s, err := ScanExamSession(q.QueryRowContext(ctx, `SELECT `+ExamSessionColumns+` FROM assessment.exam_sessions WHERE exam_session_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exam session: %w", err)
	}
	return &s, nil
}

// Papers implements read.ExamReadModel.
func (ExamModel) Papers(ctx context.Context, sessionID uuid.UUID) ([]write.ExamPaperRow, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT `+ExamPaperColumns+` FROM assessment.exam_papers WHERE exam_session_id = $1 AND student_id IS NULL
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY variant_no`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("exam papers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ExamPaperRow
	for rows.Next() {
		p, err := ScanExamPaper(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Participation implements read.ExamReadModel.
func (ExamModel) Participation(ctx context.Context, sessionID, studentID uuid.UUID) (*write.ExamParticipationRow, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var p write.ExamParticipationRow
	var last uuid.NullUUID
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT exam_session_id, student_id, tenant_id, school_id, status, attempts_used, last_attempt_id,
		effective_duration_sec, extra_time_percent, version FROM assessment.exam_participations WHERE exam_session_id = $1 AND student_id = $2
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, sessionID, studentID).Scan(&p.SessionID, &p.StudentID, &p.TenantID,
		&p.SchoolID, &p.Status, &p.AttemptsUsed, &last, &p.EffectiveDurationSec, &p.ExtraTimePercent, &p.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exam participation: %w", err)
	}
	if last.Valid {
		p.LastAttemptID = &last.UUID
	}
	return &p, nil
}

// AttemptPaper implements read.ExamReadModel.
func (ExamModel) AttemptPaper(ctx context.Context, attemptID uuid.UUID) (*uuid.UUID, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var paper uuid.NullUUID
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT exam_paper_id FROM assessment.attempts WHERE attempt_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, attemptID).Scan(&paper)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("attempt paper: %w", err)
	}
	if !paper.Valid {
		return nil, nil
	}
	return &paper.UUID, nil
}

// LatestAnswers implements read.ExamReadModel (attempt_answers is append-only: no deleted_at).
func (ExamModel) LatestAnswers(ctx context.Context, attemptID uuid.UUID) (map[int]json.RawMessage, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()); append-only table without deleted_at
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT ON (position) position, response FROM assessment.attempt_answers
		WHERE attempt_id = $1 AND tenant_id = infra.current_tenant_id() ORDER BY position, answered_at DESC, attempt_no DESC`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("latest answers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[int]json.RawMessage{}
	for rows.Next() {
		var pos int
		var resp []byte
		if err := rows.Scan(&pos, &resp); err != nil {
			return nil, err
		}
		out[pos] = resp
	}
	return out, rows.Err()
}
