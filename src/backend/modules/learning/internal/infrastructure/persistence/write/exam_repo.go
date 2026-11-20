package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
)

// ExamRepo implements write.ExamStore with explicit SQL on assessment.exam_sessions, exam_papers, exam_participations and
// the exam columns of assessment.attempts (F042): tenant of the session (infra.current_tenant_id(); RLS T) and
// deleted_at IS NULL on every read; soft-delete only.
type ExamRepo struct{}

// InsertSession implements write.ExamStore.
func (ExamRepo) InsertSession(ctx context.Context, s write.ExamSessionRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok INSERT in the session tenant (RLS insert policy checks tenant_id = infra.current_tenant_id())
	_, err = tx.ExecContext(ctx, `INSERT INTO assessment.exam_sessions (exam_session_id, tenant_id, school_id, subject_id, grade_level,
		course_id, assignment_id, title, exam_type, is_official, blueprint_id, blueprint_version_id, window_start, window_end, duration_sec,
		target, sections, audience, variant_strategy, variant_count, feedback_timing, result_release, show_correct_answers,
		show_explanations, late_start_policy, accommodations, max_attempts, score_policy, question_count, total_points, status, created_by,
		created_by_role, opened_at, version, created_at, updated_at, updated_by, efficacy_wave_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,
		$34,1,$35,$35,$32,$36)`,
		s.ID, s.TenantID, s.SchoolID, s.SubjectID, s.GradeLevel, s.CourseID, s.AssignmentID, s.Title, s.Type, s.IsOfficial, s.BlueprintID,
		s.BlueprintVersionID, s.WindowStart, s.WindowEnd, s.DurationSec, jsonOrEmpty(s.Target, "[]"), jsonOrEmpty(s.Sections, "[]"), s.Audience,
		s.Strategy, s.VariantCount, s.FeedbackTiming, s.ResultRelease, s.ShowCorrect, s.ShowExplanations, s.LateStartPolicy,
		jsonOrEmpty(s.Accommodations, "[]"), s.MaxAttempts, s.ScorePolicy, s.QuestionCount, s.TotalPoints, s.Status, s.CreatedBy, s.CreatedByRole,
		s.OpenedAt, s.CreatedAt, s.EfficacyWaveID)
	if err != nil {
		return fmt.Errorf("insert exam session: %w", err)
	}
	return nil
}

func jsonOrEmpty(b json.RawMessage, empty string) []byte {
	if len(b) == 0 {
		return []byte(empty)
	}
	return b
}

func oneSession(ctx context.Context, query string, args ...any) (*write.ExamSessionRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	s, err := read.ScanExamSession(tx.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exam session: %w", err)
	}
	return &s, nil
}

// Session implements write.ExamStore.
func (ExamRepo) Session(ctx context.Context, id uuid.UUID, lock bool) (*write.ExamSessionRow, error) {
	q := `SELECT ` + read.ExamSessionColumns + ` FROM assessment.exam_sessions WHERE exam_session_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return oneSession(ctx, q, id)
}

// SessionOfAssignment implements write.ExamStore.
func (ExamRepo) SessionOfAssignment(ctx context.Context, assignmentID uuid.UUID) (*write.ExamSessionRow, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return oneSession(ctx, `SELECT `+read.ExamSessionColumns+` FROM assessment.exam_sessions WHERE assignment_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1`, assignmentID)
}

// UpdateSession implements write.ExamStore.
func (ExamRepo) UpdateSession(ctx context.Context, s write.ExamSessionRow, prev int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE assessment.exam_sessions SET status = $3, opened_at = $4, closed_at = $5,
		results_released_at = $6, results_released_by = $7, cancelled_at = $8, cancel_reason = $9, assignment_id = $10,
		version = version + 1, updated_at = now()
		WHERE exam_session_id = $1 AND version = $2 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		s.ID, prev, s.Status, s.OpenedAt, s.ClosedAt, s.ReleasedAt, s.ReleasedBy, s.CancelledAt, s.CancelReason, s.AssignmentID))
}

// InsertPapers implements write.ExamStore.
func (ExamRepo) InsertPapers(ctx context.Context, ps []write.ExamPaperRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		items, err := json.Marshal(p.Items)
		if err != nil {
			return err
		}
		// tenancyguard:ok INSERT in the session tenant (RLS insert policy checks tenant_id = infra.current_tenant_id())
		if _, err := tx.ExecContext(ctx, `INSERT INTO assessment.exam_papers (exam_paper_id, tenant_id, exam_session_id, variant_no,
			student_id, attempt_no, seed, items, item_count, total_points, expected_p, generator_version, balance_warning)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, p.ID, p.TenantID, p.SessionID, p.VariantNo, p.StudentID, p.AttemptNo, p.Seed,
			items, len(p.Items), p.TotalPoints, p.ExpectedP, p.GeneratorVersion, p.BalanceWarning); err != nil {
			return fmt.Errorf("insert exam paper: %w", err)
		}
	}
	return nil
}

func papers(ctx context.Context, query string, args ...any) ([]write.ExamPaperRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exam papers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ExamPaperRow
	for rows.Next() {
		p, err := read.ScanExamPaper(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Papers implements write.ExamStore.
func (ExamRepo) Papers(ctx context.Context, sessionID uuid.UUID) ([]write.ExamPaperRow, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return papers(ctx, `SELECT `+read.ExamPaperColumns+` FROM assessment.exam_papers WHERE exam_session_id = $1 AND student_id IS NULL
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY variant_no`, sessionID)
}

// StudentPaper implements write.ExamStore.
func (ExamRepo) StudentPaper(ctx context.Context, sessionID, studentID uuid.UUID, attemptNo int) (*write.ExamPaperRow, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	ps, err := papers(ctx, `SELECT `+read.ExamPaperColumns+` FROM assessment.exam_papers WHERE exam_session_id = $1 AND student_id = $2
		AND attempt_no = $3 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, sessionID, studentID, attemptNo)
	if err != nil || len(ps) == 0 {
		return nil, err
	}
	return &ps[0], nil
}

// Paper implements write.ExamStore.
func (ExamRepo) Paper(ctx context.Context, id uuid.UUID) (*write.ExamPaperRow, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	ps, err := papers(ctx, `SELECT `+read.ExamPaperColumns+` FROM assessment.exam_papers WHERE exam_paper_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, id)
	if err != nil || len(ps) == 0 {
		return nil, err
	}
	return &ps[0], nil
}

// InsertParticipations implements write.ExamStore.
func (ExamRepo) InsertParticipations(ctx context.Context, ps []write.ExamParticipationRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		// tenancyguard:ok INSERT in the session tenant (RLS insert policy checks tenant_id = infra.current_tenant_id())
		if _, err := tx.ExecContext(ctx, `INSERT INTO assessment.exam_participations (exam_session_id, student_id, tenant_id, school_id,
			status, effective_duration_sec, extra_time_percent) VALUES ($1,$2,$3,$4,'not_started',$5,$6) ON CONFLICT DO NOTHING`,
			p.SessionID, p.StudentID, p.TenantID, p.SchoolID, p.EffectiveDurationSec, p.ExtraTimePercent); err != nil {
			return fmt.Errorf("insert exam participation: %w", err)
		}
	}
	return nil
}

// Participation implements write.ExamStore.
func (ExamRepo) Participation(ctx context.Context, sessionID, studentID uuid.UUID, lock bool) (*write.ExamParticipationRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT exam_session_id, student_id, tenant_id, school_id, status, attempts_used, last_attempt_id, effective_duration_sec,
		extra_time_percent, version FROM assessment.exam_participations WHERE exam_session_id = $1 AND student_id = $2
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	var p write.ExamParticipationRow
	var last uuid.NullUUID
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, q, sessionID, studentID).Scan(&p.SessionID, &p.StudentID, &p.TenantID, &p.SchoolID, &p.Status,
		&p.AttemptsUsed, &last, &p.EffectiveDurationSec, &p.ExtraTimePercent, &p.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exam participation: %w", err)
	}
	p.LastAttemptID = nullUUID(last)
	return &p, nil
}

// UpdateParticipation implements write.ExamStore.
func (ExamRepo) UpdateParticipation(ctx context.Context, p write.ExamParticipationRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE assessment.exam_participations SET status = $3, attempts_used = $4, last_attempt_id = $5,
		version = version + 1, updated_at = now() WHERE exam_session_id = $1 AND student_id = $2
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, p.SessionID, p.StudentID, p.Status, p.AttemptsUsed, p.LastAttemptID)
	if err != nil {
		return fmt.Errorf("update exam participation: %w", err)
	}
	return nil
}

// SetAccommodations implements write.ExamStore.
func (ExamRepo) SetAccommodations(ctx context.Context, sessionID uuid.UUID, acc json.RawMessage, prev int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE assessment.exam_sessions SET accommodations = $3, version = version + 1, updated_at = now()
		WHERE exam_session_id = $1 AND version = $2 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, sessionID, prev, acc))
}

// SetParticipationTime implements write.ExamStore.
func (ExamRepo) SetParticipationTime(ctx context.Context, sessionID, studentID uuid.UUID, extraPercent, effectiveSec int) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE assessment.exam_participations SET extra_time_percent = $3, effective_duration_sec = $4,
		version = version + 1, updated_at = now() WHERE exam_session_id = $1 AND student_id = $2
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, sessionID, studentID, extraPercent, effectiveSec))
}

// MarkAbsent implements write.ExamStore.
func (ExamRepo) MarkAbsent(ctx context.Context, sessionID uuid.UUID) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE assessment.exam_participations SET status = 'absent', version = version + 1, updated_at = now()
		WHERE exam_session_id = $1 AND status = 'not_started' AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, sessionID)
	if err != nil {
		return 0, fmt.Errorf("mark absent: %w", err)
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// SetAttemptExam implements write.ExamStore.
func (ExamRepo) SetAttemptExam(ctx context.Context, attemptID, paperID uuid.UUID, deviceToken *string) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE assessment.attempts SET exam_paper_id = $2, active_device_token = $3 WHERE attempt_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, attemptID, paperID, deviceToken)
	if err != nil {
		return fmt.Errorf("set attempt exam: %w", err)
	}
	return nil
}

// SetAttemptDevice implements write.ExamStore.
func (ExamRepo) SetAttemptDevice(ctx context.Context, attemptID uuid.UUID, deviceToken *string) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE assessment.attempts SET active_device_token = $2 WHERE attempt_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, attemptID, deviceToken)
	if err != nil {
		return fmt.Errorf("set attempt device: %w", err)
	}
	return nil
}

// AttemptExam implements write.ExamStore.
func (ExamRepo) AttemptExam(ctx context.Context, attemptID uuid.UUID) (write.ExamAttemptInfo, error) {
	var out write.ExamAttemptInfo
	tx, err := txOf(ctx)
	if err != nil {
		return out, err
	}
	var paper uuid.NullUUID
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT exam_paper_id, active_device_token, submit_reason FROM assessment.attempts WHERE attempt_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, attemptID).Scan(&paper, &out.DeviceToken, &out.SubmitReason)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, fmt.Errorf("attempt exam: %w", err)
	}
	out.PaperID = nullUUID(paper)
	return out, nil
}

// SetSubmitReason implements write.ExamStore.
func (ExamRepo) SetSubmitReason(ctx context.Context, attemptID uuid.UUID, reason string) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE assessment.attempts SET submit_reason = $2 WHERE attempt_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, attemptID, reason)
	if err != nil {
		return fmt.Errorf("set submit reason: %w", err)
	}
	return nil
}

func attemptIDs(ctx context.Context, query string, args ...any) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exam attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// OpenAttempts implements write.ExamStore.
func (ExamRepo) OpenAttempts(ctx context.Context, sessionID uuid.UUID) ([]uuid.UUID, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return attemptIDs(ctx, `SELECT attempt_id FROM assessment.attempts WHERE context_type = 'exam_session' AND context_id = $1
		AND status IN ('in_progress','expired') AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY attempt_id`, sessionID)
}

// SubmittedAttempts implements write.ExamStore.
func (ExamRepo) SubmittedAttempts(ctx context.Context, sessionID uuid.UUID) ([]uuid.UUID, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return attemptIDs(ctx, `SELECT attempt_id FROM assessment.attempts WHERE context_type = 'exam_session' AND context_id = $1
		AND status = 'submitted' AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY attempt_id`, sessionID)
}

// GradedAttempts implements write.ExamStore.
func (ExamRepo) GradedAttempts(ctx context.Context, sessionID, questionVersionID uuid.UUID) ([]uuid.UUID, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on both tables
	return attemptIDs(ctx, `SELECT a.attempt_id FROM assessment.attempts a WHERE a.context_type = 'exam_session' AND a.context_id = $1
		AND a.status = 'graded' AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM assessment.attempt_items i WHERE i.attempt_id = a.attempt_id AND i.question_version_id = $2
			AND i.deleted_at IS NULL) ORDER BY a.attempt_id`, sessionID, questionVersionID)
}

// DueToOpen implements write.ExamStore (every tenant: tenancy.Bypass(platform_job)).
func (ExamRepo) DueToOpen(ctx context.Context, now time.Time, limit int) ([]write.DueAssignment, error) {
	// tenancyguard:ok job exam-window under tenancy.Bypass(platform_job): every tenant, deleted_at IS NULL
	return dueAssignments(ctx, `SELECT exam_session_id, tenant_id FROM assessment.exam_sessions WHERE status = 'scheduled'
		AND window_start <= $1 AND deleted_at IS NULL ORDER BY window_start LIMIT $2`, now, limit)
}

// DueToClose implements write.ExamStore (every tenant: tenancy.Bypass(platform_job)).
func (ExamRepo) DueToClose(ctx context.Context, now time.Time, limit int) ([]write.DueAssignment, error) {
	// tenancyguard:ok job exam-window under tenancy.Bypass(platform_job): every tenant, deleted_at IS NULL
	return dueAssignments(ctx, `SELECT exam_session_id, tenant_id FROM assessment.exam_sessions WHERE status IN ('scheduled','open')
		AND window_end <= $1 AND deleted_at IS NULL ORDER BY window_end LIMIT $2`, now, limit)
}

// ExpiredAttempts implements write.ExamStore (every tenant: tenancy.Bypass(platform_job); SKIP LOCKED lets several
// workers share the work without submitting an attempt twice).
func (ExamRepo) ExpiredAttempts(ctx context.Context, cutoff time.Time, limit int) ([]write.ExamAttemptRef, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok job exam.autosubmit under tenancy.Bypass(platform_job): every tenant, deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT attempt_id, tenant_id FROM assessment.attempts WHERE mode = 'exam'
		AND status IN ('in_progress','expired') AND deadline_at < $1 AND deleted_at IS NULL ORDER BY deadline_at LIMIT $2
		FOR UPDATE SKIP LOCKED`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("expired exam attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ExamAttemptRef
	for rows.Next() {
		var r write.ExamAttemptRef
		if err := rows.Scan(&r.AttemptID, &r.TenantID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LatestAnswers implements write.ExamStore (attempt_answers is append-only: no deleted_at).
func (ExamRepo) LatestAnswers(ctx context.Context, attemptID uuid.UUID) (map[int]json.RawMessage, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()); append-only table without deleted_at
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT ON (position) position, response FROM assessment.attempt_answers
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

// ExamObjectiveResults implements write.ExamStore: per objective of the items of the student's graded exam attempts
// since a time, in sessions whose results are visible (immediate, after_close closed, released).
func (ExamRepo) ExamObjectiveResults(ctx context.Context, studentID uuid.UUID, since time.Time) ([]write.ExamObjectiveResult, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on every table
	rows, err := tx.QueryContext(ctx, `SELECT o.objective_id::uuid, count(*) FILTER (WHERE i.final_is_correct), count(*)
		FROM assessment.attempts a
		JOIN assessment.exam_sessions s ON s.exam_session_id = a.context_id AND s.deleted_at IS NULL
		JOIN assessment.attempt_items i ON i.attempt_id = a.attempt_id AND i.deleted_at IS NULL
		CROSS JOIN LATERAL jsonb_array_elements_text(i.objective_ids) AS o(objective_id)
		WHERE a.student_id = $1 AND a.mode = 'exam' AND a.context_type = 'exam_session' AND a.status = 'graded' AND a.graded_at >= $2
			AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL
			AND (s.status = 'results_released' OR (s.result_release = 'immediate' AND s.status <> 'cancelled')
				OR (s.result_release = 'after_close' AND s.status = 'closed'))
		GROUP BY o.objective_id ORDER BY o.objective_id`, studentID, since)
	if err != nil {
		return nil, fmt.Errorf("exam objective results: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ExamObjectiveResult
	for rows.Next() {
		var r write.ExamObjectiveResult
		if err := rows.Scan(&r.ObjectiveID, &r.Correct, &r.Total); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LatestResponseMS implements write.ExamStore (attempt_answers is append-only: no deleted_at).
func (ExamRepo) LatestResponseMS(ctx context.Context, attemptID uuid.UUID) (map[int]int64, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()); append-only table without deleted_at
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT ON (position) position, COALESCE(response_ms, 0) FROM assessment.attempt_answers
		WHERE attempt_id = $1 AND tenant_id = infra.current_tenant_id() ORDER BY position, answered_at DESC, attempt_no DESC`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("latest response times: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[int]int64{}
	for rows.Next() {
		var pos int
		var ms int64
		if err := rows.Scan(&pos, &ms); err != nil {
			return nil, err
		}
		out[pos] = ms
	}
	return out, rows.Err()
}

var _ write.ExamExclusions = ExamRepo{}

// ExcludeParticipations implements write.ExamExclusions.
func (ExamRepo) ExcludeParticipations(ctx context.Context, sessionID uuid.UUID, students []uuid.UUID, by uuid.UUID) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	// tenancyguard:ok UPDATE in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE assessment.exam_participations SET deleted_at = now(), deleted_by = $3, delete_reason = 'efficacy_withdrawn',
		updated_at = now() WHERE exam_session_id = $1 AND student_id = ANY($2::uuid[]) AND status = 'not_started'
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, sessionID, uuidArray(students), by)
	if err != nil {
		return 0, fmt.Errorf("exclude participations: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
