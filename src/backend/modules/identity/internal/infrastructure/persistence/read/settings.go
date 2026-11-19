package read

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// SettingsModel implements read.SettingsReader with explicit SQL in a READ ONLY session (AGENTS R27):
// platform settings are global; tenants are global rows read by id; schools are read under the TenantScope
// (RLS) of the caller; deleted_at filtered.
type SettingsModel struct {
	Session *entx.ReadSession
}

var _ read.SettingsReader = SettingsModel{}

func (m SettingsModel) one(ctx context.Context, q string, args []any, dest ...any) (found bool, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		err := tx.QueryRowContext(ctx, q, args...).Scan(dest...)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("settings read: %w", err)
		}
		found = true
		return nil
	})
	return found, err
}

// PlatformSetting implements read.SettingsReader.
func (m SettingsModel) PlatformSetting(ctx context.Context, key string) (read.StoredSettings, error) {
	var s read.StoredSettings
	var at time.Time
	// tenancyguard:ok platform.platform_settings is global (G, no tenant_id, no soft-delete)
	found, err := m.one(ctx, `SELECT value, version, updated_at FROM platform.platform_settings WHERE key = $1`, []any{key}, &s.Value, &s.Version, &at)
	if found {
		s.UpdatedAt = &at
	}
	return s, err
}

// TenantSettings implements read.SettingsReader.
func (m SettingsModel) TenantSettings(ctx context.Context, tenantID uuid.UUID) (read.StoredSettings, bool, error) {
	var s read.StoredSettings
	var at time.Time
	// tenancyguard:ok platform.tenants is global (G): one live tenant by id
	found, err := m.one(ctx, `SELECT settings, version, updated_at, tenant_id FROM platform.tenants WHERE tenant_id = $1 AND deleted_at IS NULL`,
		[]any{tenantID}, &s.Value, &s.Version, &at, &s.TenantID)
	if found {
		s.UpdatedAt = &at
	}
	return s, found, err
}

// SchoolSettings implements read.SettingsReader.
func (m SettingsModel) SchoolSettings(ctx context.Context, schoolID uuid.UUID) (read.StoredSettings, bool, error) {
	var s read.StoredSettings
	var at time.Time
	// tenancyguard:ok org.schools under the TenantScope (RLS) of the caller; deleted_at filtered
	found, err := m.one(ctx, `SELECT settings, version, updated_at, tenant_id FROM org.schools WHERE school_id = $1 AND deleted_at IS NULL`,
		[]any{schoolID}, &s.Value, &s.Version, &at, &s.TenantID)
	if found {
		s.UpdatedAt = &at
	}
	return s, found, err
}
