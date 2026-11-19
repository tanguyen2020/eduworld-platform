package read

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// AuditModel implements read.AuditReadModel with explicit SQL (AGENTS R27): every statement carries its
// tenant condition (session tenant, the tenant_ids of a CrossTenantScope, or tenant_id IS NULL for the
// platform view) and the time range; audit.audit_logs is append-only (no deleted_at). actor_type user is
// returned as the user_type of the actor (student/staff, ADR-0081).
type AuditModel struct {
	Session *entx.ReadSession
}

var _ read.AuditReadModel = AuditModel{}

// AuditLogs implements read.AuditReadModel.
func (m AuditModel) AuditLogs(ctx context.Context, f read.AuditFilter) (out []dto.AuditLogEntry, err error) {
	var (
		where []string
		args  []any
	)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	switch {
	case f.PlatformOnly:
		where = append(where, "a.tenant_id IS NULL")
	case len(f.TenantIDs) > 0:
		where = append(where, "a.tenant_id = ANY("+arg(f.TenantIDs)+"::uuid[])")
	default:
		return nil, fmt.Errorf("audit read: no tenant condition")
	}
	where = append(where, "a.occurred_at >= "+arg(f.From), "a.occurred_at < "+arg(f.To))
	if f.SchoolIDs != nil {
		where = append(where, "a.school_id = ANY("+arg(f.SchoolIDs)+"::uuid[])")
	}
	if f.SchoolID != nil {
		where = append(where, "a.school_id = "+arg(*f.SchoolID))
	}
	if f.ActorID != nil {
		where = append(where, "a.actor_id = "+arg(*f.ActorID))
	}
	if f.Action != "" {
		where = append(where, "a.action = "+arg(f.Action))
	}
	if f.TargetType != "" {
		where = append(where, "a.target_type = "+arg(f.TargetType))
	}
	if f.TargetID != "" {
		where = append(where, "a.target_id = "+arg(f.TargetID))
	}
	if f.NoModeration {
		where = append(where, "a.action NOT LIKE 'moderation.%'", "a.target_type <> 'moderation_case'")
	}
	if f.After != nil {
		at, err := time.Parse(time.RFC3339Nano, f.After.Key)
		if err != nil {
			return nil, fmt.Errorf("audit read: cursor: %w", err)
		}
		where = append(where, "(a.occurred_at, a.audit_id) < ("+arg(at)+"::timestamptz, "+arg(f.After.ID)+"::uuid)")
	}
	limit := arg(f.Limit + 1)
	// tenancyguard:ok explicit tenant condition (session tenant, CrossTenantScope tenant_ids or platform rows) and time range; append-only table
	q := `SELECT a.audit_id, a.tenant_id, a.school_id, a.actor_id,
			CASE WHEN a.actor_type = 'user' THEN coalesce(u.user_type, 'staff') ELSE a.actor_type END,
			a.action, a.target_type, a.target_id, a.result, coalesce(a.reason, ''), a.metadata, coalesce(a.trace_id, ''), a.occurred_at
		FROM audit.audit_logs a
		LEFT JOIN identity.users u ON u.user_id = a.actor_id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY a.occurred_at DESC, a.audit_id DESC LIMIT ` + limit
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		rows, err := tx.QueryContext(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("audit read: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var (
				e                     dto.AuditLogEntry
				tenant, school, actor uuid.NullUUID
				meta                  []byte
			)
			if err := rows.Scan(&e.AuditID, &tenant, &school, &actor, &e.ActorType, &e.Action, &e.TargetType, &e.TargetID, &e.Result,
				&e.Reason, &meta, &e.TraceID, &e.OccurredAt); err != nil {
				return fmt.Errorf("audit read: %w", err)
			}
			e.TenantID, e.SchoolID, e.ActorID = nullable(tenant), nullable(school), nullable(actor)
			if len(meta) > 2 {
				_ = json.Unmarshal(meta, &e.Changes)
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

func nullable(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	v := n.UUID
	return &v
}
