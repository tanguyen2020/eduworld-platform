package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MasteryRepo implements write.MasteryStore, write.EvidenceReader and write.ProgressStore with explicit SQL in the
// transaction of ctx (TenantScope: RLS T on every table it touches; F031 data-model §1).
type MasteryRepo struct{}

var (
	_ write.MasteryStore   = MasteryRepo{}
	_ write.EvidenceReader = MasteryRepo{}
	_ write.ProgressStore  = MasteryRepo{}
)

// ErrNoActiveModel is returned when no mastery model version is active (migration seeds we-1.0).
var ErrNoActiveModel = errors.New("mastery: no active model version")

func txOf(ctx context.Context) (*sql.Tx, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return tx, nil
}

// ActiveModel implements write.MasteryStore.
func (MasteryRepo) ActiveModel(ctx context.Context) (write.ModelConfig, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return write.ModelConfig{}, err
	}
	var c write.ModelConfig
	var params []byte
	// tenancyguard:ok G table learning.mastery_model_configs (platform configuration, no tenant_id)
	err = tx.QueryRowContext(ctx, `SELECT config_id, model_version, algorithm, params, status FROM learning.mastery_model_configs
		WHERE status = 'active' AND deleted_at IS NULL`).Scan(&c.ID, &c.ModelVersion, &c.Algorithm, &params, &c.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNoActiveModel
	}
	if err != nil {
		return c, fmt.Errorf("active mastery model: %w", err)
	}
	c.Params = params
	return c, nil
}

const masteryColumns = `objective_mastery_id, tenant_id, school_id, student_id, objective_id, value::float8, level, evidence_count,
	distinct_days, last_evidence_at, to_char(last_evidence_date, 'YYYY-MM-DD'), first_mastered_at, model_version, prior_source, version,
	prior_value::float8, prior_diagnostic_attempt_id`

func scanMastery(row interface{ Scan(...any) error }) (write.MasteryRow, error) {
	var r write.MasteryRow
	var level string
	var lastDate sql.NullString
	err := row.Scan(&r.ID, &r.TenantID, &r.SchoolID, &r.StudentID, &r.ObjectiveID, &r.State.Value, &level, &r.State.EvidenceCount,
		&r.State.DistinctDays, &r.State.LastEvidenceAt, &lastDate, &r.State.FirstMasteredAt, &r.ModelVersion, &r.State.PriorSource,
		&r.Version, &r.State.PriorValue, &r.PriorAttemptID)
	r.Level = mastery.Level(level)
	r.State.LastEvidenceDate = lastDate.String
	return r, err
}

// Mastery implements write.MasteryStore.
func (MasteryRepo) Mastery(ctx context.Context, studentID, objectiveID uuid.UUID) (*write.MasteryRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// RLS T limits the rows to the session tenant; the filter repeats it with infra.current_tenant_id().
	// tenancyguard:ok tenant of the session (RLS) and deleted_at IS NULL
	r, err := scanMastery(tx.QueryRowContext(ctx, `SELECT `+masteryColumns+` FROM learning.objective_masteries
		WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND objective_id = $2 AND deleted_at IS NULL`, studentID, objectiveID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mastery: %w", err)
	}
	return &r, nil
}

func lastDate(s mastery.State) any {
	if s.LastEvidenceDate == "" {
		return nil
	}
	return s.LastEvidenceDate
}

// InsertMastery implements write.MasteryStore (ON CONFLICT DO NOTHING: a concurrent first evidence keeps the
// transaction alive and is applied again on the row it created).
func (MasteryRepo) InsertMastery(ctx context.Context, r write.MasteryRow) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	s := r.State
	prior := s.PriorSource
	if prior == "" {
		prior = mastery.PriorDefault
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy); conflict on the live unique (tenant, student, objective)
	res, err := tx.ExecContext(ctx, `INSERT INTO learning.objective_masteries (objective_mastery_id, tenant_id, school_id, student_id,
		objective_id, value, level, evidence_count, distinct_days, last_evidence_at, last_evidence_date, first_mastered_at, model_version,
		prior_source, version, prior_value, prior_diagnostic_attempt_id) VALUES ($1, $2, $3, $4, $5, round($6::numeric, 3), $7, $8, $9, $10,
		$11::date, $12, $13, $14, 1, round($15::numeric, 3), $16)
		ON CONFLICT (tenant_id, student_id, objective_id) WHERE deleted_at IS NULL DO NOTHING`,
		r.ID, r.TenantID, r.SchoolID, r.StudentID, r.ObjectiveID, s.Value, string(r.Level), s.EvidenceCount, s.DistinctDays,
		s.LastEvidenceAt, lastDate(s), s.FirstMasteredAt, r.ModelVersion, prior, s.PriorValue, r.PriorAttemptID)
	if err != nil {
		return false, fmt.Errorf("insert mastery: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func priorSource(s mastery.State) string {
	if s.PriorSource == "" {
		return mastery.PriorDefault
	}
	return s.PriorSource
}

// UpdateMastery implements write.MasteryStore (optimistic: WHERE version = r.Version, F031-AC-14).
func (MasteryRepo) UpdateMastery(ctx context.Context, r write.MasteryRow) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	s := r.State
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.objective_masteries SET value = round($2::numeric, 3), level = $3, evidence_count = $4,
		distinct_days = $5, last_evidence_at = $6, last_evidence_date = $7::date, first_mastered_at = $8, model_version = $9,
		school_id = $10, prior_source = $12, prior_value = round($13::numeric, 3), prior_diagnostic_attempt_id = $14, version = version + 1,
		updated_at = now()
		WHERE objective_mastery_id = $1 AND version = $11 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		r.ID, s.Value, string(r.Level), s.EvidenceCount, s.DistinctDays, s.LastEvidenceAt, lastDate(s), s.FirstMasteredAt, r.ModelVersion,
		r.SchoolID, r.Version, priorSource(s), s.PriorValue, r.PriorAttemptID)
	if err != nil {
		return false, fmt.Errorf("update mastery: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// AppendHistory implements write.MasteryStore.
func (MasteryRepo) AppendHistory(ctx context.Context, h write.HistoryRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var lb any
	if h.LevelBefore != nil {
		lb = string(*h.LevelBefore)
	}
	var vb any
	if h.ValueBefore != nil {
		vb = *h.ValueBefore
	}
	// tenancyguard:ok append-only mastery_history without deleted_at; insert of the session tenant (RLS insert policy)
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.mastery_history (history_id, tenant_id, student_id, objective_id, value_before,
		value_after, level_before, level_after, evidence_count, source_event_id, source_kind, model_version, occurred_at)
		VALUES ($1, $2, $3, $4, round($5::numeric, 3), round($6::numeric, 3), $7, $8, $9, $10, $11, $12, $13)`,
		h.ID, h.TenantID, h.StudentID, h.ObjectiveID, vb, h.ValueAfter, lb, string(h.LevelAfter), h.EvidenceCount, h.SourceEventID,
		h.SourceKind, h.ModelVersion, h.OccurredAt.UTC())
	if err != nil {
		return fmt.Errorf("mastery history: %w", err)
	}
	return nil
}

// Attempt implements write.EvidenceReader: the subject is the activity version's, else the first item's question version's.
func (MasteryRepo) Attempt(ctx context.Context, attemptID uuid.UUID) (write.AttemptFacts, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return write.AttemptFacts{}, err
	}
	var f write.AttemptFacts
	var subject uuid.NullUUID
	var graded sql.NullTime
	var duration sql.NullInt64
	var rec uuid.NullUUID
	// tenancyguard:ok tenant of the session on every table (infra.current_tenant_id() / RLS) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT a.attempt_id, a.student_id, a.school_id, a.mode, a.graded_at, a.duration_ms, a.started_at,
			a.recommendation_id,
			COALESCE(
				(SELECT v.subject_id FROM content.activity_versions v WHERE v.activity_version_id = a.source_version_id AND v.deleted_at IS NULL),
				(SELECT q.subject_id FROM assessment.attempt_items i JOIN content.question_versions q ON q.question_version_id = i.question_version_id
				  WHERE i.attempt_id = a.attempt_id AND i.tenant_id = a.tenant_id AND i.deleted_at IS NULL AND q.deleted_at IS NULL
				  ORDER BY i.position LIMIT 1))
		FROM assessment.attempts a WHERE a.attempt_id = $1 AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL`, attemptID).
		Scan(&f.AttemptID, &f.StudentID, &f.SchoolID, &f.Mode, &graded, &duration, &f.StartedAt, &rec, &subject)
	if rec.Valid {
		id := rec.UUID
		f.RecommendationID = &id
	}
	if errors.Is(err, sql.ErrNoRows) {
		return f, problem.New(problem.NotFound, "attempt not found")
	}
	if err != nil {
		return f, fmt.Errorf("attempt facts: %w", err)
	}
	if subject.Valid {
		f.SubjectID = subject.UUID
	}
	if graded.Valid {
		f.GradedAt = graded.Time.UTC()
	}
	f.DurationMS = duration.Int64
	return f, nil
}

// evidenceFilter keeps the question.answered events that are mastery evidence (is_scored and not rapid_guess,
// LEARNING_ANALYTICS §3.1; is_scored defaults to true in the schema) and not part of an efficacy wave (F035). The answers
// of a diagnostic set the prior M0 instead (LEARNING_ANALYTICS §3.5, F032) and are no evidence.
const evidenceFilter = `le.event_type = 'question.answered' AND COALESCE((le.payload->>'is_scored')::boolean, true)
	AND NOT COALESCE((le.payload->>'rapid_guess')::boolean, false) AND le.context <> 'diagnostic' AND le.efficacy_wave_id IS NULL`

// AttemptObjectives implements write.EvidenceReader.
func (MasteryRepo) AttemptObjectives(ctx context.Context, attemptID uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only learning_events without deleted_at, tenant of the session (RLS + infra.current_tenant_id())
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT o FROM learning.learning_events le, unnest(le.objective_ids) o
		WHERE le.tenant_id = infra.current_tenant_id() AND le.payload->>'attempt_id' = $1::text AND `+evidenceFilter+` ORDER BY o`,
		attemptID)
	if err != nil {
		return nil, fmt.Errorf("attempt objectives: %w", err)
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

// ObjectiveEvidence implements write.EvidenceReader: every evidence of (student, objective) in occurred_at order with
// the difficulty of the question version, the options shown (presentation of the attempt item) and the repeat flag
// (the same question answered by the student in the 24 hours before; question versions invisible to the tenant fall
// back to the version id and difficulty 3 [Proposed]). The effective re-evaluation of the attempt (F031-AC-21) replaces the
// outcome of the item (score, max_score of evaluation details) and drops the evidence of a dropped item.
func (MasteryRepo) ObjectiveEvidence(ctx context.Context, studentID, objectiveID uuid.UUID) ([]write.EvidenceRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session on every table (infra.current_tenant_id() / RLS); learning_events append-only, others deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `WITH ev AS (
			SELECT le.event_id, le.occurred_at, le.context, le.objective_ids, le.payload,
				COALESCE(q.question_id, (le.payload->>'question_version_id')::uuid) AS question_key, q.difficulty,
				(SELECT jsonb_array_length(i.presentation->'orders'->'options') FROM assessment.attempt_items i
				  WHERE i.attempt_id = (le.payload->>'attempt_id')::uuid AND i.position = (le.payload->>'position')::int
				    AND i.tenant_id = le.tenant_id AND i.deleted_at IS NULL) AS options
				, ov.item AS override
			FROM learning.learning_events le
			LEFT JOIN content.question_versions q ON q.question_version_id = (le.payload->>'question_version_id')::uuid AND q.deleted_at IS NULL
			LEFT JOIN LATERAL (SELECT it FROM assessment.evaluations e2, jsonb_array_elements(COALESCE(e2.details->'items', '[]'::jsonb)) it
				WHERE e2.attempt_id = (le.payload->>'attempt_id')::uuid AND e2.effective AND e2.source = 're_evaluation'
					AND e2.tenant_id = le.tenant_id AND e2.deleted_at IS NULL AND (it->>'position')::int = (le.payload->>'position')::int
				LIMIT 1) ov(item) ON true
			WHERE le.tenant_id = infra.current_tenant_id() AND le.student_id = $1 AND le.event_type = 'question.answered'
				AND le.efficacy_wave_id IS NULL)
		SELECT e.event_id, e.occurred_at, e.context, array_to_string(e.objective_ids, ','), e.payload->>'question_type',
			COALESCE(e.difficulty, 0),
			COALESCE((e.override->>'score')::float8, (e.payload->'score'->>'raw')::float8, 0),
			COALESCE((e.override->>'max_score')::float8, (e.payload->'score'->>'max')::float8, 0),
			COALESCE((e.payload->>'hints_used')::int, 0), COALESCE(e.options, 0),
			EXISTS (SELECT 1 FROM ev p WHERE p.question_key = e.question_key AND p.occurred_at < e.occurred_at
				AND p.occurred_at >= e.occurred_at - interval '24 hours')
		FROM ev e
		WHERE $2 = ANY(e.objective_ids) AND COALESCE((e.payload->>'is_scored')::boolean, true) AND e.context <> 'diagnostic'
			AND NOT COALESCE((e.payload->>'rapid_guess')::boolean, false) AND NOT COALESCE((e.override->>'dropped')::boolean, false)
		ORDER BY e.occurred_at, e.event_id`, studentID, objectiveID)
	if err != nil {
		return nil, fmt.Errorf("objective evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.EvidenceRow
	for rows.Next() {
		var r write.EvidenceRow
		var objectives string
		var qtype sql.NullString
		var occurred time.Time
		if err := rows.Scan(&r.EventID, &occurred, &r.Mode, &objectives, &qtype, &r.Difficulty, &r.Raw, &r.Max,
			&r.HintsUsed, &r.OptionCount, &r.Repeat); err != nil {
			return nil, err
		}
		r.OccurredAt = occurred.UTC()
		r.QuestionType = qtype.String
		for _, o := range strings.Split(objectives, ",") {
			if id, err := uuid.Parse(o); err == nil {
				r.ObjectiveIDs = append(r.ObjectiveIDs, id)
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FirstAnswers implements write.EvidenceReader: the first scored answer of each item (attempt_no = 1).
func (MasteryRepo) FirstAnswers(ctx context.Context, attemptID uuid.UUID) (write.ItemCounts, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return write.ItemCounts{}, err
	}
	var c write.ItemCounts
	// tenancyguard:ok append-only learning_events without deleted_at, tenant of the session (RLS + infra.current_tenant_id())
	err = tx.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE (le.payload->>'is_correct')::boolean),
			count(*) FILTER (WHERE COALESCE((le.payload->>'rapid_guess')::boolean, false)),
			count(*) FILTER (WHERE COALESCE((le.payload->>'hints_used')::int, 0) > 0),
			count(*) FILTER (WHERE (le.payload->>'is_correct')::boolean AND COALESCE((le.payload->>'rapid_guess')::boolean, false))
		FROM learning.learning_events le
		WHERE le.tenant_id = infra.current_tenant_id() AND le.event_type = 'question.answered' AND le.payload->>'attempt_id' = $1::text
			AND COALESCE((le.payload->>'attempt_no')::int, 1) = 1 AND COALESCE((le.payload->>'is_scored')::boolean, true)`, attemptID).
		Scan(&c.Answered, &c.Correct, &c.RapidGuess, &c.WithHint, &c.CorrectRapid)
	if err != nil {
		return c, fmt.Errorf("first answers: %w", err)
	}
	return c, nil
}

// AddDaily implements write.ProgressStore (upsert of counters; grade_level is the grade of the student when the bucket
// is first written — ADR-0095 — and class_id the enrollment snapshot, kept for the class rollups until WP-L2).
func (MasteryRepo) AddDaily(ctx context.Context, d write.ProgressDelta) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok derived progress_daily has no deleted_at (DATABASE_DESIGN §2.7); insert/update of the session tenant (RLS)
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.progress_daily (tenant_id, student_id, subject_id, bucket_date, metric_version, school_id,
			class_id, activities_completed, items_answered, items_correct, items_rapid_guess, items_with_hint, active_seconds, lessons_completed,
			items_correct_rapid_guess, grade_level)
		VALUES ($1, $2, $3, $4::date, 1, $5,
			COALESCE($6::uuid, (SELECT en.class_id FROM ref.enrollments en WHERE en.student_id = $2 AND en.tenant_id = $1
				AND en.status = 'active' ORDER BY en.start_date DESC LIMIT 1)),
			$7, $8, $9, $10, $11, $12, $13, $14,
			COALESCE($15::smallint, (SELECT st.grade_level FROM ref.students st WHERE st.student_id = $2 AND st.tenant_id = $1)))
		ON CONFLICT (tenant_id, student_id, subject_id, bucket_date, metric_version) DO UPDATE SET
			items_correct_rapid_guess = progress_daily.items_correct_rapid_guess + EXCLUDED.items_correct_rapid_guess,
			activities_completed = progress_daily.activities_completed + EXCLUDED.activities_completed,
			items_answered = progress_daily.items_answered + EXCLUDED.items_answered,
			items_correct = progress_daily.items_correct + EXCLUDED.items_correct,
			items_rapid_guess = progress_daily.items_rapid_guess + EXCLUDED.items_rapid_guess,
			items_with_hint = progress_daily.items_with_hint + EXCLUDED.items_with_hint,
			active_seconds = progress_daily.active_seconds + EXCLUDED.active_seconds,
			lessons_completed = progress_daily.lessons_completed + EXCLUDED.lessons_completed,
			grade_level = COALESCE(progress_daily.grade_level, EXCLUDED.grade_level),
			computed_at = now()`,
		d.TenantID, d.StudentID, d.SubjectID, d.Day.Format(time.DateOnly), d.SchoolID, d.ClassID, d.ActivitiesCompleted, d.Items.Answered,
		d.Items.Correct, d.Items.RapidGuess, d.Items.WithHint, d.ActiveSeconds, d.LessonsCompleted, d.Items.CorrectRapid, d.GradeLevel)
	if err != nil {
		return fmt.Errorf("progress daily: %w", err)
	}
	return nil
}
