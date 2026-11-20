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
)

// RecomputeRepo implements write.RecomputeStore with explicit SQL in the transaction of ctx.
type RecomputeRepo struct{}

var _ write.RecomputeStore = RecomputeRepo{}

// ModelByVersion implements write.RecomputeStore.
func (RecomputeRepo) ModelByVersion(ctx context.Context, version string) (*write.ModelConfig, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var c write.ModelConfig
	var params []byte
	// tenancyguard:ok G table learning.mastery_model_configs (platform configuration, no tenant_id), deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT config_id, model_version, algorithm, params, status FROM learning.mastery_model_configs
		WHERE model_version = $1 AND deleted_at IS NULL`, version).Scan(&c.ID, &c.ModelVersion, &c.Algorithm, &params, &c.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("model %s: %w", version, err)
	}
	c.Params = params
	return &c, nil
}

// InsertModel implements write.RecomputeStore.
func (RecomputeRepo) InsertModel(ctx context.Context, c write.ModelConfig) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok G table learning.mastery_model_configs (platform configuration); conflict on the live version unique
	res, err := tx.ExecContext(ctx, `INSERT INTO learning.mastery_model_configs (config_id, model_version, algorithm, params, status)
		VALUES ($1, $2, $3, $4::jsonb, 'draft') ON CONFLICT (model_version) WHERE deleted_at IS NULL DO NOTHING`,
		c.ID, c.ModelVersion, c.Algorithm, []byte(c.Params))
	if err != nil {
		return false, fmt.Errorf("insert model: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// SetModelStatus implements write.RecomputeStore.
func (RecomputeRepo) SetModelStatus(ctx context.Context, version, status string) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok G table learning.mastery_model_configs; only a draft or shadow version moves, deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.mastery_model_configs SET status = $2, updated_at = now()
		WHERE model_version = $1 AND status IN ('draft','shadow') AND deleted_at IS NULL`, version, status)
	if err != nil {
		return fmt.Errorf("model status: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("model %s is not draft or shadow", version)
	}
	return nil
}

// ActivateModel implements write.RecomputeStore.
func (RecomputeRepo) ActivateModel(ctx context.Context, version string, by uuid.UUID, at time.Time) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok G table learning.mastery_model_configs: retire the active version, deleted_at IS NULL
	if _, err := tx.ExecContext(ctx, `UPDATE learning.mastery_model_configs SET status = 'retired', updated_at = $1
		WHERE status = 'active' AND model_version <> $2 AND deleted_at IS NULL`, at.UTC(), version); err != nil {
		return fmt.Errorf("retire model: %w", err)
	}
	// tenancyguard:ok G table learning.mastery_model_configs: activate the shadow version, deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.mastery_model_configs SET status = 'active', activated_at = $2, activated_by = $3,
		updated_at = $2 WHERE model_version = $1 AND status = 'shadow' AND deleted_at IS NULL`, version, at.UTC(), by)
	if err != nil {
		return fmt.Errorf("activate model: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("model %s is not in shadow", version)
	}
	return nil
}

// InsertRun implements write.RecomputeStore.
func (RecomputeRepo) InsertRun(ctx context.Context, r write.RecomputeRun) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok G table learning.mastery_recompute_runs (platform operations, no tenant_id)
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.mastery_recompute_runs (run_id, model_version, status, started_by, started_at)
		VALUES ($1, $2, $3, $4, $5)`, r.ID, r.ModelVersion, r.Status, r.StartedBy, r.StartedAt.UTC())
	if err != nil {
		return fmt.Errorf("insert recompute run: %w", err)
	}
	return nil
}

// FinishRun implements write.RecomputeStore.
func (RecomputeRepo) FinishRun(ctx context.Context, id uuid.UUID, status string, report json.RawMessage, at time.Time) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var rep any
	if len(report) > 0 {
		rep = []byte(report)
	}
	// tenancyguard:ok G table learning.mastery_recompute_runs, one run by id, deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE learning.mastery_recompute_runs SET status = $2, report = $3::jsonb, finished_at = $4, updated_at = $4
		WHERE run_id = $1 AND deleted_at IS NULL`, id, status, rep, at.UTC())
	if err != nil {
		return fmt.Errorf("finish recompute run: %w", err)
	}
	return nil
}

// Run implements write.RecomputeStore.
func (RecomputeRepo) Run(ctx context.Context, id uuid.UUID) (*write.RecomputeRun, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var r write.RecomputeRun
	var by uuid.NullUUID
	var finished sql.NullTime
	var report []byte
	// tenancyguard:ok G table learning.mastery_recompute_runs, one run by id, deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT run_id, model_version, status, report, started_by, started_at, finished_at
		FROM learning.mastery_recompute_runs WHERE run_id = $1 AND deleted_at IS NULL`, id).
		Scan(&r.ID, &r.ModelVersion, &r.Status, &report, &by, &r.StartedAt, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recompute run: %w", err)
	}
	r.Report, r.StartedBy, r.FinishedAt = report, nullUUID(by), nullTime(finished)
	return &r, nil
}

// MarkRunActivated implements write.RecomputeStore.
func (RecomputeRepo) MarkRunActivated(ctx context.Context, id, by uuid.UUID, at time.Time) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok G table learning.mastery_recompute_runs, one completed run by id, deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.mastery_recompute_runs SET status = 'activated', activated_at = $2, activated_by = $3,
		updated_at = $2 WHERE run_id = $1 AND status = 'completed' AND deleted_at IS NULL`, id, at.UTC(), by)
	if err != nil {
		return fmt.Errorf("activate run: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("recompute run %s is not completed", id)
	}
	return nil
}

func scanPairs(rows *sql.Rows) ([]write.MasteryPair, error) {
	defer func() { _ = rows.Close() }()
	var out []write.MasteryPair
	for rows.Next() {
		var p write.MasteryPair
		if err := rows.Scan(&p.TenantID, &p.SchoolID, &p.StudentID, &p.ObjectiveID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Pairs implements write.RecomputeStore (ctx: tenancy.Bypass(platform_job)).
func (RecomputeRepo) Pairs(ctx context.Context) ([]write.MasteryPair, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok recompute under tenancy.Bypass(platform_job): pairs of every tenant, ids only, deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id, school_id, student_id, objective_id FROM learning.objective_masteries
		WHERE deleted_at IS NULL ORDER BY tenant_id, student_id, objective_id`)
	if err != nil {
		return nil, fmt.Errorf("mastery pairs: %w", err)
	}
	return scanPairs(rows)
}

// ShadowPairs implements write.RecomputeStore (ctx: tenancy.Bypass(platform_job)).
func (RecomputeRepo) ShadowPairs(ctx context.Context, runID uuid.UUID) ([]write.MasteryPair, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok activation under tenancy.Bypass(platform_job): shadow pairs of one run in every tenant, deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id, school_id, student_id, objective_id FROM learning.objective_mastery_shadows
		WHERE run_id = $1 AND deleted_at IS NULL ORDER BY tenant_id, student_id, objective_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("shadow pairs: %w", err)
	}
	return scanPairs(rows)
}

// InsertShadow implements write.RecomputeStore (TenantScope of the pair).
func (RecomputeRepo) InsertShadow(ctx context.Context, r write.ShadowRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	s := r.State
	// tenancyguard:ok insert of the session tenant (RLS insert policy) into the shadow table of a run
	_, err = tx.ExecContext(ctx, `INSERT INTO learning.objective_mastery_shadows (run_id, tenant_id, school_id, student_id, objective_id,
		value, level, current_level, evidence_count, distinct_days, last_evidence_at, first_mastered_at, model_version)
		VALUES ($1, $2, $3, $4, $5, round($6::numeric, 3), $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (run_id, tenant_id, student_id, objective_id) DO NOTHING`,
		r.RunID, r.Pair.TenantID, r.Pair.SchoolID, r.Pair.StudentID, r.Pair.ObjectiveID, s.Value, string(r.Level), string(r.CurrentLevel),
		s.EvidenceCount, s.DistinctDays, s.LastEvidenceAt, s.FirstMasteredAt, r.ModelVersion)
	if err != nil {
		return fmt.Errorf("insert shadow: %w", err)
	}
	return nil
}
