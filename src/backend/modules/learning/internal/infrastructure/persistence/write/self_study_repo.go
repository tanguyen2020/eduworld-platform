package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/retrywrong"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
)

// SelfStudyRepo implements write.WrongQuestionStore, write.RecommendationStore and write.SelfStudyReader of F032 with
// explicit SQL in the transaction of ctx: every table it writes is tenant (RLS T, infra.current_tenant_id()) and
// soft-delete only (AGENTS R21).
type SelfStudyRepo struct{}

var (
	_ write.WrongQuestionStore  = SelfStudyRepo{}
	_ write.RecommendationStore = SelfStudyRepo{}
	_ write.SelfStudyReader     = SelfStudyRepo{}
)

// ---------- review list (learning.wrong_question_entries)

const wrongColumns = `entry_id, tenant_id, school_id, student_id, question_id, question_version_id, subject_id, array_to_string(objective_ids, ','),
	question_type, difficulty, first_wrong_attempt_id, wrong_count, last_wrong_at, stage, next_due_at, correct_streak,
	COALESCE(to_char(last_correct_on, 'YYYY-MM-DD'), ''), status, retry_attempt_id, retry_position, version`

func scanWrong(row interface{ Scan(...any) error }) (write.WrongEntryRow, error) {
	var r write.WrongEntryRow
	var subject, retry uuid.NullUUID
	var los string
	var pos sql.NullInt32
	err := row.Scan(&r.ID, &r.TenantID, &r.SchoolID, &r.StudentID, &r.QuestionID, &r.QuestionVersionID, &subject, &los, &r.QuestionType,
		&r.Difficulty, &r.FirstWrongAttemptID, &r.Entry.WrongCount, &r.Entry.LastWrongAt, &r.Entry.Stage, &r.Entry.NextDueAt,
		&r.Entry.CorrectStreak, &r.Entry.LastCorrectOn, &r.Entry.Status, &retry, &pos, &r.Version)
	if subject.Valid {
		s := subject.UUID
		r.SubjectID = &s
	}
	if retry.Valid {
		a := retry.UUID
		r.RetryAttemptID = &a
	}
	if pos.Valid {
		p := int(pos.Int32)
		r.RetryPosition = &p
	}
	r.ObjectiveIDs = parseIDs(los)
	r.Entry.LastWrongAt, r.Entry.NextDueAt = r.Entry.LastWrongAt.UTC(), r.Entry.NextDueAt.UTC()
	return r, err
}

func parseIDs(list string) []uuid.UUID {
	var out []uuid.UUID
	for _, s := range strings.Split(list, ",") {
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func idArray(ids []uuid.UUID) string { return uuidArray(ids) }

func wrongOne(ctx context.Context, where string, args ...any) (*write.WrongEntryRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL in every caller condition
	r, err := scanWrong(tx.QueryRowContext(ctx, `SELECT `+wrongColumns+` FROM learning.wrong_question_entries
		WHERE tenant_id = infra.current_tenant_id() AND deleted_at IS NULL AND status = 'active' AND `+where+` LIMIT 1`, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("wrong question entry: %w", err)
	}
	return &r, nil
}

// ActiveEntry implements write.WrongQuestionStore.
func (SelfStudyRepo) ActiveEntry(ctx context.Context, studentID, questionID uuid.UUID) (*write.WrongEntryRow, error) {
	return wrongOne(ctx, `student_id = $1 AND question_id = $2`, studentID, questionID)
}

// RetryEntry implements write.WrongQuestionStore.
func (SelfStudyRepo) RetryEntry(ctx context.Context, attemptID uuid.UUID, position int) (*write.WrongEntryRow, error) {
	return wrongOne(ctx, `retry_attempt_id = $1 AND retry_position = $2`, attemptID, position)
}

func lastCorrect(e retrywrong.Entry) any {
	if e.LastCorrectOn == "" {
		return nil
	}
	return e.LastCorrectOn
}

func clearedAt(e retrywrong.Entry, now time.Time) any {
	if e.Status == retrywrong.StatusCleared {
		return now
	}
	return nil
}

// InsertEntry implements write.WrongQuestionStore (ON CONFLICT DO NOTHING on the live active unique).
func (SelfStudyRepo) InsertEntry(ctx context.Context, r write.WrongEntryRow) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	e := r.Entry
	// tenancyguard:ok insert of the session tenant (RLS insert policy); conflict on the live active unique (tenant, student, question)
	res, err := tx.ExecContext(ctx, `INSERT INTO learning.wrong_question_entries (entry_id, tenant_id, school_id, student_id, question_id,
		question_version_id, subject_id, objective_ids, question_type, difficulty, first_wrong_attempt_id, wrong_count, last_wrong_at, stage,
		next_due_at, correct_streak, last_correct_on, status, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::uuid[], $9, $10, $11, $12, $13, $14, $15, $16, $17::date, $18, 1)
		ON CONFLICT (tenant_id, student_id, question_id) WHERE deleted_at IS NULL AND status = 'active' DO NOTHING`,
		r.ID, r.TenantID, r.SchoolID, r.StudentID, r.QuestionID, r.QuestionVersionID, r.SubjectID, idArray(r.ObjectiveIDs), r.QuestionType,
		r.Difficulty, r.FirstWrongAttemptID, e.WrongCount, e.LastWrongAt, e.Stage, e.NextDueAt, e.CorrectStreak, lastCorrect(e), e.Status)
	if err != nil {
		return false, fmt.Errorf("insert wrong question entry: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// UpdateEntry implements write.WrongQuestionStore (optimistic on version).
func (SelfStudyRepo) UpdateEntry(ctx context.Context, r write.WrongEntryRow) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	e := r.Entry
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.wrong_question_entries SET wrong_count = $2, last_wrong_at = $3, stage = $4,
		next_due_at = $5, correct_streak = $6, last_correct_on = $7::date, status = $8, cleared_at = $9, retry_attempt_id = $10,
		retry_position = $11, version = version + 1, updated_at = now()
		WHERE entry_id = $1 AND version = $12 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		r.ID, e.WrongCount, e.LastWrongAt, e.Stage, e.NextDueAt, e.CorrectStreak, lastCorrect(e), e.Status, clearedAt(e, time.Now().UTC()),
		r.RetryAttemptID, r.RetryPosition, r.Version)
	if err != nil {
		return false, fmt.Errorf("update wrong question entry: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func wrongList(ctx context.Context, where string, args ...any) ([]write.WrongEntryRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT `+wrongColumns+` FROM learning.wrong_question_entries
		WHERE tenant_id = infra.current_tenant_id() AND deleted_at IS NULL AND status = 'active' AND `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("wrong question entries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.WrongEntryRow
	for rows.Next() {
		r, err := scanWrong(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DueEntries implements write.WrongQuestionStore.
func (SelfStudyRepo) DueEntries(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID, now time.Time, limit int) ([]write.WrongEntryRow, error) {
	return wrongList(ctx, `student_id = $1 AND next_due_at <= $2 AND ($3::uuid IS NULL OR subject_id = $3::uuid)
		ORDER BY next_due_at, entry_id LIMIT $4`, studentID, now, subjectID, limit)
}

// AttemptEntries implements write.WrongQuestionStore.
func (SelfStudyRepo) AttemptEntries(ctx context.Context, attemptID uuid.UUID) ([]write.WrongEntryRow, error) {
	return wrongList(ctx, `first_wrong_attempt_id = $1 ORDER BY entry_id`, attemptID)
}

// ---------- recommendations (learning.rm_student_recommendations)

const recColumns = `recommendation_id, tenant_id, school_id, student_id, rule_code, kind, target_type, target_id, subject_id, grade_level,
	array_to_string(objective_ids, ','), reason, reason_code, reason_params, priority, status, skip_count, started_attempt_id, generated_at, expires_at, closed_at,
	version`

func scanRec(row interface{ Scan(...any) error }) (write.RecommendationRow, error) {
	var r write.RecommendationRow
	var subject, started uuid.NullUUID
	var grade sql.NullInt32
	var los string
	var closed sql.NullTime
	err := row.Scan(&r.ID, &r.TenantID, &r.SchoolID, &r.StudentID, &r.RuleCode, &r.Kind, &r.TargetType, &r.TargetID, &subject, &grade, &los,
		&r.Reason, &r.ReasonCode, &r.ReasonParams, &r.Priority, &r.Status, &r.SkipCount, &started, &r.GeneratedAt, &r.ExpiresAt, &closed,
		&r.Version)
	if subject.Valid {
		s := subject.UUID
		r.SubjectID = &s
	}
	if grade.Valid {
		g := int(grade.Int32)
		r.Grade = &g
	}
	if started.Valid {
		s := started.UUID
		r.StartedAttemptID = &s
	}
	if closed.Valid {
		c := closed.Time.UTC()
		r.ClosedAt = &c
	}
	r.ObjectiveIDs = parseIDs(los)
	r.GeneratedAt, r.ExpiresAt = r.GeneratedAt.UTC(), r.ExpiresAt.UTC()
	return r, err
}

func recQuery(ctx context.Context, where string, args ...any) ([]write.RecommendationRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT `+recColumns+` FROM learning.rm_student_recommendations
		WHERE tenant_id = infra.current_tenant_id() AND deleted_at IS NULL AND `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("recommendations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.RecommendationRow
	for rows.Next() {
		r, err := scanRec(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func recOne(ctx context.Context, where string, args ...any) (*write.RecommendationRow, error) {
	rs, err := recQuery(ctx, where+` LIMIT 1`, args...)
	if err != nil || len(rs) == 0 {
		return nil, err
	}
	return &rs[0], nil
}

// ActiveRecommendations implements write.RecommendationStore.
func (SelfStudyRepo) ActiveRecommendations(ctx context.Context, studentID uuid.UUID) ([]write.RecommendationRow, error) {
	return recQuery(ctx, `student_id = $1 AND status = 'active' ORDER BY priority, generated_at, recommendation_id`, studentID)
}

// BlockedTargets implements write.RecommendationStore.
func (SelfStudyRepo) BlockedTargets(ctx context.Context, studentID uuid.UUID, skips int) ([]string, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT kind || '|' || target_type || '|' || target_id::text FROM learning.rm_student_recommendations
		WHERE tenant_id = infra.current_tenant_id() AND deleted_at IS NULL AND student_id = $1 AND skip_count >= $2`, studentID, skips)
	if err != nil {
		return nil, fmt.Errorf("blocked recommendation targets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Recommendation implements write.RecommendationStore.
func (SelfStudyRepo) Recommendation(ctx context.Context, studentID, id uuid.UUID) (*write.RecommendationRow, error) {
	return recOne(ctx, `student_id = $1 AND recommendation_id = $2`, studentID, id)
}

// RecommendationByAttempt implements write.RecommendationStore.
func (SelfStudyRepo) RecommendationByAttempt(ctx context.Context, attemptID uuid.UUID) (*write.RecommendationRow, error) {
	return recOne(ctx, `started_attempt_id = $1`, attemptID)
}

func params(p json.RawMessage) []byte {
	if len(p) == 0 {
		return []byte("{}")
	}
	return p
}

// InsertRecommendation implements write.RecommendationStore.
func (SelfStudyRepo) InsertRecommendation(ctx context.Context, r write.RecommendationRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy)
	if _, err := tx.ExecContext(ctx, `INSERT INTO learning.rm_student_recommendations (recommendation_id, tenant_id, school_id, student_id,
		rule_code, kind, target_type, target_id, subject_id, grade_level, objective_ids, reason, reason_code, reason_params, priority, status,
		skip_count, generated_at, expires_at, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::uuid[], $12, $13, $14::jsonb, $15, 'active', $16, $17, $18, 1)`,
		r.ID, r.TenantID, r.SchoolID, r.StudentID, r.RuleCode, r.Kind, r.TargetType, r.TargetID, r.SubjectID, r.Grade, idArray(r.ObjectiveIDs),
		r.Reason, r.ReasonCode, params(r.ReasonParams), r.Priority, r.SkipCount, r.GeneratedAt, r.ExpiresAt); err != nil {
		return fmt.Errorf("insert recommendation: %w", err)
	}
	return nil
}

// UpdateRecommendation implements write.RecommendationStore (optimistic on version).
func (SelfStudyRepo) UpdateRecommendation(ctx context.Context, r write.RecommendationRow) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.rm_student_recommendations SET objective_ids = $2::uuid[], reason_params = $3::jsonb,
		priority = $4, status = $5, skip_count = $6, started_attempt_id = $7, closed_at = $8, version = version + 1, updated_at = now()
		WHERE recommendation_id = $1 AND version = $9 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		r.ID, idArray(r.ObjectiveIDs), params(r.ReasonParams), r.Priority, r.Status, r.SkipCount, r.StartedAttemptID, r.ClosedAt, r.Version)
	if err != nil {
		return false, fmt.Errorf("update recommendation: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// ---------- reads of the self-study use cases

// Objectives implements write.SelfStudyReader: objectives of the published curriculum versions visible to the tenant
// (own or assigned, MULTITENANCY §4.2).
func (SelfStudyRepo) Objectives(ctx context.Context, f write.ObjectiveFilter) ([]write.ObjectiveInfo, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	cond, args, err := read.HybridCondition(ctx, "lo", contentmodel.ResourceCurriculum, "curriculum_id", 1)
	if err != nil {
		return nil, err
	}
	n := len(args)
	args = append(args, f.Grade, f.SubjectID, idArray(f.TopicIDs), idArray(f.ObjectiveIDs))
	where := fmt.Sprintf(`($%d::int IS NULL OR t.grade_level = $%d::int) AND ($%d::uuid IS NULL OR t.subject_id = $%d::uuid)
		AND (cardinality($%d::uuid[]) = 0 OR lo.topic_id = ANY($%d::uuid[])) AND (cardinality($%d::uuid[]) = 0 OR lo.objective_id = ANY($%d::uuid[]))`,
		n+1, n+1, n+2, n+2, n+3, n+3, n+4, n+4)
	if f.WithPrerequisites {
		// The selected objectives and their direct prerequisites (prerequisite_ids, jsonb array of ids).
		where = fmt.Sprintf(`(%[1]s) OR lo.objective_id IN (SELECT (p.value #>> '{}')::uuid FROM curriculum.learning_objectives s
			JOIN curriculum.topics st ON st.topic_id = s.topic_id AND st.deleted_at IS NULL, jsonb_array_elements(s.prerequisite_ids) p
			WHERE s.deleted_at IS NULL AND ($%[2]d::int IS NULL OR st.grade_level = $%[2]d::int) AND ($%[3]d::uuid IS NULL OR st.subject_id = $%[3]d::uuid)
			AND (cardinality($%[4]d::uuid[]) = 0 OR s.topic_id = ANY($%[4]d::uuid[])) AND (cardinality($%[5]d::uuid[]) = 0 OR s.objective_id = ANY($%[5]d::uuid[])))`,
			where, n+1, n+2, n+3, n+4)
	}
	// tenancyguard:ok HybridCondition on lo holds tenant_id and deleted_at IS NULL; topics and versions are reached through lo (same curriculum)
	rows, err := tx.QueryContext(ctx, `SELECT lo.objective_id, lo.code, lo.description, lo.topic_id, t.name, t.sort_order, t.subject_id,
		t.grade_level, lo.sort_order, COALESCE(lo.prerequisite_ids, '[]'::jsonb)
		FROM curriculum.learning_objectives lo
		JOIN curriculum.topics t ON t.topic_id = lo.topic_id AND t.deleted_at IS NULL
		JOIN curriculum.curriculum_versions cv ON cv.curriculum_version_id = lo.curriculum_version_id AND cv.deleted_at IS NULL
			AND cv.status = 'published'
		WHERE `+cond+` AND (`+where+`) ORDER BY t.grade_level, t.sort_order, lo.sort_order, lo.objective_id LIMIT 2000`, args...)
	if err != nil {
		return nil, fmt.Errorf("self-study objectives: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ObjectiveInfo
	for rows.Next() {
		var o write.ObjectiveInfo
		var pre []byte
		if err := rows.Scan(&o.ID, &o.Code, &o.Name, &o.TopicID, &o.TopicName, &o.TopicOrder, &o.SubjectID, &o.Grade, &o.SortOrder, &pre); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(pre, &o.Prerequisites)
		out = append(out, o)
	}
	return out, rows.Err()
}

// StudentMasteries implements write.SelfStudyReader.
func (SelfStudyRepo) StudentMasteries(ctx context.Context, studentID uuid.UUID) ([]write.MasteryRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT `+masteryColumns+` FROM learning.objective_masteries
		WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND deleted_at IS NULL ORDER BY objective_id LIMIT 5000`, studentID)
	if err != nil {
		return nil, fmt.Errorf("student masteries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.MasteryRow
	for rows.Next() {
		r, err := scanMastery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecentQuestions implements write.SelfStudyReader: the questions of the items delivered to the student since t.
func (SelfStudyRepo) RecentQuestions(ctx context.Context, studentID uuid.UUID, since time.Time) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session on attempts and items (infra.current_tenant_id() / RLS) and deleted_at IS NULL; question_versions reached by id
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT q.question_id FROM assessment.attempts a
		JOIN assessment.attempt_items i ON i.attempt_id = a.attempt_id AND i.tenant_id = a.tenant_id AND i.deleted_at IS NULL
		JOIN content.question_versions q ON q.question_version_id = i.question_version_id
		WHERE a.tenant_id = infra.current_tenant_id() AND a.student_id = $1 AND a.started_at >= $2 AND a.deleted_at IS NULL`, studentID, since)
	if err != nil {
		return nil, fmt.Errorf("recent questions: %w", err)
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

// LastDiagnostics implements write.SelfStudyReader: the latest diagnostic start per context (subject or topic) and,
// for a topic, its subject too.
func (SelfStudyRepo) LastDiagnostics(ctx context.Context, studentID uuid.UUID) (map[uuid.UUID]time.Time, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session on attempts (infra.current_tenant_id() / RLS) and deleted_at IS NULL; topics reached by id
	rows, err := tx.QueryContext(ctx, `SELECT a.context_id, t.subject_id, max(a.started_at) FROM assessment.attempts a
		LEFT JOIN curriculum.topics t ON t.topic_id = a.context_id
		WHERE a.tenant_id = infra.current_tenant_id() AND a.student_id = $1 AND a.mode = 'diagnostic' AND a.deleted_at IS NULL
		GROUP BY a.context_id, t.subject_id`, studentID)
	if err != nil {
		return nil, fmt.Errorf("last diagnostics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]time.Time{}
	for rows.Next() {
		var id uuid.UUID
		var subject uuid.NullUUID
		var at time.Time
		if err := rows.Scan(&id, &subject, &at); err != nil {
			return nil, err
		}
		out[id] = latest(out[id], at)
		if subject.Valid {
			out[subject.UUID] = latest(out[subject.UUID], at)
		}
	}
	return out, rows.Err()
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b.UTC()
	}
	return a
}

// LearnedTopics implements write.SelfStudyReader: the topics of a subject where the student has evidence, the most
// recent evidence first.
func (SelfStudyRepo) LearnedTopics(ctx context.Context, studentID, subjectID uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session on masteries (infra.current_tenant_id() / RLS) and deleted_at IS NULL; objectives and topics reached by id
	rows, err := tx.QueryContext(ctx, `SELECT lo.topic_id FROM learning.objective_masteries m
		JOIN curriculum.learning_objectives lo ON lo.objective_id = m.objective_id AND lo.deleted_at IS NULL
		JOIN curriculum.topics t ON t.topic_id = lo.topic_id AND t.deleted_at IS NULL
		WHERE m.tenant_id = infra.current_tenant_id() AND m.student_id = $1 AND m.deleted_at IS NULL AND m.evidence_count > 0
			AND t.subject_id = $2
		GROUP BY lo.topic_id ORDER BY max(m.last_evidence_at) DESC, lo.topic_id LIMIT 20`, studentID, subjectID)
	if err != nil {
		return nil, fmt.Errorf("learned topics: %w", err)
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

// DiagnosticOutcomes implements write.SelfStudyReader.
func (SelfStudyRepo) DiagnosticOutcomes(ctx context.Context, attemptID uuid.UUID) (map[uuid.UUID][]float64, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only learning_events without deleted_at, tenant of the session (RLS + infra.current_tenant_id())
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT ON ((le.payload->>'position')::int) array_to_string(le.objective_ids, ','),
			COALESCE((le.payload->'score'->>'raw')::float8, 0), COALESCE((le.payload->'score'->>'max')::float8, 0),
			COALESCE((le.payload->>'rapid_guess')::boolean, false)
		FROM learning.learning_events le
		WHERE le.tenant_id = infra.current_tenant_id() AND le.event_type = 'question.answered' AND le.payload->>'attempt_id' = $1::text
			AND COALESCE((le.payload->>'is_scored')::boolean, true)
		ORDER BY (le.payload->>'position')::int, le.occurred_at DESC, le.event_id DESC`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("diagnostic outcomes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID][]float64{}
	for rows.Next() {
		var los string
		var raw, maxScore float64
		var rapid bool
		if err := rows.Scan(&los, &raw, &maxScore, &rapid); err != nil {
			return nil, err
		}
		if rapid || maxScore <= 0 {
			continue
		}
		for _, lo := range parseIDs(los) {
			out[lo] = append(out[lo], raw/maxScore)
		}
	}
	return out, rows.Err()
}

// RecomputeStudents implements write.RecommendationStore (ctx: tenancy.Bypass(platform_job)).
func (SelfStudyRepo) RecomputeStudents(ctx context.Context, since time.Time, limit int) ([]write.StudentRef, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok job recommendation-nightly under tenancy.Bypass(platform_job): ids of every tenant only, deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id, student_id FROM (
			SELECT tenant_id, student_id FROM learning.objective_masteries WHERE deleted_at IS NULL AND last_evidence_at >= $1
			UNION SELECT tenant_id, student_id FROM learning.rm_student_recommendations WHERE deleted_at IS NULL AND status = 'active'
			UNION SELECT tenant_id, student_id FROM learning.wrong_question_entries WHERE deleted_at IS NULL AND status = 'active'
		) s ORDER BY tenant_id, student_id LIMIT $2`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("recompute students: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.StudentRef
	for rows.Next() {
		var r write.StudentRef
		if err := rows.Scan(&r.TenantID, &r.StudentID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReEvaluatedCorrect implements write.SelfStudyReader.
func (SelfStudyRepo) ReEvaluatedCorrect(ctx context.Context, attemptID uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session on evaluations and items (infra.current_tenant_id() / RLS) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT i.question_version_id FROM assessment.evaluations e
		CROSS JOIN LATERAL jsonb_array_elements(COALESCE(e.details->'items', '[]'::jsonb)) it
		JOIN assessment.attempt_items i ON i.attempt_id = e.attempt_id AND i.position = (it->>'position')::int AND i.tenant_id = e.tenant_id
			AND i.deleted_at IS NULL
		WHERE e.tenant_id = infra.current_tenant_id() AND e.attempt_id = $1 AND e.effective AND e.source = 're_evaluation'
			AND e.deleted_at IS NULL AND (COALESCE((it->>'is_correct')::boolean, false) OR COALESCE((it->>'dropped')::boolean, false))`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("re-evaluated items: %w", err)
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
