// Package economysql reads the published economy content of module game (F075, G tables: item definitions, shop items,
// collections, achievements, currencies) on the transaction of ctx (use case, consumer or read session). Explicit SQL;
// platform content carries its deleted_at condition and the current published pointer (AGENTS R27). The tenant
// assignment of a shop item is read from infra.tenant_resource_assignments with an explicit tenant (ADR-0029).
package economysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/achievement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Content implements port.EconomyContent.
type Content struct{}

var _ port.EconomyContent = Content{}

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// Items implements port.EconomyContent.
func (Content) Items(ctx context.Context, codes []string) (map[string]port.ItemDef, error) {
	out := map[string]port.ItemDef{}
	if len(codes) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT d.item_id, v.kind, v.rarity, v.equip_slot, v.stackable, v.max_stack, v.name
		FROM game.item_definitions d JOIN game.item_definition_versions v ON v.item_version_id = d.current_version_id
		WHERE d.item_id = ANY($1::varchar[]) AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, codes)
	if err != nil {
		return nil, fmt.Errorf("item definitions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var d port.ItemDef
		var name []byte
		if err := rows.Scan(&d.ItemID, &d.Kind, &d.Rarity, &d.EquipSlot, &d.Stackable, &d.MaxStack, &name); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(name, &d.Name)
		out[d.ItemID] = d
	}
	return out, rows.Err()
}

// Listing implements port.EconomyContent: the current published version; Assigned when the shop item has no assignment
// row (every tenant) or one for tenant.
func (Content) Listing(ctx context.Context, tenant, shopItem uuid.UUID) (*port.ListingRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r port.ListingRow
	var grades []byte
	var from, to sql.NullTime
	var event, season uuid.NullUUID
	// tenancyguard:ok platform content (G tables) + infra.tenant_resource_assignments (no RLS, ADR-0089 §4): explicit tenant_id and
	// deleted_at IS NULL on every table
	err = t.QueryRowContext(ctx, `SELECT s.shop_item_id, v.shop_item_version_id, v.item_id, v.item_kind, v.price_amount, v.currency_code,
			v.purchase_limit, v.available_from, v.available_to, v.grade_levels, v.featured_order, v.event_id, v.season_id,
			(NOT EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a WHERE a.resource_type = 'shop_item' AND a.resource_id = s.shop_item_id
				AND a.deleted_at IS NULL)
			 OR EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a WHERE a.resource_type = 'shop_item' AND a.resource_id = s.shop_item_id
				AND a.tenant_id = $2 AND a.deleted_at IS NULL))
		FROM gamification.shop_items s JOIN gamification.shop_item_versions v ON v.shop_item_version_id = s.current_version_id
		WHERE s.shop_item_id = $1 AND s.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, shopItem, tenant).
		Scan(&r.ShopItemID, &r.VersionID, &r.ItemID, &r.ItemKind, &r.Price, &r.Currency, &r.PurchaseLimit, &from, &to, &grades, &r.FeaturedOrder,
			&event, &season, &r.Assigned)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("shop item: %w", err)
	}
	if from.Valid {
		f := from.Time.UTC()
		r.From = &f
	}
	if to.Valid {
		v := to.Time.UTC()
		r.To = &v
	}
	if event.Valid {
		r.EventID = &event.UUID
	}
	if season.Valid {
		r.SeasonID = &season.UUID
	}
	_ = json.Unmarshal(grades, &r.Grades)
	return &r, nil
}

// CollectionsOf implements port.EconomyContent.
func (Content) CollectionsOf(ctx context.Context, item string) ([]port.CollectionRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G/GA tables, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT c.collection_id, v.collection_version_id, v.code, v.collection_type, v.item_count,
			coalesce(v.payload->'completion_rewards', '[]'::jsonb)
		FROM gamification.collection_items i
		JOIN gamification.collections c ON c.current_version_id = i.collection_version_id AND c.deleted_at IS NULL
		JOIN gamification.collection_versions v ON v.collection_version_id = i.collection_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		WHERE i.item_id = $1 ORDER BY v.code`, item)
	if err != nil {
		return nil, fmt.Errorf("collections of item: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.CollectionRef
	for rows.Next() {
		var c port.CollectionRef
		var rewards []byte
		if err := rows.Scan(&c.CollectionID, &c.VersionID, &c.Code, &c.Type, &c.Total, &rewards); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rewards, &c.Rewards)
		out = append(out, c)
	}
	return out, rows.Err()
}

// CardFor implements port.EconomyContent.
func (Content) CardFor(ctx context.Context, objective uuid.UUID) (*port.CardRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var c port.CardRef
	// tenancyguard:ok platform content (G/GA tables, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT i.item_id, c.collection_id FROM gamification.collection_items i
		JOIN gamification.collections c ON c.current_version_id = i.collection_version_id AND c.deleted_at IS NULL
		JOIN gamification.collection_versions v ON v.collection_version_id = i.collection_version_id AND v.deleted_at IS NULL
			AND v.status = 'published' AND v.collection_type = 'knowledge_card'
		WHERE i.objective_id = $1 ORDER BY v.code LIMIT 1`, objective).Scan(&c.ItemID, &c.CollectionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("knowledge card: %w", err)
	}
	return &c, nil
}

// AchievementsFor implements port.EconomyContent.
func (Content) AchievementsFor(ctx context.Context, eventType string) ([]port.AchievementRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT v.achievement_version_id, v.version_no, v.payload FROM gamification.achievements a
		JOIN gamification.achievement_versions v ON v.achievement_version_id = a.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		WHERE a.deleted_at IS NULL AND v.trigger_event_types @> jsonb_build_array($1::text) ORDER BY v.code`, eventType)
	if err != nil {
		return nil, fmt.Errorf("achievements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.AchievementRow
	for rows.Next() {
		var r port.AchievementRow
		var no int
		var payload []byte
		if err := rows.Scan(&r.VersionID, &no, &payload); err != nil {
			return nil, err
		}
		d, err := achievement.Parse(payload)
		if err != nil {
			return nil, err
		}
		d.Version = no
		r.Definition = d
		out = append(out, r)
	}
	return out, rows.Err()
}

// Currency implements port.EconomyContent.
func (Content) Currency(ctx context.Context, code string) (*port.CurrencyRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var c port.CurrencyRow
	var status string
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT currency_code, daily_cap, status FROM gamification.currencies WHERE currency_code = $1 AND deleted_at IS NULL`,
		code).Scan(&c.Code, &c.DailyCap, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("currency: %w", err)
	}
	c.Active = status == "active"
	return &c, nil
}

// NoLiveOps implements port.LiveOps until the event and season runs of F077 exist: no event or season runs, so a shop
// listing limited to one is not sold (fail closed, INVENTORY_ECONOMY §5.1 — never sold outside its event). Not a mock:
// it is the real answer while module game has no event run.
type NoLiveOps struct{}

var _ port.LiveOps = NoLiveOps{}

// Running implements port.LiveOps.
func (NoLiveOps) Running(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID, time.Time) (bool, error) {
	return false, nil
}
