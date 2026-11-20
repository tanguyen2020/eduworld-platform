package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// SchoolAssignmentModel implements read.SchoolAssignmentReadModel with explicit SQL in the read transaction of ctx (TenantScope:
// RLS + infra.current_tenant_id(), deleted_at IS NULL).
type SchoolAssignmentModel struct{}

var _ read.SchoolAssignmentReadModel = SchoolAssignmentModel{}

const assignmentSelect = `a.assignment_id, a.tenant_id, a.school_id, a.course_id, a.subject_id, a.created_by, a.created_by_role, a.title,
	a.instructions, a.content_type, a.content_id, a.pinned_version_id, a.follow_latest, a.mode, a.open_at, a.due_at, a.allow_late,
	a.late_window_hours, a.max_attempts, a.score_policy, a.feedback_timing, a.result_release, a.status, a.exam_session_id,
	a.results_released_at, a.version`

func scanAssignmentView(row interface{ Scan(...any) error }) (read.AssignmentRecord, error) {
	var r read.AssignmentRecord
	v := &r.View
	var instructions sql.NullString
	var pinned, exam uuid.NullUUID
	var released sql.NullTime
	err := row.Scan(&v.AssignmentID, &v.TenantID, &v.SchoolID, &v.CourseID, &r.SubjectID, &v.CreatedBy, &v.CreatedByRole, &v.Title,
		&instructions, &v.ContentRef.ContentType, &v.ContentRef.ContentID, &pinned, &v.FollowLatest, &v.Mode, &v.OpenAt, &v.DueAt,
		&v.LatePolicy.AllowLate, &v.LatePolicy.LateWindowHours, &v.MaxAttempts, &v.ScorePolicy, &v.FeedbackTiming, &v.ResultRelease, &v.Status,
		&exam, &released, &v.RowVersion)
	if instructions.Valid {
		s := instructions.String
		v.Instructions = &s
	}
	v.ContentRef.PinnedVersionID, v.ExamSessionID = nullUUID(pinned), nullUUID(exam)
	if released.Valid {
		t := released.Time.UTC()
		v.ResultsReleasedAt = &t
	}
	v.OpenAt, v.DueAt, v.Targets = v.OpenAt.UTC(), v.DueAt.UTC(), []dto.AssignmentTargetView{}
	return r, err
}

func (SchoolAssignmentModel) targets(ctx context.Context, q *sql.Tx, recs []read.AssignmentRecord) error {
	if len(recs) == 0 {
		return nil
	}
	idx := map[uuid.UUID]int{}
	idList := make([]uuid.UUID, len(recs))
	for i, r := range recs {
		idx[r.View.AssignmentID], idList[i] = i, r.View.AssignmentID
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT assignment_id, target_type, target_ref_id, array_to_string(student_ids, ',')
		FROM lms.assignment_targets WHERE assignment_id = ANY($1::uuid[]) AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL
		ORDER BY assignment_id, position`, uuidArray(idList))
	if err != nil {
		return fmt.Errorf("assignment targets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var t dto.AssignmentTargetView
		var ref uuid.NullUUID
		var students string
		if err := rows.Scan(&id, &t.TargetType, &ref, &students); err != nil {
			return err
		}
		t.TargetID = nullUUID(ref)
		for _, s := range strings.Split(students, ",") {
			if x, err := uuid.Parse(s); err == nil {
				t.StudentIDs = append(t.StudentIDs, x)
			}
		}
		i := idx[id]
		recs[i].View.Targets = append(recs[i].View.Targets, t)
	}
	return rows.Err()
}

// scopeCondition is the OR of the (school, subject) pairs of the reader; "TRUE" when a pair is unlimited.
func scopeCondition(scopes []read.AssignmentScope, next int) (string, []any) {
	var conds []string
	var args []any
	for _, s := range scopes {
		var parts []string
		if s.SchoolID != nil {
			args = append(args, *s.SchoolID)
			parts = append(parts, fmt.Sprintf("a.school_id = $%d", next+len(args)-1))
		}
		if s.SubjectID != nil {
			args = append(args, *s.SubjectID)
			parts = append(parts, fmt.Sprintf("a.subject_id = $%d", next+len(args)-1))
		}
		if len(parts) == 0 {
			return "TRUE", nil
		}
		conds = append(conds, "("+strings.Join(parts, " AND ")+")")
	}
	if len(conds) == 0 {
		return "FALSE", nil
	}
	return "(" + strings.Join(conds, " OR ") + ")", args
}

// Assignments implements read.SchoolAssignmentReadModel (newest first, keyset on the id).
func (m SchoolAssignmentModel) Assignments(ctx context.Context, f read.AssignmentFilter) ([]read.AssignmentRecord, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var status any
	if f.Status != "" {
		status = f.Status
	}
	scope, scopeArgs := scopeCondition(f.Scopes, 6)
	args := append([]any{f.ClassID, f.CourseID, status, f.After, f.Limit}, scopeArgs...)
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL; scope condition built from fixed column names
	rows, err := q.QueryContext(ctx, `SELECT `+assignmentSelect+` FROM lms.assignments a
		WHERE a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL
			AND ($1::uuid IS NULL OR EXISTS (SELECT 1 FROM lms.assignment_targets t WHERE t.assignment_id = a.assignment_id
				AND t.target_type = 'class' AND t.target_ref_id = $1::uuid AND t.deleted_at IS NULL)
				OR EXISTS (SELECT 1 FROM lms.courses c WHERE c.course_id = a.course_id AND c.class_id = $1::uuid AND c.deleted_at IS NULL))
			AND ($2::uuid IS NULL OR a.course_id = $2::uuid) AND ($3::varchar IS NULL OR a.status = $3::varchar)
			AND ($4::uuid IS NULL OR a.assignment_id < $4::uuid) AND `+scope+`
		ORDER BY a.assignment_id DESC LIMIT $5`, args...)
	if err != nil {
		return nil, fmt.Errorf("assignments: %w", err)
	}
	out, err := scanAssignmentViews(rows)
	if err != nil {
		return nil, err
	}
	return out, m.targets(ctx, q, out)
}

func scanAssignmentViews(rows *sql.Rows) ([]read.AssignmentRecord, error) {
	defer func() { _ = rows.Close() }()
	var out []read.AssignmentRecord
	for rows.Next() {
		r, err := scanAssignmentView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Assignment implements read.SchoolAssignmentReadModel.
func (m SchoolAssignmentModel) Assignment(ctx context.Context, id uuid.UUID) (*read.AssignmentRecord, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	r, err := scanAssignmentView(q.QueryRowContext(ctx, `SELECT `+assignmentSelect+` FROM lms.assignments a
		WHERE a.assignment_id = $1 AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("assignment: %w", err)
	}
	recs := []read.AssignmentRecord{r}
	if err := m.targets(ctx, q, recs); err != nil {
		return nil, err
	}
	return &recs[0], nil
}

// Completion implements read.SchoolAssignmentReadModel: assigned, submitted (on time or late), missed, excused, the average of the
// kept scores scaled to 10 and the students with rapid guesses (the excused are out of the completion rate).
func (SchoolAssignmentModel) Completion(ctx context.Context, idList []uuid.UUID) (map[uuid.UUID]dto.AssignmentCompletionView, error) {
	out := map[uuid.UUID]dto.AssignmentCompletionView{}
	if len(idList) == 0 {
		return out, nil
	}
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT assignment_id, count(*) FILTER (WHERE status <> 'excused'),
		count(*) FILTER (WHERE status IN ('submitted','late')), count(*) FILTER (WHERE status = 'missed'),
		count(*) FILTER (WHERE status = 'excused'),
		avg(score / NULLIF(max_score, 0) * 10) FILTER (WHERE status IN ('submitted','late') AND score IS NOT NULL)::float8,
		count(*) FILTER (WHERE rapid_guess_count > 0)
		FROM lms.assignment_submissions WHERE assignment_id = ANY($1::uuid[]) AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL
		GROUP BY assignment_id`, uuidArray(idList))
	if err != nil {
		return nil, fmt.Errorf("completion: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var c dto.AssignmentCompletionView
		var avg sql.NullFloat64
		if err := rows.Scan(&id, &c.Assigned, &c.Submitted, &c.Missed, &c.Excused, &avg, &c.RapidGuessStudents); err != nil {
			return nil, err
		}
		if avg.Valid {
			v := float64(int(avg.Float64*10+0.5)) / 10
			c.AverageScore = &v
		}
		out[id] = c
	}
	return out, rows.Err()
}

// Submissions implements read.SchoolAssignmentReadModel (keyset on the student id).
func (SchoolAssignmentModel) Submissions(ctx context.Context, f read.SubmissionFilter) ([]dto.AssignmentSubmissionView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var status any
	if f.Status != "" {
		status = f.Status
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT assignment_id, student_id, class_id, status, attempts_used, best_attempt_id, score::float8,
		max_score::float8, rapid_guess_count, due_at_override, submitted_at FROM lms.assignment_submissions
		WHERE assignment_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL
			AND ($2::varchar IS NULL OR status = $2::varchar) AND ($3::uuid IS NULL OR class_id = $3::uuid)
			AND ($4::uuid IS NULL OR student_id > $4::uuid)
		ORDER BY student_id LIMIT $5`, f.AssignmentID, status, f.ClassID, f.After, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("submissions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []dto.AssignmentSubmissionView
	for rows.Next() {
		var s dto.AssignmentSubmissionView
		var class, best uuid.NullUUID
		var score, maxScore sql.NullFloat64
		var due, submitted sql.NullTime
		if err := rows.Scan(&s.AssignmentID, &s.StudentID, &class, &s.Status, &s.AttemptsUsed, &best, &score, &maxScore, &s.RapidGuessCount,
			&due, &submitted); err != nil {
			return nil, err
		}
		s.ClassID, s.BestAttemptID = nullUUID(class), nullUUID(best)
		if score.Valid {
			v := score.Float64
			s.Score = &v
		}
		if maxScore.Valid {
			v := maxScore.Float64
			s.MaxScore = &v
		}
		if due.Valid {
			t := due.Time.UTC()
			s.DueAtOverride = &t
		}
		if submitted.Valid {
			t := submitted.Time.UTC()
			s.SubmittedAt = &t
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// MyAssignments implements read.SchoolAssignmentReadModel: the open, closed and released assignments targeting the student,
// newest first, with the name of the subject.
func (SchoolAssignmentModel) MyAssignments(ctx context.Context, studentID uuid.UUID, after *uuid.UUID, limit int) ([]read.MyAssignmentRecord, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on lms tables; subjects through RLS hybrid read
	rows, err := q.QueryContext(ctx, `SELECT `+assignmentSelect+`, sub.status, sub.attempts_used, sub.current_attempt_id, sub.due_at_override,
			(SELECT s.name FROM curriculum.subjects s WHERE s.subject_id = a.subject_id AND s.deleted_at IS NULL)
		FROM lms.assignments a JOIN lms.assignment_submissions sub ON sub.assignment_id = a.assignment_id AND sub.student_id = $1
			AND sub.tenant_id = a.tenant_id AND sub.deleted_at IS NULL
		WHERE a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL AND a.status IN ('open','closed','results_released')
			AND ($2::uuid IS NULL OR a.assignment_id < $2::uuid)
		ORDER BY a.assignment_id DESC LIMIT $3`, studentID, after, limit)
	if err != nil {
		return nil, fmt.Errorf("my assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.MyAssignmentRecord
	for rows.Next() {
		var r read.MyAssignmentRecord
		var rec read.AssignmentRecord
		v := &rec.View
		var instructions sql.NullString
		var pinned, exam, current uuid.NullUUID
		var released, due sql.NullTime
		var name []byte
		if err := rows.Scan(&v.AssignmentID, &v.TenantID, &v.SchoolID, &v.CourseID, &rec.SubjectID, &v.CreatedBy, &v.CreatedByRole, &v.Title,
			&instructions, &v.ContentRef.ContentType, &v.ContentRef.ContentID, &pinned, &v.FollowLatest, &v.Mode, &v.OpenAt, &v.DueAt,
			&v.LatePolicy.AllowLate, &v.LatePolicy.LateWindowHours, &v.MaxAttempts, &v.ScorePolicy, &v.FeedbackTiming, &v.ResultRelease,
			&v.Status, &exam, &released, &v.RowVersion, &r.Submission, &r.AttemptsUsed, &current, &due, &name); err != nil {
			return nil, err
		}
		v.ExamSessionID, v.OpenAt, v.DueAt = nullUUID(exam), v.OpenAt.UTC(), v.DueAt.UTC()
		r.Assignment, r.SubjectID, r.CurrentAttemptID = rec.View, rec.SubjectID, nullUUID(current)
		if due.Valid {
			t := due.Time.UTC()
			r.DueAtOverride = &t
		}
		if len(name) > 0 {
			r.SubjectName = json.RawMessage(name)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Release implements read.SchoolAssignmentReadModel.
func (SchoolAssignmentModel) Release(ctx context.Context, id uuid.UUID) (string, string, bool, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return "", "", false, err
	}
	var release, status string
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT result_release, status FROM lms.assignments WHERE assignment_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, id).Scan(&release, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	return release, status, err == nil, err
}

// GameActivities implements read.SchoolAssignmentReadModel: the published quests and mini-games of the tenant (projection of the
// events of module game), by objective and grade.
func (SchoolAssignmentModel) GameActivities(ctx context.Context, f read.GameActivityFilter) ([]dto.GameActivityView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var los any
	if len(f.ObjectiveIDs) > 0 {
		los = uuidArray(f.ObjectiveIDs)
	}
	// tenancyguard:ok projection ref.game_activities (no deleted_at) of the session tenant (RLS + infra.current_tenant_id())
	rows, err := q.QueryContext(ctx, `SELECT item_type, item_id, current_version_id, title_i18n, array_to_string(grade_levels, ','),
		array_to_string(objective_ids, ',') FROM ref.game_activities
		WHERE tenant_id = infra.current_tenant_id() AND status = 'published' AND ($1::uuid[] IS NULL OR objective_ids && $1::uuid[])
			AND ($2::smallint IS NULL OR $2::smallint = ANY(grade_levels)) AND ($3::uuid IS NULL OR item_id > $3::uuid)
		ORDER BY item_id LIMIT $4`, los, f.Grade, f.After, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("game activities: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []dto.GameActivityView
	for rows.Next() {
		var g dto.GameActivityView
		var grades, objectives string
		if err := rows.Scan(&g.ContentType, &g.ContentID, &g.ContentVersionID, &g.TitleI18n, &grades, &objectives); err != nil {
			return nil, err
		}
		g.GradeLevels, g.ObjectiveIDs = []int{}, []uuid.UUID{}
		for _, s := range strings.Split(grades, ",") {
			var n int
			if _, err := fmt.Sscan(s, &n); err == nil {
				g.GradeLevels = append(g.GradeLevels, n)
			}
		}
		for _, s := range strings.Split(objectives, ",") {
			if x, err := uuid.Parse(s); err == nil {
				g.ObjectiveIDs = append(g.ObjectiveIDs, x)
			}
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
