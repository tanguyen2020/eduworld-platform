package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
)

// PolicyRow is one live chat policy document of a level (chat.chat_policies, F120).
type PolicyRow struct {
	ID        uuid.UUID
	TenantID  *uuid.UUID
	ScopeType string
	ScopeID   *uuid.UUID
	SchoolID  *uuid.UUID
	Rules     json.RawMessage
	Version   int64
	Reason    *string
	UpdatedBy *uuid.UUID
	UpdatedAt time.Time
}

// CommonOverrideRow is the System Admin override of a tenant common (chat.common_tenant_social_overrides).
type CommonOverrideRow struct {
	TenantID  uuid.UUID
	Override  policy.CommonOverride
	Reason    string
	Version   int64
	UpdatedBy *uuid.UUID
	UpdatedAt time.Time
}

// PolicyReader reads the chat policy levels of one tenant (or the platform) under entx.ReadSession or a write
// transaction: explicit SQL with tenant and deleted_at conditions on top of RLS (R27).
type PolicyReader interface {
	// Level returns the live document of a level (platform: tenant and scope nil); nil when none.
	Level(ctx context.Context, tenant *uuid.UUID, scopeType string, scopeID *uuid.UUID) (*PolicyRow, error)
	// TenantType returns tenant_type of the projected tenant ("" when not projected).
	TenantType(ctx context.Context, tenant uuid.UUID) (string, error)
	// SchoolActive reports a projected school of the tenant that is not disabled.
	SchoolActive(ctx context.Context, tenant, school uuid.UUID) (bool, error)
	// CommonOverride returns the override of a tenant common (nil: the defaults).
	CommonOverride(ctx context.Context, tenant uuid.UUID) (*CommonOverrideRow, error)
}
