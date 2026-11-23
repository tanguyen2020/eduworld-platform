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

// GameProfile — game.game_profiles (T, DATABASE_DESIGN §2.8, F070): the game profile of one student in one tenant
// (nickname, avatar, level read from F074). Created by EnsureGameProfile in the transaction of createGameSession
// (F014). One live profile per (tenant, student): a surrogate key game_profile_id replaces the student_id key of
// DATABASE_DESIGN §2.8 because a student may hold memberships in several tenants.
// The nickname is unique in the tenant (case-insensitive partial index written by hand: ent cannot express lower()). Never returns or stores a real name, username, school or class (R16).
type GameProfile struct{ ent.Schema }

func (GameProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "game_profiles"}}
}
func (GameProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "game_profile_id"}}}
}
func (GameProfile) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}),
		field.String("nickname").MaxLen(32).Optional().Nillable(),
		field.Int("level").Default(1),
		field.Int64("xp_total").Default(0),
		field.Int("curve_version").Optional().Nillable(),
		field.String("title_code").MaxLen(64).Optional().Nillable(),
		field.JSON("avatar_config", map[string]any{}).Optional(),
		field.String("avatar_config_hash").MaxLen(128).Optional().Nillable(),
		field.Time("avatar_created_at").Optional().Nillable(),
		field.String("last_area_id").MaxLen(64).Optional().Nillable(),
		field.JSON("settings", map[string]any{}).Default(map[string]any{}),
		field.String("preferred_language").MaxLen(64).Optional().Nillable(),
		field.Time("last_game_login_at").Optional().Nillable(),
		field.String("grade_transition_year").MaxLen(16).Optional().Nillable(), // F074: school year of the last ApplyGradeTransition
	}
}
func (GameProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "student_id").Unique().StorageKey("game_profiles_student_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "school_id").StorageKey("game_profiles_tenant_school_idx"),
	}
}

// NicknameChange — game.nickname_changes (TA, F070): history of every nickname set (initial, player_change, reset).
// Append-only: the migration revokes UPDATE, DELETE, TRUNCATE.
type NicknameChange struct{ ent.Schema }

func (NicknameChange) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "nickname_changes"}}
}
func (NicknameChange) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "nickname_change_id"}}}
}
func (NicknameChange) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("old_nickname").MaxLen(32).Optional().Nillable().Immutable(),
		field.String("new_nickname").MaxLen(32).Immutable(),
		field.String("reason").MaxLen(64).Immutable(),
		field.UUID("changed_by", uuid.UUID{}).Immutable(),
		field.Time("changed_at").Default(nowUTC).Immutable(),
	}
}
func (NicknameChange) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "changed_at").StorageKey("nickname_changes_student_idx")}
}

// AvatarPreset — game.avatar_presets (T, DATABASE_DESIGN §2.8a): saved outfits of a student, one row per slot
// 1..saved_outfits_max. A surrogate key plus a partial unique (student_id, slot) replaces the composite key of
// DATABASE_DESIGN (ent has no composite primary keys; soft-delete keeps the row unique among live rows).
type AvatarPreset struct{ ent.Schema }

func (AvatarPreset) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "avatar_presets"}}
}
func (AvatarPreset) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "avatar_preset_id"}}}
}
func (AvatarPreset) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.Int16("slot").Immutable(),
		field.String("name").MaxLen(64).Optional().Nillable(),
		field.JSON("avatar_config", map[string]any{}),
	}
}
func (AvatarPreset) Indexes() []ent.Index {
	return []ent.Index{index.Fields("student_id", "slot").Unique().StorageKey("avatar_presets_slot_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// CharacterCatalog — game.character_catalogs (G, DATABASE_DESIGN §3.13 pattern, ADR-0043): the platform content
// "character catalog" (code default) and the pointer to its current published version.
type CharacterCatalog struct{ ent.Schema }

func (CharacterCatalog) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "character_catalogs"}}
}
func (CharacterCatalog) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "catalog_id"}}}
}
func (CharacterCatalog) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64),
		field.UUID("current_version_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("latest_version_no").Default(0),
	}
}
func (CharacterCatalog) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("character_catalogs_code_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// CharacterCatalogVersion — game.character_catalog_versions (G): one immutable version of the catalog payload
// (specs/schemas/character-catalog.schema.json). Published versions never change (R11, R31).
type CharacterCatalogVersion struct{ ent.Schema }

func (CharacterCatalogVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "character_catalog_versions"}}
}
func (CharacterCatalogVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "catalog_version_id"}}}
}
func (CharacterCatalogVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("catalog_id", uuid.UUID{}).Immutable(),
		field.Int("version_no").Immutable(),
		field.String("status").MaxLen(64),
		field.JSON("payload", map[string]any{}).Immutable(),
		field.String("payload_sha256").MaxLen(64).Immutable(),
		field.String("change_note").MaxLen(500).Optional().Nillable(),
		field.String("snapshot_key").MaxLen(512).Optional().Nillable(),
		field.String("snapshot_sha256").MaxLen(64).Optional().Nillable(),
		field.Time("published_at").Optional().Nillable(),
	}
}
func (CharacterCatalogVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("catalog_id", "version_no").Unique().StorageKey("character_catalog_versions_no_uq")}
}

// ItemDefinition — game.item_definitions (G, INVENTORY_ECONOMY §8): a cosmetic item by code (hair_ponytail_01) and
// its current version. F070 creates the table and the default items; F075 extends it (shop, rewards).
type ItemDefinition struct{ ent.Schema }

func (ItemDefinition) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "item_definitions"}}
}
func (ItemDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "item_id", StringLen: 64}}}
}
func (ItemDefinition) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("current_version_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("latest_version_no").Default(0),
	}
}

// ItemDefinitionVersion — game.item_definition_versions (G): kind, equip slot, rarity, bilingual name, sources
// (default = free for every student), asset references. Immutable once published.
type ItemDefinitionVersion struct{ ent.Schema }

func (ItemDefinitionVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "item_definition_versions"}}
}
func (ItemDefinitionVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "item_version_id"}}}
}
func (ItemDefinitionVersion) Fields() []ent.Field {
	return []ent.Field{
		field.String("item_id").MaxLen(64).Immutable(),
		field.Int("version_no").Immutable(),
		field.String("kind").MaxLen(64).Immutable(),
		field.String("equip_slot").MaxLen(64).Immutable(),
		field.String("rarity").MaxLen(64).Immutable(),
		field.JSON("name", map[string]string{}).Immutable(),
		field.JSON("sources", []string{}).Immutable(),
		field.JSON("asset_refs", map[string]any{}).Optional().Immutable(),
		field.String("status").MaxLen(64),
	}
}
func (ItemDefinitionVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_id", "version_no").Unique().StorageKey("item_definition_versions_no_uq")}
}

// InventoryItem — game.inventory_items (T, DATABASE_DESIGN §2.8): an item a student owns. F070 only reads it (to
// check ownership); F075 writes it. Surrogate key + partial unique (student_id, item_id) (ent: no composite key).
type InventoryItem struct{ ent.Schema }

func (InventoryItem) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "inventory_items"}}
}
func (InventoryItem) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "inventory_item_id"}}}
}
func (InventoryItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("item_id").MaxLen(64).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("quantity").Default(1),
		field.Time("acquired_at").Default(nowUTC),
		field.UUID("reward_grant_id", uuid.UUID{}).Optional().Nillable(),
	}
}
func (InventoryItem) Indexes() []ent.Index {
	return []ent.Index{index.Fields("student_id", "item_id").Unique().StorageKey("inventory_items_student_item_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}
