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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
)

// ReportRepo implements write.ReportJobStore and write.ReportJobQueue on learning.report_jobs (F044 US1). Tenant jobs are
// written in the TenantScope of ctx (RLS); cross-tenant jobs of System Admin (tenant_id NULL) and the worker queue run on
// learning_platform under tenancy.Bypass.
type ReportRepo struct{}

var (
	_ write.ReportJobStore = ReportRepo{}
	_ write.ReportJobQueue = ReportRepo{}
)

const reportJobColumns = read.ReportJobColumns

// InsertReportJob implements write.ReportJobStore.
func (ReportRepo) InsertReportJob(ctx context.Context, j write.ReportJobRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	params, err := json.Marshal(j.Params)
	if err != nil {
		return err
	}
	// tenancyguard:ok INSERT in the session tenant (RLS insert policy); platform jobs (tenant_id NULL) on learning_platform
	if _, err := tx.ExecContext(ctx, `INSERT INTO learning.report_jobs (report_id, tenant_id, scope_level, report_type, format, params,
		requested_by, status, idempotency_key, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)`,
		j.ID, j.TenantID, j.Scope, j.Type, j.Format, params, j.RequestedBy, j.Status, j.IdempotencyKey, j.CreatedAt); err != nil {
		return fmt.Errorf("insert report job: %w", err)
	}
	return nil
}

// ReportJobByKey implements write.ReportJobStore.
func (ReportRepo) ReportJobByKey(ctx context.Context, requestedBy uuid.UUID, key string) (write.ReportJobRow, bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return write.ReportJobRow{}, false, err
	}
	// tenancyguard:ok rows of the requester in the session tenant (RLS); deleted_at IS NULL
	j, err := read.ScanReportJob(tx.QueryRowContext(ctx, `SELECT `+reportJobColumns+` FROM learning.report_jobs
		WHERE requested_by = $1 AND idempotency_key = $2 AND deleted_at IS NULL`, requestedBy, key))
	if errors.Is(err, sql.ErrNoRows) {
		return j, false, nil
	}
	return j, err == nil, err
}

// ActiveReportJobs implements write.ReportJobStore.
func (ReportRepo) ActiveReportJobs(ctx context.Context, requestedBy uuid.UUID) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok jobs of the requester visible in the session (RLS tenant, or learning_platform for System Admin); deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM learning.report_jobs WHERE requested_by = $1 AND status IN ('queued','running')
		AND deleted_at IS NULL`, requestedBy).Scan(&n)
	return n, err
}

// ClaimReportJobs implements write.ReportJobQueue: queued jobs and running jobs started before staleBefore, oldest first,
// set running with run_count + 1 (learning_platform, every tenant).
func (ReportRepo) ClaimReportJobs(ctx context.Context, staleBefore time.Time, limit int) ([]write.ReportJobRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok worker queue of every tenant under tenancy.Bypass(platform_job) on learning_platform; deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `UPDATE learning.report_jobs j SET status = 'running', run_count = j.run_count + 1,
			started_at = now(), updated_at = now(), version = j.version + 1
		WHERE j.report_id IN (SELECT report_id FROM learning.report_jobs
			WHERE deleted_at IS NULL AND (status = 'queued' OR (status = 'running' AND started_at < $1))
			ORDER BY created_at LIMIT $2 FOR UPDATE SKIP LOCKED)
		RETURNING `+prefixed("j.", reportJobColumns), staleBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("claim report jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ReportJobRow
	for rows.Next() {
		j, err := read.ScanReportJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// FinishReportJob implements write.ReportJobQueue.
func (ReportRepo) FinishReportJob(ctx context.Context, j write.ReportJobRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok update of one job by id under tenancy.Bypass(platform_job) (learning_platform); deleted_at IS NULL
	_, err = tx.ExecContext(ctx, `UPDATE learning.report_jobs SET status = $2, row_count = $3, storage_key = $4, error = $5,
		finished_at = $6, file_expires_at = $7, updated_at = now(), version = version + 1 WHERE report_id = $1 AND deleted_at IS NULL`,
		j.ID, j.Status, j.RowCount, j.StorageKey, j.Error, j.FinishedAt, j.FileExpiresAt)
	if err != nil {
		return fmt.Errorf("finish report job: %w", err)
	}
	return nil
}

// ExpireReportJobs implements write.ReportJobQueue: succeeded jobs whose file passed its retention become expired (the file
// itself expires by the bucket lifecycle).
func (ReportRepo) ExpireReportJobs(ctx context.Context, now time.Time) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	// tenancyguard:ok expiry of every tenant under tenancy.Bypass(platform_job) (learning_platform); deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE learning.report_jobs SET status = 'expired', updated_at = now(), version = version + 1
		WHERE status = 'succeeded' AND file_expires_at <= $1 AND deleted_at IS NULL`, now)
	if err != nil {
		return 0, fmt.Errorf("expire report jobs: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// prefixed prefixes every column of a comma-separated list.
func prefixed(p, cols string) string {
	out := []byte{}
	for i, part := range splitCols(cols) {
		if i > 0 {
			out = append(out, ", "...)
		}
		out = append(out, p+part...)
	}
	return string(out)
}

func splitCols(cols string) []string {
	var out []string
	cur := []byte{}
	for i := 0; i < len(cols); i++ {
		switch c := cols[i]; c {
		case ',':
			out = append(out, string(cur))
			cur = cur[:0]
		case ' ', '\n', '\t':
		default:
			cur = append(cur, c)
		}
	}
	return append(out, string(cur))
}
