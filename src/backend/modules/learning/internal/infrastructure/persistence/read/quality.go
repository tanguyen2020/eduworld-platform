package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// QualityModel implements the read ports of F044 US2–US6 on the read session (tenant of the session: explicit tenant_id =
// infra.current_tenant_id() plus RLS; platform rows only where RLS shows them).
type QualityModel struct{}

var (
	_ read.CourseResultsReader = QualityModel{}
	_ read.ReferenceReader     = QualityModel{}
	_ read.QualityReader       = QualityModel{}
	_ read.ExamQualityReader   = QualityModel{}
)

// Course implements read.CourseResultsReader.
func (QualityModel) Course(ctx context.Context, id uuid.UUID) (*read.CourseRef, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var c read.CourseRef
	// tenancyguard:ok one course of the session tenant (explicit tenant_id + RLS); deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT course_id, school_id, subject_id, class_id FROM lms.courses
		WHERE tenant_id = infra.current_tenant_id() AND course_id = $1 AND deleted_at IS NULL`, id).Scan(&c.CourseID, &c.SchoolID, &c.SubjectID, &c.ClassID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &c, err
}

// GradedAssignments implements read.CourseResultsReader: graded assignments of the course that were published, by due date.
func (QualityModel) GradedAssignments(ctx context.Context, courseID uuid.UUID) ([]read.GradedAssignment, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok assignments of one course in the session tenant (explicit tenant_id + RLS); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT assignment_id, title, due_at, score_policy FROM lms.assignments
		WHERE tenant_id = infra.current_tenant_id() AND course_id = $1 AND mode = 'graded' AND deleted_at IS NULL
			AND status NOT IN ('draft','cancelled')
		ORDER BY due_at, assignment_id LIMIT 100`, courseID)
	if err != nil {
		return nil, fmt.Errorf("graded assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.GradedAssignment
	for rows.Next() {
		var a read.GradedAssignment
		if err := rows.Scan(&a.AssignmentID, &a.Title, &a.DueAt, &a.ScorePolicy); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CourseStudents implements read.CourseResultsReader: active students of the class (deleted students excluded).
func (QualityModel) CourseStudents(ctx context.Context, classID uuid.UUID, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok enrolments of one class in the session tenant (explicit tenant_id + RLS); deleted students excluded
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT e.student_id FROM ref.enrollments e
		JOIN ref.students st ON st.student_id = e.student_id AND st.tenant_id = e.tenant_id AND st.deleted_at IS NULL
		WHERE e.tenant_id = infra.current_tenant_id() AND e.class_id = $1 AND e.status = 'active' AND e.student_id > $2
		ORDER BY e.student_id LIMIT $3`, classID, after, limit+1)
	if err != nil {
		return nil, fmt.Errorf("course students: %w", err)
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

// ResultCells implements read.CourseResultsReader: the submission of each student to each graded assignment (score =
// effective score under the score policy, kept by F041 and the re-evaluations).
func (QualityModel) ResultCells(ctx context.Context, courseID uuid.UUID, students []uuid.UUID) ([]read.ResultCell, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	if len(students) == 0 {
		return nil, nil
	}
	// tenancyguard:ok submissions of the course in the session tenant (explicit tenant_id + RLS); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT s.student_id, s.assignment_id, s.status, s.score::float8, s.max_score::float8, s.rapid_guess_count
		FROM lms.assignment_submissions s
		JOIN lms.assignments a ON a.assignment_id = s.assignment_id AND a.tenant_id = s.tenant_id AND a.deleted_at IS NULL
		WHERE s.tenant_id = infra.current_tenant_id() AND a.course_id = $1 AND a.mode = 'graded' AND s.deleted_at IS NULL
			AND s.student_id = ANY($2::uuid[])`, courseID, uuidArray(students))
	if err != nil {
		return nil, fmt.Errorf("result cells: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ResultCell
	for rows.Next() {
		var c read.ResultCell
		var score, maxScore sql.NullFloat64
		if err := rows.Scan(&c.StudentID, &c.AssignmentID, &c.Status, &score, &maxScore, &c.RapidGuessCount); err != nil {
			return nil, err
		}
		if score.Valid {
			c.Score = &score.Float64
		}
		if maxScore.Valid {
			c.MaxScore = &maxScore.Float64
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ReferenceLevels implements read.ReferenceReader: the latest profile version computed for each student of the period.
func (QualityModel) ReferenceLevels(ctx context.Context, schoolID, subjectID uuid.UUID, year int, period string, classID *uuid.UUID,
	after uuid.UUID, limit int) ([]read.ReferenceLevel, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok derived read model (no deleted_at) of the session tenant (explicit tenant_id + RLS); deleted students excluded
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT ON (r.student_id) r.student_id, r.class_id, r.level_code, r.coverage_ratio::float8,
			r.proficient_ratio::float8, r.mastered_ratio::float8, r.official_score::float8, r.rapid_guess_ratio::float8,
			r.low_reliability_exam_ids::text, r.profile_code, r.profile_version, r.computed_at
		FROM learning.rm_student_subject_reference_levels r
		JOIN ref.students st ON st.student_id = r.student_id AND st.tenant_id = r.tenant_id AND st.deleted_at IS NULL
		WHERE r.tenant_id = infra.current_tenant_id() AND r.school_id = $1 AND r.subject_id = $2 AND r.academic_year = $3 AND r.period = $4
			AND ($5::uuid IS NULL OR r.class_id = $5) AND r.student_id > $6
		ORDER BY r.student_id, r.profile_version DESC LIMIT $7`, schoolID, subjectID, year, period, classID, after, limit+1)
	if err != nil {
		return nil, fmt.Errorf("reference levels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ReferenceLevel
	for rows.Next() {
		var l read.ReferenceLevel
		var official sql.NullFloat64
		var low string
		if err := rows.Scan(&l.StudentID, &l.ClassID, &l.LevelCode, &l.Coverage, &l.Proficient, &l.Mastered, &official, &l.RapidGuessRatio,
			&low, &l.ProfileCode, &l.ProfileVersion, &l.ComputedAt); err != nil {
			return nil, err
		}
		if official.Valid {
			l.OfficialScore = &official.Float64
		}
		for _, part := range strings.Split(strings.Trim(low, "{}"), ",") {
			if id, err := uuid.Parse(part); err == nil {
				l.LowReliabilityExamIDs = append(l.LowReliabilityExamIDs, id)
			}
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ProfileDisclaimer implements read.ReferenceReader.
func (QualityModel) ProfileDisclaimer(ctx context.Context, code string, version int) (map[string]string, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	// tenancyguard:ok profiles of the session tenant or of the platform (RLS content_read); deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT disclaimer FROM learning.assessment_profiles WHERE profile_code = $1 AND profile_version = $2
		AND deleted_at IS NULL AND (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL) ORDER BY tenant_id NULLS LAST LIMIT 1`,
		code, version).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	return out, json.Unmarshal(raw, &out)
}

// Question implements read.QualityReader: a question visible in the session (tenant row or assigned platform row).
func (QualityModel) Question(ctx context.Context, id uuid.UUID) (*read.QuestionOwner, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var q read.QuestionOwner
	var tenant uuid.NullUUID
	// tenancyguard:ok one question visible in the session (RLS content_read: tenant rows or assigned platform rows); deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT question_id, tenant_id FROM content.questions WHERE question_id = $1 AND deleted_at IS NULL
		AND (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL)`, id).Scan(&q.QuestionID, &tenant)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tenant.Valid {
		t := tenant.UUID
		q.TenantID = &t
	}
	return &q, nil
}

// QuestionStats implements read.QualityReader: every version × grade row of the question visible in the session.
func (QualityModel) QuestionStats(ctx context.Context, questionID uuid.UUID) ([]read.QuestionStat, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok derived statistics (no deleted_at) visible in the session (RLS content_read); versions not filtered on deleted_at (history)
	rows, err := tx.QueryContext(ctx, `SELECT s.tenant_id, s.question_id, s.question_version_id, v.version_no, s.grade_level, s.first_attempt_count,
			s.excluded_rapid_guess_count, s.with_hint_count, s.p_value::float8, s.p_value_with_hint::float8, s.discrimination::float8,
			s.distractor_rates, s.avg_time_ms, s.rapid_guess_rate::float8, s.hint_usage_rate::float8, s.flags::text, s.computed_at
		FROM content.question_stats s JOIN content.question_versions v ON v.question_version_id = s.question_version_id
		WHERE s.question_id = $1 AND (s.tenant_id = infra.current_tenant_id() OR s.tenant_id IS NULL)
		ORDER BY v.version_no DESC, s.grade_level`, questionID)
	if err != nil {
		return nil, fmt.Errorf("question stats: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.QuestionStat
	for rows.Next() {
		var s read.QuestionStat
		var tenant uuid.NullUUID
		var p, ph, d sql.NullFloat64
		var dr []byte
		var flags string
		if err := rows.Scan(&tenant, &s.QuestionID, &s.QuestionVersionID, &s.VersionNo, &s.GradeLevel, &s.FirstAttempts, &s.ExcludedRapidGuess,
			&s.WithHint, &p, &ph, &d, &dr, &s.AvgTimeMS, &s.RapidGuessRate, &s.HintUsageRate, &flags, &s.ComputedAt); err != nil {
			return nil, err
		}
		if tenant.Valid {
			t := tenant.UUID
			s.TenantID = &t
		}
		s.PValue, s.PValueWithHint, s.Discrimination = nullFloat(p), nullFloat(ph), nullFloat(d)
		_ = json.Unmarshal(dr, &s.DistractorRates)
		s.Flags = []string{}
		for _, f := range strings.Split(strings.Trim(flags, "{}"), ",") {
			if f != "" {
				s.Flags = append(s.Flags, f)
			}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func nullFloat(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

// Proposals implements read.QualityReader.
func (QualityModel) Proposals(ctx context.Context, platform bool, status string, after uuid.UUID, limit int) ([]write.ProposalRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	owner := "tenant_id = infra.current_tenant_id()"
	if platform {
		owner = "tenant_id IS NULL"
	}
	// tenancyguard:ok proposals of the session tenant (explicit tenant_id + RLS) or platform proposals (learning_platform session); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT proposal_id, tenant_id, question_id, based_on_version_id, current_difficulty, proposed_difficulty,
			p_value::float8, sample_size, grade_level, grade_distribution, status, decided_by, decided_at, created_version_id,
			consecutive_dismissals, suppressed_until, created_at, version
		FROM content.difficulty_calibration_proposals WHERE `+owner+` AND deleted_at IS NULL AND ($1 = '' OR status = $1) AND proposal_id > $2
		ORDER BY proposal_id LIMIT $3`, status, after, limit+1)
	if err != nil {
		return nil, fmt.Errorf("calibration proposals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ProposalRow
	for rows.Next() {
		var p write.ProposalRow
		var tenant, by, created uuid.NullUUID
		var grades []byte
		var until sql.NullTime
		if err := rows.Scan(&p.ID, &tenant, &p.QuestionID, &p.BasedOnVersionID, &p.CurrentDifficulty, &p.ProposedDifficulty, &p.PValue,
			&p.SampleSize, &p.GradeLevel, &grades, &p.Status, &by, &p.DecidedAt, &created, &p.ConsecutiveDismissals, &until, &p.CreatedAt,
			&p.Version); err != nil {
			return nil, err
		}
		p.TenantID, p.DecidedBy, p.CreatedVersionID = nullUUID(tenant), nullUUID(by), nullUUID(created)
		if until.Valid {
			p.SuppressedUntil = &until.Time
		}
		_ = json.Unmarshal(grades, &p.Grades)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ExamSession implements read.ExamQualityReader.
func (QualityModel) ExamSession(ctx context.Context, id uuid.UUID) (*write.ExamSessionRow, error) {
	return ExamModel{}.Session(ctx, id)
}

// ExamQuality implements read.ExamQualityReader.
func (QualityModel) ExamQuality(ctx context.Context, id uuid.UUID) (*write.ExamQualityRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var r write.ExamQualityRow
	var dist string
	var mean, median, pass sql.NullFloat64
	// tenancyguard:ok derived report (no deleted_at) of one session in the session tenant (explicit tenant_id + RLS)
	err = tx.QueryRowContext(ctx, `SELECT exam_session_id, tenant_id, school_id, student_count, variants, variant_gap_flag, problem_items, items,
			score_distribution::text, mean_score::float8, median_score::float8, pass_rate::float8, reliability_warning, computed_at
		FROM assessment.exam_quality_reports WHERE tenant_id = infra.current_tenant_id() AND exam_session_id = $1`, id).Scan(&r.SessionID,
		&r.TenantID, &r.SchoolID, &r.StudentCount, &r.Variants, &r.VariantGap, &r.ProblemItems, &r.Items, &dist, &mean, &median, &pass,
		&r.ReliabilityWarning, &r.ComputedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Mean, r.Median, r.PassRate = nullFloat(mean), nullFloat(median), nullFloat(pass)
	for _, part := range strings.Split(strings.Trim(dist, "{}"), ",") {
		var n int
		if _, err := fmt.Sscan(part, &n); err == nil {
			r.Distribution = append(r.Distribution, n)
		}
	}
	return &r, nil
}

// ExamPapersExpected implements read.ExamQualityReader: expected_p of the variants of a session.
func (QualityModel) ExamPapersExpected(ctx context.Context, id uuid.UUID) (map[int]*float64, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok papers of one session in the session tenant (explicit tenant_id + RLS); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT variant_no, expected_p::float8 FROM assessment.exam_papers
		WHERE tenant_id = infra.current_tenant_id() AND exam_session_id = $1 AND student_id IS NULL AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int]*float64{}
	for rows.Next() {
		var v int
		var p sql.NullFloat64
		if err := rows.Scan(&v, &p); err != nil {
			return nil, err
		}
		out[v] = nullFloat(p)
	}
	return out, rows.Err()
}

var _ read.ReportScopeReader = QualityModel{}

// AssignmentScope implements read.ReportScopeReader.
func (QualityModel) AssignmentScope(ctx context.Context, id uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, false, err
	}
	var school, subject uuid.UUID
	// tenancyguard:ok one assignment of the session tenant (explicit tenant_id + RLS); deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT school_id, subject_id FROM lms.assignments WHERE tenant_id = infra.current_tenant_id()
		AND assignment_id = $1 AND deleted_at IS NULL`, id).Scan(&school, &subject)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, uuid.Nil, false, nil
	}
	return school, subject, err == nil, err
}

// ClassSchool implements read.ReportScopeReader.
func (QualityModel) ClassSchool(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	var school uuid.UUID
	// tenancyguard:ok ref.classes projection (no deleted_at) of the session tenant (explicit tenant_id + RLS)
	err = tx.QueryRowContext(ctx, `SELECT school_id FROM ref.classes WHERE tenant_id = infra.current_tenant_id() AND class_id = $1`, id).Scan(&school)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return school, err == nil, err
}
