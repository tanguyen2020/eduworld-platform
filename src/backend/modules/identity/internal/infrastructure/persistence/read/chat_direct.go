package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// ChatDirectModel implements read.ChatDirectPolicyReader on the projections of the chat policy levels (delta F112-7) in
// a READ ONLY session: the platform row is global, the other rows are read under the TenantScope (RLS) of the caller
// with an explicit tenant_id.
type ChatDirectModel struct {
	Session *entx.ReadSession
}

var _ read.ChatDirectPolicyReader = ChatDirectModel{}

// ChatDirectLevels implements read.ChatDirectPolicyReader.
func (m ChatDirectModel) ChatDirectLevels(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (settings.ChatDirectLevels, error) {
	var out settings.ChatDirectLevels
	err := m.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		var schoolID uuid.NullUUID
		if school != nil {
			schoolID = uuid.NullUUID{UUID: *school, Valid: true}
		}
		// Projections without soft-delete: the global platform row and the rows of the session tenant (RLS + explicit tenant_id).
		// tenancyguard:ok projection chat_direct_policy_platform (G) + chat_direct_policies of the session tenant (RLS, tenant_id)
		rows, err := tx.QueryContext(ctx, `SELECT 'platform', direct_message_enabled FROM identity.chat_direct_policy_platform
			UNION ALL
			SELECT scope_type, direct_message_enabled FROM identity.chat_direct_policies
			WHERE tenant_id = $1 AND (scope_type IN ('common', 'tenant') AND scope_id = $1 OR scope_type = 'school' AND scope_id = $2)`,
			tenant, schoolID)
		if err != nil {
			return fmt.Errorf("chat direct policy read: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var level string
			var enabled bool
			if err := rows.Scan(&level, &enabled); err != nil {
				return fmt.Errorf("chat direct policy read: %w", err)
			}
			v := enabled
			switch level {
			case settings.ChatLevelPlatform:
				out.Platform = &v
			case settings.ChatLevelCommon:
				out.Common = &v
			case settings.ChatLevelTenant:
				out.Tenant = &v
			case settings.ChatLevelSchool:
				out.School = &v
			}
		}
		return rows.Err()
	})
	return out, err
}
