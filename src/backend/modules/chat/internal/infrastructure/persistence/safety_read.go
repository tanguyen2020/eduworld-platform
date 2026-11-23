package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// tenantFilter renders "alias.tenant_id = ANY($n)" when ids are given (the filter of a cross-tenant read under
// tenancy.Bypass(moderation_case)), "TRUE" otherwise; a tenant session sees its own tenant through RLS anyway.
func tenantFilter(alias string, ids []uuid.UUID, args *[]any) string {
	if len(ids) == 0 {
		return "TRUE"
	}
	*args = append(*args, idStrings(ids))
	return fmt.Sprintf("%s.tenant_id = ANY($%d::uuid[])", alias, len(*args))
}

func arg(args *[]any, v any) string {
	*args = append(*args, v)
	return fmt.Sprintf("$%d", len(*args))
}

// Cases implements read.SafetyReader: the queue, most urgent first (severity, sla_due_at, case_id).
func (Safety) Cases(ctx context.Context, f read.CaseFilter) ([]read.CaseRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	args := []any{}
	where := []string{"c.deleted_at IS NULL", tenantFilter("c", f.TenantIDs, &args)}
	if f.Status != "" {
		where = append(where, "c.status = "+arg(&args, f.Status))
	}
	if f.Severity != "" {
		where = append(where, "c.severity = "+arg(&args, f.Severity))
	}
	switch f.Assignee {
	case "me":
		where = append(where, "c.assignee_moderator_id = "+arg(&args, f.Me))
	case "unassigned":
		where = append(where, "c.assignee_moderator_id IS NULL")
	}
	if f.Category != "" {
		where = append(where, "c.category_code = "+arg(&args, f.Category))
	}
	if f.After != nil {
		where = append(where, fmt.Sprintf("(c.severity, c.sla_due_at, c.case_id) > (%s, %s, %s)", arg(&args, f.After.Severity),
			arg(&args, f.After.Due), arg(&args, f.After.ID)))
	}
	limit := arg(&args, f.Limit)
	// tenancyguard:ok queue of the Safety Team: tenant_ids filter (tenant_id = ANY) under tenancy.Bypass(moderation_case), deleted_at IS NULL
	q := `SELECT ` + caseCols + ` FROM chat.moderation_cases c WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY c.severity, c.sla_due_at, c.case_id LIMIT ` + limit
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: cases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.CaseRow{}
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: cases: %w", err)
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// ActionsOf implements read.SafetyReader.
func (Safety) ActionsOf(ctx context.Context, tenant, id uuid.UUID, limit int) ([]read.ActionRow, error) {
	return caseActions(ctx, tenant, id, limit)
}

// ReportsOf implements read.SafetyReader.
func (Safety) ReportsOf(ctx context.Context, tenant, id uuid.UUID, limit int) ([]read.ReportRow, error) {
	return caseReports(ctx, tenant, id, limit)
}

// EvidenceOf implements read.SafetyReader.
func (s Safety) EvidenceOf(ctx context.Context, tenant, id uuid.UUID) ([]read.EvidenceRow, error) {
	return s.CaseEvidence(ctx, tenant, id)
}

// Sanctions implements read.SafetyReader: newest first.
func (Safety) Sanctions(ctx context.Context, f read.SanctionFilter) ([]read.SanctionDetail, error) {
	args := []any{}
	where := []string{"s.deleted_at IS NULL", tenantFilter("s", f.TenantIDs, &args)}
	if f.UserID != nil {
		where = append(where, "s.user_id = "+arg(&args, *f.UserID))
	}
	if f.Active != nil {
		now := arg(&args, f.Now)
		active := fmt.Sprintf("(s.revoked_at IS NULL AND s.lifted_at IS NULL AND (s.ends_at IS NULL OR s.ends_at > %s))", now)
		if *f.Active {
			where = append(where, active)
		} else {
			where = append(where, "NOT "+active)
		}
	}
	if f.After != nil {
		where = append(where, fmt.Sprintf("(s.created_at, s.sanction_id) < (%s, %s)", arg(&args, f.After.At), arg(&args, f.After.ID)))
	}
	limit := arg(&args, f.Limit)
	// tenancyguard:ok sanctions of the Safety Team: tenant_ids filter (tenant_id = ANY) under tenancy.Bypass(moderation_case), deleted_at IS NULL
	return sanctions(ctx, `SELECT `+sanctionCols+` FROM chat.sanctions s WHERE `+strings.Join(where, " AND ")+
		` ORDER BY s.created_at DESC, s.sanction_id DESC LIMIT `+limit, args...)
}

// Notifications implements read.SafetyReader: newest first.
func (Safety) Notifications(ctx context.Context, f read.NotificationFilter) ([]read.NotificationRow, error) {
	args := []any{}
	where := []string{"n.deleted_at IS NULL", tenantFilter("n", f.TenantIDs, &args)}
	if f.SchoolIDs != nil {
		where = append(where, fmt.Sprintf("n.school_id = ANY(%s::uuid[])", arg(&args, idStrings(f.SchoolIDs))))
	}
	if f.Acknowledged != nil {
		if *f.Acknowledged {
			where = append(where, "n.school_ack_at IS NOT NULL")
		} else {
			where = append(where, "n.school_ack_at IS NULL")
		}
	}
	if f.Overdue != nil {
		cond := fmt.Sprintf("(n.school_ack_at IS NULL AND n.ack_due_at <= %s)", arg(&args, f.Now))
		if *f.Overdue {
			where = append(where, cond)
		} else {
			where = append(where, "NOT "+cond)
		}
	}
	if f.CreatedFrom != nil {
		where = append(where, "n.created_at >= "+arg(&args, f.CreatedFrom.UTC()))
	}
	if f.After != nil {
		where = append(where, fmt.Sprintf("(n.created_at, n.notification_id) < (%s, %s)", arg(&args, f.After.At), arg(&args, f.After.ID)))
	}
	limit := arg(&args, f.Limit)
	// tenancyguard:ok school notifications: tenant_ids filter (tenant_id = ANY, or the tenant session) and deleted_at IS NULL
	return notifications(ctx, `SELECT `+notificationCols+` FROM chat.school_safety_notifications n WHERE `+strings.Join(where, " AND ")+
		` ORDER BY n.created_at DESC, n.notification_id DESC LIMIT `+limit, args...)
}

// CaseTenants implements read.SafetyReader: the tenants having live cases, tenant_id order (keyset), with the name
// projected into ref.tenants (code until a name is known).
func (Safety) CaseTenants(ctx context.Context, after *uuid.UUID, limit int) ([]read.CaseTenant, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	from := uuid.Nil
	if after != nil {
		from = *after
	}
	// tenancyguard:ok tenants of the cases of every tenant under tenancy.Bypass(moderation_case) (ADR-0072): ids and names only, deleted_at IS NULL
	rows, err := t.QueryContext(ctx, `SELECT d.tenant_id, rt.name_or_code, rt.tenant_type
		FROM (SELECT DISTINCT c.tenant_id FROM chat.moderation_cases c WHERE c.deleted_at IS NULL AND c.tenant_id > $1) d
		LEFT JOIN LATERAL (SELECT coalesce(x.name, x.code) AS name_or_code, x.tenant_type FROM ref.tenants x WHERE x.tenant_id = d.tenant_id) rt ON true
		ORDER BY d.tenant_id LIMIT $2`, from, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: case tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.CaseTenant{}
	for rows.Next() {
		var ct read.CaseTenant
		var name, typ sql.NullString
		if err := rows.Scan(&ct.TenantID, &name, &typ); err != nil {
			return nil, fmt.Errorf("chat: case tenants: %w", err)
		}
		ct.Name, ct.Type = nullString(name), nullString(typ)
		out = append(out, ct)
	}
	return out, rows.Err()
}

// OpenCounts implements read.SafetyReader (every tenant, platform job).
func (Safety) OpenCounts(ctx context.Context) ([]read.OpenCount, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok gauge of the queue across tenants under tenancy.Bypass(platform_job): counts only, deleted_at IS NULL
	rows, err := t.QueryContext(ctx, `SELECT c.severity, c.assignee_moderator_id IS NOT NULL, count(*),
		min(c.created_at) FILTER (WHERE c.assignee_moderator_id IS NULL AND c.status = 'open')
		FROM chat.moderation_cases c WHERE c.deleted_at IS NULL AND c.status <> 'resolved'
		GROUP BY c.severity, c.assignee_moderator_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("chat: open cases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.OpenCount
	for rows.Next() {
		var o read.OpenCount
		var oldest *time.Time
		if err := rows.Scan(&o.Severity, &o.Assigned, &o.Count, &oldest); err != nil {
			return nil, fmt.Errorf("chat: open cases: %w", err)
		}
		if oldest != nil {
			o.OldestUnclaimed = oldest.UTC()
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// OverdueNotifications implements read.SafetyReader (every tenant, platform job).
func (Safety) OverdueNotifications(ctx context.Context, now time.Time) (int, error) {
	t, err := readTx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok gauge across tenants under tenancy.Bypass(platform_job): a count only, deleted_at IS NULL
	if err := t.QueryRowContext(ctx, `SELECT count(*) FROM chat.school_safety_notifications n WHERE n.deleted_at IS NULL
		AND n.school_ack_at IS NULL AND n.ack_due_at <= $1`, now).Scan(&n); err != nil {
		return 0, fmt.Errorf("chat: overdue notifications: %w", err)
	}
	return n, nil
}
