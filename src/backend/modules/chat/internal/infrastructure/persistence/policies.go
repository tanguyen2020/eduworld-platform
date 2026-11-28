package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
)

// Policies implements read.PolicyReader and write.PolicyStore on chat.chat_policies and
// chat.common_tenant_social_overrides (F120 US2): explicit SQL with tenant and deleted_at conditions on top of RLS (R27).
type Policies struct{}

var (
	_ read.PolicyReader = Policies{}
	_ write.PolicyStore = Policies{}
)

const policyColumns = `p.policy_id, p.tenant_id, p.scope_type, p.scope_id, p.school_id, p.rules, p.version, p.reason, p.updated_by,
	p.updated_at`

func scanPolicy(row interface{ Scan(...any) error }) (*read.PolicyRow, error) {
	var r read.PolicyRow
	var tenant, scope, school, by uuid.NullUUID
	var reason sql.NullString
	if err := row.Scan(&r.ID, &tenant, &r.ScopeType, &scope, &school, &r.Rules, &r.Version, &reason, &by, &r.UpdatedAt); err != nil {
		return nil, err
	}
	r.TenantID, r.ScopeID, r.SchoolID, r.UpdatedBy, r.Reason = nullUUID(tenant), nullUUID(scope), nullUUID(school), nullUUID(by),
		nullString(reason)
	r.UpdatedAt = r.UpdatedAt.UTC()
	return &r, nil
}

// Level implements read.PolicyReader.
func (Policies) Level(ctx context.Context, tenant *uuid.UUID, scopeType string, scopeID *uuid.UUID) (*read.PolicyRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	var row *sql.Row
	if tenant == nil {
		// tenancyguard:ok chat.chat_policies (T*): the platform row (tenant_id NULL), live only
		row = t.QueryRowContext(ctx, `SELECT `+policyColumns+` FROM chat.chat_policies p
			WHERE p.tenant_id IS NULL AND p.scope_type = 'platform' AND p.deleted_at IS NULL`)
	} else {
		// tenancyguard:ok chat.chat_policies (T*) of the session tenant, live only
		row = t.QueryRowContext(ctx, `SELECT `+policyColumns+` FROM chat.chat_policies p
			WHERE p.tenant_id = $1 AND p.scope_type = $2 AND p.scope_id = $3 AND p.deleted_at IS NULL`, *tenant, scopeType, scopeID)
	}
	r, err := scanPolicy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: policy level: %w", err)
	}
	return r, nil
}

// TenantType implements read.PolicyReader.
func (Policies) TenantType(ctx context.Context, tenant uuid.UUID) (string, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return "", err
	}
	var typ string
	// tenancyguard:ok ref.tenants is not tenant-scoped (projection of identity, no deleted_at)
	err = t.QueryRowContext(ctx, `SELECT tn.tenant_type FROM ref.tenants tn WHERE tn.tenant_id = $1`, tenant).Scan(&typ)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("chat: tenant type: %w", err)
	}
	return typ, nil
}

// SchoolActive implements read.PolicyReader.
func (Policies) SchoolActive(ctx context.Context, tenant, school uuid.UUID) (bool, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return false, err
	}
	var n int
	// tenancyguard:ok projection ref.schools of the session tenant (no deleted_at: rule 2)
	if err := t.QueryRowContext(ctx, `SELECT count(*) FROM ref.schools sc WHERE sc.tenant_id = $1 AND sc.school_id = $2
		AND sc.status <> 'disabled'`, tenant, school).Scan(&n); err != nil {
		return false, fmt.Errorf("chat: school: %w", err)
	}
	return n == 1, nil
}

// CommonOverride implements read.PolicyReader.
func (Policies) CommonOverride(ctx context.Context, tenant uuid.UUID) (*read.CommonOverrideRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	r := read.CommonOverrideRow{TenantID: tenant}
	var by uuid.NullUUID
	// tenancyguard:ok chat.common_tenant_social_overrides of the session tenant, live only
	err = t.QueryRowContext(ctx, `SELECT o.direct_enabled, o.world_enabled, o.area_free_text_enabled, o.friends_enabled, o.reason, o.version, o.updated_by,
		o.updated_at FROM chat.common_tenant_social_overrides o WHERE o.tenant_id = $1 AND o.deleted_at IS NULL`, tenant).
		Scan(&r.Override.DirectEnabled, &r.Override.WorldEnabled, &r.Override.AreaFreeTextEnabled, &r.Override.FriendsEnabled, &r.Reason, &r.Version, &by, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: common override: %w", err)
	}
	r.UpdatedBy, r.UpdatedAt = nullUUID(by), r.UpdatedAt.UTC()
	return &r, nil
}

// Lock implements write.PolicyStore.
func (Policies) Lock(ctx context.Context, tenant *uuid.UUID, scopeType string, scopeID *uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	key := "chat.policy:" + scopeType
	if tenant != nil {
		key += ":" + tenant.String()
	}
	if scopeID != nil {
		key += ":" + scopeID.String()
	}
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
		return fmt.Errorf("chat: policy lock: %w", err)
	}
	return nil
}

// InsertLevel implements write.PolicyStore.
func (Policies) InsertLevel(ctx context.Context, r read.PolicyRow) error {
	_, err := exec(ctx, `INSERT INTO chat.chat_policies (policy_id, tenant_id, created_at, created_by, updated_at, updated_by, version,
		scope_type, scope_id, school_id, rules, reason) VALUES ($1, $2, $3, $4, $3, $4, 1, $5, $6, $7, $8::jsonb, $9)`,
		r.ID, r.TenantID, r.UpdatedAt, r.UpdatedBy, r.ScopeType, r.ScopeID, r.SchoolID, string(r.Rules), r.Reason)
	if err != nil {
		return fmt.Errorf("chat: insert policy: %w", err)
	}
	return nil
}

// UpdateLevel implements write.PolicyStore.
func (Policies) UpdateLevel(ctx context.Context, r read.PolicyRow, prev int64) (bool, error) {
	// tenancyguard:ok chat.chat_policies: the row of the level (platform row under chat_platform, else the session tenant), live
	n, err := exec(ctx, `UPDATE chat.chat_policies SET rules = $2::jsonb, reason = $3, updated_at = $4, updated_by = $5,
		version = version + 1 WHERE policy_id = $1 AND version = $6 AND deleted_at IS NULL`,
		r.ID, string(r.Rules), r.Reason, r.UpdatedAt, r.UpdatedBy, prev)
	if err != nil {
		return false, fmt.Errorf("chat: update policy: %w", err)
	}
	return n == 1, nil
}

// SaveCommonOverride implements write.PolicyStore.
func (Policies) SaveCommonOverride(ctx context.Context, r read.CommonOverrideRow) (read.CommonOverrideRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return r, err
	}
	err = t.QueryRowContext(ctx, `INSERT INTO chat.common_tenant_social_overrides AS o (tenant_id, direct_enabled, world_enabled,
		area_free_text_enabled, friends_enabled, reason, created_by, updated_by, created_at, updated_at) VALUES ($1, $2, $3, $4, $8, $5, $6, $6, $7, $7)
		ON CONFLICT (tenant_id) DO UPDATE SET direct_enabled = EXCLUDED.direct_enabled, world_enabled = EXCLUDED.world_enabled,
			area_free_text_enabled = EXCLUDED.area_free_text_enabled, friends_enabled = EXCLUDED.friends_enabled, reason = EXCLUDED.reason, updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at, deleted_at = NULL, deleted_by = NULL, version = o.version + 1
		RETURNING o.version`, r.TenantID, r.Override.DirectEnabled, r.Override.WorldEnabled, r.Override.AreaFreeTextEnabled, r.Reason,
		r.UpdatedBy, r.UpdatedAt, r.Override.FriendsEnabled).Scan(&r.Version)
	if err != nil {
		return r, fmt.Errorf("chat: save common override: %w", err)
	}
	return r, nil
}
