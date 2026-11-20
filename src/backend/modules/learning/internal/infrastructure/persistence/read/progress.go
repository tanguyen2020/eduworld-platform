package read

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// ProgressModel implements read.ProgressReadModel with explicit SQL in the read transaction of ctx (TenantScope: RLS T).
type ProgressModel struct{}

var _ read.ProgressReadModel = ProgressModel{}

// ActiveModel implements read.ProgressReadModel.
func (ProgressModel) ActiveModel(ctx context.Context) (read.MasteryModelConfig, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return read.MasteryModelConfig{}, err
	}
	var c read.MasteryModelConfig
	var params []byte
	// tenancyguard:ok G table learning.mastery_model_configs (platform configuration, no tenant_id), deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT model_version, params FROM learning.mastery_model_configs
		WHERE status = 'active' AND deleted_at IS NULL`).Scan(&c.ModelVersion, &params)
	if errors.Is(err, sql.ErrNoRows) {
		return c, errors.New("mastery: no active model version")
	}
	if err != nil {
		return c, fmt.Errorf("active mastery model: %w", err)
	}
	c.Params = params
	return c, nil
}

// Masteries implements read.ProgressReadModel.
func (ProgressModel) Masteries(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID) ([]read.ObjectiveMastery, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// The subject of an objective is the subject of its topic (curriculum.topics, H: visible to the tenant through RLS).
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on every table
	rows, err := q.QueryContext(ctx, `SELECT m.objective_id, m.value::float8, m.evidence_count, m.distinct_days, m.last_evidence_at,
			COALESCE(to_char(m.last_evidence_date, 'YYYY-MM-DD'), ''), m.first_mastered_at, m.prior_source, m.prior_value::float8
		FROM learning.objective_masteries m
		WHERE m.tenant_id = infra.current_tenant_id() AND m.student_id = $1 AND m.deleted_at IS NULL
			AND ($2::uuid IS NULL OR EXISTS (SELECT 1 FROM curriculum.learning_objectives lo
				JOIN curriculum.topics t ON t.topic_id = lo.topic_id AND t.deleted_at IS NULL
				WHERE lo.objective_id = m.objective_id AND lo.deleted_at IS NULL AND t.subject_id = $2))
		ORDER BY m.objective_id`, studentID, subjectID)
	if err != nil {
		return nil, fmt.Errorf("masteries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ObjectiveMastery
	for rows.Next() {
		var m read.ObjectiveMastery
		s := &m.State
		if err := rows.Scan(&m.ObjectiveID, &s.Value, &s.EvidenceCount, &s.DistinctDays, &s.LastEvidenceAt, &s.LastEvidenceDate,
			&s.FirstMasteredAt, &s.PriorSource, &s.PriorValue); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Totals implements read.ProgressReadModel.
func (ProgressModel) Totals(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID) (read.ProgressTotals, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return read.ProgressTotals{}, err
	}
	var t read.ProgressTotals
	// tenancyguard:ok derived progress_daily has no deleted_at (DATABASE_DESIGN §2.7); tenant of the session (RLS + filter)
	err = q.QueryRowContext(ctx, `SELECT COALESCE(sum(lessons_completed), 0), COALESCE(sum(items_answered), 0),
			COALESCE(sum(items_correct), 0), COALESCE(sum(active_seconds), 0)
		FROM learning.progress_daily
		WHERE tenant_id = infra.current_tenant_id() AND student_id = $1 AND metric_version = 1 AND ($2::uuid IS NULL OR subject_id = $2)`,
		studentID, subjectID).Scan(&t.LessonsCompleted, &t.ItemsAnswered, &t.ItemsCorrect, &t.ActiveSeconds)
	if err != nil {
		return t, fmt.Errorf("progress totals: %w", err)
	}
	return t, nil
}

// Student implements read.ProgressReadModel.
func (ProgressModel) Student(ctx context.Context, studentID uuid.UUID) (*read.StudentRef, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	s := read.StudentRef{StudentID: studentID}
	var grade sql.NullInt32
	// tenancyguard:ok projection ref.students of the session tenant (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT school_id, grade_level FROM ref.students
		WHERE student_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, studentID).Scan(&s.SchoolID, &grade)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("student: %w", err)
	}
	s.GradeLevel = int(grade.Int32)
	return &s, nil
}

// sqlTx is the read transaction of ctx (entx.ReadSession).
func sqlTx(ctx context.Context) (*sql.Tx, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return tx, nil
}
