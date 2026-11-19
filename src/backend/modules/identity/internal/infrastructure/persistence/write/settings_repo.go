package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SettingsRepo implements write.SettingsRepository with explicit SQL (AGENTS R27). Platform settings are
// global (G, written by identity_platform under a platform scope); tenant and school settings are read and
// written under the TenantScope of the tenant (RLS on org.schools; platform.tenants is global and filtered by
// id), always with deleted_at IS NULL.
type SettingsRepo struct{}

var _ write.SettingsRepository = SettingsRepo{}

// stale is the optimistic-concurrency failure (412 VERSION_MISMATCH at the presentation).
func stale() error { return problem.New(problem.VersionMismatch, "") }

// Platform implements write.SettingsRepository.
func (SettingsRepo) Platform(ctx context.Context, key string) (write.StoredSetting, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return write.StoredSetting{}, err
	}
	var s write.StoredSetting
	var at time.Time
	// tenancyguard:ok platform.platform_settings is global (G, no tenant_id, no soft-delete)
	err = tx.QueryRowContext(ctx, `SELECT value, version, updated_at FROM platform.platform_settings WHERE key = $1`, key).Scan(&s.Value, &s.Version, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return write.StoredSetting{}, nil
	}
	if err != nil {
		return s, fmt.Errorf("platform setting: %w", err)
	}
	s.UpdatedAt = &at
	return s, nil
}

// PutPlatform implements write.SettingsRepository.
func (SettingsRepo) PutPlatform(ctx context.Context, key string, value []byte, expected int64, by uuid.UUID) (int64, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return 0, err
	}
	var v int64
	if expected == 0 {
		// tenancyguard:ok platform.platform_settings is global (G)
		err = tx.QueryRowContext(ctx, `INSERT INTO platform.platform_settings (key, value, updated_by) VALUES ($1, $2::jsonb, $3)
			ON CONFLICT (key) DO NOTHING RETURNING version`, key, string(value), by).Scan(&v)
	} else {
		// tenancyguard:ok platform.platform_settings is global (G)
		err = tx.QueryRowContext(ctx, `UPDATE platform.platform_settings SET value = $2::jsonb, version = version + 1, updated_at = now(), updated_by = $4
			WHERE key = $1 AND version = $3 RETURNING version`, key, string(value), expected, by).Scan(&v)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, stale()
	}
	if err != nil {
		return 0, fmt.Errorf("put platform setting: %w", err)
	}
	return v, nil
}

// Tenant implements write.SettingsRepository.
func (SettingsRepo) Tenant(ctx context.Context, tenantID uuid.UUID) (write.StoredSetting, bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return write.StoredSetting{}, false, err
	}
	var s write.StoredSetting
	var at time.Time
	// tenancyguard:ok platform.tenants is global (G): one live tenant by id
	err = tx.QueryRowContext(ctx, `SELECT settings, version, updated_at, tenant_type, status FROM platform.tenants
		WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).Scan(&s.Value, &s.Version, &at, &s.TenantType, &s.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	if err != nil {
		return s, false, fmt.Errorf("tenant settings: %w", err)
	}
	s.UpdatedAt = &at
	return s, true, nil
}

// PutTenant implements write.SettingsRepository.
func (SettingsRepo) PutTenant(ctx context.Context, tenantID uuid.UUID, value []byte, expected int64, by uuid.UUID) (int64, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return 0, err
	}
	var v int64
	// tenancyguard:ok platform.tenants is global (G): one live tenant by id and version
	err = tx.QueryRowContext(ctx, `UPDATE platform.tenants SET settings = $2::jsonb, version = version + 1, updated_at = now(), updated_by = $4
		WHERE tenant_id = $1 AND version = $3 AND deleted_at IS NULL RETURNING version`, tenantID, string(value), expected, by).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, stale()
	}
	if err != nil {
		return 0, fmt.Errorf("put tenant settings: %w", err)
	}
	return v, nil
}

// School implements write.SettingsRepository.
func (SettingsRepo) School(ctx context.Context, schoolID uuid.UUID) (write.StoredSetting, uuid.UUID, bool, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return write.StoredSetting{}, uuid.Nil, false, err
	}
	var s write.StoredSetting
	var at time.Time
	var tenant uuid.UUID
	// tenancyguard:ok org.schools under the TenantScope (RLS) or the platform scope of a job; deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT settings, version, updated_at, tenant_id, status FROM org.schools WHERE school_id = $1 AND deleted_at IS NULL`, schoolID).
		Scan(&s.Value, &s.Version, &at, &tenant, &s.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return s, uuid.Nil, false, nil
	}
	if err != nil {
		return s, uuid.Nil, false, fmt.Errorf("school settings: %w", err)
	}
	s.UpdatedAt = &at
	return s, tenant, true, nil
}

// PutSchool implements write.SettingsRepository.
func (SettingsRepo) PutSchool(ctx context.Context, schoolID uuid.UUID, value []byte, expected int64, by uuid.UUID) (int64, error) {
	tx, err := portalTx(ctx)
	if err != nil {
		return 0, err
	}
	var v int64
	// tenancyguard:ok org.schools under the TenantScope (RLS); deleted_at filtered
	err = tx.QueryRowContext(ctx, `UPDATE org.schools SET settings = $2::jsonb, version = version + 1, updated_at = now(), updated_by = $4
		WHERE school_id = $1 AND version = $3 AND deleted_at IS NULL RETURNING version`, schoolID, string(value), expected, by).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, stale()
	}
	if err != nil {
		return 0, fmt.Errorf("put school settings: %w", err)
	}
	return v, nil
}
