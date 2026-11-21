package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CatalogVersionRow is a published catalogue version with its immutable snapshot.
type CatalogVersionRow struct {
	Version  int64
	Checksum string
	Snapshot json.RawMessage
}

// SwitchRow is a configured moderation switch (chat.moderation_switches) of the platform (TenantID nil) or a tenant.
type SwitchRow struct {
	ID         uuid.UUID
	TenantID   *uuid.UUID
	EnabledAll bool
	Steps      map[string]bool
	Reason     *string
	ExpiresAt  *time.Time
	UpdatedBy  *uuid.UUID
	UpdatedAt  time.Time
	Version    int64
}

// SanctionRow is an active sanction of a user (chat.sanctions, not revoked, not ended).
type SanctionRow struct {
	ID        uuid.UUID
	Type      string
	Scope     string
	ChannelID *uuid.UUID
	EndsAt    *time.Time
}

// QuickMessageRow is an approved quick message (chat.quick_messages).
type QuickMessageRow struct {
	ID        string
	Category  string
	TextVI    string
	TextEN    string
	Emoji     *string
	SortOrder int
}

// ModerationReader is the read side of the pipeline (PostgreSQL, through the read session; the platform rows are
// tenant_id NULL, readable by every tenant session).
type ModerationReader interface {
	// LatestCatalog returns the newest version of the platform catalogue (tenant nil) or of tenant (nil: none
	// published).
	LatestCatalog(ctx context.Context, tenant *uuid.UUID, after int64) (*CatalogVersionRow, error)
	// CatalogVersions returns the newest version numbers of the platform and of tenant (0 when none).
	CatalogVersions(ctx context.Context, tenant uuid.UUID) (platform, tenantV int64, err error)
	// TenantCategorySettings returns the categories switched on/off for tenant (category id → enabled).
	TenantCategorySettings(ctx context.Context, tenant uuid.UUID) (map[uuid.UUID]bool, error)
	// Switches returns the configured switch of the platform and of tenant (nil when not configured).
	Switches(ctx context.Context, tenant uuid.UUID) (platform, tenantRow *SwitchRow, err error)
	// ActiveSanctions returns the sanctions of user in tenant that are not revoked and not ended at now.
	ActiveSanctions(ctx context.Context, tenant, user uuid.UUID, now time.Time) ([]SanctionRow, error)
	// QuickMessages returns the active quick messages, in order.
	QuickMessages(ctx context.Context) ([]QuickMessageRow, error)
}
