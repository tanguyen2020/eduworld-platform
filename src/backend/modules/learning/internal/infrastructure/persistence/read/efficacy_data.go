package read

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

var _ read.EfficacyDataReader = EfficacyModel{}

// StudyOf implements read.EfficacyDataReader.
func (m EfficacyModel) StudyOf(ctx context.Context, id uuid.UUID) (*write.StudyRow, error) {
	return m.Study(ctx, id)
}

// ClassesOf implements read.EfficacyDataReader.
func (m EfficacyModel) ClassesOf(ctx context.Context, studyID uuid.UUID) ([]read.StudyClassRead, error) {
	return m.StudyClasses(ctx, studyID, nil)
}

// WavesOf implements read.EfficacyDataReader.
func (m EfficacyModel) WavesOf(ctx context.Context, studyID uuid.UUID) ([]write.WaveRow, error) {
	return m.Waves(ctx, studyID)
}

// ParticipantsOf implements read.EfficacyDataReader.
func (EfficacyModel) ParticipantsOf(ctx context.Context, studyID uuid.UUID) ([]write.ParticipantRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of the study under the audited bypass efficacy_study; deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT study_id, student_id, tenant_id, school_id, class_id, arm, form_order, status, exclusion_reason,
		enrolled_at, withdrawn_at FROM learning.efficacy_participants WHERE study_id = $1 AND deleted_at IS NULL ORDER BY student_id`, studyID)
	if err != nil {
		return nil, fmt.Errorf("participants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ParticipantRow
	for rows.Next() {
		var p write.ParticipantRow
		var reason sql.NullString
		var withdrawn sql.NullTime
		if err := rows.Scan(&p.StudyID, &p.StudentID, &p.TenantID, &p.SchoolID, &p.ClassID, &p.Arm, &p.FormOrder, &p.Status, &reason, &p.EnrolledAt,
			&withdrawn); err != nil {
			return nil, err
		}
		p.ExclusionReason = optStr(reason)
		if withdrawn.Valid {
			t := withdrawn.Time.UTC()
			p.WithdrawnAt = &t
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Answers implements read.EfficacyDataReader.
func (EfficacyModel) Answers(ctx context.Context, waveID uuid.UUID) ([]read.EfficacyAnswerRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of the wave under the audited bypass efficacy_study; tenant_id joins and deleted_at IS NULL on each table
	rows, err := tx.QueryContext(ctx, `WITH s AS (
			SELECT ws.exam_session_id, ws.tenant_id, ws.class_id, ws.form, w.kind FROM learning.efficacy_wave_sessions ws
			JOIN learning.efficacy_waves w ON w.wave_id = ws.wave_id AND w.deleted_at IS NULL
			WHERE ws.wave_id = $1 AND ws.status = 'active' AND ws.deleted_at IS NULL),
		a AS (
			SELECT DISTINCT ON (at.student_id, at.context_id) at.attempt_id, at.student_id, at.tenant_id, s.class_id, s.form, s.kind
			FROM assessment.attempts at JOIN s ON at.context_id = s.exam_session_id AND at.tenant_id = s.tenant_id
			WHERE at.mode = 'exam' AND at.status = 'graded' AND at.deleted_at IS NULL
			ORDER BY at.student_id, at.context_id, at.graded_at DESC, at.attempt_id)
		SELECT a.student_id, a.tenant_id, a.class_id, a.kind, a.form, i.question_version_id, i.objective_ids, COALESCE(i.final_is_correct, false),
			COALESCE(i.final_score, 0)::float8, i.max_score::float8
		FROM a JOIN assessment.attempt_items i ON i.attempt_id = a.attempt_id AND i.tenant_id = a.tenant_id AND i.deleted_at IS NULL
		ORDER BY a.student_id, i.position`, waveID)
	if err != nil {
		return nil, fmt.Errorf("efficacy answers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.EfficacyAnswerRow
	for rows.Next() {
		var r read.EfficacyAnswerRow
		var los []byte
		if err := rows.Scan(&r.StudentID, &r.TenantID, &r.ClassID, &r.WaveKind, &r.Form, &r.QuestionVersionID, &los, &r.Correct, &r.Score,
			&r.Max); err != nil {
			return nil, err
		}
		r.ObjectiveIDs = jsonIDs(los)
		out = append(out, r)
	}
	return out, rows.Err()
}

// SnapshotsOf implements read.EfficacyDataReader.
func (EfficacyModel) SnapshotsOf(ctx context.Context, studyID uuid.UUID) ([]write.SnapshotRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of the study under the audited bypass efficacy_study; deleted_at IS NULL (withdrawn ones are soft-deleted)
	rows, err := tx.QueryContext(ctx, `SELECT study_id, student_id, objective_id, tenant_id, m_eff::float8, level, model_version, evidence_count,
		taken_at FROM learning.efficacy_mastery_snapshots WHERE study_id = $1 AND deleted_at IS NULL ORDER BY student_id, objective_id`, studyID)
	if err != nil {
		return nil, fmt.Errorf("snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.SnapshotRow
	for rows.Next() {
		var s write.SnapshotRow
		if err := rows.Scan(&s.StudyID, &s.StudentID, &s.ObjectiveID, &s.TenantID, &s.MEff, &s.Level, &s.ModelVersion, &s.EvidenceCount,
			&s.TakenAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Usage implements read.EfficacyDataReader: graded practice attempts (no exam) and their question answers.
func (EfficacyModel) Usage(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, from, to time.Time) ([]read.EfficacyUsageRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT s.id,
			COALESCE((SELECT sum(a.duration_ms) FROM assessment.attempts a WHERE a.tenant_id = $1 AND a.student_id = s.id AND a.mode <> 'exam'
				AND a.started_at >= $3 AND a.started_at < $4 AND a.deleted_at IS NULL), 0)::float8 / 1000,
			(SELECT count(DISTINCT (a.started_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) FROM assessment.attempts a WHERE a.tenant_id = $1
				AND a.student_id = s.id AND a.mode <> 'exam' AND a.started_at >= $3 AND a.started_at < $4 AND a.deleted_at IS NULL),
			(SELECT count(*) FROM assessment.attempts a WHERE a.tenant_id = $1 AND a.student_id = s.id AND a.mode <> 'exam'
				AND a.started_at >= $3 AND a.started_at < $4 AND a.deleted_at IS NULL),
			(SELECT count(*) FROM learning.learning_events le WHERE le.tenant_id = $1 AND le.student_id = s.id AND le.event_type = 'question.answered'
				AND le.efficacy_wave_id IS NULL AND le.context <> 'exam' AND le.occurred_at >= $3 AND le.occurred_at < $4),
			(SELECT count(*) FROM learning.learning_events le WHERE le.tenant_id = $1 AND le.student_id = s.id AND le.event_type = 'question.answered'
				AND le.efficacy_wave_id IS NULL AND le.context <> 'exam' AND le.occurred_at >= $3 AND le.occurred_at < $4
				AND COALESCE((le.payload->>'rapid_guess')::boolean, false))
		FROM unnest($2::uuid[]) AS s(id) ORDER BY s.id`, tenant, uuidArray(students), from, to)
	if err != nil {
		return nil, fmt.Errorf("efficacy usage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.EfficacyUsageRow
	for rows.Next() {
		var u read.EfficacyUsageRow
		if err := rows.Scan(&u.StudentID, &u.ActiveSeconds, &u.Days, &u.PracticeAttempts, &u.Answers, &u.RapidGuesses); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Recommendations implements read.EfficacyDataReader.
func (EfficacyModel) Recommendations(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, from, to time.Time) ([]read.EfficacyRecommendationRow, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.student_id, to_jsonb(r.objective_ids), r.started_attempt_id IS NOT NULL
		FROM learning.rm_student_recommendations r WHERE r.tenant_id = $1 AND r.student_id = ANY($2::uuid[]) AND r.generated_at >= $3
		AND r.generated_at < $4 AND r.deleted_at IS NULL ORDER BY r.student_id, r.recommendation_id`, tenant, uuidArray(students), from, to)
	if err != nil {
		return nil, fmt.Errorf("efficacy recommendations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.EfficacyRecommendationRow
	for rows.Next() {
		var r read.EfficacyRecommendationRow
		var los []byte
		if err := rows.Scan(&r.StudentID, &los, &r.Followed); err != nil {
			return nil, err
		}
		r.ObjectiveIDs = jsonIDs(los)
		out = append(out, r)
	}
	return out, rows.Err()
}
