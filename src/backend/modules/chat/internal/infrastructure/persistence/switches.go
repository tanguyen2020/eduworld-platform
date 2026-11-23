package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
)

// Switches implements write.SwitchRepository and read.SwitchAdminReader on chat.moderation_switches,
// chat.moderation_switch_requests and chat.moderation_switch_changes (F116 US3): explicit SQL with tenant and
// deleted_at conditions. The platform row (tenant_id NULL) is written in the platform scope (chat_platform); a tenant
// row in the scope of that tenant.
type Switches struct{}

var (
	_ write.SwitchRepository = Switches{}
	_ read.SwitchAdminReader = Switches{}
)

// stepsJSON stores the switchable steps only (a legacy pii key is never written back, ADR-0078).
func stepsJSON(s moderation.Switch) string {
	m := map[string]bool{}
	for _, st := range moderation.SwitchableSteps {
		v, ok := s.Steps[st]
		m[st] = !ok || v
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func parseSteps(raw []byte) (map[string]bool, error) {
	m := map[string]bool{}
	if len(raw) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("chat: switch steps: %w", err)
	}
	return m, nil
}

// ---------- switches

// LockSwitch implements write.SwitchRepository.
func (Switches) LockSwitch(ctx context.Context, tenant *uuid.UUID) (*read.SwitchRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var row *sql.Row
	if tenant == nil {
		// tenancyguard:ok the platform row (tenant_id NULL, platform scope), live only
		row = t.QueryRowContext(ctx, `SELECT `+switchColumns+` FROM chat.moderation_switches s
			WHERE s.tenant_id IS NULL AND s.deleted_at IS NULL FOR UPDATE`)
	} else {
		row = t.QueryRowContext(ctx, `SELECT `+switchColumns+` FROM chat.moderation_switches s
			WHERE s.tenant_id = $1 AND s.deleted_at IS NULL FOR UPDATE`, *tenant)
	}
	r, err := scanSwitch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: lock switch: %w", err)
	}
	return r, nil
}

// SaveSwitch implements write.SwitchRepository.
func (Switches) SaveSwitch(ctx context.Context, tenant *uuid.UUID, cur *read.SwitchRow, next moderation.Switch, reason *string,
	actor *uuid.UUID, now time.Time) (*read.SwitchRow, error) {
	var exp any
	if next.ExpiresAt != nil {
		exp = next.ExpiresAt.UTC()
	}
	if cur == nil {
		scope := "platform"
		if tenant != nil {
			scope = "tenant"
		}
		id := uuid.Must(uuid.NewV7())
		_, err := exec(ctx, `INSERT INTO chat.moderation_switches (switch_id, tenant_id, created_at, created_by, updated_at, updated_by,
			scope, enabled_all, steps, reason, expires_at) VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $7::jsonb, $8, $9)`,
			id, tenant, now, actor, scope, next.EnabledAll, stepsJSON(next), reason, exp)
		if err != nil {
			return nil, dup(err, "insert switch")
		}
		steps, _ := parseSteps([]byte(stepsJSON(next)))
		return &read.SwitchRow{ID: id, TenantID: tenant, EnabledAll: next.EnabledAll, Steps: steps, Reason: reason,
			ExpiresAt: next.ExpiresAt, UpdatedBy: actor, UpdatedAt: now, Version: 1}, nil
	}
	n, err := exec(ctx, `UPDATE chat.moderation_switches SET enabled_all = $2, steps = $3::jsonb, reason = $4, expires_at = $5,
		version = version + 1, updated_at = $6, updated_by = $7
		WHERE switch_id = $1 AND tenant_id IS NOT DISTINCT FROM $8 AND deleted_at IS NULL AND version = $9`,
		cur.ID, next.EnabledAll, stepsJSON(next), reason, exp, now, actor, tenant, cur.Version)
	if err != nil {
		return nil, fmt.Errorf("chat: update switch: %w", err)
	}
	if n == 0 {
		return nil, write.ErrStale
	}
	steps, _ := parseSteps([]byte(stepsJSON(next)))
	return &read.SwitchRow{ID: cur.ID, TenantID: tenant, EnabledAll: next.EnabledAll, Steps: steps, Reason: reason,
		ExpiresAt: next.ExpiresAt, UpdatedBy: actor, UpdatedAt: now, Version: cur.Version + 1}, nil
}

// InsertChange implements write.SwitchRepository (append-only history).
func (Switches) InsertChange(ctx context.Context, c write.SwitchChangeRecord) error {
	scope := "platform"
	if c.TenantID != nil {
		scope = "tenant"
	}
	var exp any
	if c.ExpiresAt != nil {
		exp = c.ExpiresAt.UTC()
	}
	_, err := exec(ctx, `INSERT INTO chat.moderation_switch_changes (change_id, tenant_id, created_at, created_by, scope,
		enabled_all_before, enabled_all_after, before, after, reason, expires_at, actor_id, approved_by, request_id, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11, $4, $12, $13, $14)`,
		c.ID, c.TenantID, c.At, c.ActorID, scope, c.Before.EnabledAll, c.After.EnabledAll, stepsJSON(c.Before), stepsJSON(c.After),
		c.Reason, exp, c.ApprovedBy, c.RequestID, c.Source)
	if err != nil {
		return fmt.Errorf("chat: insert switch change: %w", err)
	}
	return nil
}

// ---------- requests

const requestColumns = `r.request_id, r.tenant_id, r.enabled_all, r.steps, r.reason, r.switch_expires_at, r.status, r.proposed_by,
	r.approved_by, r.created_at, r.decided_at, r.request_expires_at, r.version`

func scanRequest(sc interface{ Scan(...any) error }) (read.SwitchRequestRow, error) {
	var r read.SwitchRequestRow
	var tenant, by uuid.NullUUID
	var steps []byte
	var decided sql.NullTime
	if err := sc.Scan(&r.ID, &tenant, &r.EnabledAll, &steps, &r.Reason, &r.ExpiresAt, &r.Status, &r.ProposedBy, &by, &r.CreatedAt,
		&decided, &r.RequestExpiresAt, &r.Version); err != nil {
		return r, err
	}
	r.TenantID, r.ApprovedBy, r.DecidedAt = nullUUID(tenant), nullUUID(by), nullTime(decided)
	r.ExpiresAt, r.CreatedAt, r.RequestExpiresAt = r.ExpiresAt.UTC(), r.CreatedAt.UTC(), r.RequestExpiresAt.UTC()
	var err error
	r.Steps, err = parseSteps(steps)
	return r, err
}

// InsertRequest implements write.SwitchRepository.
func (Switches) InsertRequest(ctx context.Context, r read.SwitchRequestRow) error {
	scope := "platform"
	if r.TenantID != nil {
		scope = "tenant"
	}
	_, err := exec(ctx, `INSERT INTO chat.moderation_switch_requests (request_id, tenant_id, created_at, created_by, updated_at, updated_by,
		scope, enabled_all, steps, reason, switch_expires_at, status, proposed_by, request_expires_at)
		VALUES ($1, $2, $3, $4, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $4, $11)`,
		r.ID, r.TenantID, r.CreatedAt, r.ProposedBy, scope, r.EnabledAll,
		stepsJSON(moderation.Switch{EnabledAll: r.EnabledAll, Steps: r.Steps}), r.Reason, r.ExpiresAt.UTC(), r.Status,
		r.RequestExpiresAt.UTC())
	if err != nil {
		return fmt.Errorf("chat: insert switch request: %w", err)
	}
	return nil
}

// LockRequest implements write.SwitchRepository.
func (Switches) LockRequest(ctx context.Context, tenant *uuid.UUID, id uuid.UUID) (*read.SwitchRequestRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var row *sql.Row
	if tenant == nil {
		// tenancyguard:ok a platform request (tenant_id NULL, platform scope), live only
		row = t.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM chat.moderation_switch_requests r
			WHERE r.request_id = $1 AND r.tenant_id IS NULL AND r.deleted_at IS NULL FOR UPDATE`, id)
	} else {
		row = t.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM chat.moderation_switch_requests r
			WHERE r.request_id = $1 AND r.tenant_id = $2 AND r.deleted_at IS NULL FOR UPDATE`, id, *tenant)
	}
	r, err := scanRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: lock switch request: %w", err)
	}
	return &r, nil
}

// DecideRequest implements write.SwitchRepository.
func (Switches) DecideRequest(ctx context.Context, id uuid.UUID, status string, approver *uuid.UUID, now time.Time) error {
	// tenancyguard:ok the row was locked by LockRequest in the scope of its level (RLS for a tenant session)
	n, err := exec(ctx, `UPDATE chat.moderation_switch_requests SET status = $2, approved_by = $3, decided_at = $4, updated_at = $4,
		updated_by = $3, version = version + 1 WHERE request_id = $1 AND status = 'pending_approval' AND deleted_at IS NULL`,
		id, status, approver, now)
	if err != nil {
		return fmt.Errorf("chat: decide switch request: %w", err)
	}
	if n == 0 {
		return write.ErrStale
	}
	return nil
}

// RejectRequest implements write.SwitchRepository.
func (Switches) RejectRequest(ctx context.Context, id, by uuid.UUID, version int64, now time.Time) error {
	// tenancyguard:ok the row was locked by LockRequest in the scope of its level (RLS for a tenant session)
	n, err := exec(ctx, `UPDATE chat.moderation_switch_requests SET status = 'rejected', decided_at = $3, updated_at = $3, updated_by = $2,
		version = version + 1 WHERE request_id = $1 AND status = 'pending_approval' AND version = $4 AND deleted_at IS NULL`,
		id, by, now, version)
	if err != nil {
		return fmt.Errorf("chat: reject switch request: %w", err)
	}
	if n == 0 {
		return write.ErrStale
	}
	return nil
}

// ExpiredSwitches implements write.SwitchRepository.
func (Switches) ExpiredSwitches(ctx context.Context, now time.Time, limit int) ([]write.SwitchLevel, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok system job moderation-switch-expirer (system platform scope): the lapsed switches of every level
	rows, err := t.QueryContext(ctx, `SELECT s.tenant_id FROM chat.moderation_switches s
		WHERE s.deleted_at IS NULL AND s.expires_at IS NOT NULL AND s.expires_at <= $1 ORDER BY s.expires_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: expired switches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.SwitchLevel
	for rows.Next() {
		var tenant uuid.NullUUID
		if err := rows.Scan(&tenant); err != nil {
			return nil, fmt.Errorf("chat: expired switches: %w", err)
		}
		out = append(out, write.SwitchLevel{TenantID: nullUUID(tenant)})
	}
	return out, rows.Err()
}

// ExpiredRequests implements write.SwitchRepository.
func (Switches) ExpiredRequests(ctx context.Context, now time.Time, limit int) ([]write.ExpiredRequest, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok system job moderation-switch-expirer (system platform scope): pending requests of every level
	rows, err := t.QueryContext(ctx, `SELECT r.request_id, r.tenant_id FROM chat.moderation_switch_requests r
		WHERE r.deleted_at IS NULL AND r.status = 'pending_approval' AND r.request_expires_at <= $1
		ORDER BY r.request_expires_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: expired switch requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ExpiredRequest
	for rows.Next() {
		var r write.ExpiredRequest
		var tenant uuid.NullUUID
		if err := rows.Scan(&r.ID, &tenant); err != nil {
			return nil, fmt.Errorf("chat: expired switch requests: %w", err)
		}
		r.TenantID = nullUUID(tenant)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------- read side

// TenantSwitches implements read.SwitchAdminReader.
func (Switches) TenantSwitches(ctx context.Context, tenants []uuid.UUID) (map[uuid.UUID]*read.SwitchRow, error) {
	out := map[uuid.UUID]*read.SwitchRow{}
	if len(tenants) == 0 {
		return out, nil
	}
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT `+switchColumns+` FROM chat.moderation_switches s
		WHERE s.tenant_id = ANY($1::uuid[]) AND s.deleted_at IS NULL`, idStrings(tenants))
	if err != nil {
		return nil, fmt.Errorf("chat: tenant switches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		r, err := scanSwitch(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: tenant switches: %w", err)
		}
		out[*r.TenantID] = r
	}
	return out, rows.Err()
}

// PendingRequests implements read.SwitchAdminReader.
func (Switches) PendingRequests(ctx context.Context, tenants []uuid.UUID, now time.Time, limit int) ([]read.SwitchRequestRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if tenants == nil {
		// tenancyguard:ok platform requests only (tenant_id NULL), live only
		rows, err = t.QueryContext(ctx, `SELECT `+requestColumns+` FROM chat.moderation_switch_requests r
			WHERE r.tenant_id IS NULL AND r.deleted_at IS NULL AND r.status = 'pending_approval' AND r.request_expires_at > $1
			ORDER BY r.created_at, r.request_id LIMIT $2`, now, limit)
	} else {
		rows, err = t.QueryContext(ctx, `SELECT `+requestColumns+` FROM chat.moderation_switch_requests r
			WHERE r.tenant_id = ANY($1::uuid[]) AND r.deleted_at IS NULL AND r.status = 'pending_approval' AND r.request_expires_at > $2
			ORDER BY r.created_at, r.request_id LIMIT $3`, idStrings(tenants), now, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("chat: pending switch requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SwitchRequestRow
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: pending switch requests: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// History implements read.SwitchAdminReader.
func (Switches) History(ctx context.Context, tenants []uuid.UUID, after *read.Cursor, limit int) ([]read.SwitchChangeRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), uuid.Max
	if after != nil {
		at, id = after.At, after.ID
	}
	const cols = `c.change_id, c.tenant_id, c.enabled_all_before, c.enabled_all_after, c.before, c.after, c.reason, c.expires_at,
		c.actor_id, c.approved_by, c.request_id, c.source, c.created_at`
	var rows *sql.Rows
	if tenants == nil {
		// tenancyguard:ok chat.moderation_switch_changes is append-only (no deleted_at); platform changes only
		rows, err = t.QueryContext(ctx, `SELECT `+cols+` FROM chat.moderation_switch_changes c
			WHERE c.tenant_id IS NULL AND (c.created_at, c.change_id) < ($1, $2) ORDER BY c.created_at DESC, c.change_id DESC LIMIT $3`,
			at, id, limit)
	} else {
		// tenancyguard:ok chat.moderation_switch_changes is append-only (no deleted_at); changes of the given tenants only
		rows, err = t.QueryContext(ctx, `SELECT `+cols+` FROM chat.moderation_switch_changes c
			WHERE c.tenant_id = ANY($1::uuid[]) AND (c.created_at, c.change_id) < ($2, $3)
			ORDER BY c.created_at DESC, c.change_id DESC LIMIT $4`, idStrings(tenants), at, id, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("chat: switch history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SwitchChangeRow
	for rows.Next() {
		var c read.SwitchChangeRow
		var tenant, actor, approver, req uuid.NullUUID
		var before, after []byte
		var reason sql.NullString
		var exp sql.NullTime
		if err := rows.Scan(&c.ID, &tenant, &c.EnabledAllBefore, &c.EnabledAllAfter, &before, &after, &reason, &exp, &actor, &approver,
			&req, &c.Source, &c.ChangedAt); err != nil {
			return nil, fmt.Errorf("chat: switch history: %w", err)
		}
		c.TenantID, c.ActorID, c.ApprovedBy, c.RequestID = nullUUID(tenant), nullUUID(actor), nullUUID(approver), nullUUID(req)
		c.Reason, c.ExpiresAt, c.ChangedAt = nullString(reason), nullTime(exp), c.ChangedAt.UTC()
		if c.Before, err = parseSteps(before); err != nil {
			return nil, err
		}
		if c.After, err = parseSteps(after); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DisabledTenantCount implements read.SwitchAdminReader.
func (Switches) DisabledTenantCount(ctx context.Context, now time.Time) (int, error) {
	t, err := readTx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok System Admin platform read (chat.policy.toggle, platform scope): a count of tenant rows, never a list; deleted_at IS NULL
	err = t.QueryRowContext(ctx, `SELECT count(DISTINCT s.tenant_id) FROM chat.moderation_switches s
		WHERE s.tenant_id IS NOT NULL AND s.deleted_at IS NULL AND (s.expires_at IS NULL OR s.expires_at > $1)
		AND (NOT s.enabled_all OR EXISTS (SELECT 1 FROM jsonb_each(s.steps) e WHERE e.key = ANY($2::text[]) AND e.value = 'false'::jsonb))`,
		now, "{"+strings.Join(moderation.SwitchableSteps, ",")+"}").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("chat: disabled tenant count: %w", err)
	}
	return n, nil
}

// TenantNames implements read.SwitchAdminReader.
func (Switches) TenantNames(ctx context.Context, tenants []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(tenants) == 0 {
		return out, nil
	}
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok ref.tenants is the tenant registry projection (not tenant-scoped, no deleted_at); the given tenants only
	rows, err := t.QueryContext(ctx, `SELECT rt.tenant_id, coalesce(rt.name, rt.code) FROM ref.tenants rt
		WHERE rt.tenant_id = ANY($1::uuid[]) AND coalesce(rt.name, rt.code) IS NOT NULL`, idStrings(tenants))
	if err != nil {
		return nil, fmt.Errorf("chat: tenant names: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("chat: tenant names: %w", err)
		}
		out[id] = name
	}
	return out, rows.Err()
}

// DisabledSwitches implements read.SwitchAdminReader.
func (Switches) DisabledSwitches(ctx context.Context) ([]read.SwitchRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok system gauge sync (system platform scope): the switches with a value off of every level, live only
	rows, err := t.QueryContext(ctx, `SELECT `+switchColumns+` FROM chat.moderation_switches s
		WHERE s.deleted_at IS NULL AND s.expires_at IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("chat: disabled switches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SwitchRow
	for rows.Next() {
		r, err := scanSwitch(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: disabled switches: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}
