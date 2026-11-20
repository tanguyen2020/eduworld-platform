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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
)

// ExperimentRepo implements write.ExperimentStore with explicit SQL (F035 US5): learning.experiments and
// experiment_results are G tables; assignments and arm mastery states are T tables filtered by the tenant given.
type ExperimentRepo struct{}

var _ write.ExperimentStore = ExperimentRepo{}

const experimentCols = `experiment_id, code, kind, unit, arms, to_jsonb(tenant_ids), study_id, status, start_at, end_at, stop_reason, stop_note,
	revision, owner_id, idempotency_key, started_at, stopped_at, version, created_at, updated_at`

func scanExperiment(row interface{ Scan(...any) error }) (write.ExperimentRow, error) {
	var e write.ExperimentRow
	var tenants []byte
	var study uuid.NullUUID
	var reason, note sql.NullString
	var started, stopped sql.NullTime
	err := row.Scan(&e.ID, &e.Code, &e.Kind, &e.Unit, &e.Arms, &tenants, &study, &e.Status, &e.StartAt, &e.EndAt, &reason, &note, &e.Revision,
		&e.OwnerID, &e.IdempotencyKey, &started, &stopped, &e.Version, &e.CreatedAt, &e.UpdatedAt)
	e.TenantIDs, e.StudyID, e.StopReason, e.StopNote = jsonUUIDs(tenants), nullUUID(study), nullString(reason), nullString(note)
	e.StartedAt, e.StoppedAt = nullTime(started), nullTime(stopped)
	e.StartAt, e.EndAt = e.StartAt.UTC(), e.EndAt.UTC()
	return e, err
}

func experiments(ctx context.Context, q string, args ...any) ([]write.ExperimentRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("experiments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ExperimentRow
	for rows.Next() {
		e, err := scanExperiment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func oneExperiment(ctx context.Context, q string, args ...any) (*write.ExperimentRow, error) {
	es, err := experiments(ctx, q, args...)
	if err != nil || len(es) == 0 {
		return nil, err
	}
	return &es[0], nil
}

// InsertExperiment implements write.ExperimentStore.
func (ExperimentRepo) InsertExperiment(ctx context.Context, e write.ExperimentRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.experiments (experiment_id, code, kind, unit, arms, tenant_ids, study_id, status, start_at,
		end_at, revision, owner_id, idempotency_key, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6::uuid[],$7,$8,$9,$10,$11,$12,$13,$14,$14)`,
		e.ID, e.Code, e.Kind, e.Unit, []byte(e.Arms), uuidArray(e.TenantIDs), e.StudyID, e.Status, e.StartAt, e.EndAt, e.Revision, e.OwnerID,
		e.IdempotencyKey, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert experiment: %w", err)
	}
	return nil
}

// Experiment implements write.ExperimentStore.
func (ExperimentRepo) Experiment(ctx context.Context, id uuid.UUID, lock bool) (*write.ExperimentRow, error) {
	q := `SELECT ` + experimentCols + ` FROM learning.experiments WHERE experiment_id = $1 AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	// tenancyguard:ok platform table learning.experiments (G, no tenant_id), deleted_at IS NULL
	return oneExperiment(ctx, q, id)
}

// ExperimentByKey implements write.ExperimentStore.
func (ExperimentRepo) ExperimentByKey(ctx context.Context, owner uuid.UUID, key string) (*write.ExperimentRow, error) {
	// tenancyguard:ok platform table learning.experiments (G, no tenant_id), deleted_at IS NULL
	return oneExperiment(ctx, `SELECT `+experimentCols+` FROM learning.experiments WHERE owner_id = $1 AND idempotency_key = $2
		AND deleted_at IS NULL`, owner, key)
}

// ExperimentCodeTaken implements write.ExperimentStore.
func (ExperimentRepo) ExperimentCodeTaken(ctx context.Context, code string) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var n int
	// tenancyguard:ok platform table learning.experiments (G, no tenant_id), deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM learning.experiments WHERE code = $1 AND deleted_at IS NULL`, code).Scan(&n)
	return n > 0, err
}

// UpdateExperiment implements write.ExperimentStore.
func (ExperimentRepo) UpdateExperiment(ctx context.Context, e write.ExperimentRow, prev int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform table learning.experiments (G, no tenant_id), deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE learning.experiments SET unit = $3, arms = $4, tenant_ids = $5::uuid[], study_id = $6, status = $7,
		start_at = $8, end_at = $9, stop_reason = $10, stop_note = $11, revision = $12, started_at = $13, stopped_at = $14, version = version + 1,
		updated_at = now() WHERE experiment_id = $1 AND version = $2 AND deleted_at IS NULL`,
		e.ID, prev, e.Unit, []byte(e.Arms), uuidArray(e.TenantIDs), e.StudyID, e.Status, e.StartAt, e.EndAt, e.StopReason, e.StopNote, e.Revision,
		e.StartedAt, e.StoppedAt))
}

// RunningExperiments implements write.ExperimentStore.
func (ExperimentRepo) RunningExperiments(ctx context.Context, tenant *uuid.UUID) ([]write.ExperimentRow, error) {
	// tenancyguard:ok platform table learning.experiments (G, no tenant_id); tenant_ids filter on $1; deleted_at IS NULL
	return experiments(ctx, `SELECT `+experimentCols+` FROM learning.experiments WHERE status IN ('shadow','live') AND deleted_at IS NULL
		AND ($1::uuid IS NULL OR $1::uuid = ANY(tenant_ids)) ORDER BY code`, tenant)
}

// DueExperiments implements write.ExperimentStore.
func (ExperimentRepo) DueExperiments(ctx context.Context, now time.Time) ([]write.ExperimentRow, error) {
	// tenancyguard:ok platform table learning.experiments (G, no tenant_id), deleted_at IS NULL
	return experiments(ctx, `SELECT `+experimentCols+` FROM learning.experiments WHERE status IN ('shadow','live') AND end_at <= $1
		AND deleted_at IS NULL ORDER BY end_at`, now)
}

// ModelVersions implements write.ExperimentStore.
func (ExperimentRepo) ModelVersions(ctx context.Context) (map[string]bool, string, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, "", err
	}
	// tenancyguard:ok G table learning.mastery_model_configs (platform configuration, no tenant_id), deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT model_version, status FROM learning.mastery_model_configs WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, "", fmt.Errorf("model versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out, active := map[string]bool{}, ""
	for rows.Next() {
		var v, st string
		if err := rows.Scan(&v, &st); err != nil {
			return nil, "", err
		}
		out[v] = true
		if st == "active" {
			active = v
		}
	}
	return out, active, rows.Err()
}

// ModelConfig implements write.ExperimentStore.
func (ExperimentRepo) ModelConfig(ctx context.Context, version string) (*write.ModelConfig, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var c write.ModelConfig
	// tenancyguard:ok G table learning.mastery_model_configs (platform configuration, no tenant_id), deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT config_id, model_version, algorithm, params, status FROM learning.mastery_model_configs
		WHERE model_version = $1 AND deleted_at IS NULL`, version).Scan(&c.ID, &c.ModelVersion, &c.Algorithm, &c.Params, &c.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("model config: %w", err)
	}
	return &c, nil
}

// Assignment implements write.ExperimentStore.
func (ExperimentRepo) Assignment(ctx context.Context, tenant, experimentID, studentID uuid.UUID) (*write.ExperimentAssignmentRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	a := write.ExperimentAssignmentRow{ExperimentID: experimentID, StudentID: studentID, TenantID: tenant}
	err = tx.QueryRowContext(ctx, `SELECT unit_id, bucket, arm_code, status, assigned_at FROM learning.experiment_assignments
		WHERE tenant_id = $1 AND experiment_id = $2 AND student_id = $3 AND deleted_at IS NULL`, tenant, experimentID, studentID).
		Scan(&a.UnitID, &a.Bucket, &a.ArmCode, &a.Status, &a.AssignedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("experiment assignment: %w", err)
	}
	return &a, nil
}

// InsertAssignment implements write.ExperimentStore: the first contact wins (a concurrent one keeps its row).
func (ExperimentRepo) InsertAssignment(ctx context.Context, a write.ExperimentAssignmentRow) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	return affected(tx.ExecContext(ctx, `INSERT INTO learning.experiment_assignments (experiment_id, student_id, tenant_id, unit_id, bucket, arm_code,
		status, assigned_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (experiment_id, student_id) DO NOTHING`,
		a.ExperimentID, a.StudentID, a.TenantID, a.UnitID, a.Bucket, a.ArmCode, a.Status, a.AssignedAt))
}

// RemoveAssignments implements write.ExperimentStore.
func (ExperimentRepo) RemoveAssignments(ctx context.Context, experimentID uuid.UUID, tenant *uuid.UUID) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	// tenancyguard:ok every tenant of the experiment under the audited bypass efficacy_study, or the tenant $2; deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.experiment_assignments SET status = 'removed', version = version + 1, updated_at = now()
		WHERE experiment_id = $1 AND ($2::uuid IS NULL OR tenant_id = $2) AND status = 'active' AND deleted_at IS NULL`, experimentID, tenant)
	if err != nil {
		return 0, fmt.Errorf("remove assignments: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// StudentClass implements write.ExperimentStore.
func (ExperimentRepo) StudentClass(ctx context.Context, tenant, studentID uuid.UUID) (*uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	// tenancyguard:ok projection ref.enrollments (no deleted_at, status active) of the tenant given
	err = tx.QueryRowContext(ctx, `SELECT class_id FROM ref.enrollments WHERE tenant_id = $1 AND student_id = $2 AND status = 'active'
		ORDER BY start_date DESC, class_id LIMIT 1`, tenant, studentID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("student class: %w", err)
	}
	return &id, nil
}

// InterventionClasses implements write.ExperimentStore.
func (ExperimentRepo) InterventionClasses(ctx context.Context, studyID uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of the study (platform role) or the session tenant (RLS); deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT class_id FROM learning.efficacy_study_classes WHERE study_id = $1 AND arm = 'intervention'
		AND status = 'enrolled' AND deleted_at IS NULL ORDER BY class_id`, studyID)
	if err != nil {
		return nil, fmt.Errorf("intervention classes: %w", err)
	}
	return scanIDs(rows)
}

// ArmStates implements write.ExperimentStore.
func (ExperimentRepo) ArmStates(ctx context.Context, tenant, experimentID uuid.UUID, arm string, studentID uuid.UUID) (map[uuid.UUID]mastery.State,
	error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT objective_id, value::float8, evidence_count, distinct_days, last_evidence_at,
		COALESCE(to_char(last_evidence_date, 'YYYY-MM-DD'), '') FROM learning.experiment_mastery_states WHERE tenant_id = $1 AND experiment_id = $2
		AND arm_code = $3 AND student_id = $4 AND deleted_at IS NULL`, tenant, experimentID, arm, studentID)
	if err != nil {
		return nil, fmt.Errorf("arm states: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]mastery.State{}
	for rows.Next() {
		var o uuid.UUID
		var s mastery.State
		if err := rows.Scan(&o, &s.Value, &s.EvidenceCount, &s.DistinctDays, &s.LastEvidenceAt, &s.LastEvidenceDate); err != nil {
			return nil, err
		}
		out[o] = s
	}
	return out, rows.Err()
}

// UpsertArmState implements write.ExperimentStore.
func (ExperimentRepo) UpsertArmState(ctx context.Context, tenant, experimentID uuid.UUID, arm string, studentID, objectiveID uuid.UUID,
	s mastery.State, level mastery.Level, modelVersion string) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var day any
	if s.LastEvidenceDate != "" {
		day = s.LastEvidenceDate
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.experiment_mastery_states (experiment_id, arm_code, tenant_id, student_id, objective_id, value,
		level, evidence_count, distinct_days, last_evidence_at, last_evidence_date, model_version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::date,$12)
		ON CONFLICT (experiment_id, arm_code, tenant_id, student_id, objective_id) DO UPDATE SET value = EXCLUDED.value, level = EXCLUDED.level,
			evidence_count = EXCLUDED.evidence_count, distinct_days = EXCLUDED.distinct_days, last_evidence_at = EXCLUDED.last_evidence_at,
			last_evidence_date = EXCLUDED.last_evidence_date, model_version = EXCLUDED.model_version,
			version = learning.experiment_mastery_states.version + 1, updated_at = now()`,
		experimentID, arm, tenant, studentID, objectiveID, s.Value, string(level), s.EvidenceCount, s.DistinctDays, s.LastEvidenceAt, day, modelVersion)
	if err != nil {
		return fmt.Errorf("upsert arm state: %w", err)
	}
	return nil
}

// ArmMetrics implements write.ExperimentStore: per arm the units and students assigned, the first scored, non-rapid
// answers tagged with the arm at least delay after the assignment (primary metric: share correct), the log-loss of the
// arm mastery against those answers and the practice minutes since the assignment.
func (ExperimentRepo) ArmMetrics(ctx context.Context, e write.ExperimentRow, delay time.Duration) ([]write.ArmMetric, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of the experiment under the audited bypass efficacy_study; tenant_id joins, deleted_at on T tables,
	// learning_events append-only
	rows, err := tx.QueryContext(ctx, `WITH a AS (
			SELECT tenant_id, student_id, unit_id, arm_code, assigned_at FROM learning.experiment_assignments
			WHERE experiment_id = $1 AND status = 'active' AND deleted_at IS NULL),
		ev AS (
			SELECT a.arm_code, a.student_id, COALESCE((le.payload->>'is_correct')::boolean, false) AS correct,
				(SELECT s.value::float8 FROM learning.experiment_mastery_states s WHERE s.tenant_id = a.tenant_id AND s.experiment_id = $1
					AND s.arm_code = a.arm_code AND s.student_id = a.student_id AND s.objective_id = ANY(le.objective_ids) AND s.deleted_at IS NULL
					LIMIT 1) AS p
			FROM a JOIN learning.learning_events le ON le.tenant_id = a.tenant_id AND le.student_id = a.student_id
			WHERE le.event_type = 'question.answered' AND le.efficacy_wave_id IS NULL
				AND le.experiment_arms @> ARRAY[$2 || ':' || a.arm_code]::varchar(97)[]
				AND le.occurred_at >= a.assigned_at + $3::interval AND COALESCE((le.payload->>'attempt_no')::int, 1) = 1
				AND COALESCE((le.payload->>'is_scored')::boolean, true) AND NOT COALESCE((le.payload->>'rapid_guess')::boolean, false)),
		m AS (
			SELECT a.arm_code, COALESCE(sum(t.duration_ms), 0)::float8 / 60000 AS minutes, count(DISTINCT t.student_id) AS active
			FROM a LEFT JOIN assessment.attempts t ON t.tenant_id = a.tenant_id AND t.student_id = a.student_id AND t.mode <> 'exam'
				AND t.started_at >= a.assigned_at AND t.deleted_at IS NULL
			GROUP BY a.arm_code)
		SELECT a.arm_code, count(DISTINCT a.unit_id), count(DISTINCT a.student_id),
			(SELECT count(*) FROM ev WHERE ev.arm_code = a.arm_code), (SELECT count(*) FILTER (WHERE correct) FROM ev WHERE ev.arm_code = a.arm_code),
			(SELECT avg(-(CASE WHEN correct THEN ln(GREATEST(LEAST(p, 0.999), 0.001)) ELSE ln(GREATEST(LEAST(1 - p, 0.999), 0.001)) END))
				FROM ev WHERE ev.arm_code = a.arm_code AND p IS NOT NULL),
			COALESCE((SELECT minutes FROM m WHERE m.arm_code = a.arm_code), 0), COALESCE((SELECT active FROM m WHERE m.arm_code = a.arm_code), 0)
		FROM a GROUP BY a.arm_code ORDER BY a.arm_code`, e.ID, e.Code, fmt.Sprintf("%d seconds", int64(delay.Seconds())))
	if err != nil {
		return nil, fmt.Errorf("arm metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ArmMetric
	for rows.Next() {
		var m write.ArmMetric
		var ll sql.NullFloat64
		if err := rows.Scan(&m.ArmCode, &m.Units, &m.Students, &m.Answers, &m.Correct, &ll, &m.ActiveMinutes, &m.ActiveStudents); err != nil {
			return nil, err
		}
		if ll.Valid {
			v := ll.Float64
			m.LogLoss = &v
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertResult implements write.ExperimentStore.
func (ExperimentRepo) InsertResult(ctx context.Context, experimentID uuid.UUID, at time.Time, results json.RawMessage, breached bool) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.experiment_results (result_id, experiment_id, computed_at, results, guardrail_breached)
		VALUES ($1,$2,$3,$4,$5)`, uuid.Must(uuid.NewV7()), experimentID, at, []byte(results), breached)
	if err != nil {
		return fmt.Errorf("insert experiment result: %w", err)
	}
	return nil
}

// LatestResult implements write.ExperimentStore.
func (ExperimentRepo) LatestResult(ctx context.Context, experimentID uuid.UUID) (json.RawMessage, *time.Time, bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	var r []byte
	var at time.Time
	var breached bool
	// tenancyguard:ok append-only platform table learning.experiment_results (G, no tenant_id, no deleted_at)
	err = tx.QueryRowContext(ctx, `SELECT results, computed_at, guardrail_breached FROM learning.experiment_results WHERE experiment_id = $1
		ORDER BY computed_at DESC, result_id DESC LIMIT 1`, experimentID).Scan(&r, &at, &breached)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("experiment result: %w", err)
	}
	t := at.UTC()
	return r, &t, breached, nil
}
