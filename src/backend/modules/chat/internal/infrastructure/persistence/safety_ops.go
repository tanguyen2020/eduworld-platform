package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
)

// SafetyOps implements the on-call shifts and the break-glass grants of the Platform Safety Team (F124) with explicit SQL:
// chat.moderation_oncall_shifts is platform data (G, no tenant: deleted_at condition only); chat.break_glass_grants is
// tenant data (tenant_id + deleted_at in every statement, R27). Cross-tenant statements (grant lookup and lists, the
// messages of a grant, job work lists) run only under tenancy.Bypass on the chat_platform pool; callers decide when.
// Pages are read with LIMIT n+1 in SQL (keyset), never sliced in memory.
type SafetyOps struct{}

var (
	_ write.OncallStore     = SafetyOps{}
	_ write.BreakGlassStore = SafetyOps{}
	_ read.OncallReader     = SafetyOps{}
	_ read.BreakGlassReader = SafetyOps{}
)

// ---------- on-call shifts ----------

const shiftCols = `s.shift_id, s.moderator_user_id, s.oncall_role, s.starts_at, s.ends_at, s.note, s.created_by, s.created_at, s.version`

func scanShift(sc scanner) (*read.OncallShiftRow, error) {
	var r read.OncallShiftRow
	var note sql.NullString
	var by uuid.NullUUID
	if err := sc.Scan(&r.ID, &r.Moderator, &r.Role, &r.Start, &r.End, &note, &by, &r.CreatedAt, &r.Version); err != nil {
		return nil, err
	}
	r.Note, r.CreatedBy = nullString(note), nullUUID(by)
	r.Start, r.End, r.CreatedAt = r.Start.UTC(), r.End.UTC(), r.CreatedAt.UTC()
	return &r, nil
}

func oneShift(ctx context.Context, q string, args ...any) (*read.OncallShiftRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	r, err := scanShift(t.QueryRowContext(ctx, q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: oncall shift: %w", err)
	}
	return r, nil
}

// LockRole implements write.OncallStore.
func (SafetyOps) LockRole(ctx context.Context, role string) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('chat:oncall:' || $1::text))`, role); err != nil {
		return fmt.Errorf("chat: oncall lock: %w", err)
	}
	return nil
}

// Overlapping implements write.OncallStore.
func (SafetyOps) Overlapping(ctx context.Context, role string, start, end time.Time) (bool, error) {
	// tenancyguard:ok chat.moderation_oncall_shifts is platform data (G, no tenant_id); deleted_at IS NULL
	n, err := count(ctx, `SELECT count(*) FROM chat.moderation_oncall_shifts s WHERE s.oncall_role = $1 AND s.deleted_at IS NULL
		AND s.starts_at < $3 AND s.ends_at > $2`, role, start, end)
	return n > 0, err
}

// InsertShift implements write.OncallStore.
func (SafetyOps) InsertShift(ctx context.Context, s read.OncallShiftRow) error {
	// tenancyguard:ok chat.moderation_oncall_shifts is platform data (G, no tenant_id)
	_, err := exec(ctx, `INSERT INTO chat.moderation_oncall_shifts (shift_id, created_at, created_by, updated_at, updated_by, version,
		moderator_user_id, oncall_role, starts_at, ends_at, note) VALUES ($1, $2, $3, $2, $3, 1, $4, $5, $6, $7, $8)`,
		s.ID, s.CreatedAt, s.CreatedBy, s.Moderator, s.Role, s.Start, s.End, s.Note)
	if err != nil {
		return fmt.Errorf("chat: insert oncall shift: %w", err)
	}
	return nil
}

// LockShift implements write.OncallStore.
func (SafetyOps) LockShift(ctx context.Context, id uuid.UUID) (*read.OncallShiftRow, error) {
	// tenancyguard:ok chat.moderation_oncall_shifts is platform data (G, no tenant_id); deleted_at IS NULL
	return oneShift(ctx, `SELECT `+shiftCols+` FROM chat.moderation_oncall_shifts s WHERE s.shift_id = $1 AND s.deleted_at IS NULL FOR UPDATE`, id)
}

// SoftDeleteShift implements write.OncallStore (R21: deleted_at/deleted_by, never DELETE).
func (SafetyOps) SoftDeleteShift(ctx context.Context, id, by uuid.UUID, at time.Time) error {
	// tenancyguard:ok chat.moderation_oncall_shifts is platform data (G, no tenant_id); soft-delete of one live row
	n, err := exec(ctx, `UPDATE chat.moderation_oncall_shifts SET deleted_at = $2, deleted_by = $3, updated_at = $2, updated_by = $3,
		version = version + 1 WHERE shift_id = $1 AND deleted_at IS NULL`, id, at, by)
	if err != nil {
		return fmt.Errorf("chat: delete oncall shift: %w", err)
	}
	if n == 0 {
		return write.ErrStale
	}
	return nil
}

// CurrentShift implements write.OncallStore.
func (SafetyOps) CurrentShift(ctx context.Context, role string, at time.Time) (*read.OncallShiftRow, error) {
	// tenancyguard:ok chat.moderation_oncall_shifts is platform data (G, no tenant_id); deleted_at IS NULL
	return oneShift(ctx, `SELECT `+shiftCols+` FROM chat.moderation_oncall_shifts s WHERE s.oncall_role = $1 AND s.deleted_at IS NULL
		AND s.starts_at <= $2 AND s.ends_at > $2 ORDER BY s.starts_at, s.shift_id LIMIT 1`, role, at)
}

// ListShifts implements read.OncallReader.
func (SafetyOps) ListShifts(ctx context.Context, f read.ShiftFilter) ([]read.OncallShiftRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"s.deleted_at IS NULL"}, []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if f.From != nil {
		where = append(where, "s.ends_at > "+arg(*f.From))
	}
	if f.To != nil {
		where = append(where, "s.starts_at < "+arg(*f.To))
	}
	if f.Role != "" {
		where = append(where, "s.oncall_role = "+arg(f.Role))
	}
	if f.AfterStart != nil && f.AfterID != nil {
		where = append(where, "(s.starts_at, s.shift_id) > ("+arg(*f.AfterStart)+", "+arg(*f.AfterID)+")")
	}
	// tenancyguard:ok chat.moderation_oncall_shifts is platform data (G, no tenant_id); deleted_at IS NULL; keyset page
	rows, err := t.QueryContext(ctx, `SELECT `+shiftCols+` FROM chat.moderation_oncall_shifts s WHERE `+strings.Join(where, " AND ")+
		` ORDER BY s.starts_at, s.shift_id LIMIT `+arg(f.Limit+1), args...)
	if err != nil {
		return nil, fmt.Errorf("chat: oncall shifts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.OncallShiftRow{}
	for rows.Next() {
		r, err := scanShift(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: oncall shifts: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// UnclaimedP1 implements write.OncallStore.
func (SafetyOps) UnclaimedP1(ctx context.Context, limit int) ([]read.CaseRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok cross-tenant work list of oncall-escalator under tenancy.Bypass(platform_job); deleted_at IS NULL
	rows, err := t.QueryContext(ctx, `SELECT `+caseCols+` FROM chat.moderation_cases c WHERE c.status = 'open' AND c.severity = 'p1'
		AND c.deleted_at IS NULL AND (c.oncall_page_level IS NULL OR c.oncall_page_level <> 'system_admin')
		ORDER BY c.created_at LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: unclaimed p1: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.CaseRow
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: unclaimed p1: %w", err)
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// ---------- break-glass grants ----------

const grantCols = `g.grant_id, g.tenant_id, g.scope_type, g.scope_id, g.window_from, g.window_to, g.duration_minutes, g.reason, g.status,
	g.requested_by, g.requested_at, g.decided_by, g.decided_at, g.decision_note, g.expires_at, g.revoked_by, g.revoked_at, g.revoke_reason,
	g.messages_viewed_count, g.version`

func scanGrant(sc scanner) (*read.GrantRow, error) {
	var g read.GrantRow
	var decidedBy, revokedBy uuid.NullUUID
	var decidedAt, expires, revokedAt sql.NullTime
	var note, revokeReason sql.NullString
	if err := sc.Scan(&g.ID, &g.TenantID, &g.ScopeType, &g.ScopeID, &g.WindowFrom, &g.WindowTo, &g.DurationMinutes, &g.Reason, &g.Status,
		&g.RequestedBy, &g.RequestedAt, &decidedBy, &decidedAt, &note, &expires, &revokedBy, &revokedAt, &revokeReason,
		&g.MessagesViewedCount, &g.Version); err != nil {
		return nil, err
	}
	g.DecidedBy, g.DecidedAt, g.DecisionNote, g.ExpiresAt = nullUUID(decidedBy), nullTime(decidedAt), nullString(note), nullTime(expires)
	g.RevokedBy, g.RevokedAt, g.RevokeReason = nullUUID(revokedBy), nullTime(revokedAt), nullString(revokeReason)
	g.WindowFrom, g.WindowTo, g.RequestedAt = g.WindowFrom.UTC(), g.WindowTo.UTC(), g.RequestedAt.UTC()
	return &g, nil
}

func grants(ctx context.Context, q string, args ...any) ([]read.GrantRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: break-glass grants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.GrantRow{}
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: break-glass grants: %w", err)
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

func oneGrant(ctx context.Context, q string, args ...any) (*read.GrantRow, error) {
	list, err := grants(ctx, q, args...)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// InsertGrant implements write.BreakGlassStore.
func (SafetyOps) InsertGrant(ctx context.Context, g read.GrantRow) error {
	_, err := exec(ctx, `INSERT INTO chat.break_glass_grants (grant_id, tenant_id, created_at, created_by, updated_at, updated_by, version,
		scope_type, scope_id, window_from, window_to, duration_minutes, reason, status, requested_by, requested_at)
		VALUES ($1, $2, $3, $4, $3, $4, 1, $5, $6, $7, $8, $9, $10, $11, $4, $3)`,
		g.ID, g.TenantID, g.RequestedAt, g.RequestedBy, g.ScopeType, g.ScopeID, g.WindowFrom, g.WindowTo, g.DurationMinutes, g.Reason, g.Status)
	if err != nil {
		return fmt.Errorf("chat: insert break-glass grant: %w", err)
	}
	return nil
}

// LockGrant implements write.BreakGlassStore.
func (SafetyOps) LockGrant(ctx context.Context, tenant, id uuid.UUID) (*read.GrantRow, error) {
	return oneGrant(ctx, `SELECT `+grantCols+` FROM chat.break_glass_grants g WHERE g.tenant_id = $1 AND g.grant_id = $2
		AND g.deleted_at IS NULL FOR UPDATE`, tenant, id)
}

// UpdateGrant implements write.BreakGlassStore (optimistic: version = previous + 1).
func (SafetyOps) UpdateGrant(ctx context.Context, g read.GrantRow, actor *uuid.UUID) error {
	n, err := exec(ctx, `UPDATE chat.break_glass_grants SET updated_at = now(), updated_by = $3, version = $4, status = $5, decided_by = $6,
		decided_at = $7, decision_note = $8, expires_at = $9, revoked_by = $10, revoked_at = $11, revoke_reason = $12
		WHERE tenant_id = $1 AND grant_id = $2 AND version = $4 - 1 AND deleted_at IS NULL`,
		g.TenantID, g.ID, actor, g.Version, g.Status, g.DecidedBy, g.DecidedAt, g.DecisionNote, g.ExpiresAt, g.RevokedBy, g.RevokedAt,
		g.RevokeReason)
	if err != nil {
		return fmt.Errorf("chat: update break-glass grant: %w", err)
	}
	if n == 0 {
		return write.ErrStale
	}
	return nil
}

// AddViewed implements write.BreakGlassStore.
func (SafetyOps) AddViewed(ctx context.Context, tenant, id uuid.UUID, n int) error {
	_, err := exec(ctx, `UPDATE chat.break_glass_grants SET messages_viewed_count = messages_viewed_count + $3, updated_at = now()
		WHERE tenant_id = $1 AND grant_id = $2 AND deleted_at IS NULL`, tenant, id, n)
	if err != nil {
		return fmt.Errorf("chat: break-glass viewed: %w", err)
	}
	return nil
}

// ExpiredGrants implements write.BreakGlassStore.
func (SafetyOps) ExpiredGrants(ctx context.Context, now time.Time, limit int) ([]read.GrantRow, error) {
	// tenancyguard:ok cross-tenant work list of break-glass-expirer under tenancy.Bypass(platform_job); deleted_at IS NULL
	return grants(ctx, `SELECT `+grantCols+` FROM chat.break_glass_grants g WHERE g.status = 'approved' AND g.expires_at <= $1
		AND g.deleted_at IS NULL ORDER BY g.expires_at LIMIT $2`, now, limit)
}

// ListGrants implements read.BreakGlassReader.
func (SafetyOps) ListGrants(ctx context.Context, f read.GrantFilter) ([]read.GrantRow, error) {
	where, args := []string{"g.deleted_at IS NULL"}, []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if len(f.TenantIDs) > 0 {
		where = append(where, "g.tenant_id = ANY("+arg(idStrings(f.TenantIDs))+"::uuid[])")
	}
	if f.Status != "" {
		where = append(where, "g.status = "+arg(f.Status))
	}
	if f.RequestedBy != nil {
		where = append(where, "g.requested_by = "+arg(*f.RequestedBy))
	}
	if f.BeforeAt != nil && f.BeforeID != nil {
		where = append(where, "(g.requested_at, g.grant_id) < ("+arg(*f.BeforeAt)+", "+arg(*f.BeforeID)+")")
	}
	// tenancyguard:ok cross-tenant list of the System Admin under tenancy.Bypass(break_glass); tenant filter tenant_ids; deleted_at IS NULL
	return grants(ctx, `SELECT `+grantCols+` FROM chat.break_glass_grants g WHERE `+strings.Join(where, " AND ")+
		` ORDER BY g.requested_at DESC, g.grant_id DESC LIMIT `+arg(f.Limit+1), args...)
}

// Grant implements read.BreakGlassReader.
func (SafetyOps) Grant(ctx context.Context, id uuid.UUID) (*read.GrantRow, error) {
	// tenancyguard:ok one grant of any tenant under tenancy.Bypass(break_glass); deleted_at IS NULL
	return oneGrant(ctx, `SELECT `+grantCols+` FROM chat.break_glass_grants g WHERE g.grant_id = $1 AND g.deleted_at IS NULL`, id)
}

// breakGlassSQL reads the messages of one table in the scope of a grant: tenant, channel or sender, [from, to), keyset.
// Purged gameplay rows no longer exist (ADR-0033) and are never restored from evidence (F124-AC-22).
const breakGlassSQL = `SELECT m.message_id, m.channel_id, m.seq, m.sender_id, m.school_id, m.kind, m.body, m.body_display,
		m.quick_message_id, m.revision, m.moderation_state, m.recalled_at, m.created_at
	FROM chat.{table} m WHERE m.tenant_id = $1 AND m.{scope} = $2 AND m.created_at >= $3 AND m.created_at < $4 AND m.deleted_at IS NULL
		AND ($5::timestamptz IS NULL OR (m.created_at, m.message_id) > ($5::timestamptz, $6::uuid))`

// Messages implements read.BreakGlassReader: both message tables, one keyset order (created_at, message_id), LIMIT n+1.
func (SafetyOps) Messages(ctx context.Context, f read.MessageFilter) ([]read.BreakGlassMessage, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	scope := "channel_id"
	if f.ScopeType == "user" {
		scope = "sender_id"
	}
	part := func(table string) string {
		return strings.NewReplacer("{table}", table, "{scope}", scope).Replace(breakGlassSQL)
	}
	var afterAt any
	var afterID any
	if f.AfterAt != nil && f.AfterID != nil {
		afterAt, afterID = *f.AfterAt, *f.AfterID
	}
	// tenancyguard:ok scope of an approved break-glass grant under tenancy.Bypass(break_glass): explicit tenant, deleted_at IS NULL
	rows, err := t.QueryContext(ctx, `SELECT * FROM (`+part("messages")+` UNION ALL `+part("gameplay_messages")+`) x
		ORDER BY x.created_at, x.message_id LIMIT $7`, f.TenantID, f.ScopeID, f.From, f.To, afterAt, afterID, f.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("chat: break-glass messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.BreakGlassMessage{}
	for rows.Next() {
		var m read.BreakGlassMessage
		var school uuid.NullUUID
		var body, display, quick sql.NullString
		var recalled sql.NullTime
		if err := rows.Scan(&m.MessageID, &m.ChannelID, &m.Seq, &m.SenderID, &school, &m.Kind, &body, &display, &quick, &m.Revision,
			&m.ModerationState, &recalled, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: break-glass messages: %w", err)
		}
		m.SenderSchoolID, m.Body, m.BodyDisplay, m.QuickMessageID = nullUUID(school), nullString(body), nullString(display), nullString(quick)
		m.RecalledAt, m.CreatedAt = nullTime(recalled), m.CreatedAt.UTC()
		out = append(out, m)
	}
	return out, rows.Err()
}
