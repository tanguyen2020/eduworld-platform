package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// ModerationReader implements read.ModerationReader (F116): explicit SQL with tenant and deleted_at conditions; the
// platform rows (tenant_id NULL) are readable by every tenant session (RLS policy tenant_read of T* tables).
type ModerationReader struct{}

var _ read.ModerationReader = ModerationReader{}

// LatestCatalog implements read.ModerationReader.
func (ModerationReader) LatestCatalog(ctx context.Context, tenant *uuid.UUID, after int64) (*read.CatalogVersionRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	var r read.CatalogVersionRow
	var raw []byte
	if tenant == nil {
		// tenancyguard:ok chat.sensitive_catalog_versions is append-only (no deleted_at); platform rows have tenant_id NULL
		err = t.QueryRowContext(ctx, `SELECT v.catalog_version, v.checksum, v.snapshot FROM chat.sensitive_catalog_versions v
			WHERE v.tenant_id IS NULL AND v.scope = 'platform' AND v.catalog_version > $1
			ORDER BY v.catalog_version DESC LIMIT 1`, after).Scan(&r.Version, &r.Checksum, &raw)
	} else {
		// tenancyguard:ok chat.sensitive_catalog_versions is append-only (no deleted_at); rows of the tenant only
		err = t.QueryRowContext(ctx, `SELECT v.catalog_version, v.checksum, v.snapshot FROM chat.sensitive_catalog_versions v
			WHERE v.tenant_id = $1 AND v.scope = 'tenant' AND v.catalog_version > $2
			ORDER BY v.catalog_version DESC LIMIT 1`, *tenant, after).Scan(&r.Version, &r.Checksum, &raw)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: catalog version: %w", err)
	}
	r.Snapshot = raw
	return &r, nil
}

// CatalogVersions implements read.ModerationReader.
func (ModerationReader) CatalogVersions(ctx context.Context, tenant uuid.UUID) (int64, int64, error) {
	t, err := readTx(ctx)
	if err != nil {
		return 0, 0, err
	}
	var p, tv int64
	// tenancyguard:ok chat.sensitive_catalog_versions is append-only (no deleted_at): the platform and this tenant only
	err = t.QueryRowContext(ctx, `SELECT
		coalesce((SELECT max(v.catalog_version) FROM chat.sensitive_catalog_versions v WHERE v.tenant_id IS NULL AND v.scope = 'platform'), 0),
		coalesce((SELECT max(v.catalog_version) FROM chat.sensitive_catalog_versions v WHERE v.tenant_id = $1 AND v.scope = 'tenant'), 0)`,
		tenant).Scan(&p, &tv)
	if err != nil {
		return 0, 0, fmt.Errorf("chat: catalog versions: %w", err)
	}
	return p, tv, nil
}

// TenantCategorySettings implements read.ModerationReader.
func (ModerationReader) TenantCategorySettings(ctx context.Context, tenant uuid.UUID) (map[uuid.UUID]bool, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT s.category_id, s.enabled FROM chat.tenant_category_settings s
		WHERE s.tenant_id = $1 AND s.deleted_at IS NULL`, tenant)
	if err != nil {
		return nil, fmt.Errorf("chat: tenant categories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		var on bool
		if err := rows.Scan(&id, &on); err != nil {
			return nil, fmt.Errorf("chat: tenant categories: %w", err)
		}
		out[id] = on
	}
	return out, rows.Err()
}

const switchColumns = `s.switch_id, s.tenant_id, s.enabled_all, s.steps, s.reason, s.expires_at, s.updated_by, s.updated_at, s.version`

func scanSwitch(sc interface{ Scan(...any) error }) (*read.SwitchRow, error) {
	var r read.SwitchRow
	var tenant, by uuid.NullUUID
	var steps []byte
	var reason sql.NullString
	var exp sql.NullTime
	if err := sc.Scan(&r.ID, &tenant, &r.EnabledAll, &steps, &reason, &exp, &by, &r.UpdatedAt, &r.Version); err != nil {
		return nil, err
	}
	r.TenantID, r.UpdatedBy, r.ExpiresAt = nullUUID(tenant), nullUUID(by), nullTime(exp)
	if reason.Valid {
		r.Reason = &reason.String
	}
	r.Steps = map[string]bool{}
	if err := json.Unmarshal(steps, &r.Steps); err != nil {
		return nil, fmt.Errorf("chat: switch steps: %w", err)
	}
	return &r, nil
}

// Switches implements read.ModerationReader.
func (ModerationReader) Switches(ctx context.Context, tenant uuid.UUID) (*read.SwitchRow, *read.SwitchRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, nil, err
	}
	// tenancyguard:ok chat.moderation_switches (T*): the platform row (tenant_id NULL) and the row of this tenant, live only
	rows, err := t.QueryContext(ctx, `SELECT `+switchColumns+` FROM chat.moderation_switches s
		WHERE s.deleted_at IS NULL AND (s.tenant_id IS NULL OR s.tenant_id = $1)`, tenant)
	if err != nil {
		return nil, nil, fmt.Errorf("chat: switches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var p, tr *read.SwitchRow
	for rows.Next() {
		r, err := scanSwitch(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("chat: switches: %w", err)
		}
		if r.TenantID == nil {
			p = r
		} else {
			tr = r
		}
	}
	return p, tr, rows.Err()
}

// ActiveSanctions implements read.ModerationReader.
func (ModerationReader) ActiveSanctions(ctx context.Context, tenant, user uuid.UUID, now time.Time) ([]read.SanctionRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT s.sanction_id, s.type, s.scope, s.channel_id, s.ends_at FROM chat.sanctions s
		WHERE s.tenant_id = $1 AND s.user_id = $2 AND s.deleted_at IS NULL AND s.revoked_at IS NULL
			AND s.starts_at <= $3 AND (s.ends_at IS NULL OR s.ends_at > $3)`, tenant, user, now)
	if err != nil {
		return nil, fmt.Errorf("chat: sanctions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SanctionRow
	for rows.Next() {
		var r read.SanctionRow
		var ch uuid.NullUUID
		var ends sql.NullTime
		if err := rows.Scan(&r.ID, &r.Type, &r.Scope, &ch, &ends); err != nil {
			return nil, fmt.Errorf("chat: sanctions: %w", err)
		}
		r.ChannelID, r.EndsAt = nullUUID(ch), nullTime(ends)
		out = append(out, r)
	}
	return out, rows.Err()
}

// QuickMessages implements read.ModerationReader.
func (ModerationReader) QuickMessages(ctx context.Context) ([]read.QuickMessageRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok chat.quick_messages is a G table (no tenant): live, active rows only
	rows, err := t.QueryContext(ctx, `SELECT q.quick_message_id, q.category, q.text_vi, q.text_en, q.emoji, q.sort_order
		FROM chat.quick_messages q WHERE q.deleted_at IS NULL AND q.status = 'active' ORDER BY q.sort_order, q.quick_message_id`)
	if err != nil {
		return nil, fmt.Errorf("chat: quick messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.QuickMessageRow
	for rows.Next() {
		var r read.QuickMessageRow
		var emoji sql.NullString
		if err := rows.Scan(&r.ID, &r.Category, &r.TextVI, &r.TextEN, &emoji, &r.SortOrder); err != nil {
			return nil, fmt.Errorf("chat: quick messages: %w", err)
		}
		if emoji.Valid {
			r.Emoji = &emoji.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
