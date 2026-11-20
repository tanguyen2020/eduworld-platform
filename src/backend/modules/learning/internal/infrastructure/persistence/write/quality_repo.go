package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// QualityRepo implements the stores of F044 US3–US6: item analysis and p_values (content.question_stats), calibration
// proposals (content.difficulty_calibration_proposals), exam quality (assessment.exam_quality_reports) and reference levels
// (learning.rm_student_subject_reference_levels, learning.assessment_profiles). Jobs that pool every tenant run on
// learning_platform under tenancy.Bypass(platform_job); the others in the TenantScope of ctx (RLS + explicit tenant_id).
type QualityRepo struct{}

var (
	_ write.QuestionPValues   = QualityRepo{}
	_ write.ItemAnalysisStore = QualityRepo{}
	_ write.CalibrationStore  = QualityRepo{}
	_ write.ExamQualityStore  = QualityRepo{}
	_ write.ReferenceStore    = QualityRepo{}
)

const nilTenant = "'00000000-0000-0000-0000-000000000000'::uuid"

// ---------- item analysis

// PValues implements write.QuestionPValues: the pooled row (grade 0) of the versions with enough data.
func (QualityRepo) PValues(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID]float64, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]float64{}
	if len(versionIDs) == 0 {
		return out, nil
	}
	// tenancyguard:ok derived statistics (no deleted_at) visible in the session: tenant rows or assigned platform rows (RLS content_read)
	rows, err := tx.QueryContext(ctx, `SELECT question_version_id, p_value::float8 FROM content.question_stats
		WHERE question_version_id = ANY($1::uuid[]) AND grade_level = 0 AND p_value IS NOT NULL
		AND (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL)`, uuidList(versionIDs))
	if err != nil {
		return nil, fmt.Errorf("question p_values: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var p float64
		if err := rows.Scan(&id, &p); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}

// ChangedVersions implements write.ItemAnalysisStore: versions answered since `since` (every tenant, platform_job).
func (QualityRepo) ChangedVersions(ctx context.Context, since time.Time, limit int) ([]write.AnalysedVersion, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok item analysis of every tenant under tenancy.Bypass(platform_job) (learning_platform); deleted_at IS NULL on versions; attempt_answers is append-only
	rows, err := tx.QueryContext(ctx, `SELECT v.question_id, v.question_version_id, v.tenant_id, v.type, v.difficulty, v.render_payload,
			v.answer_key
		FROM content.question_versions v
		WHERE v.deleted_at IS NULL AND v.question_version_id IN (
			SELECT DISTINCT a.question_version_id FROM assessment.attempt_answers a WHERE a.answered_at >= $1)
		ORDER BY v.question_version_id LIMIT $2`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("changed versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AnalysedVersion
	for rows.Next() {
		var v write.AnalysedVersion
		var tenant uuid.NullUUID
		if err := rows.Scan(&v.QuestionID, &v.QuestionVersionID, &tenant, &v.Type, &v.Difficulty, &v.Render, &v.AnswerKey); err != nil {
			return nil, err
		}
		v.TenantID = nullUUIDPtr(tenant)
		out = append(out, v)
	}
	return out, rows.Err()
}

// FirstAnswers implements write.ItemAnalysisStore: the first answer (attempt_no 1, is_scored) of every graded attempt.
func (QualityRepo) FirstAnswers(ctx context.Context, versionID uuid.UUID) ([]write.FirstAnswer, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok item analysis of every tenant under tenancy.Bypass(platform_job) (learning_platform); deleted_at IS NULL on attempts/items; answers append-only
	rows, err := tx.QueryContext(ctx, `SELECT COALESCE(st.grade_level, 0), COALESCE(a.is_correct, false), COALESCE(a.score, 0)::float8,
			ai.max_score::float8, att.score::float8 / att.max_score::float8, a.response_ms, a.rapid_guess, a.hints_used, a.response
		FROM assessment.attempt_answers a
		JOIN assessment.attempts att ON att.attempt_id = a.attempt_id AND att.deleted_at IS NULL AND att.status = 'graded' AND att.max_score > 0
		JOIN assessment.attempt_items ai ON ai.attempt_id = a.attempt_id AND ai.position = a.position AND ai.deleted_at IS NULL
		LEFT JOIN ref.students st ON st.student_id = att.student_id AND st.tenant_id = att.tenant_id
		WHERE a.question_version_id = $1 AND a.attempt_no = 1 AND a.is_scored`, versionID)
	if err != nil {
		return nil, fmt.Errorf("first answers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.FirstAnswer
	for rows.Next() {
		var f write.FirstAnswer
		var ms sql.NullInt64
		if err := rows.Scan(&f.GradeLevel, &f.Correct, &f.Score, &f.MaxScore, &f.AttemptScore, &ms, &f.RapidGuess, &f.HintsUsed, &f.Response); err != nil {
			return nil, err
		}
		if ms.Valid {
			n := int(ms.Int64)
			f.ResponseMS = &n
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func round4p(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := math.Round(*p*1e4) / 1e4
	return &v
}

// UpsertQuestionStats implements write.ItemAnalysisStore.
func (QualityRepo) UpsertQuestionStats(ctx context.Context, rs []write.QuestionStatRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	for _, r := range rs {
		dr, err := json.Marshal(r.DistractorRates)
		if err != nil {
			return err
		}
		flags := r.Flags
		if flags == nil {
			flags = []string{}
		}
		// tenancyguard:ok derived statistics (no deleted_at) written under tenancy.Bypass(platform_job) (learning_platform)
		if _, err := tx.ExecContext(ctx, `INSERT INTO content.question_stats (tenant_id, question_id, question_version_id, grade_level,
				first_attempt_count, excluded_rapid_guess_count, with_hint_count, p_value, p_value_with_hint, discrimination, distractor_rates,
				avg_time_ms, rapid_guess_rate, hint_usage_rate, flags, computed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::varchar[],$16)
			ON CONFLICT ((coalesce(tenant_id, `+nilTenant+`)), question_version_id, grade_level) DO UPDATE SET
				first_attempt_count = EXCLUDED.first_attempt_count, excluded_rapid_guess_count = EXCLUDED.excluded_rapid_guess_count,
				with_hint_count = EXCLUDED.with_hint_count, p_value = EXCLUDED.p_value, p_value_with_hint = EXCLUDED.p_value_with_hint,
				discrimination = EXCLUDED.discrimination, distractor_rates = EXCLUDED.distractor_rates, avg_time_ms = EXCLUDED.avg_time_ms,
				rapid_guess_rate = EXCLUDED.rapid_guess_rate, hint_usage_rate = EXCLUDED.hint_usage_rate, flags = EXCLUDED.flags,
				computed_at = EXCLUDED.computed_at`,
			r.TenantID, r.QuestionID, r.QuestionVersionID, r.GradeLevel, r.FirstAttempts, r.ExcludedRapidGuess, r.WithHint, round4p(r.PValue),
			round4p(r.PValueWithHint), round4p(r.Discrimination), dr, r.AvgTimeMS, r.RapidGuessRate, r.HintUsageRate, textArray(flags),
			r.ComputedAt); err != nil {
			return fmt.Errorf("upsert question stats: %w", err)
		}
	}
	return nil
}

// ---------- calibration

// CalibrationCandidates implements write.CalibrationStore: per question, the grade of its current version with the most
// plain first answers (first − rapid − with hint) when that is ≥ minN; the distribution of every grade.
func (QualityRepo) CalibrationCandidates(ctx context.Context, minN int) ([]write.CalibrationCandidate, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok weekly calibration of every tenant under tenancy.Bypass(platform_job) (learning_platform); deleted_at IS NULL on questions/versions; stats derived
	rows, err := tx.QueryContext(ctx, `SELECT q.question_id, v.question_version_id, q.tenant_id, v.difficulty, s.grade_level, s.p_value::float8,
			s.first_attempt_count - s.excluded_rapid_guess_count - s.with_hint_count
		FROM content.questions q
		JOIN content.question_versions v ON v.question_version_id = q.current_version_id AND v.deleted_at IS NULL
		JOIN content.question_stats s ON s.question_version_id = v.question_version_id AND s.grade_level > 0 AND s.p_value IS NOT NULL
		WHERE q.deleted_at IS NULL
		ORDER BY q.question_id, 7 DESC, s.grade_level`)
	if err != nil {
		return nil, fmt.Errorf("calibration candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.CalibrationCandidate
	for rows.Next() {
		var c write.CalibrationCandidate
		var tenant uuid.NullUUID
		if err := rows.Scan(&c.QuestionID, &c.QuestionVersionID, &tenant, &c.Difficulty, &c.GradeLevel, &c.PValue, &c.SampleSize); err != nil {
			return nil, err
		}
		c.TenantID = nullUUIDPtr(tenant)
		grade := map[string]any{"n": c.SampleSize, "p_value": math.Round(c.PValue*1e4) / 1e4}
		if n := len(out); n > 0 && out[n-1].QuestionID == c.QuestionID {
			out[n-1].Grades[fmt.Sprint(c.GradeLevel)] = grade
			continue
		}
		c.Grades = map[string]any{fmt.Sprint(c.GradeLevel): grade}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(out, func(c write.CalibrationCandidate) bool { return c.SampleSize < minN }), nil
}

// CalibrationState implements write.CalibrationStore from the latest proposal of the question.
func (QualityRepo) CalibrationState(ctx context.Context, questionID uuid.UUID) (write.CalibrationState, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return write.CalibrationState{}, err
	}
	var st write.CalibrationState
	var status string
	var until sql.NullTime
	// tenancyguard:ok proposals of one question visible in the session (platform_job: learning_platform); deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT status, consecutive_dismissals, suppressed_until FROM content.difficulty_calibration_proposals
		WHERE question_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1`, questionID).Scan(&status, &st.ConsecutiveDismissals, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("calibration state: %w", err)
	}
	switch status {
	case "pending":
		st.Pending = true
	case "accepted":
		st.ConsecutiveDismissals = 0
	}
	if until.Valid && status == "dismissed" {
		t := until.Time
		st.SuppressedUntil = &t
	}
	return st, nil
}

const proposalColumns = `proposal_id, tenant_id, question_id, based_on_version_id, current_difficulty, proposed_difficulty, p_value::float8,
	sample_size, grade_level, grade_distribution, status, decided_by, decided_at, created_version_id, consecutive_dismissals, suppressed_until,
	created_at, version`

// ProposalColumns are the columns read by ScanProposal.
const ProposalColumns = proposalColumns

// ScanProposal scans the ProposalColumns of a row.
func ScanProposal(r interface{ Scan(...any) error }) (write.ProposalRow, error) {
	var p write.ProposalRow
	var tenant, by, created uuid.NullUUID
	var grades []byte
	var until sql.NullTime
	if err := r.Scan(&p.ID, &tenant, &p.QuestionID, &p.BasedOnVersionID, &p.CurrentDifficulty, &p.ProposedDifficulty, &p.PValue,
		&p.SampleSize, &p.GradeLevel, &grades, &p.Status, &by, &p.DecidedAt, &created, &p.ConsecutiveDismissals, &until, &p.CreatedAt,
		&p.Version); err != nil {
		return p, err
	}
	p.TenantID, p.DecidedBy, p.CreatedVersionID = nullUUIDPtr(tenant), nullUUIDPtr(by), nullUUIDPtr(created)
	if until.Valid {
		t := until.Time
		p.SuppressedUntil = &t
	}
	_ = json.Unmarshal(grades, &p.Grades)
	return p, nil
}

// InsertProposal implements write.CalibrationStore.
func (QualityRepo) InsertProposal(ctx context.Context, p write.ProposalRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	grades, err := json.Marshal(p.Grades)
	if err != nil {
		return err
	}
	owner := "tenant"
	if p.TenantID == nil {
		owner = "platform"
	}
	// tenancyguard:ok INSERT under tenancy.Bypass(platform_job) (learning_platform): tenant_id is the owner of the question
	if _, err := tx.ExecContext(ctx, `INSERT INTO content.difficulty_calibration_proposals (proposal_id, tenant_id, owner_scope, question_id,
			based_on_version_id, current_difficulty, proposed_difficulty, p_value, sample_size, grade_level, grade_distribution, status,
			consecutive_dismissals, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'pending',$12,$13,$13)`, p.ID, p.TenantID, owner, p.QuestionID, p.BasedOnVersionID,
		p.CurrentDifficulty, p.ProposedDifficulty, math.Round(p.PValue*1e4)/1e4, p.SampleSize, p.GradeLevel, grades, p.ConsecutiveDismissals,
		p.CreatedAt); err != nil {
		return fmt.Errorf("insert calibration proposal: %w", err)
	}
	return nil
}

// Proposal implements write.CalibrationStore.
func (QualityRepo) Proposal(ctx context.Context, id uuid.UUID, lock bool) (write.ProposalRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return write.ProposalRow{}, err
	}
	forUpdate := ""
	if lock {
		forUpdate = " FOR UPDATE"
	}
	// tenancyguard:ok one proposal visible in the session (tenant RLS; platform proposals on learning_platform); deleted_at IS NULL
	p, err := ScanProposal(tx.QueryRowContext(ctx, `SELECT `+proposalColumns+` FROM content.difficulty_calibration_proposals
		WHERE proposal_id = $1 AND deleted_at IS NULL`+forUpdate, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, problem.New(problem.NotFound, "")
	}
	return p, err
}

// UpdateProposal implements write.CalibrationStore.
func (QualityRepo) UpdateProposal(ctx context.Context, p write.ProposalRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var until any
	if p.SuppressedUntil != nil {
		until = p.SuppressedUntil.Format(time.DateOnly)
	}
	// tenancyguard:ok update of one proposal by id visible in the session (RLS update policy); deleted_at IS NULL
	if _, err := tx.ExecContext(ctx, `UPDATE content.difficulty_calibration_proposals SET status = $2, decided_by = $3, decided_at = $4,
			created_version_id = $5, consecutive_dismissals = $6, suppressed_until = $7::date, updated_at = now(), version = version + 1
		WHERE proposal_id = $1 AND deleted_at IS NULL`, p.ID, p.Status, p.DecidedBy, p.DecidedAt, p.CreatedVersionID,
		p.ConsecutiveDismissals, until); err != nil {
		return fmt.Errorf("update calibration proposal: %w", err)
	}
	return nil
}

// ---------- exam quality

// ExamSessionForQuality implements write.ExamQualityStore.
func (QualityRepo) ExamSessionForQuality(ctx context.Context, id uuid.UUID) (write.ExamSessionRow, error) {
	s, err := ExamRepo{}.Session(ctx, id, false)
	if err != nil {
		return write.ExamSessionRow{}, err
	}
	if s == nil {
		return write.ExamSessionRow{}, problem.New(problem.NotFound, "")
	}
	return *s, nil
}

// ExamSheets implements write.ExamQualityStore: the latest graded attempt of every participant (score policy last
// [Proposed]) with its items and their first answer.
func (QualityRepo) ExamSheets(ctx context.Context, id uuid.UUID) ([]write.ExamSheet, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok participations of one session in the session tenant (explicit tenant_id + RLS); deleted_at IS NULL on every source
	rows, err := tx.QueryContext(ctx, `SELECT p.student_id, pp.variant_no, att.attempt_id, att.score::float8, att.max_score::float8
		FROM assessment.exam_participations p
		JOIN assessment.attempts att ON att.attempt_id = p.last_attempt_id AND att.deleted_at IS NULL AND att.status = 'graded'
		JOIN assessment.exam_papers pp ON pp.exam_paper_id = att.exam_paper_id AND pp.deleted_at IS NULL
		JOIN ref.students st ON st.student_id = p.student_id AND st.tenant_id = p.tenant_id AND st.deleted_at IS NULL
		WHERE p.tenant_id = infra.current_tenant_id() AND p.exam_session_id = $1 AND p.deleted_at IS NULL
		ORDER BY p.student_id`, id)
	if err != nil {
		return nil, fmt.Errorf("exam sheets: %w", err)
	}
	defer func() { _ = rows.Close() }() // fully read below (Next = false closes it) before the next query
	var out []write.ExamSheet
	index := map[uuid.UUID]int{}
	var attempts []uuid.UUID
	for rows.Next() {
		var s write.ExamSheet
		var attempt uuid.UUID
		var score, maxScore sql.NullFloat64
		if err := rows.Scan(&s.StudentID, &s.VariantNo, &attempt, &score, &maxScore); err != nil {
			return nil, err
		}
		s.Score, s.MaxScore = score.Float64, maxScore.Float64
		index[attempt] = len(out)
		attempts = append(attempts, attempt)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil || len(out) == 0 {
		return out, err
	}
	// tenancyguard:ok items and answers of the attempts read above (tenant of the session, RLS); deleted_at IS NULL on items; answers append-only
	items, err := tx.QueryContext(ctx, `SELECT ai.attempt_id, ai.question_version_id, COALESCE(ai.final_score, 0)::float8, ai.max_score::float8,
			COALESCE(ai.final_is_correct, false), COALESCE(fa.rapid_guess, false), fa.response_ms, fa.response
		FROM assessment.attempt_items ai
		LEFT JOIN LATERAL (SELECT a.rapid_guess, a.response_ms, a.response FROM assessment.attempt_answers a
			WHERE a.attempt_id = ai.attempt_id AND a.position = ai.position ORDER BY a.answered_at LIMIT 1) fa ON true
		WHERE ai.attempt_id = ANY($1::uuid[]) AND ai.deleted_at IS NULL AND ai.tenant_id = infra.current_tenant_id()
		ORDER BY ai.attempt_id, ai.position`, uuidList(attempts))
	if err != nil {
		return nil, fmt.Errorf("exam sheet items: %w", err)
	}
	defer func() { _ = items.Close() }()
	for items.Next() {
		var attempt uuid.UUID
		var it write.ExamSheetItem
		var ms sql.NullInt64
		if err := items.Scan(&attempt, &it.QuestionVersionID, &it.Score, &it.MaxScore, &it.Correct, &it.RapidGuess, &ms, &it.Response); err != nil {
			return nil, err
		}
		if ms.Valid {
			n := int(ms.Int64)
			it.ResponseMS = &n
		}
		i := index[attempt]
		out[i].Items = append(out[i].Items, it)
	}
	return out, items.Err()
}

// OptionMisconceptions implements write.ExamQualityStore from content.question_versions.option_feedback.
func (QualityRepo) OptionMisconceptions(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID]map[string]string, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]map[string]string{}
	// tenancyguard:ok versions visible in the session (tenant rows or assigned platform rows, RLS content_read); deleted_at not filtered: attempts keep their version
	rows, err := tx.QueryContext(ctx, `SELECT question_version_id, option_feedback FROM content.question_versions
		WHERE question_version_id = ANY($1::uuid[]) AND option_feedback IS NOT NULL`, uuidList(versionIDs))
	if err != nil {
		return nil, fmt.Errorf("option misconceptions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var fb map[string]struct {
			Tag string `json:"misconception_tag"`
		}
		if json.Unmarshal(raw, &fb) != nil {
			continue
		}
		for opt, f := range fb {
			if f.Tag != "" {
				if out[id] == nil {
					out[id] = map[string]string{}
				}
				out[id][opt] = f.Tag
			}
		}
	}
	return out, rows.Err()
}

// UpsertExamQuality implements write.ExamQualityStore.
func (QualityRepo) UpsertExamQuality(ctx context.Context, r write.ExamQualityRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok derived report (no deleted_at) of one session in the session tenant (RLS insert/update policies)
	if _, err := tx.ExecContext(ctx, `INSERT INTO assessment.exam_quality_reports (exam_session_id, tenant_id, school_id, student_count, variants,
			variant_gap_flag, problem_items, items, score_distribution, mean_score, median_score, pass_rate, reliability_warning, computed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::int[],$10,$11,$12,$13,$14)
		ON CONFLICT (exam_session_id) DO UPDATE SET student_count = EXCLUDED.student_count, variants = EXCLUDED.variants,
			variant_gap_flag = EXCLUDED.variant_gap_flag, problem_items = EXCLUDED.problem_items, items = EXCLUDED.items,
			score_distribution = EXCLUDED.score_distribution, mean_score = EXCLUDED.mean_score, median_score = EXCLUDED.median_score,
			pass_rate = EXCLUDED.pass_rate, reliability_warning = EXCLUDED.reliability_warning, computed_at = EXCLUDED.computed_at`,
		r.SessionID, r.TenantID, r.SchoolID, r.StudentCount, []byte(r.Variants), r.VariantGap, []byte(r.ProblemItems), []byte(r.Items),
		intArray(r.Distribution), r.Mean, r.Median, r.PassRate, r.ReliabilityWarning, r.ComputedAt); err != nil {
		return fmt.Errorf("upsert exam quality: %w", err)
	}
	return nil
}

// ---------- reference levels

// ActiveProfiles implements write.ReferenceStore: the active profiles of the tenant, else of the platform.
func (QualityRepo) ActiveProfiles(ctx context.Context) ([]write.AssessmentProfileRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok profiles of the session tenant or of the platform (RLS content_read); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT profile_id, tenant_id, profile_code, profile_version, grade_levels, rules, disclaimer, status, created_at
		FROM learning.assessment_profiles WHERE status = 'active' AND deleted_at IS NULL
		AND (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL) ORDER BY profile_code, tenant_id NULLS LAST`)
	if err != nil {
		return nil, fmt.Errorf("assessment profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AssessmentProfileRow
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].Code == p.Code {
			continue // the tenant profile comes first
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanProfile(r interface{ Scan(...any) error }) (write.AssessmentProfileRow, error) {
	var p write.AssessmentProfileRow
	var tenant uuid.NullUUID
	var grades string
	var disclaimer []byte
	if err := r.Scan(&p.ID, &tenant, &p.Code, &p.Version, &grades, &p.Rules, &disclaimer, &p.Status, &p.CreatedAt); err != nil {
		return p, err
	}
	p.TenantID = nullUUIDPtr(tenant)
	p.GradeLevels = parseIntArray(grades)
	_ = json.Unmarshal(disclaimer, &p.Disclaimer)
	return p, nil
}

// ReferenceStudents implements write.ReferenceStore.
func (QualityRepo) ReferenceStudents(ctx context.Context, share float64) ([]write.ReferenceStudent, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	type course struct {
		school, class, subject uuid.UUID
		grade                  int
		objectives             []uuid.UUID
	}
	// tenancyguard:ok courses of the session tenant (explicit tenant_id + RLS), deleted_at IS NULL on courses/topics/objectives (curriculum: RLS content_read)
	rows, err := tx.QueryContext(ctx, `SELECT c.school_id, c.class_id, c.subject_id, cl.grade_level,
			ARRAY(SELECT o.objective_id FROM curriculum.learning_objectives o
				JOIN curriculum.topics t ON t.topic_id = o.topic_id AND t.deleted_at IS NULL
				WHERE t.curriculum_version_id = c.curriculum_version_id AND t.subject_id = c.subject_id AND t.grade_level = cl.grade_level
				AND o.deleted_at IS NULL ORDER BY t.sort_order, t.topic_id, o.sort_order, o.objective_id)::text
		FROM lms.courses c JOIN ref.classes cl ON cl.class_id = c.class_id AND cl.tenant_id = c.tenant_id
		WHERE c.tenant_id = infra.current_tenant_id() AND c.deleted_at IS NULL AND c.status = 'active' AND cl.grade_level BETWEEN 1 AND 9`)
	if err != nil {
		return nil, fmt.Errorf("reference courses: %w", err)
	}
	defer func() { _ = rows.Close() }() // fully read below (Next = false closes it) before the next query
	byClass := map[uuid.UUID][]course{}
	for rows.Next() {
		var c course
		var objs string
		if err := rows.Scan(&c.school, &c.class, &c.subject, &c.grade, &objs); err != nil {
			return nil, err
		}
		all := parseUUIDArray(objs)
		c.objectives = all[:int(math.Ceil(share*float64(len(all))))]
		byClass[c.class] = append(byClass[c.class], c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// tenancyguard:ok enrolments and students of the session tenant (explicit tenant_id + RLS); deleted students excluded
	srows, err := tx.QueryContext(ctx, `SELECT e.class_id, e.student_id FROM ref.enrollments e
		JOIN ref.students st ON st.student_id = e.student_id AND st.tenant_id = e.tenant_id AND st.deleted_at IS NULL
		WHERE e.tenant_id = infra.current_tenant_id() AND e.status = 'active' ORDER BY e.class_id, e.student_id`)
	if err != nil {
		return nil, fmt.Errorf("reference students: %w", err)
	}
	defer func() { _ = srows.Close() }()
	var out []write.ReferenceStudent
	for srows.Next() {
		var class, student uuid.UUID
		if err := srows.Scan(&class, &student); err != nil {
			return nil, err
		}
		for _, c := range byClass[class] {
			out = append(out, write.ReferenceStudent{SchoolID: c.school, ClassID: class, StudentID: student, SubjectID: c.subject,
				GradeLevel: c.grade, Objectives: c.objectives})
		}
	}
	return out, srows.Err()
}

// ObjectiveLevels implements write.ReferenceStore.
func (QualityRepo) ObjectiveLevels(ctx context.Context, studentID uuid.UUID, objectives []uuid.UUID) ([]write.ObjectiveLevel, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	if len(objectives) == 0 {
		return nil, nil
	}
	// tenancyguard:ok masteries of one student in the session tenant (explicit tenant_id + RLS); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT objective_id, level, evidence_count FROM learning.objective_masteries
		WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND objective_id = ANY($2::uuid[]) AND deleted_at IS NULL`,
		studentID, uuidList(objectives))
	if err != nil {
		return nil, fmt.Errorf("objective levels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ObjectiveLevel
	for rows.Next() {
		var l write.ObjectiveLevel
		if err := rows.Scan(&l.ObjectiveID, &l.Level, &l.EvidenceCount); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// OfficialScores implements write.ReferenceStore.
func (QualityRepo) OfficialScores(ctx context.Context, studentID, subjectID uuid.UUID, from, to time.Time) (*float64, []uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, nil, err
	}
	// tenancyguard:ok official sessions of one student in the session tenant (explicit tenant_id + RLS); deleted_at IS NULL on every source
	rows, err := tx.QueryContext(ctx, `SELECT es.exam_session_id, att.score::float8, att.max_score::float8, COALESCE(q.reliability_warning, false)
		FROM assessment.exam_participations p
		JOIN assessment.exam_sessions es ON es.exam_session_id = p.exam_session_id AND es.deleted_at IS NULL
		JOIN assessment.attempts att ON att.attempt_id = p.last_attempt_id AND att.deleted_at IS NULL AND att.status = 'graded'
		LEFT JOIN assessment.exam_quality_reports q ON q.exam_session_id = es.exam_session_id
		WHERE p.tenant_id = infra.current_tenant_id() AND p.student_id = $1 AND p.deleted_at IS NULL AND es.subject_id = $2 AND es.is_official
			AND es.status IN ('closed','results_released') AND es.window_end >= $3::date AND es.window_end < $4::date + 1
		ORDER BY es.window_end`, studentID, subjectID, from.Format(time.DateOnly), to.Format(time.DateOnly))
	if err != nil {
		return nil, nil, fmt.Errorf("official scores: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var sum float64
	var n int
	low := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		var score, maxScore sql.NullFloat64
		var warn bool
		if err := rows.Scan(&id, &score, &maxScore, &warn); err != nil {
			return nil, nil, err
		}
		if score.Valid && maxScore.Valid && maxScore.Float64 > 0 {
			sum += score.Float64 / maxScore.Float64 * 10
			n++
		}
		if warn && len(low) < 20 {
			low = append(low, id)
		}
	}
	if n == 0 {
		return nil, low, rows.Err()
	}
	avg := math.Round(sum/float64(n)*10) / 10
	return &avg, low, rows.Err()
}

// RapidGuessRatio implements write.ReferenceStore from learning.progress_daily.
func (QualityRepo) RapidGuessRatio(ctx context.Context, studentID, subjectID uuid.UUID, from, to time.Time) (float64, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	var rapid, answered int64
	// tenancyguard:ok derived progress_daily (no deleted_at) of one student in the session tenant (explicit tenant_id + RLS)
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(items_rapid_guess), 0), COALESCE(sum(items_answered), 0) FROM learning.progress_daily
		WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND subject_id = $2 AND bucket_date BETWEEN $3::date AND $4::date`,
		studentID, subjectID, from.Format(time.DateOnly), to.Format(time.DateOnly)).Scan(&rapid, &answered)
	if err != nil || answered == 0 {
		return 0, err
	}
	return math.Round(float64(rapid)/float64(answered)*1e4) / 1e4, nil
}

// UpsertReferenceLevels implements write.ReferenceStore.
func (QualityRepo) UpsertReferenceLevels(ctx context.Context, rs []write.ReferenceLevelRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	for _, r := range rs {
		low := r.LowReliabilityExamIDs
		if low == nil {
			low = []uuid.UUID{}
		}
		// tenancyguard:ok derived read model (no deleted_at) in the session tenant (RLS insert/update policies)
		if _, err := tx.ExecContext(ctx, `INSERT INTO learning.rm_student_subject_reference_levels (tenant_id, school_id, class_id, student_id,
				subject_id, academic_year, period, profile_id, profile_code, profile_version, level_code, objectives_in_scope, coverage_ratio,
				proficient_ratio, mastered_ratio, official_score, low_reliability_exam_ids, rapid_guess_ratio, computed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17::uuid[],$18,$19)
			ON CONFLICT (tenant_id, student_id, subject_id, academic_year, period, profile_version) DO UPDATE SET school_id = EXCLUDED.school_id,
				class_id = EXCLUDED.class_id, level_code = EXCLUDED.level_code, objectives_in_scope = EXCLUDED.objectives_in_scope,
				coverage_ratio = EXCLUDED.coverage_ratio, proficient_ratio = EXCLUDED.proficient_ratio, mastered_ratio = EXCLUDED.mastered_ratio,
				official_score = EXCLUDED.official_score, low_reliability_exam_ids = EXCLUDED.low_reliability_exam_ids,
				rapid_guess_ratio = EXCLUDED.rapid_guess_ratio, computed_at = EXCLUDED.computed_at`,
			r.TenantID, r.SchoolID, r.ClassID, r.StudentID, r.SubjectID, r.AcademicYear, r.Period, r.ProfileID, r.ProfileCode, r.ProfileVersion,
			r.LevelCode, r.InScope, math.Round(r.Coverage*1e4)/1e4, math.Round(r.Proficient*1e4)/1e4, math.Round(r.Mastered*1e4)/1e4,
			r.OfficialScore, uuidList(low), r.RapidGuessRatio, r.ComputedAt); err != nil {
			return fmt.Errorf("upsert reference level: %w", err)
		}
	}
	return nil
}

// InsertProfileVersion implements write.ReferenceStore.
func (QualityRepo) InsertProfileVersion(ctx context.Context, p write.AssessmentProfileRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	disclaimer, err := json.Marshal(p.Disclaimer)
	if err != nil {
		return err
	}
	owner := "tenant"
	if p.TenantID == nil {
		owner = "platform"
	}
	// tenancyguard:ok INSERT in the session tenant (RLS insert policy); platform profiles on learning_platform
	if _, err := tx.ExecContext(ctx, `INSERT INTO learning.assessment_profiles (profile_id, tenant_id, owner_scope, profile_code, profile_version,
			grade_levels, rules, disclaimer, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6::smallint[],$7,$8,'active',$9,$9)`,
		p.ID, p.TenantID, owner, p.Code, p.Version, intArray(p.GradeLevels), []byte(p.Rules), disclaimer, p.CreatedAt); err != nil {
		return fmt.Errorf("insert assessment profile: %w", err)
	}
	return nil
}

// SupersedeProfile implements write.ReferenceStore.
func (QualityRepo) SupersedeProfile(ctx context.Context, id uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok one profile by id visible in the session (RLS update policy); deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE learning.assessment_profiles SET status = 'superseded', updated_at = now(), version = version + 1
		WHERE profile_id = $1 AND deleted_at IS NULL`, id)
	return err
}

// ---------- helpers

func nullUUIDPtr(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	v := n.UUID
	return &v
}

func uuidList(ids []uuid.UUID) string {
	b := []byte{'{'}
	for i, id := range ids {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, id.String()...)
	}
	return string(append(b, '}'))
}

func textArray(xs []string) string {
	b, _ := json.Marshal(xs)
	s := string(b)
	return "{" + s[1:len(s)-1] + "}"
}

func intArray(xs []int) string {
	b := []byte{'{'}
	for i, x := range xs {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, fmt.Sprint(x)...)
	}
	return string(append(b, '}'))
}

func parseIntArray(s string) []int {
	var out []int
	for _, part := range splitCols(trimBraces(s)) {
		var n int
		if _, err := fmt.Sscan(part, &n); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func parseUUIDArray(s string) []uuid.UUID {
	var out []uuid.UUID
	for _, part := range splitCols(trimBraces(s)) {
		if id, err := uuid.Parse(part); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func trimBraces(s string) string {
	if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
		return s[1 : len(s)-1]
	}
	return s
}
