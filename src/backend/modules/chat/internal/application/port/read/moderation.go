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

// SwitchRequestRow is a request to disable moderation switches (chat.moderation_switch_requests, ADR-0078).
type SwitchRequestRow struct {
	ID               uuid.UUID
	TenantID         *uuid.UUID
	EnabledAll       bool
	Steps            map[string]bool
	Reason           string
	ExpiresAt        time.Time
	Status           string
	ProposedBy       uuid.UUID
	ApprovedBy       *uuid.UUID
	CreatedAt        time.Time
	DecidedAt        *time.Time
	RequestExpiresAt time.Time
}

// Statuses of a switch request.
const (
	RequestPending  = "pending_approval"
	RequestApproved = "approved"
	RequestRejected = "rejected"
	RequestExpired  = "expired"
)

// SwitchChangeRow is one effective change of a switch (chat.moderation_switch_changes, append-only).
type SwitchChangeRow struct {
	ID               uuid.UUID
	TenantID         *uuid.UUID
	EnabledAllBefore bool
	EnabledAllAfter  bool
	Before           map[string]bool
	After            map[string]bool
	Reason           *string
	ExpiresAt        *time.Time
	ActorID          *uuid.UUID
	ApprovedBy       *uuid.UUID
	RequestID        *uuid.UUID
	Source           string
	ChangedAt        time.Time
}

// SwitchAdminReader is the read side of the switch administration (read session; explicit tenant and deleted_at
// conditions). tenants nil addresses the platform level (tenant_id NULL); otherwise the rows of those tenants only.
type SwitchAdminReader interface {
	// TenantSwitches returns the live configured switches of tenants (a tenant without a row is absent).
	TenantSwitches(ctx context.Context, tenants []uuid.UUID) (map[uuid.UUID]*SwitchRow, error)
	// PendingRequests returns the requests pending_approval and not past request_expires_at at now, oldest first.
	PendingRequests(ctx context.Context, tenants []uuid.UUID, now time.Time, limit int) ([]SwitchRequestRow, error)
	// History returns the changes, newest first, after the cursor.
	History(ctx context.Context, tenants []uuid.UUID, after *Cursor, limit int) ([]SwitchChangeRow, error)
	// DisabledSwitches returns every live switch row with a value off (platform and every tenant: the gauge
	// eduworld_chat_policy_disabled of a system job).
	DisabledSwitches(ctx context.Context) ([]SwitchRow, error)
}
