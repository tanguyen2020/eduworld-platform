package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// StoredSetting is a JSON settings value with its version (0 = not configured yet: defaults apply).
type StoredSetting struct {
	Value     []byte
	Version   int64
	UpdatedAt *time.Time
	// TenantType and Status of a tenant row (tenant.updated payload); empty for other settings.
	TenantType, Status string
}

// SettingsRepository reads and writes platform.platform_settings (G) and the settings JSON of
// platform.tenants and org.schools (F015 US3/US4). Writes check the expected version (409 → 412 by the
// caller); the tenant/school row version serves as the settings version [Proposed, F015 data-model].
type SettingsRepository interface {
	Platform(ctx context.Context, key string) (StoredSetting, error)
	// PutPlatform inserts (expected 0) or updates (expected = current) a key; returns the new version.
	PutPlatform(ctx context.Context, key string, value []byte, expected int64, by uuid.UUID) (int64, error)
	// Tenant returns the settings of a live tenant (found false when absent).
	Tenant(ctx context.Context, tenantID uuid.UUID) (s StoredSetting, found bool, err error)
	PutTenant(ctx context.Context, tenantID uuid.UUID, value []byte, expected int64, by uuid.UUID) (int64, error)
	// School returns the settings of a live school of the scope tenant.
	School(ctx context.Context, schoolID uuid.UUID) (s StoredSetting, tenantID uuid.UUID, found bool, err error)
	PutSchool(ctx context.Context, schoolID uuid.UUID, value []byte, expected int64, by uuid.UUID) (int64, error)
}
