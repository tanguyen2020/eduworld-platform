package write

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
)

// ChatDirectPolicyRepo implements write.ChatDirectPolicyWriter on identity.chat_direct_policy_platform and
// identity.chat_direct_policies (delta F112-7): upserts that only move a level to a newer source version.
type ChatDirectPolicyRepo struct{}

var _ write.ChatDirectPolicyWriter = ChatDirectPolicyRepo{}

// SetPlatformChatDirect implements write.ChatDirectPolicyWriter (platform scope of the consumer).
func (ChatDirectPolicyRepo) SetPlatformChatDirect(ctx context.Context, enabled bool, version int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok projection identity.chat_direct_policy_platform is global (G, one row, no soft-delete)
	res, err := tx.ExecContext(ctx, `INSERT INTO identity.chat_direct_policy_platform AS p (scope_key, direct_message_enabled, source_version, updated_at)
		VALUES ('platform', $1, $2, now())
		ON CONFLICT (scope_key) DO UPDATE SET direct_message_enabled = EXCLUDED.direct_message_enabled,
			source_version = EXCLUDED.source_version, updated_at = now()
		WHERE p.source_version < EXCLUDED.source_version`, enabled, version)
	if err != nil {
		return false, fmt.Errorf("chat direct policy: platform: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// SetChatDirect implements write.ChatDirectPolicyWriter (TenantScope of the event: RLS and explicit tenant_id).
func (ChatDirectPolicyRepo) SetChatDirect(ctx context.Context, tenant uuid.UUID, level string, scopeID uuid.UUID, enabled bool,
	version int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok projection identity.chat_direct_policies (no soft-delete) under the event TenantScope (RLS) and explicit tenant_id
	res, err := tx.ExecContext(ctx, `INSERT INTO identity.chat_direct_policies AS p (tenant_id, scope_type, scope_id, direct_message_enabled,
			source_version, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (tenant_id, scope_type, scope_id) DO UPDATE SET direct_message_enabled = EXCLUDED.direct_message_enabled,
			source_version = EXCLUDED.source_version, updated_at = now()
		WHERE p.tenant_id = $1 AND p.source_version < EXCLUDED.source_version`, tenant, level, scopeID, enabled, version)
	if err != nil {
		return false, fmt.Errorf("chat direct policy: %s: %w", level, err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
