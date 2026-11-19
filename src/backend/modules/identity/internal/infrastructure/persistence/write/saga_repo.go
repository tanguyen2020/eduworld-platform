package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SagaRepo implements write.SagaRepository with explicit SQL on the global tables platform.saga_runs and
// platform.saga_steps (platform scope "saga", identity_platform; no row is ever removed).
type SagaRepo struct{}

var _ write.SagaRepository = SagaRepo{}

func nullJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return string(raw)
}

// Create implements write.SagaRepository.
func (r SagaRepo) Create(ctx context.Context, run write.SagaRun) (write.SagaRun, bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return run, false, err
	}
	// tenancyguard:ok platform.saga_runs is global (G); the saga subject names its tenant
	res, err := tx.ExecContext(ctx, `INSERT INTO platform.saga_runs (saga_id, saga_type, tenant_id, subject, status, requested_by,
			idempotency_key, started_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $8) ON CONFLICT (saga_type, idempotency_key) DO NOTHING`,
		run.ID, run.Type, run.TenantID, string(run.Subject), run.Status, run.RequestedBy, run.IdempotencyKey, run.StartedAt)
	if err != nil {
		return run, false, fmt.Errorf("create saga: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var id uuid.UUID
		// tenancyguard:ok platform.saga_runs is global (G)
		if err := tx.QueryRowContext(ctx, `SELECT saga_id FROM platform.saga_runs WHERE saga_type = $1 AND idempotency_key = $2`,
			run.Type, run.IdempotencyKey).Scan(&id); err != nil {
			return run, false, fmt.Errorf("existing saga: %w", err)
		}
		existing, err := r.Lock(ctx, id)
		if err != nil || existing == nil {
			return run, false, err
		}
		return *existing, false, nil
	}
	for _, s := range run.Steps {
		// tenancyguard:ok platform.saga_steps is global (G), keyed by its saga
		if _, err := tx.ExecContext(ctx, `INSERT INTO platform.saga_steps (saga_id, step_no, service, action, status)
			VALUES ($1, $2, $3, $4, $5)`, run.ID, s.No, s.Service, s.Action, s.Status); err != nil {
			return run, false, fmt.Errorf("create saga step: %w", err)
		}
	}
	run.Version = 1
	return run, true, nil
}

// Lock implements write.SagaRepository.
func (SagaRepo) Lock(ctx context.Context, id uuid.UUID) (*write.SagaRun, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	var (
		run          write.SagaRun
		tenant, by   uuid.NullUUID
		next, finish sql.NullTime
		lastErr      sql.NullString
	)
	// tenancyguard:ok platform.saga_runs is global (G): one run by id
	err = tx.QueryRowContext(ctx, `SELECT saga_id, saga_type, tenant_id, subject, status, requested_by, idempotency_key, attempts,
			next_attempt_at, last_error, started_at, finished_at, version
		FROM platform.saga_runs WHERE saga_id = $1 FOR UPDATE`, id).
		Scan(&run.ID, &run.Type, &tenant, &run.Subject, &run.Status, &by, &run.IdempotencyKey, &run.Attempts, &next, &lastErr,
			&run.StartedAt, &finish, &run.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock saga: %w", err)
	}
	if tenant.Valid {
		t := tenant.UUID
		run.TenantID = &t
	}
	if by.Valid {
		b := by.UUID
		run.RequestedBy = &b
	}
	if next.Valid {
		run.NextAttemptAt = &next.Time
	}
	if finish.Valid {
		run.FinishedAt = &finish.Time
	}
	run.LastError = lastErr.String
	// tenancyguard:ok platform.saga_steps is global (G), keyed by its saga
	rows, err := tx.QueryContext(ctx, `SELECT step_no, service, action, status, attempts, response, coalesce(error, '')
		FROM platform.saga_steps WHERE saga_id = $1 ORDER BY step_no`, id)
	if err != nil {
		return nil, fmt.Errorf("saga steps: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var s write.SagaStep
		var resp []byte
		if err := rows.Scan(&s.No, &s.Service, &s.Action, &s.Status, &s.Attempts, &resp, &s.Error); err != nil {
			return nil, fmt.Errorf("saga steps: %w", err)
		}
		s.Response = resp
		run.Steps = append(run.Steps, s)
	}
	return &run, rows.Err()
}

// SaveStep implements write.SagaRepository.
func (SagaRepo) SaveStep(ctx context.Context, sagaID uuid.UUID, s write.SagaStep) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	var errText *string
	if s.Error != "" {
		errText = &s.Error
	}
	// tenancyguard:ok platform.saga_steps is global (G), keyed by its saga
	_, err = tx.ExecContext(ctx, `UPDATE platform.saga_steps SET status = $3, attempts = $4, response = $5::jsonb, error = $6, updated_at = now()
		WHERE saga_id = $1 AND step_no = $2`, sagaID, s.No, s.Status, s.Attempts, nullJSON(s.Response), errText)
	if err != nil {
		return fmt.Errorf("save saga step: %w", err)
	}
	return nil
}

// SaveRun implements write.SagaRepository.
func (SagaRepo) SaveRun(ctx context.Context, r *write.SagaRun) error {
	tx, err := portalTx(ctx)
	if err != nil {
		return err
	}
	var lastErr *string
	if r.LastError != "" {
		lastErr = &r.LastError
	}
	// tenancyguard:ok platform.saga_runs is global (G): one run by id and version
	err = tx.QueryRowContext(ctx, `UPDATE platform.saga_runs SET status = $3, attempts = $4, next_attempt_at = $5, last_error = $6,
			finished_at = $7, updated_at = now(), version = version + 1
		WHERE saga_id = $1 AND version = $2 RETURNING version`,
		r.ID, r.Version, r.Status, r.Attempts, r.NextAttemptAt, lastErr, r.FinishedAt).Scan(&r.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return problem.New(problem.Conflict, "saga changed")
	}
	if err != nil {
		return fmt.Errorf("save saga: %w", err)
	}
	return nil
}

// Due implements write.SagaRepository.
func (SagaRepo) Due(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform.saga_runs is global (G): due retries of every tenant (job saga-retry)
	rows, err := tx.QueryContext(ctx, `SELECT saga_id FROM platform.saga_runs WHERE status IN ('running', 'compensating')
		AND next_attempt_at IS NOT NULL AND next_attempt_at <= $1 ORDER BY next_attempt_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("due sagas: %w", err)
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

// Stuck implements write.SagaRepository.
func (SagaRepo) Stuck(ctx context.Context, before time.Time) (int, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok platform.saga_runs is global (G): count for the SagaStuck alert
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM platform.saga_runs WHERE status IN ('running', 'compensating') AND started_at < $1`, before).Scan(&n)
	return n, err
}
