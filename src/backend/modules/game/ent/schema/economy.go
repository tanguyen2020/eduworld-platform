package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// F075 — coin economy, cosmetic shop, inventory, collection, achievements and badges (INVENTORY_ECONOMY §2–§9,
// ADR-0049, DATABASE_DESIGN §2.8a). The Xu ledger, the item ledger, the purchases and the per-event progress records are
// append-only tenant tables (TA, R21); wallets, collection progress, student achievements and economy adjustments are
// tenant tables (T); shop items, collections, achievements and currencies are platform content (G, ADR-0043).

// Currency — gamification.currencies (G): the in-game currency (Release 1.0: only xu, OQ-14) and its daily earning cap
// (xu_daily_cap, INVENTORY_ECONOMY §4.3).
type Currency struct{ ent.Schema }

func (Currency) Annotations() []schema.Annotation { return gamification("currencies") }
func (Currency) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "currency_code", StringLen: 64}}}
}
func (Currency) Fields() []ent.Field {
	return []ent.Field{
		field.String("name_vi").MaxLen(128),
		field.String("name_en").MaxLen(128),
		field.Int("daily_cap"),
		field.String("status").MaxLen(64).Default("active"),
	}
}

// Wallet — gamification.wallets (T): the balance of one currency of a student, a cache of Σ currency_ledger.delta written
// in the ledger transaction under a lock (FOR UPDATE). A surrogate key plus a partial unique (tenant_id, student_id,
// currency_code) replaces the composite key of DATABASE_DESIGN (ent: no composite primary key).
type Wallet struct{ ent.Schema }

func (Wallet) Annotations() []schema.Annotation { return gamification("wallets") }
func (Wallet) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "wallet_id"}}}
}
func (Wallet) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("currency_code").MaxLen(64).Immutable(),
		field.Int64("balance").Default(0),
	}
}
func (Wallet) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "currency_code").Unique().StorageKey("wallets_student_currency_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// CurrencyLedgerEntry — gamification.currency_ledger (TA): one Xu posting (delta ≠ 0, balance after ≥ 0). Never updated
// or deleted: wallets.balance = Σ delta. Idempotent per idempotency_key (monthly partitions when the table is large,
// like xp_ledger — spec Clarifications).
type CurrencyLedgerEntry struct{ ent.Schema }

func (CurrencyLedgerEntry) Annotations() []schema.Annotation { return gamification("currency_ledger") }
func (CurrencyLedgerEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "entry_id"}}}
}
func (CurrencyLedgerEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("currency_code").MaxLen(64).Immutable(),
		field.Int64("delta").Immutable(),
		field.Int64("balance_after").Immutable(),
		field.String("reason_code").MaxLen(64).Immutable(),
		field.String("source_type").MaxLen(64).Immutable(),
		field.UUID("source_id", uuid.UUID{}).Immutable(),
		field.String("item_id").MaxLen(64).Optional().Nillable().Immutable(),
		field.Bool("capped").Default(false).Immutable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
	}
}
func (CurrencyLedgerEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("idempotency_key").Unique().StorageKey("currency_ledger_idem_uq"),
		index.Fields("tenant_id", "student_id", "created_at").StorageKey("currency_ledger_student_idx"),
	}
}

// ItemLedgerEntry — gamification.item_ledger (TA): one change of ownership of an item (received, bought, adjusted).
type ItemLedgerEntry struct{ ent.Schema }

func (ItemLedgerEntry) Annotations() []schema.Annotation { return gamification("item_ledger") }
func (ItemLedgerEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "entry_id"}}}
}
func (ItemLedgerEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("item_id").MaxLen(64).Immutable(),
		field.Int("delta").Immutable(),
		field.String("reason_code").MaxLen(64).Immutable(),
		field.String("source_type").MaxLen(64).Immutable(),
		field.UUID("source_id", uuid.UUID{}).Immutable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
	}
}
func (ItemLedgerEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("idempotency_key").Unique().StorageKey("item_ledger_idem_uq"),
		index.Fields("tenant_id", "student_id", "created_at").StorageKey("item_ledger_student_idx"),
	}
}

// ShopItem — gamification.shop_items (G): identity of a shop listing and its release pointer.
type ShopItem struct{ ent.Schema }

func (ShopItem) Annotations() []schema.Annotation { return gamification("shop_items") }
func (ShopItem) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "shop_item_id"}}}
}
func (ShopItem) Fields() []ent.Field { return pointerFields() }

// ShopItemVersion — gamification.shop_item_versions (G): payload of specs/schemas/shop-item.schema.json plus the columns
// the shop filters on (price in Xu, purchase limit, sale window, grades, event/season of a limited listing).
type ShopItemVersion struct{ ent.Schema }

func (ShopItemVersion) Annotations() []schema.Annotation { return gamification("shop_item_versions") }
func (ShopItemVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "shop_item_version_id"}}}
}
func (ShopItemVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("shop_item_id", uuid.UUID{}).Immutable()),
		field.String("item_id").MaxLen(64).Immutable(),
		field.String("item_kind").MaxLen(64).Immutable(),
		field.Int("price_amount").Immutable(),
		field.String("currency_code").MaxLen(64).Immutable(),
		field.Int("purchase_limit").Immutable(),
		field.Time("available_from").Optional().Nillable().Immutable(),
		field.Time("available_to").Optional().Nillable().Immutable(),
		field.JSON("grade_levels", []int{}).Immutable(),
		field.Int("featured_order").Default(0).Immutable(),
		field.UUID("event_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("season_id", uuid.UUID{}).Optional().Nillable().Immutable(),
	)
}
func (ShopItemVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("shop_item_id", "version_no").Unique().StorageKey("shop_item_versions_no_uq")}
}

// ShopPurchase — gamification.shop_purchases (TA): one purchase (price paid, quantity, the Xu posting). Append-only:
// purchase_limit counts the quantities of a shop item over every version.
type ShopPurchase struct{ ent.Schema }

func (ShopPurchase) Annotations() []schema.Annotation { return gamification("shop_purchases") }
func (ShopPurchase) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "purchase_id"}}}
}
func (ShopPurchase) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("shop_item_id", uuid.UUID{}).Immutable(),
		field.UUID("shop_item_version_id", uuid.UUID{}).Immutable(),
		field.String("item_id").MaxLen(64).Immutable(),
		field.Int("price_amount").Immutable(),
		field.Int("quantity").Default(1).Immutable(),
		field.UUID("currency_ledger_entry_id", uuid.UUID{}).Immutable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
	}
}
func (ShopPurchase) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("idempotency_key").Unique().StorageKey("shop_purchases_idem_uq"),
		index.Fields("tenant_id", "student_id", "shop_item_id").StorageKey("shop_purchases_student_item_idx"),
	}
}

// Collection — gamification.collections (G): identity of a collection and its release pointer.
type Collection struct{ ent.Schema }

func (Collection) Annotations() []schema.Annotation { return gamification("collections") }
func (Collection) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "collection_id"}}}
}
func (Collection) Fields() []ent.Field {
	return append([]ent.Field{field.String("code").MaxLen(64).Optional().Nillable()}, pointerFields()...)
}
func (Collection) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("collections_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// CollectionVersion — gamification.collection_versions (G): payload of specs/schemas/collection.schema.json.
type CollectionVersion struct{ ent.Schema }

func (CollectionVersion) Annotations() []schema.Annotation {
	return gamification("collection_versions")
}
func (CollectionVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "collection_version_id"}}}
}
func (CollectionVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("collection_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.String("collection_type").MaxLen(64).Immutable(),
		field.String("visibility").MaxLen(64).Immutable(),
		field.Int("item_count").Immutable(),
	)
}
func (CollectionVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("collection_id", "version_no").Unique().StorageKey("collection_versions_no_uq")}
}

// CollectionItem — gamification.collection_items (GA): the items of a published collection version (written in the publish
// transaction, never changed), with the public "how to obtain" hint and the learning objective of a knowledge card.
type CollectionItem struct{ ent.Schema }

func (CollectionItem) Annotations() []schema.Annotation { return gamification("collection_items") }
func (CollectionItem) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.AppendOnly{PK: entx.PK{Column: "collection_item_id"}}}
}
func (CollectionItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("collection_version_id", uuid.UUID{}).Immutable(),
		field.UUID("collection_id", uuid.UUID{}).Immutable(),
		field.String("item_id").MaxLen(64).Immutable(),
		field.Int("position").Immutable(),
		field.JSON("source_hint", map[string]string{}).Immutable(),
		field.UUID("objective_id", uuid.UUID{}).Optional().Nillable().Immutable(),
	}
}
func (CollectionItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("collection_version_id", "item_id").Unique().StorageKey("collection_items_uq"),
		index.Fields("item_id").StorageKey("collection_items_item_idx"),
		index.Fields("objective_id").StorageKey("collection_items_objective_idx"),
	}
}

// CollectionProgress — gamification.collection_progress (T): the items of a collection a student owns (by code) and the
// completion time (completion rewards granted once).
type CollectionProgress struct{ ent.Schema }

func (CollectionProgress) Annotations() []schema.Annotation {
	return gamification("collection_progress")
}
func (CollectionProgress) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "collection_progress_id"}}}
}
func (CollectionProgress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("collection_id", uuid.UUID{}).Immutable(),
		field.JSON("owned_item_ids", []string{}),
		field.Int("owned_count").Default(0),
		field.Time("completed_at").Optional().Nillable(),
	}
}
func (CollectionProgress) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "collection_id").Unique().StorageKey("collection_progress_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// Achievement — gamification.achievements (G): identity of an achievement and its release pointer.
type Achievement struct{ ent.Schema }

func (Achievement) Annotations() []schema.Annotation { return gamification("achievements") }
func (Achievement) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "achievement_id"}}}
}
func (Achievement) Fields() []ent.Field {
	return append([]ent.Field{field.String("code").MaxLen(64).Optional().Nillable()}, pointerFields()...)
}
func (Achievement) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("achievements_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// AchievementVersion — gamification.achievement_versions (G): payload of specs/schemas/achievement.schema.json plus the
// columns the evaluator selects on (trigger event types) and the shop of the student filters on (category, visibility).
type AchievementVersion struct{ ent.Schema }

func (AchievementVersion) Annotations() []schema.Annotation {
	return gamification("achievement_versions")
}
func (AchievementVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "achievement_version_id"}}}
}
func (AchievementVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("achievement_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.String("category").MaxLen(64).Immutable(),
		field.String("visibility").MaxLen(64).Immutable(),
		field.JSON("trigger_event_types", []string{}).Immutable(),
	)
}
func (AchievementVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("achievement_id", "version_no").Unique().StorageKey("achievement_versions_no_uq")}
}

// StudentAchievement — gamification.student_achievements (T): the progress of a student on one achievement (never
// lowered, F075-AC-24), the counted distinct values or days (progress_state) and the highest tier unlocked. The per-tier
// unlock record is the reward grant (trigger_kind achievement, unique per (student, achievement, tier)) — spec
// Clarifications.
type StudentAchievement struct{ ent.Schema }

func (StudentAchievement) Annotations() []schema.Annotation {
	return gamification("student_achievements")
}
func (StudentAchievement) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "student_achievement_id"}}}
}
func (StudentAchievement) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("achievement_id", uuid.UUID{}).Immutable(),
		field.String("tier").MaxLen(64).Optional().Nillable(),
		field.Int("progress").Default(0),
		field.JSON("progress_state", map[string]any{}).Default(map[string]any{}),
		field.Int("achievement_version").Default(1),
		field.Time("unlocked_at").Optional().Nillable(),
	}
}
func (StudentAchievement) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "achievement_id").Unique().StorageKey("student_achievements_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// AchievementProgressEvent — gamification.achievement_progress_events (TA): one event counted for one achievement of a
// student; a redelivered event finds its row and counts nothing again (F075-AC-21).
type AchievementProgressEvent struct{ ent.Schema }

func (AchievementProgressEvent) Annotations() []schema.Annotation {
	return gamification("achievement_progress_events")
}
func (AchievementProgressEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "progress_event_id"}}}
}
func (AchievementProgressEvent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("achievement_id", uuid.UUID{}).Immutable(),
		field.UUID("event_id", uuid.UUID{}).Immutable(),
		field.String("event_type").MaxLen(64).Immutable(),
	}
}
func (AchievementProgressEvent) Indexes() []ent.Index {
	return []ent.Index{index.Fields("student_id", "achievement_id", "event_id").Unique().StorageKey("achievement_progress_events_uq")}
}

// EconomyAdjustment — gamification.economy_adjustments (T): a compensating posting of Xu, an item or XP asked by one
// platform staff and applied only when a second one approves (four-eyes, ADR-0049, ANTI_CHEAT §7).
type EconomyAdjustment struct{ ent.Schema }

func (EconomyAdjustment) Annotations() []schema.Annotation {
	return gamification("economy_adjustments")
}
func (EconomyAdjustment) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "adjustment_id"}}}
}
func (EconomyAdjustment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("kind").MaxLen(64).Immutable(),
		field.String("currency_code").MaxLen(64).Optional().Nillable().Immutable(),
		field.String("item_id").MaxLen(64).Optional().Nillable().Immutable(),
		field.Int64("delta").Immutable(),
		field.String("reason").MaxLen(1000).Immutable(),
		field.String("ticket_ref").MaxLen(128).Optional().Nillable().Immutable(),
		field.String("status").MaxLen(64).Default("pending_approval"),
		field.UUID("approved_by", uuid.UUID{}).Optional().Nillable(),
		field.UUID("rejected_by", uuid.UUID{}).Optional().Nillable(),
		field.String("reject_reason").MaxLen(500).Optional().Nillable(),
		field.UUID("ledger_entry_id", uuid.UUID{}).Optional().Nillable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
		field.Time("applied_at").Optional().Nillable(),
		field.Time("rejected_at").Optional().Nillable(),
	}
}
func (EconomyAdjustment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("idempotency_key").Unique().StorageKey("economy_adjustments_idem_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "status", "created_at").StorageKey("economy_adjustments_status_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
