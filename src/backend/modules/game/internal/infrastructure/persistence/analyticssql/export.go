package analyticssql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Exports implements port.ExportStore (analytics.export_jobs; platform exports have tenant_id NULL and run on the platform
// pool).
type Exports struct{}

var _ port.ExportStore = Exports{}

const exportColumns = `export_id, tenant_id, requested_by, report, params::text, status, row_count, object_key, error_code, created_at, finished_at, expires_at`

func scanExport(s interface{ Scan(...any) error }) (*port.ExportJob, error) {
	var j port.ExportJob
	var tenant uuid.NullUUID
	var params string
	var key, code sql.NullString
	var fin, exp sql.NullTime
	if err := s.Scan(&j.ID, &tenant, &j.RequestedBy, &j.Report, &params, &j.Status, &j.RowCount, &key, &code, &j.CreatedAt, &fin, &exp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	j.Params = []byte(params)
	if tenant.Valid {
		t := tenant.UUID
		j.TenantID = &t
	}
	if key.Valid {
		j.ObjectKey = &key.String
	}
	if code.Valid {
		j.ErrorCode = &code.String
	}
	if fin.Valid {
		j.FinishedAt = &fin.Time
	}
	if exp.Valid {
		j.ExpiresAt = &exp.Time
	}
	return &j, nil
}

// ActiveExports implements port.ExportStore.
func (Exports) ActiveExports(ctx context.Context, user uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok platform export jobs of the requester (tenant_id NULL, no deleted_at — job table)
	err = t.QueryRowContext(ctx, `SELECT count(*) FROM analytics.export_jobs WHERE requested_by = $1 AND tenant_id IS NULL
		AND status IN ('queued', 'running')`, user).Scan(&n)
	return n, err
}

// InsertExport implements port.ExportStore.
func (Exports) InsertExport(ctx context.Context, j port.ExportJob) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	_, err = t.ExecContext(ctx, `INSERT INTO analytics.export_jobs (export_id, tenant_id, requested_by, report, params, status, created_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)`, j.ID, j.TenantID, j.RequestedBy, j.Report, string(j.Params), j.Status, j.CreatedAt)
	if err != nil {
		return fmt.Errorf("analytics insert export: %w", err)
	}
	return nil
}

// Export implements port.ExportStore.
func (Exports) Export(ctx context.Context, id uuid.UUID) (*port.ExportJob, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform export job by id (tenant_id NULL, no deleted_at — job table); the caller checks the requester
	return scanExport(t.QueryRowContext(ctx, `SELECT `+exportColumns+` FROM analytics.export_jobs WHERE export_id = $1 AND tenant_id IS NULL`, id))
}

// ClaimExport implements port.ExportStore: the oldest queued job, FOR UPDATE SKIP LOCKED (several workers).
func (Exports) ClaimExport(ctx context.Context, now time.Time) (*port.ExportJob, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform export queue (tenant_id NULL, no deleted_at — job table)
	j, err := scanExport(t.QueryRowContext(ctx, `UPDATE analytics.export_jobs SET status = 'running', started_at = $1
		WHERE export_id = (SELECT export_id FROM analytics.export_jobs WHERE status = 'queued' AND tenant_id IS NULL ORDER BY created_at
			FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING `+exportColumns, now))
	if err != nil {
		return nil, fmt.Errorf("analytics claim export: %w", err)
	}
	return j, nil
}

// FinishExport implements port.ExportStore.
func (Exports) FinishExport(ctx context.Context, j port.ExportJob) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform export job by id (tenant_id NULL, no deleted_at — job table)
	_, err = t.ExecContext(ctx, `UPDATE analytics.export_jobs SET status = $2, row_count = $3, object_key = $4, error_code = $5, finished_at = $6,
		expires_at = $7 WHERE export_id = $1 AND tenant_id IS NULL`, j.ID, j.Status, j.RowCount, j.ObjectKey, j.ErrorCode, j.FinishedAt, j.ExpiresAt)
	if err != nil {
		return fmt.Errorf("analytics finish export: %w", err)
	}
	return nil
}
