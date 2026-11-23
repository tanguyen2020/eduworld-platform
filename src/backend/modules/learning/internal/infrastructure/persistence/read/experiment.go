package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
)

// ExperimentModel implements read.ExperimentReadModel and read.ArmReader with explicit SQL (F035 US5).
type ExperimentModel struct{}

var (
	_ read.ExperimentReadModel = ExperimentModel{}
	_ read.ArmReader           = ExperimentModel{}
)

const experimentReadCols = `experiment_id, code, kind, unit, arms, to_jsonb(tenant_ids), study_id, status, start_at, end_at, stop_reason, revision,
	owner_id, version, created_at, updated_at`

func experimentRows(ctx context.Context, q string, args ...any) ([]write.ExperimentRow, error) {
	tx, err := sqlTx(ctx)
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
		var e write.ExperimentRow
		var tenants []byte
		var study uuid.NullUUID
		var reason sql.NullString
		if err := rows.Scan(&e.ID, &e.Code, &e.Kind, &e.Unit, &e.Arms, &tenants, &study, &e.Status, &e.StartAt, &e.EndAt, &reason, &e.Revision,
			&e.OwnerID, &e.Version, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.TenantIDs, e.StudyID, e.StopReason = jsonIDs(tenants), nullUUID(study), optStr(reason)
		e.StartAt, e.EndAt = e.StartAt.UTC(), e.EndAt.UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

// Experiments implements read.ExperimentReadModel.
func (ExperimentModel) Experiments(ctx context.Context, status, kind string, seek read.Seek) ([]write.ExperimentRow, error) {
	// tenancyguard:ok platform table learning.experiments (G, no tenant_id), deleted_at IS NULL; one keyset page (created_at DESC, id DESC)
	return experimentRows(ctx, `SELECT `+experimentReadCols+` FROM learning.experiments WHERE deleted_at IS NULL AND ($1 = '' OR status = $1)
		AND ($2 = '' OR kind = $2) AND ($3::timestamptz IS NULL OR (created_at, experiment_id) < ($3::timestamptz, $4::uuid))
		ORDER BY created_at DESC, experiment_id DESC LIMIT $5`, append([]any{status, kind}, seekArgs(seek, 1)...)...)
}

// Experiment implements read.ExperimentReadModel.
func (ExperimentModel) Experiment(ctx context.Context, id uuid.UUID) (*write.ExperimentRow, error) {
	// tenancyguard:ok platform table learning.experiments (G, no tenant_id), deleted_at IS NULL
	es, err := experimentRows(ctx, `SELECT `+experimentReadCols+` FROM learning.experiments WHERE experiment_id = $1 AND deleted_at IS NULL`, id)
	if err != nil || len(es) == 0 {
		return nil, err
	}
	return &es[0], nil
}

// LatestResult implements read.ExperimentReadModel.
func (ExperimentModel) LatestResult(ctx context.Context, id uuid.UUID) (json.RawMessage, *time.Time, bool, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	var r []byte
	var at time.Time
	var breached bool
	// tenancyguard:ok append-only platform table learning.experiment_results (G, no tenant_id, no deleted_at)
	err = tx.QueryRowContext(ctx, `SELECT results, computed_at, guardrail_breached FROM learning.experiment_results WHERE experiment_id = $1
		ORDER BY computed_at DESC, result_id DESC LIMIT 1`, id).Scan(&r, &at, &breached)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("experiment result: %w", err)
	}
	t := at.UTC()
	return r, &t, breached, nil
}

// LiveArm implements read.ArmReader.
func (ExperimentModel) LiveArm(ctx context.Context, tenant, student uuid.UUID) (*read.LiveArm, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	var code, arm string
	var arms []byte
	// tenancyguard:ok experiments (G) joined with the assignments of the tenant given (tenant_id = $1), deleted_at IS NULL on both
	err = tx.QueryRowContext(ctx, `SELECT e.experiment_id, e.code, e.arms, a.arm_code FROM learning.experiments e
		JOIN learning.experiment_assignments a ON a.experiment_id = e.experiment_id AND a.tenant_id = $1 AND a.student_id = $2
			AND a.status = 'active' AND a.deleted_at IS NULL
		WHERE e.status = 'live' AND e.kind = 'mastery_model' AND e.deleted_at IS NULL ORDER BY e.started_at, e.experiment_id LIMIT 1`,
		tenant, student).Scan(&id, &code, &arms, &arm)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("live arm: %w", err)
	}
	var defs []struct {
		Code         string `json:"code"`
		IsControl    bool   `json:"is_control"`
		ModelVersion string `json:"model_version"`
	}
	_ = json.Unmarshal(arms, &defs)
	out := &read.LiveArm{ExperimentCode: code, ArmCode: arm, States: map[uuid.UUID]mastery.State{}}
	for _, d := range defs {
		if d.Code == arm {
			if d.IsControl {
				return nil, nil // the control arm sees the active model
			}
			out.ModelVersion = d.ModelVersion
		}
	}
	// tenancyguard:ok G table learning.mastery_model_configs (platform configuration, no tenant_id), deleted_at IS NULL
	if err := tx.QueryRowContext(ctx, `SELECT params FROM learning.mastery_model_configs WHERE model_version = $1 AND deleted_at IS NULL`,
		out.ModelVersion).Scan(&out.Params); err != nil {
		return nil, fmt.Errorf("arm model: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT objective_id, value::float8, evidence_count, distinct_days, last_evidence_at FROM
		learning.experiment_mastery_states WHERE tenant_id = $1 AND experiment_id = $2 AND arm_code = $3 AND student_id = $4 AND deleted_at IS NULL`,
		tenant, id, arm, student)
	if err != nil {
		return nil, fmt.Errorf("arm states: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var o uuid.UUID
		var s mastery.State
		if err := rows.Scan(&o, &s.Value, &s.EvidenceCount, &s.DistinctDays, &s.LastEvidenceAt); err != nil {
			return nil, err
		}
		out.States[o] = s
	}
	return out, rows.Err()
}
