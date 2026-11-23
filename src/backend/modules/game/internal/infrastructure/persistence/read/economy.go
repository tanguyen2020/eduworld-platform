package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
)

// Economy implements read.EconomyReadModel (F075): explicit tenant_id on every tenant table, deleted_at IS NULL on live
// rows and published pointers on the platform content (R27); every list is a keyset page in SQL (LIMIT n+1).
type Economy struct{}

var _ read.EconomyReadModel = Economy{}

func strMap(raw []byte) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]string
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// Balance implements read.EconomyReadModel (0 without a wallet).
func (Economy) Balance(ctx context.Context, tenant, student uuid.UUID, currency string) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	// tenancyguard:ok tenant table gamification.wallets: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT balance FROM gamification.wallets WHERE tenant_id = $1 AND student_id = $2 AND currency_code = $3
		AND deleted_at IS NULL`, tenant, student, currency).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("wallet: %w", err)
	}
	return n, nil
}

// Grade implements read.EconomyReadModel.
func (Economy) Grade(ctx context.Context, tenant, student uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var g sql.NullInt64
	// tenancyguard:ok projection ref.students (RLS + explicit tenant_id); projections have no deleted_at
	err = t.QueryRowContext(ctx, `SELECT grade_level FROM ref.students WHERE tenant_id = $1 AND student_id = $2`, tenant, student).Scan(&g)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !g.Valid) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("ref students: grade: %w", err)
	}
	return int(g.Int64), nil
}

// DailyCap implements read.EconomyReadModel (0 for an unknown or retired currency).
func (Economy) DailyCap(ctx context.Context, currency string) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT daily_cap FROM gamification.currencies WHERE currency_code = $1 AND status = 'active'
		AND deleted_at IS NULL`, currency).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("currency: %w", err)
	}
	return n, nil
}

// EarnedToday implements read.EconomyReadModel.
func (Economy) EarnedToday(ctx context.Context, tenant, student uuid.UUID, day, bucket string) (int, error) {
	return Progression{}.DailyGranted(ctx, tenant, student, day, bucket)
}

// CurrencyLedger implements read.EconomyReadModel.
func (Economy) CurrencyLedger(ctx context.Context, tenant, student uuid.UUID, after *read.TimeCursor, limit int) ([]read.CurrencyLedgerRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := time.Now().Add(time.Hour).UTC(), uuid.Max
	if after != nil {
		at, id = after.At.UTC(), after.ID
	}
	// tenancyguard:ok append-only tenant table gamification.currency_ledger (no deleted_at): explicit tenant_id (RLS as well)
	rows, err := t.QueryContext(ctx, `SELECT entry_id, currency_code, delta, balance_after, reason_code, source_type, created_at
		FROM gamification.currency_ledger WHERE tenant_id = $1 AND student_id = $2 AND (created_at, entry_id) < ($3, $4)
		ORDER BY created_at DESC, entry_id DESC LIMIT $5`, tenant, student, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("currency ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.CurrencyLedgerRow
	for rows.Next() {
		var r read.CurrencyLedgerRow
		if err := rows.Scan(&r.ID, &r.Currency, &r.Delta, &r.BalanceAfter, &r.ReasonCode, &r.SourceType, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.CreatedAt = r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// ItemHistory implements read.EconomyReadModel.
func (Economy) ItemHistory(ctx context.Context, tenant, student uuid.UUID, after *read.TimeCursor, limit int) ([]read.ItemHistoryRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := time.Now().Add(time.Hour).UTC(), uuid.Max
	if after != nil {
		at, id = after.At.UTC(), after.ID
	}
	// tenancyguard:ok append-only tenant table gamification.item_ledger (explicit tenant_id, RLS) + platform content (deleted_at filtered)
	rows, err := t.QueryContext(ctx, `SELECT l.entry_id, l.item_id, coalesce(v.kind, ''), v.name, l.delta, l.reason_code, l.source_type, l.created_at
		FROM gamification.item_ledger l
		LEFT JOIN game.item_definitions d ON d.item_id = l.item_id AND d.deleted_at IS NULL
		LEFT JOIN game.item_definition_versions v ON v.item_version_id = d.current_version_id AND v.deleted_at IS NULL
		WHERE l.tenant_id = $1 AND l.student_id = $2 AND (l.created_at, l.entry_id) < ($3, $4)
		ORDER BY l.created_at DESC, l.entry_id DESC LIMIT $5`, tenant, student, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("item history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ItemHistoryRow
	for rows.Next() {
		var r read.ItemHistoryRow
		var name []byte
		if err := rows.Scan(&r.ID, &r.ItemID, &r.Kind, &name, &r.Delta, &r.ReasonCode, &r.SourceType, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Name, r.CreatedAt = strMap(name), r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// Inventory implements read.EconomyReadModel (owned items by code, optional kind).
func (Economy) Inventory(ctx context.Context, tenant, student uuid.UUID, kind, after string, limit int) ([]read.InventoryRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.inventory_items (explicit tenant_id + deleted_at IS NULL, RLS) + platform content (deleted_at filtered)
	rows, err := t.QueryContext(ctx, `SELECT i.item_id, v.kind, v.rarity, v.equip_slot, v.stackable, i.quantity, i.acquired_at, v.name
		FROM game.inventory_items i
		JOIN game.item_definitions d ON d.item_id = i.item_id AND d.deleted_at IS NULL
		JOIN game.item_definition_versions v ON v.item_version_id = d.current_version_id AND v.deleted_at IS NULL
		WHERE i.tenant_id = $1 AND i.student_id = $2 AND i.deleted_at IS NULL AND i.quantity > 0 AND ($3 = '' OR v.kind = $3) AND i.item_id > $4
		ORDER BY i.item_id LIMIT $5`, tenant, student, kind, after, limit)
	if err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.InventoryRow
	for rows.Next() {
		var r read.InventoryRow
		var name []byte
		if err := rows.Scan(&r.ItemID, &r.Kind, &r.Rarity, &r.EquipSlot, &r.Stackable, &r.Quantity, &r.AcquiredAt, &name); err != nil {
			return nil, err
		}
		r.Name, r.AcquiredAt = strMap(name), r.AcquiredAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// Shop implements read.EconomyReadModel (F075-AC-06): published current versions on sale now, assigned to the tenant (or
// to every tenant), of the grade of the student when grades are set; a listing limited to an event is listed while the
// event has a live run of the tenant's audience, one limited to a season while the season is started and holds now (F077,
// same rule as port.LiveOps of the purchase).
func (Economy) Shop(ctx context.Context, f read.ShopFilter) ([]read.ShopRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	afterOrder := -1
	if f.AfterOrder != nil {
		afterOrder = *f.AfterOrder
	}
	// tenancyguard:ok platform content (G tables, deleted_at filtered) + infra.tenant_resource_assignments (explicit tenant_id + deleted_at)
	// + the tenant tables of the student (shop_purchases append-only, inventory_items live rows), explicit tenant_id (RLS as well)
	// + the G/T event runs (platform rows or explicit tenant_id, deleted_at filtered) and the platform projection ref.seasons
	rows, err := t.QueryContext(ctx, `SELECT s.shop_item_id, v.item_id, iv.kind, iv.rarity, iv.name, iv.payload->'description', v.price_amount,
			v.currency_code, v.purchase_limit, v.available_to, v.featured_order,
			(SELECT coalesce(sum(p.quantity), 0) FROM gamification.shop_purchases p WHERE p.tenant_id = $1 AND p.student_id = $2
				AND p.shop_item_id = s.shop_item_id),
			EXISTS (SELECT 1 FROM game.inventory_items i WHERE i.tenant_id = $1 AND i.student_id = $2 AND i.item_id = v.item_id
				AND i.quantity > 0 AND i.deleted_at IS NULL)
		FROM gamification.shop_items s
		JOIN gamification.shop_item_versions v ON v.shop_item_version_id = s.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		JOIN game.item_definitions d ON d.item_id = v.item_id AND d.deleted_at IS NULL
		JOIN game.item_definition_versions iv ON iv.item_version_id = d.current_version_id AND iv.deleted_at IS NULL
		WHERE s.deleted_at IS NULL AND v.currency_code = 'xu' AND v.price_amount > 0
			AND (v.available_from IS NULL OR v.available_from <= $3) AND (v.available_to IS NULL OR v.available_to > $3)
			AND (jsonb_array_length(v.grade_levels) = 0 OR v.grade_levels @> to_jsonb($4::int))
			AND (v.event_id IS NULL OR EXISTS (SELECT 1 FROM game.event_runs r WHERE r.event_id = v.event_id AND r.status = 'live'
				AND r.deleted_at IS NULL AND (r.tenant_id = $1 OR (r.tenant_id IS NULL
					AND (jsonb_array_length(coalesce(r.audience->'tenant_ids', '[]'::jsonb)) = 0 OR r.audience->'tenant_ids' @> to_jsonb($1::text))))))
			AND (v.season_id IS NULL OR EXISTS (SELECT 1 FROM ref.seasons se WHERE se.season_id = v.season_id AND se.status = 'started'
				AND se.starts_at <= $3 AND se.ends_at > $3))
			AND (NOT EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a WHERE a.resource_type = 'shop_item'
					AND a.resource_id = s.shop_item_id AND a.deleted_at IS NULL)
				OR EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a WHERE a.resource_type = 'shop_item' AND a.resource_id = s.shop_item_id
					AND a.tenant_id = $1 AND a.deleted_at IS NULL))
			AND ($5 = '' OR v.item_kind = $5) AND (v.featured_order, s.shop_item_id) > ($6, $7)
		ORDER BY v.featured_order, s.shop_item_id LIMIT $8`, f.Tenant, f.Student, f.Now.UTC(), f.Grade, f.Kind, afterOrder, f.AfterID, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("shop items: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ShopRow
	for rows.Next() {
		var r read.ShopRow
		var name, desc []byte
		var to sql.NullTime
		if err := rows.Scan(&r.ShopItemID, &r.ItemID, &r.Kind, &r.Rarity, &name, &desc, &r.Price, &r.Currency, &r.PurchaseLimit, &to, &r.FeaturedOrder,
			&r.Purchased, &r.Owned); err != nil {
			return nil, err
		}
		r.Name, r.Description, r.AvailableTo = strMap(name), strMap(desc), nullTime(to)
		out = append(out, r)
	}
	return out, rows.Err()
}

const collectionSelect = `SELECT c.collection_id, v.code, v.collection_type, v.payload->'title', coalesce(p.owned_item_ids, '[]'::jsonb), v.item_count,
		p.completed_at, coalesce(v.payload->'completion_rewards', '[]'::jsonb)
	FROM gamification.collections c
	JOIN gamification.collection_versions v ON v.collection_version_id = c.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
	LEFT JOIN gamification.collection_progress p ON p.tenant_id = $1 AND p.student_id = $2 AND p.collection_id = c.collection_id
		AND p.deleted_at IS NULL
	WHERE c.deleted_at IS NULL AND (v.visibility = 'always' OR p.collection_progress_id IS NOT NULL)`

func scanCollection(row interface{ Scan(...any) error }) (read.CollectionRow, error) {
	var r read.CollectionRow
	var title, owned, rewards []byte
	var done sql.NullTime
	if err := row.Scan(&r.CollectionID, &r.Code, &r.Type, &title, &owned, &r.Total, &done, &rewards); err != nil {
		return r, err
	}
	r.Title, r.CompletedAt = strMap(title), nullTime(done)
	_ = json.Unmarshal(owned, &r.Owned)
	_ = json.Unmarshal(rewards, &r.Rewards)
	return r, nil
}

// Collections implements read.EconomyReadModel (F075-AC-20): a hidden_until_first collection only once the student owns
// an item of it; keyset by code.
func (Economy) Collections(ctx context.Context, tenant, student uuid.UUID, after string, limit int) ([]read.CollectionRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, deleted_at filtered) + collection_progress (explicit tenant_id + deleted_at, RLS)
	rows, err := t.QueryContext(ctx, collectionSelect+` AND v.code > $3 ORDER BY v.code LIMIT $4`, tenant, student, after, limit)
	if err != nil {
		return nil, fmt.Errorf("collections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.CollectionRow
	for rows.Next() {
		r, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Collection implements read.EconomyReadModel: the collection (nil when unknown or still hidden) and its items in order
// with the public "how to obtain" hint (bounded by the collection: item_count ≤ 200).
func (Economy) Collection(ctx context.Context, tenant, student, coll uuid.UUID) (*read.CollectionRow, []read.CollectionItemView, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, nil, err
	}
	// tenancyguard:ok platform content (G tables, deleted_at filtered) + collection_progress (explicit tenant_id + deleted_at, RLS)
	r, err := scanCollection(t.QueryRowContext(ctx, collectionSelect+` AND c.collection_id = $3`, tenant, student, coll))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("collection: %w", err)
	}
	// tenancyguard:ok platform content (G/GA tables, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT ci.item_id, iv.name, coalesce(iv.rarity, ''), ci.source_hint
		FROM gamification.collections c
		JOIN gamification.collection_items ci ON ci.collection_version_id = c.current_version_id
		LEFT JOIN game.item_definitions d ON d.item_id = ci.item_id AND d.deleted_at IS NULL
		LEFT JOIN game.item_definition_versions iv ON iv.item_version_id = d.current_version_id AND iv.deleted_at IS NULL
		WHERE c.collection_id = $1 AND c.deleted_at IS NULL ORDER BY ci.position`, coll)
	if err != nil {
		return nil, nil, fmt.Errorf("collection items: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var items []read.CollectionItemView
	for rows.Next() {
		var it read.CollectionItemView
		var name, hint []byte
		if err := rows.Scan(&it.ItemID, &name, &it.Rarity, &hint); err != nil {
			return nil, nil, err
		}
		it.Name, it.Hint = strMap(name), strMap(hint)
		items = append(items, it)
	}
	return &r, items, rows.Err()
}

// Achievements implements read.EconomyReadModel (F075-AC-22): published achievements with the progress of the student;
// a hidden_until_unlocked achievement not unlocked yet is not returned at all; keyset by code.
func (Economy) Achievements(ctx context.Context, tenant, student uuid.UUID, category, after string, limit int) ([]read.AchievementView, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, deleted_at filtered) + student_achievements (explicit tenant_id + deleted_at, RLS)
	rows, err := t.QueryContext(ctx, `SELECT a.achievement_id, v.code, v.payload, coalesce(s.progress, 0), s.tier, s.unlocked_at
		FROM gamification.achievements a
		JOIN gamification.achievement_versions v ON v.achievement_version_id = a.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		LEFT JOIN gamification.student_achievements s ON s.tenant_id = $1 AND s.student_id = $2 AND s.achievement_id = a.achievement_id
			AND s.deleted_at IS NULL
		WHERE a.deleted_at IS NULL AND (v.visibility <> 'hidden_until_unlocked' OR s.tier IS NOT NULL) AND ($3 = '' OR v.category = $3)
			AND v.code > $4
		ORDER BY v.code LIMIT $5`, tenant, student, category, after, limit)
	if err != nil {
		return nil, fmt.Errorf("achievements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.AchievementView
	for rows.Next() {
		var r read.AchievementView
		var tier sql.NullString
		var unlocked sql.NullTime
		if err := rows.Scan(&r.AchievementID, &r.Code, &r.Payload, &r.Progress, &tier, &unlocked); err != nil {
			return nil, err
		}
		if tier.Valid {
			r.Tier = &tier.String
		}
		r.UnlockedAt = nullTime(unlocked)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Adjustments implements read.EconomyReadModel: explicit tenant filter (the tenants of the CrossTenantScope, or every
// tenant under the audited bypass of System Admin), keyset newest first.
func (Economy) Adjustments(ctx context.Context, f read.AdjustmentFilter) ([]read.AdjustmentRecord, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := time.Now().Add(time.Hour).UTC(), uuid.Max
	if f.After != nil {
		at, id = f.After.At.UTC(), f.After.ID
	}
	tenants := f.TenantIDs
	if tenants == nil {
		tenants = []uuid.UUID{}
	}
	// tenancyguard:ok tenant table gamification.economy_adjustments: explicit tenant_id list (or the audited all-tenant bypass) +
	// deleted_at IS NULL
	rows, err := t.QueryContext(ctx, `SELECT adjustment_id, tenant_id, student_id, kind, currency_code, item_id, delta, reason, ticket_ref, status,
			created_by, approved_by, rejected_by, reject_reason, created_at, applied_at, rejected_at
		FROM gamification.economy_adjustments
		WHERE deleted_at IS NULL AND ($1 OR tenant_id = ANY($2::uuid[])) AND ($3 = '' OR status = $3)
			AND ($4::uuid IS NULL OR student_id = $4) AND (created_at, adjustment_id) < ($5, $6)
		ORDER BY created_at DESC, adjustment_id DESC LIMIT $7`, f.AllTenants, tenants, f.Status, f.StudentID, at, id, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("economy adjustments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.AdjustmentRecord
	for rows.Next() {
		var a read.AdjustmentRecord
		var cur, item, ticket, rejectReason sql.NullString
		var approved, rejected uuid.NullUUID
		var applied, rejectedAt sql.NullTime
		if err := rows.Scan(&a.ID, &a.TenantID, &a.StudentID, &a.Kind, &cur, &item, &a.Delta, &a.Reason, &ticket, &a.Status, &a.CreatedBy, &approved,
			&rejected, &rejectReason, &a.CreatedAt, &applied, &rejectedAt); err != nil {
			return nil, err
		}
		s := func(v sql.NullString) *string {
			if !v.Valid {
				return nil
			}
			x := v.String
			return &x
		}
		u := func(v uuid.NullUUID) *uuid.UUID {
			if !v.Valid {
				return nil
			}
			x := v.UUID
			return &x
		}
		a.Currency, a.ItemID, a.TicketRef, a.RejectReason = s(cur), s(item), s(ticket), s(rejectReason)
		a.ApprovedBy, a.RejectedBy = u(approved), u(rejected)
		a.CreatedAt, a.AppliedAt, a.RejectedAt = a.CreatedAt.UTC(), nullTime(applied), nullTime(rejectedAt)
		out = append(out, a)
	}
	return out, rows.Err()
}
