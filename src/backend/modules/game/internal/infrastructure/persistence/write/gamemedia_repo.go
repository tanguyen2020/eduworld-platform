package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// GameMediaRepo implements write.GameMediaStore on game.media_assets, game.media_usages and game.render_jobs (F078).
// Platform media: no tenant_id, written under PlatformScope only; deleted_at filtered.
type GameMediaRepo struct{}

var _ write.GameMediaStore = GameMediaRepo{}

const assetCols = `asset_id, kind, mime_type, original_filename, size_bytes, sha256, bucket, quarantine_key, status, reject_reason, variants,
	width, height, license, alt_text, uploaded_by, upload_expires_at, created_at`

func scanAsset(row interface{ Scan(...any) error }) (*write.MediaAsset, error) {
	var a write.MediaAsset
	var qk, reject, license, alt sql.NullString
	var variants []byte
	if err := row.Scan(&a.ID, &a.Kind, &a.MIME, &a.Filename, &a.Size, &a.SHA256, &a.Bucket, &qk, &a.Status, &reject, &variants, &a.Width,
		&a.Height, &license, &alt, &a.UploadedBy, &a.UploadExpiresAt, &a.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a.QuarantineKey, a.RejectReason, a.License, a.AltText = strPtr(qk), strPtr(reject), strPtr(license), strPtr(alt)
	if len(variants) > 0 {
		_ = json.Unmarshal(variants, &a.Variants)
	}
	return &a, nil
}

func assets(rows *sql.Rows, err error) ([]write.MediaAsset, error) {
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []write.MediaAsset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// AssetBySHA implements write.GameMediaStore.
func (GameMediaRepo) AssetBySHA(ctx context.Context, sha string) (*write.MediaAsset, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform media (G table, no tenant_id); deleted_at filtered
	return scanAsset(t.QueryRowContext(ctx, `SELECT `+assetCols+` FROM game.media_assets WHERE sha256 = $1 AND deleted_at IS NULL`, sha))
}

// InsertAsset implements write.GameMediaStore.
func (GameMediaRepo) InsertAsset(ctx context.Context, a write.MediaAsset, at time.Time) error {
	var variants any
	if a.Variants != nil {
		b, err := json.Marshal(a.Variants)
		if err != nil {
			return err
		}
		variants = string(b)
	}
	// tenancyguard:ok platform media (G table, no tenant_id)
	_, err := exec(ctx, `INSERT INTO game.media_assets (asset_id, created_at, created_by, updated_at, kind, mime_type, original_filename,
			size_bytes, sha256, bucket, quarantine_key, status, reject_reason, variants, width, height, license, alt_text, uploaded_by,
			upload_expires_at, processed_at)
		VALUES ($1, $2, $3, $2, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14, $15, $16, $17, $3, $18, $19)`,
		a.ID, at, a.UploadedBy, a.Kind, a.MIME, a.Filename, a.Size, a.SHA256, a.Bucket, a.QuarantineKey, a.Status, a.RejectReason, variants,
		a.Width, a.Height, a.License, a.AltText, a.UploadExpiresAt, processedAt(a.Status, at))
	if uniqueViolation(err) {
		return write.ErrConflict
	}
	return err
}

// Pending implements write.GameMediaStore.
func (GameMediaRepo) Pending(ctx context.Context, before time.Time, limit int) ([]write.MediaAsset, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform media (G table, no tenant_id); deleted_at filtered
	return assets(t.QueryContext(ctx, `SELECT `+assetCols+` FROM game.media_assets
		WHERE status IN ('quarantined', 'processing') AND created_at <= $1 AND deleted_at IS NULL
		ORDER BY created_at, asset_id LIMIT $2 FOR UPDATE SKIP LOCKED`, before, limit))
}

// SaveAsset implements write.GameMediaStore.
func (GameMediaRepo) SaveAsset(ctx context.Context, a write.MediaAsset, at time.Time) error {
	var variants any
	if a.Variants != nil {
		b, err := json.Marshal(a.Variants)
		if err != nil {
			return err
		}
		variants = string(b)
	}
	// tenancyguard:ok platform media (G table, no tenant_id); deleted_at filtered
	_, err := exec(ctx, `UPDATE game.media_assets SET status = $2, reject_reason = $3, bucket = $4, variants = $5::jsonb, width = $6, height = $7,
			processed_at = coalesce($9, processed_at), updated_at = $8, version = version + 1
		WHERE asset_id = $1 AND deleted_at IS NULL`, a.ID, a.Status, a.RejectReason, a.Bucket, variants, a.Width, a.Height, at, processedAt(a.Status, at))
	return err
}

// Asset implements write.GameMediaStore.
func (GameMediaRepo) Asset(ctx context.Context, id uuid.UUID) (*write.MediaAsset, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform media (G table, no tenant_id); deleted_at filtered
	return scanAsset(t.QueryRowContext(ctx, `SELECT `+assetCols+` FROM game.media_assets WHERE asset_id = $1 AND deleted_at IS NULL FOR UPDATE`, id))
}

// Content3D implements write.GameMediaStore.
func (GameMediaRepo) Content3D(ctx context.Context, list []uuid.UUID) ([]write.MediaAsset, error) {
	if len(list) == 0 {
		return nil, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform media (G table, no tenant_id); deleted_at filtered
	return assets(t.QueryContext(ctx, `SELECT `+assetCols+` FROM game.media_assets WHERE asset_id = ANY($1::uuid[]) AND kind = 'model3d'
		AND status = 'ready' AND deleted_at IS NULL ORDER BY asset_id`, uuidStrings(list)))
}

// VersionPayload implements write.GameMediaStore.
func (GameMediaRepo) VersionPayload(ctx context.Context, versionID uuid.UUID) (json.RawMessage, uuid.UUID, int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, uuid.Nil, 0, err
	}
	var b []byte
	var cfg uuid.UUID
	var no int
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT payload, config_id, version_no FROM game.config_versions WHERE config_version_id = $1
		AND deleted_at IS NULL`, versionID).Scan(&b, &cfg, &no)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, uuid.Nil, 0, nil
	}
	return b, cfg, no, err
}

const renderCols = `render_job_id, config_type, item_type, config_version_id, source_asset_id, kind, status, attempts, last_error, result_asset_id,
	next_attempt_at`

func scanRender(row interface{ Scan(...any) error }) (*write.RenderJob, error) {
	var j write.RenderJob
	var last sql.NullString
	if err := row.Scan(&j.ID, &j.ConfigType, &j.ItemType, &j.VersionID, &j.SourceAssetID, &j.Kind, &j.Status, &j.Attempts, &last,
		&j.ResultAssetID, &j.NextAttemptAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	j.LastError = strPtr(last)
	return &j, nil
}

// InsertRenderJob implements write.GameMediaStore.
func (GameMediaRepo) InsertRenderJob(ctx context.Context, j write.RenderJob, at time.Time) (bool, error) {
	// tenancyguard:ok platform renders (G table, no tenant_id)
	return exec(ctx, `INSERT INTO game.render_jobs (render_job_id, created_at, updated_at, config_type, item_type, config_version_id,
			source_asset_id, kind, status, attempts) VALUES ($1, $2, $2, $3, $4, $5, $6, $7, 'queued', 0)
		ON CONFLICT (config_version_id, source_asset_id, kind) DO NOTHING`, j.ID, at, j.ConfigType, j.ItemType, j.VersionID, j.SourceAssetID, j.Kind)
}

// RenderJob implements write.GameMediaStore.
func (GameMediaRepo) RenderJob(ctx context.Context, id uuid.UUID) (*write.RenderJob, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform renders (G table, no tenant_id); deleted_at filtered
	return scanRender(t.QueryRowContext(ctx, `SELECT `+renderCols+` FROM game.render_jobs WHERE render_job_id = $1 AND deleted_at IS NULL
		FOR UPDATE`, id))
}

// SaveRenderJob implements write.GameMediaStore.
func (GameMediaRepo) SaveRenderJob(ctx context.Context, j write.RenderJob, at time.Time) error {
	// tenancyguard:ok platform renders (G table, no tenant_id); deleted_at filtered
	_, err := exec(ctx, `UPDATE game.render_jobs SET status = $2, attempts = $3, last_error = $4, result_asset_id = $5, next_attempt_at = $6,
			updated_at = $7, version = version + 1 WHERE render_job_id = $1 AND deleted_at IS NULL`,
		j.ID, j.Status, j.Attempts, j.LastError, j.ResultAssetID, j.NextAttemptAt, at)
	return err
}

// DueRenderRetries implements write.GameMediaStore.
func (GameMediaRepo) DueRenderRetries(ctx context.Context, now time.Time, limit int) ([]write.RenderJob, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform renders (G table, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT `+renderCols+` FROM game.render_jobs WHERE status = 'failed' AND next_attempt_at <= $1
		AND deleted_at IS NULL ORDER BY next_attempt_at, render_job_id LIMIT $2 FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []write.RenderJob
	for rows.Next() {
		j, err := scanRender(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// LinkRender implements write.GameMediaStore.
func (GameMediaRepo) LinkRender(ctx context.Context, assetID uuid.UUID, j write.RenderJob, at time.Time) error {
	// tenancyguard:ok platform media (G table, no tenant_id)
	_, err := exec(ctx, `INSERT INTO game.media_usages (usage_id, created_at, updated_at, asset_id, item_type, config_id, config_version_id, usage)
		SELECT $1, $2, $2, $3, $4, v.config_id, v.config_version_id, $5 FROM game.config_versions v WHERE v.config_version_id = $6
		ON CONFLICT (asset_id, config_version_id, usage) WHERE deleted_at IS NULL DO NOTHING`, ids.New(), at, assetID, j.ItemType, j.Kind, j.VersionID)
	return err
}

// DeadLetter implements write.GameMediaStore.
func (GameMediaRepo) DeadLetter(ctx context.Context, j write.RenderJob, at time.Time) error {
	payload, err := json.Marshal(map[string]any{"render_job_id": j.ID, "config_version_id": j.VersionID, "source_asset_id": j.SourceAssetID,
		"kind": j.Kind, "attempts": j.Attempts})
	if err != nil {
		return err
	}
	reason := "render failed"
	if j.LastError != nil {
		reason = *j.LastError
	}
	// tenancyguard:ok infra.dead_letters (module infrastructure table, no tenant)
	_, err = exec(ctx, `INSERT INTO infra.dead_letters (dead_letter_id, stream, consumer, event_id, event_type, payload, error, delivery_count,
		created_at) VALUES ($1, 'GAME_RENDER_JOBS', 'game.render_results', $2, 'render_job', $3::jsonb, $4, $5, $6)`,
		ids.New(), j.ID, string(payload), reason, j.Attempts, at)
	if err != nil {
		return fmt.Errorf("render dead letter: %w", err)
	}
	return nil
}

// TenantAssignmentRepo implements write.TenantAssignmentStore on infra.tenant_resource_assignments of eduworld_game (ADR-0029):
// explicit tenant conditions; soft delete only (R21).
type TenantAssignmentRepo struct{}

var _ write.TenantAssignmentStore = TenantAssignmentRepo{}

func scanAssignments(rows *sql.Rows, err error) ([]write.Assignment, error) {
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []write.Assignment
	for rows.Next() {
		var a write.Assignment
		if err := rows.Scan(&a.ID, &a.ResourceType, &a.ResourceID, &a.TenantID, &a.AssignedBy, &a.AssignedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Active implements write.TenantAssignmentStore.
func (TenantAssignmentRepo) Active(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID) ([]write.Assignment, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok explicit tenant condition (the requested tenants) on infra.tenant_resource_assignments; deleted_at filtered
	return scanAssignments(t.QueryContext(ctx, `SELECT assignment_id, resource_type, resource_id, tenant_id, assigned_by, assigned_at
		FROM infra.tenant_resource_assignments WHERE resource_type = $1 AND resource_id = $2 AND tenant_id = ANY($3::uuid[])
		AND deleted_at IS NULL ORDER BY tenant_id FOR UPDATE`, resourceType, resourceID, uuidStrings(tenants)))
}

// Create implements write.TenantAssignmentStore.
func (TenantAssignmentRepo) Create(ctx context.Context, a write.Assignment) error {
	// tenancyguard:ok explicit tenant_id of the new assignment (CrossTenantScope of the requested tenants)
	_, err := exec(ctx, `INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at, created_by, updated_at, updated_by,
		resource_type, resource_id, assigned_by, assigned_at) VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $4, $3)`,
		a.ID, a.TenantID, a.AssignedAt, a.AssignedBy, a.ResourceType, a.ResourceID)
	return err
}

// Revoke implements write.TenantAssignmentStore (soft delete).
func (TenantAssignmentRepo) Revoke(ctx context.Context, a write.Assignment, by uuid.UUID, reason string, at time.Time) (write.Assignment, error) {
	_ = reason // the reason is kept by the audit (delete_reason is a short code)
	// tenancyguard:ok explicit tenant_id of the assignment; soft delete (R21)
	_, err := exec(ctx, `UPDATE infra.tenant_resource_assignments SET deleted_at = $3, deleted_by = $4, delete_reason = 'revoked', updated_at = $3,
		updated_by = $4, version = version + 1 WHERE assignment_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, a.ID, a.TenantID, at, by)
	a.RevokedAt = &at
	return a, err
}

// Page implements write.TenantAssignmentStore.
func (TenantAssignmentRepo) Page(ctx context.Context, resourceType string, resourceID uuid.UUID, after *uuid.UUID, limit int) ([]write.Assignment, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok every tenant of one platform resource (System Admin, platform.tenant_data.assign); deleted_at filtered; keyset
	return scanAssignments(t.QueryContext(ctx, `SELECT assignment_id, resource_type, resource_id, tenant_id, assigned_by, assigned_at
		FROM infra.tenant_resource_assignments WHERE resource_type = $1 AND resource_id = $2 AND deleted_at IS NULL
		AND ($3::uuid IS NULL OR tenant_id > $3) ORDER BY tenant_id LIMIT $4`, resourceType, resourceID, after, limit))
}

// ActiveTenants implements write.TenantAssignmentStore.
func (TenantAssignmentRepo) ActiveTenants(ctx context.Context, list []uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.tenants: explicit tenant ids of the request; projections have no deleted_at
	rows, err := t.QueryContext(ctx, `SELECT tenant_id FROM ref.tenants WHERE tenant_id = ANY($1::uuid[]) AND status = 'active'`, uuidStrings(list))
	if err != nil {
		return nil, err
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

// Published implements write.TenantAssignmentStore.
func (TenantAssignmentRepo) Published(ctx context.Context, itemType string, configID uuid.UUID) (bool, bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, false, err
	}
	var current uuid.NullUUID
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT current_version_id FROM game.config_items WHERE item_type = $1 AND config_id = $2 AND deleted_at IS NULL`,
		itemType, configID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, current.Valid, nil
}

// processedAt is the processed_at of an asset reaching status (ready or rejected end the pipeline).
func processedAt(status string, at time.Time) *time.Time {
	if status == "ready" || status == "rejected" {
		return &at
	}
	return nil
}
