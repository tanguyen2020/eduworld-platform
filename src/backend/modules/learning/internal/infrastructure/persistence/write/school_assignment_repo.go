package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// SchoolAssignmentRepo implements write.SchoolAssignmentStore with explicit SQL in the transaction of ctx (F041): every
// lms.assignment* table is tenant (RLS T, infra.current_tenant_id()) and soft-delete only (AGENTS R21). The references
// (courses, projections, study groups, content versions) are those of the LMS of F040.
type SchoolAssignmentRepo struct{ LMSRepo }

var _ write.SchoolAssignmentStore = SchoolAssignmentRepo{}

const assignmentColumns = `a.assignment_id, a.tenant_id, a.school_id, a.course_id, a.subject_id, a.created_by, a.created_by_role, a.title,
	a.instructions, a.content_type, a.content_id, a.pinned_version_id, a.follow_latest, a.activity_type, a.mode, a.open_at, a.due_at,
	a.allow_late, a.late_window_hours, a.max_attempts, a.score_policy, a.feedback_timing, a.result_release, a.status, a.exam_session_id,
	array_to_string(a.objective_ids, ','), a.published_at, a.closed_at, a.results_released_at, a.version, a.created_at, a.updated_at,
	a.updated_by`

// ScanAssignment scans assignmentColumns (shared with the read model).
func ScanAssignment(row interface{ Scan(...any) error }) (write.AssignmentRow, error) {
	var a write.AssignmentRow
	var instructions, activityType sql.NullString
	var pinned, exam, updatedBy uuid.NullUUID
	var published, closed, released sql.NullTime
	var los string
	err := row.Scan(&a.ID, &a.TenantID, &a.SchoolID, &a.CourseID, &a.SubjectID, &a.CreatedBy, &a.CreatedByRole, &a.Title, &instructions,
		&a.ContentType, &a.ContentID, &pinned, &a.FollowLatest, &activityType, &a.Mode, &a.OpenAt, &a.DueAt, &a.AllowLate, &a.LateHours,
		&a.MaxAttempts, &a.ScorePolicy, &a.FeedbackTiming, &a.ResultRelease, &a.Status, &exam, &los, &published, &closed, &released,
		&a.Version, &a.CreatedAt, &a.UpdatedAt, &updatedBy)
	if err != nil {
		return a, err
	}
	if instructions.Valid {
		s := instructions.String
		a.Instructions = &s
	}
	if activityType.Valid {
		s := activityType.String
		a.ActivityType = &s
	}
	a.PinnedVersionID, a.ExamSessionID, a.UpdatedBy = nullID(pinned), nullID(exam), nullID(updatedBy)
	a.ObjectiveIDs = parseIDs(los)
	a.PublishedAt, a.ClosedAt, a.ReleasedAt = nullTime(published), nullTime(closed), nullTime(released)
	a.OpenAt, a.DueAt, a.CreatedAt, a.UpdatedAt = a.OpenAt.UTC(), a.DueAt.UTC(), a.CreatedAt.UTC(), a.UpdatedAt.UTC()
	return a, nil
}

// AssignmentColumns is the select list of ScanAssignment on alias a.
func AssignmentColumns() string { return assignmentColumns }

func insertTargets(ctx context.Context, tx *sql.Tx, a write.AssignmentRow) error {
	for i, t := range a.Targets {
		// tenancyguard:ok tenant_id of the row is the tenant of the session (RLS tenant_insert WITH CHECK)
		if _, err := tx.ExecContext(ctx, `INSERT INTO lms.assignment_targets (assignment_target_id, tenant_id, school_id, assignment_id,
			target_type, target_ref_id, student_ids, position) VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6::uuid[], $7)`,
			ids.New(), a.SchoolID, a.ID, t.Type, t.RefID, uuidArray(t.StudentIDs), i+1); err != nil {
			return fmt.Errorf("assignment target: %w", err)
		}
	}
	return nil
}

// InsertAssignment implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) InsertAssignment(ctx context.Context, a write.AssignmentRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant_id of the row is the tenant of the session (RLS tenant_insert WITH CHECK)
	if _, err := tx.ExecContext(ctx, `INSERT INTO lms.assignments (assignment_id, tenant_id, school_id, course_id, subject_id, created_by,
		created_by_role, title, instructions, content_type, content_id, pinned_version_id, follow_latest, activity_type, mode, open_at, due_at,
		allow_late, late_window_hours, max_attempts, score_policy, feedback_timing, result_release, status, objective_ids, created_at,
		updated_at, updated_by)
		VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22,
		$23, $24::uuid[], $25, $25, $5)`,
		a.ID, a.SchoolID, a.CourseID, a.SubjectID, a.CreatedBy, a.CreatedByRole, a.Title, a.Instructions, a.ContentType, a.ContentID,
		a.PinnedVersionID, a.FollowLatest, a.ActivityType, a.Mode, a.OpenAt, a.DueAt, a.AllowLate, a.LateHours, a.MaxAttempts, a.ScorePolicy,
		a.FeedbackTiming, a.ResultRelease, a.Status, uuidArray(a.ObjectiveIDs), a.UpdatedAt); err != nil {
		return fmt.Errorf("assignment: %w", err)
	}
	return insertTargets(ctx, tx, a)
}

// Assignment implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) Assignment(ctx context.Context, id uuid.UUID, forUpdate bool) (*write.AssignmentRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE"
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	a, err := one(ScanAssignment(tx.QueryRowContext(ctx, `SELECT `+assignmentColumns+` FROM lms.assignments a
		WHERE a.assignment_id = $1 AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL`+lock, id)))
	if err != nil || a == nil {
		return a, err
	}
	a.Targets, err = AssignmentTargets(ctx, tx, a.ID)
	return a, err
}

// AssignmentTargets reads the live targets of an assignment with the number of students of each in the snapshot.
func AssignmentTargets(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, id uuid.UUID) ([]write.TargetRow, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on both lms tables
	rows, err := q.QueryContext(ctx, `SELECT t.target_type, t.target_ref_id, array_to_string(t.student_ids, ','),
		(SELECT count(*) FROM lms.assignment_target_students s WHERE s.assignment_id = t.assignment_id AND s.deleted_at IS NULL
			AND s.tenant_id = t.tenant_id AND (t.target_type <> 'class' OR s.class_id_at_assign = t.target_ref_id))
		FROM lms.assignment_targets t WHERE t.assignment_id = $1 AND t.tenant_id = infra.current_tenant_id() AND t.deleted_at IS NULL
		ORDER BY t.position`, id)
	if err != nil {
		return nil, fmt.Errorf("assignment targets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.TargetRow
	for rows.Next() {
		var t write.TargetRow
		var ref uuid.NullUUID
		var students string
		if err := rows.Scan(&t.Type, &ref, &students, &t.StudentCount); err != nil {
			return nil, err
		}
		t.RefID, t.StudentIDs = nullID(ref), parseIDs(students)
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateAssignment implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) UpdateAssignment(ctx context.Context, a write.AssignmentRow, ifVersion int64, replaceTargets bool) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	ok, err := affected(tx.ExecContext(ctx, `UPDATE lms.assignments SET title = $3, instructions = $4, pinned_version_id = $5,
		follow_latest = $6, mode = $7, open_at = $8, due_at = $9, allow_late = $10, late_window_hours = $11, max_attempts = $12,
		score_policy = $13, feedback_timing = $14, result_release = $15, status = $16::varchar, exam_session_id = $17, published_at = $18,
		closed_at = $19, results_released_at = $20, cancelled_at = CASE WHEN $16::varchar = 'cancelled' THEN now() ELSE cancelled_at END,
		archived_at = CASE WHEN $16::varchar = 'archived' THEN now() ELSE archived_at END, objective_ids = $21::uuid[], updated_by = $22,
		version = version + 1, updated_at = now()
		WHERE assignment_id = $1 AND version = $2 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		a.ID, ifVersion, a.Title, a.Instructions, a.PinnedVersionID, a.FollowLatest, a.Mode, a.OpenAt, a.DueAt, a.AllowLate, a.LateHours,
		a.MaxAttempts, a.ScorePolicy, a.FeedbackTiming, a.ResultRelease, a.Status, a.ExamSessionID, a.PublishedAt, a.ClosedAt, a.ReleasedAt,
		uuidArray(a.ObjectiveIDs), a.UpdatedBy))
	if err != nil || !ok || !replaceTargets {
		return ok, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	if _, err := tx.ExecContext(ctx, `UPDATE lms.assignment_targets SET deleted_at = now(), deleted_by = $2, delete_reason = 'replaced',
		updated_at = now() WHERE assignment_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, a.ID, a.UpdatedBy); err != nil {
		return false, fmt.Errorf("assignment targets: %w", err)
	}
	return true, insertTargets(ctx, tx, a)
}

// InsertTargetStudents implements write.SchoolAssignmentStore: one row per student (a student is targeted once) and its
// submission not_started.
func (SchoolAssignmentRepo) InsertTargetStudents(ctx context.Context, a write.AssignmentRow, students []write.TargetStudent) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range students {
		// tenancyguard:ok tenant_id of the row is the tenant of the session (RLS tenant_insert WITH CHECK)
		ok, err := affected(tx.ExecContext(ctx, `INSERT INTO lms.assignment_target_students (assignment_target_student_id, tenant_id, school_id,
			assignment_id, student_id, class_id_at_assign, source) VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6)
			ON CONFLICT (assignment_id, student_id) WHERE deleted_at IS NULL DO NOTHING`,
			ids.New(), a.SchoolID, a.ID, s.StudentID, s.ClassID, s.Source))
		if err != nil {
			return n, fmt.Errorf("assignment student: %w", err)
		}
		if !ok {
			continue
		}
		n++
		// tenancyguard:ok tenant_id of the row is the tenant of the session (RLS tenant_insert WITH CHECK)
		if _, err := tx.ExecContext(ctx, `INSERT INTO lms.assignment_submissions (assignment_submission_id, tenant_id, school_id, assignment_id,
			student_id, class_id) VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5)
			ON CONFLICT (assignment_id, student_id) WHERE deleted_at IS NULL DO NOTHING`, ids.New(), a.SchoolID, a.ID, s.StudentID, s.ClassID); err != nil {
			return n, fmt.Errorf("assignment submission: %w", err)
		}
	}
	return n, nil
}

// TargetStudents implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) TargetStudents(ctx context.Context, assignmentID uuid.UUID) ([]write.TargetStudent, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT student_id, class_id_at_assign, source FROM lms.assignment_target_students
		WHERE assignment_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY student_id`, assignmentID)
	if err != nil {
		return nil, fmt.Errorf("assignment students: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.TargetStudent
	for rows.Next() {
		var s write.TargetStudent
		var class uuid.NullUUID
		if err := rows.Scan(&s.StudentID, &class, &s.Source); err != nil {
			return nil, err
		}
		s.ClassID = nullID(class)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Targeted implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) Targeted(ctx context.Context, assignmentID, studentID uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM lms.assignment_target_students WHERE assignment_id = $1 AND student_id = $2
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL)`, assignmentID, studentID).Scan(&ok)
	return ok, err
}

func (SchoolAssignmentRepo) assignments(ctx context.Context, where string, args ...any) ([]write.AssignmentRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL; where is fixed by the callers
	rows, err := tx.QueryContext(ctx, `SELECT `+assignmentColumns+` FROM lms.assignments a
		WHERE a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL AND `+where+` ORDER BY a.assignment_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AssignmentRow
	for rows.Next() {
		a, err := ScanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// OpenClassAssignments implements write.SchoolAssignmentStore.
func (r SchoolAssignmentRepo) OpenClassAssignments(ctx context.Context, classID uuid.UUID) ([]write.AssignmentRow, error) {
	return r.assignments(ctx, `a.status = 'open' AND EXISTS (SELECT 1 FROM lms.assignment_targets t WHERE t.assignment_id = a.assignment_id
		AND t.target_type = 'class' AND t.target_ref_id = $1 AND t.deleted_at IS NULL)`, classID)
}

// OpenAssignmentsOfStudent implements write.SchoolAssignmentStore.
func (r SchoolAssignmentRepo) OpenAssignmentsOfStudent(ctx context.Context, studentID uuid.UUID) ([]write.AssignmentRow, error) {
	return r.assignments(ctx, `a.status IN ('scheduled','open') AND EXISTS (SELECT 1 FROM lms.assignment_target_students s
		WHERE s.assignment_id = a.assignment_id AND s.student_id = $1 AND s.deleted_at IS NULL)`, studentID)
}

const submissionColumns = `s.assignment_id, s.tenant_id, s.school_id, s.student_id, s.class_id, s.status, s.attempts_used, s.current_attempt_id,
	s.best_attempt_id, s.score::float8, s.max_score::float8, s.submitted_at, s.rapid_guess_count, s.due_at_override, s.completed_at, s.version`

// ScanSubmission scans submissionColumns (shared with the read model).
func ScanSubmission(row interface{ Scan(...any) error }) (write.SubmissionRow, error) {
	var s write.SubmissionRow
	var class, current, best uuid.NullUUID
	var score, maxScore sql.NullFloat64
	var submitted, due, completed sql.NullTime
	err := row.Scan(&s.AssignmentID, &s.TenantID, &s.SchoolID, &s.StudentID, &class, &s.Status, &s.AttemptsUsed, &current, &best, &score,
		&maxScore, &submitted, &s.RapidGuessCount, &due, &completed, &s.Version)
	s.ClassID, s.CurrentAttemptID, s.BestAttemptID = nullID(class), nullID(current), nullID(best)
	if score.Valid {
		v := score.Float64
		s.Score = &v
	}
	if maxScore.Valid {
		v := maxScore.Float64
		s.MaxScore = &v
	}
	s.SubmittedAt, s.DueAtOverride, s.CompletedAt = nullTime(submitted), nullTime(due), nullTime(completed)
	return s, err
}

// SubmissionColumns is the select list of ScanSubmission on alias s.
func SubmissionColumns() string { return submissionColumns }

// Submission implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) Submission(ctx context.Context, assignmentID, studentID uuid.UUID, forUpdate bool) (*write.SubmissionRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE"
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return one(ScanSubmission(tx.QueryRowContext(ctx, `SELECT `+submissionColumns+` FROM lms.assignment_submissions s
		WHERE s.assignment_id = $1 AND s.student_id = $2 AND s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL`+lock,
		assignmentID, studentID)))
}

// UpsertSubmission implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) UpsertSubmission(ctx context.Context, s write.SubmissionRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant_id of the row is the tenant of the session (RLS insert/update policies); live row only (partial unique)
	_, err = tx.ExecContext(ctx, `INSERT INTO lms.assignment_submissions (assignment_submission_id, tenant_id, school_id, assignment_id,
		student_id, class_id, status, attempts_used, current_attempt_id, best_attempt_id, score, max_score, submitted_at, rapid_guess_count,
		due_at_override, completed_at) VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (assignment_id, student_id) WHERE deleted_at IS NULL DO UPDATE SET status = EXCLUDED.status,
		attempts_used = EXCLUDED.attempts_used, current_attempt_id = EXCLUDED.current_attempt_id, best_attempt_id = EXCLUDED.best_attempt_id,
		score = EXCLUDED.score, max_score = EXCLUDED.max_score, submitted_at = EXCLUDED.submitted_at,
		rapid_guess_count = EXCLUDED.rapid_guess_count, due_at_override = EXCLUDED.due_at_override, completed_at = EXCLUDED.completed_at,
		class_id = COALESCE(lms.assignment_submissions.class_id, EXCLUDED.class_id), version = lms.assignment_submissions.version + 1,
		updated_at = now()`,
		ids.New(), s.SchoolID, s.AssignmentID, s.StudentID, s.ClassID, s.Status, s.AttemptsUsed, s.CurrentAttemptID, s.BestAttemptID, s.Score,
		s.MaxScore, s.SubmittedAt, s.RapidGuessCount, s.DueAtOverride, s.CompletedAt)
	if err != nil {
		return fmt.Errorf("assignment submission: %w", err)
	}
	return nil
}

// HasSubmission implements write.SchoolAssignmentStore: a student started an attempt of the assignment.
func (SchoolAssignmentRepo) HasSubmission(ctx context.Context, assignmentID uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM assessment.attempts WHERE context_type = 'assignment' AND context_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL)`, assignmentID).Scan(&ok)
	return ok, err
}

// MarkMissed implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) MarkMissed(ctx context.Context, assignmentID uuid.UUID, now time.Time, late time.Duration) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL on both tables
	res, err := tx.ExecContext(ctx, `UPDATE lms.assignment_submissions s SET status = 'missed', version = s.version + 1, updated_at = now()
		FROM lms.assignments a WHERE a.assignment_id = s.assignment_id AND a.deleted_at IS NULL AND s.assignment_id = $1
		AND s.status IN ('not_started','in_progress') AND COALESCE(s.due_at_override, a.due_at) + make_interval(secs => $3) < $2
		AND s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL`, assignmentID, now, late.Seconds())
	if err != nil {
		return 0, fmt.Errorf("missed: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// GradedAttempts implements write.SchoolAssignmentStore: the effective evaluation of each graded attempt (re-evaluations
// included) and its rapid guesses.
func (SchoolAssignmentRepo) GradedAttempts(ctx context.Context, assignmentID, studentID uuid.UUID) ([]write.GradedAttempt, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) on attempts, evaluations and the append-only answers; deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT a.attempt_id, e.score::float8, e.max_score::float8, a.submitted_at,
		(SELECT count(*) FROM assessment.attempt_answers x WHERE x.attempt_id = a.attempt_id AND x.rapid_guess
			AND x.tenant_id = a.tenant_id)
		FROM assessment.attempts a JOIN assessment.evaluations e ON e.attempt_id = a.attempt_id AND e.effective AND e.deleted_at IS NULL
		WHERE a.context_type = 'assignment' AND a.context_id = $1 AND a.student_id = $2 AND a.status = 'graded'
		AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL ORDER BY a.submitted_at`, assignmentID, studentID)
	if err != nil {
		return nil, fmt.Errorf("graded attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.GradedAttempt
	for rows.Next() {
		var g write.GradedAttempt
		var at sql.NullTime
		if err := rows.Scan(&g.AttemptID, &g.Score, &g.MaxScore, &at, &g.RapidGuesses); err != nil {
			return nil, err
		}
		if at.Valid {
			g.SubmittedAt = at.Time.UTC()
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// AttemptsOfContext implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) AttemptsOfContext(ctx context.Context, assignmentID, studentID uuid.UUID) ([]write.AttemptOfContext, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT attempt_id, status, started_at, submitted_at FROM assessment.attempts
		WHERE context_type = 'assignment' AND context_id = $1 AND student_id = $2 AND tenant_id = infra.current_tenant_id()
		AND deleted_at IS NULL ORDER BY started_at DESC`, assignmentID, studentID)
	if err != nil {
		return nil, fmt.Errorf("attempts of the assignment: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AttemptOfContext
	for rows.Next() {
		var a write.AttemptOfContext
		var at sql.NullTime
		if err := rows.Scan(&a.AttemptID, &a.Status, &a.StartedAt, &at); err != nil {
			return nil, err
		}
		a.StartedAt, a.SubmittedAt = a.StartedAt.UTC(), nullTime(at)
		out = append(out, a)
	}
	return out, rows.Err()
}

// AttemptContext implements write.SchoolAssignmentStore: context type, context id and student of an attempt.
func (SchoolAssignmentRepo) AttemptContext(ctx context.Context, attemptID uuid.UUID) (string, uuid.UUID, uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return "", uuid.Nil, uuid.Nil, err
	}
	var typ string
	var id, student uuid.UUID
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT context_type, context_id, student_id FROM assessment.attempts WHERE attempt_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, attemptID).Scan(&typ, &id, &student)
	if errors.Is(err, sql.ErrNoRows) {
		return "", uuid.Nil, uuid.Nil, nil
	}
	return typ, id, student, err
}

// SetException implements write.SchoolAssignmentStore: the previous exception of the same kind is replaced (soft-deleted).
func (SchoolAssignmentRepo) SetException(ctx context.Context, tenant, school, assignmentID, studentID uuid.UUID, kind string, dueAt *time.Time,
	reason string, by uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	if _, err := tx.ExecContext(ctx, `UPDATE lms.assignment_student_exceptions SET deleted_at = now(), deleted_by = $4, delete_reason = 'replaced',
		updated_at = now() WHERE assignment_id = $1 AND student_id = $2 AND kind = $3 AND tenant_id = infra.current_tenant_id()
		AND deleted_at IS NULL`, assignmentID, studentID, kind, by); err != nil {
		return fmt.Errorf("exception: %w", err)
	}
	// tenancyguard:ok tenant_id of the row is the tenant of the session (RLS tenant_insert WITH CHECK)
	if _, err := tx.ExecContext(ctx, `INSERT INTO lms.assignment_student_exceptions (exception_id, tenant_id, school_id, assignment_id,
		student_id, kind, due_at, reason, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		ids.New(), tenant, school, assignmentID, studentID, kind, dueAt, reason, by); err != nil {
		return fmt.Errorf("exception: %w", err)
	}
	return nil
}

// RemindersOn implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) RemindersOn(ctx context.Context, assignmentID uuid.UUID, day string) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok append-only table of the session tenant (RLS + infra.current_tenant_id()), no deleted_at
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM lms.assignment_reminders WHERE assignment_id = $1 AND reminded_on = $2::date
		AND tenant_id = infra.current_tenant_id()`, assignmentID, day).Scan(&n)
	return n, err
}

// InsertReminder implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) InsertReminder(ctx context.Context, tenant, school, assignmentID uuid.UUID, day string, recipients int, by uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant_id of the row is the tenant of the session (RLS tenant_insert WITH CHECK)
	_, err = tx.ExecContext(ctx, `INSERT INTO lms.assignment_reminders (reminder_id, tenant_id, school_id, assignment_id, reminded_on, recipients,
		created_by) VALUES ($1, $2, $3, $4, $5::date, $6, $7)`, ids.New(), tenant, school, assignmentID, day, recipients, by)
	return err
}

// PendingStudents implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) PendingStudents(ctx context.Context, assignmentID uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT student_id FROM lms.assignment_submissions WHERE assignment_id = $1
		AND status IN ('not_started','in_progress') AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY student_id`, assignmentID)
	if err != nil {
		return nil, fmt.Errorf("pending students: %w", err)
	}
	return scanIDs(rows)
}

func dueAssignments(ctx context.Context, query string, now time.Time, limit int) ([]write.DueAssignment, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("due assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.DueAssignment
	for rows.Next() {
		var d write.DueAssignment
		if err := rows.Scan(&d.ID, &d.TenantID); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DueToOpen implements write.SchoolAssignmentStore (every tenant: tenancy.Bypass(platform_job)).
func (SchoolAssignmentRepo) DueToOpen(ctx context.Context, now time.Time, limit int) ([]write.DueAssignment, error) {
	// tenancyguard:ok job assignment-scheduler under tenancy.Bypass(platform_job): every tenant, deleted_at IS NULL
	return dueAssignments(ctx, `SELECT assignment_id, tenant_id FROM lms.assignments WHERE status = 'scheduled' AND open_at <= $1
		AND deleted_at IS NULL ORDER BY open_at LIMIT $2`, now, limit)
}

// DueToClose implements write.SchoolAssignmentStore: open assignments whose due date plus late window is past and with no
// personal extension still running.
func (SchoolAssignmentRepo) DueToClose(ctx context.Context, now time.Time, limit int) ([]write.DueAssignment, error) {
	// tenancyguard:ok job assignment-scheduler under tenancy.Bypass(platform_job): every tenant, deleted_at IS NULL on both tables
	return dueAssignments(ctx, `SELECT a.assignment_id, a.tenant_id FROM lms.assignments a WHERE a.status = 'open'
		AND a.due_at + make_interval(hours => CASE WHEN a.allow_late THEN a.late_window_hours ELSE 0 END) < $1 AND a.deleted_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM lms.assignment_submissions s WHERE s.assignment_id = a.assignment_id AND s.deleted_at IS NULL
			AND s.status IN ('not_started','in_progress') AND s.due_at_override IS NOT NULL
			AND s.due_at_override + make_interval(hours => CASE WHEN a.allow_late THEN a.late_window_hours ELSE 0 END) >= $1)
		ORDER BY a.due_at LIMIT $2`, now, limit)
}

// ClassStudents implements write.SchoolAssignmentStore: the active students with an active enrollment in the class.
func (SchoolAssignmentRepo) ClassStudents(ctx context.Context, classID uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projections ref.enrollments/ref.students of the session tenant (RLS + infra.current_tenant_id()); deleted_at IS NULL on students
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT e.student_id FROM ref.enrollments e JOIN ref.students s ON s.student_id = e.student_id
		AND s.tenant_id = e.tenant_id AND s.status = 'active' AND s.deleted_at IS NULL
		WHERE e.class_id = $1 AND e.status = 'active' AND e.tenant_id = infra.current_tenant_id() ORDER BY e.student_id`, classID)
	if err != nil {
		return nil, fmt.Errorf("students of the class: %w", err)
	}
	return scanIDs(rows)
}

// ClassOf implements write.SchoolAssignmentStore: the active class of a student (one active class, ADR-0060).
func (SchoolAssignmentRepo) ClassOf(ctx context.Context, studentID uuid.UUID) (*uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	// tenancyguard:ok projection ref.enrollments (no deleted_at, status active) of the session tenant (RLS + infra.current_tenant_id())
	err = tx.QueryRowContext(ctx, `SELECT class_id FROM ref.enrollments WHERE student_id = $1 AND status = 'active'
		AND tenant_id = infra.current_tenant_id() ORDER BY start_date DESC LIMIT 1`, studentID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// CurrentVersion implements write.SchoolAssignmentStore: the version the item points to, visible in the scope of ctx
// (RLS content_read); an item without current version is returned with status draft and no version.
func (r SchoolAssignmentRepo) CurrentVersion(ctx context.Context, contentType string, itemID uuid.UUID) (*write.VersionAccess, uuid.UUID, error) {
	t, ok := contentTables[contentType]
	if !ok {
		return nil, uuid.Nil, nil
	}
	tx, err := txOf(ctx)
	if err != nil {
		return nil, uuid.Nil, err
	}
	item, key := t[0], t[1]
	var current, tenant uuid.NullUUID
	// tenancyguard:ok hybrid content: RLS content_read keeps tenant rows and assigned platform rows; deleted_at IS NULL (fixed identifiers)
	err = tx.QueryRowContext(ctx, `SELECT current_version_id, tenant_id FROM content.`+item+` WHERE `+key+`_id = $1 AND deleted_at IS NULL`,
		itemID).Scan(&current, &tenant)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, uuid.Nil, nil
	}
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("content item: %w", err)
	}
	if !current.Valid {
		return &write.VersionAccess{ItemID: itemID, Status: "draft", Platform: !tenant.Valid, Visible: true}, uuid.Nil, nil
	}
	v, err := r.ContentVersion(ctx, contentType, current.UUID)
	return v, current.UUID, err
}

// ActivityType implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) ActivityType(ctx context.Context, versionID uuid.UUID) (string, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return "", err
	}
	var typ sql.NullString
	// tenancyguard:ok hybrid content: RLS content_read; deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT activity_type FROM content.activity_versions WHERE activity_version_id = $1 AND deleted_at IS NULL`,
		versionID).Scan(&typ)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return typ.String, err
}

// GameActivity implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) GameActivity(ctx context.Context, itemType string, itemID uuid.UUID) (*write.GameActivityRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var g write.GameActivityRow
	var los string
	// tenancyguard:ok projection ref.game_activities (no deleted_at) of the session tenant (RLS + infra.current_tenant_id())
	err = tx.QueryRowContext(ctx, `SELECT item_type, item_id, current_version_id, status, array_to_string(objective_ids, ',')
		FROM ref.game_activities WHERE item_type = $1 AND item_id = $2 AND tenant_id = infra.current_tenant_id()`, itemType, itemID).
		Scan(&g.ItemType, &g.ItemID, &g.CurrentVersionID, &g.Status, &los)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	g.ObjectiveIDs = parseIDs(los)
	return &g, nil
}

// PendingAssignments implements write.SchoolAssignmentStore.
func (SchoolAssignmentRepo) PendingAssignments(ctx context.Context, studentID uuid.UUID) ([]write.PendingAssignment, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on both tables
	rows, err := tx.QueryContext(ctx, `SELECT a.assignment_id, a.title, a.subject_id, array_to_string(a.objective_ids, ','),
		COALESCE(s.due_at_override, a.due_at) FROM lms.assignments a JOIN lms.assignment_submissions s ON s.assignment_id = a.assignment_id
			AND s.student_id = $1 AND s.tenant_id = a.tenant_id AND s.deleted_at IS NULL AND s.status IN ('not_started','in_progress')
		WHERE a.status = 'open' AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL ORDER BY 5`, studentID)
	if err != nil {
		return nil, fmt.Errorf("pending assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.PendingAssignment
	for rows.Next() {
		var p write.PendingAssignment
		var los string
		if err := rows.Scan(&p.AssignmentID, &p.Title, &p.SubjectID, &los, &p.DueAt); err != nil {
			return nil, err
		}
		p.ObjectiveIDs, p.DueAt = parseIDs(los), p.DueAt.UTC()
		out = append(out, p)
	}
	return out, rows.Err()
}
