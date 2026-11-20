package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SimulationRunRepo implements write.SimulationRunStore with explicit SQL in the transaction of ctx (TenantScope,
// infra.current_tenant_id(); RLS T) on assessment.simulation_runs (F066).
type SimulationRunRepo struct{}

var _ write.SimulationRunStore = SimulationRunRepo{}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// Runs implements write.SimulationRunStore.
func (SimulationRunRepo) Runs(ctx context.Context, attemptID uuid.UUID) ([]write.SimulationRun, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT run_id, tenant_id, school_id, attempt_id, student_id, run_no, template_code, template_version, seed,
		action_log, client_outcome, verified_outcome, goals_met, verification_status, coalesce(reject_code, ''), idempotency_key, created_at
		FROM assessment.simulation_runs WHERE tenant_id = infra.current_tenant_id() AND attempt_id = $1 AND deleted_at IS NULL
		ORDER BY run_no`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("simulation runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []write.SimulationRun{}
	for rows.Next() {
		var r write.SimulationRun
		var client, verified []byte
		if err := rows.Scan(&r.ID, &r.TenantID, &r.SchoolID, &r.AttemptID, &r.StudentID, &r.RunNo, &r.TemplateCode, &r.TemplateVersion, &r.Seed,
			&r.ActionLog, &client, &verified, &r.GoalsMet, &r.VerificationStatus, &r.RejectCode, &r.IdempotencyKey, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("simulation runs: %w", err)
		}
		r.ClientOutcome, r.VerifiedOutcome = client, verified
		r.CreatedAt = r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// InsertRun implements write.SimulationRunStore.
func (SimulationRunRepo) InsertRun(ctx context.Context, r write.SimulationRun) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	var reject sql.NullString
	if r.RejectCode != "" {
		reject = sql.NullString{String: r.RejectCode, Valid: true}
	}
	// tenancyguard:ok the row carries the tenant of the session (RLS WITH CHECK infra.current_tenant_id())
	_, err = tx.ExecContext(ctx, `INSERT INTO assessment.simulation_runs (run_id, tenant_id, school_id, attempt_id, student_id, run_no,
		template_code, template_version, seed, action_log, client_outcome, verified_outcome, goals_met, verification_status, reject_code,
		idempotency_key, created_at) VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		r.ID, r.SchoolID, r.AttemptID, r.StudentID, r.RunNo, r.TemplateCode, r.TemplateVersion, r.Seed, []byte(r.ActionLog),
		nullJSON(r.ClientOutcome), nullJSON(r.VerifiedOutcome), []byte(r.GoalsMet), r.VerificationStatus, reject, r.IdempotencyKey, r.CreatedAt)
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return problem.New(problem.Conflict, "this run is already recorded").Wrap(err)
	}
	return fmt.Errorf("simulation runs: %w", err)
}
