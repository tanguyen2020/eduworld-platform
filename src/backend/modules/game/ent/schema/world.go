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

// F071 — world content with versions (G, DATABASE_DESIGN §3.13 pattern, ADR-0043) and the world progress of a student
// (T). Identity tables hold the pointer current_version_id (NULL = not published) and latest_version_no; version tables
// hold the whole payload of the type's JSON Schema plus the columns the queries filter on. A published version never
// changes (trigger game.forbid_published_version_change, R11/R31).

// versionFields are the columns shared by every content version table (the "VersionedContent" columns of
// data-model §1); ref is the item key column.
func versionFields(ref ent.Field) []ent.Field {
	return []ent.Field{
		ref,
		field.Int("version_no").Immutable(),
		field.String("status").MaxLen(64),
		field.JSON("payload", map[string]any{}).Immutable(),
		field.String("payload_sha256").MaxLen(64).Immutable(),
		field.String("change_note").MaxLen(500).Optional().Nillable().Immutable(),
		field.String("snapshot_key").MaxLen(512).Optional().Nillable().Immutable(),
		field.String("snapshot_sha256").MaxLen(64).Optional().Nillable().Immutable(),
		field.UUID("based_on_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("published_at").Optional().Nillable(),
		field.UUID("published_by", uuid.UUID{}).Optional().Nillable(),
	}
}

// pointerFields are the columns of an identity table (current pointer and version counter).
func pointerFields() []ent.Field {
	return []ent.Field{
		field.UUID("current_version_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("latest_version_no").Default(0),
	}
}

func table(name string) []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: name}}
}

// WorldArea — game.world_areas (G, DATABASE_DESIGN §2.8a): one area by its stable area_id (main_hub, learning_zone.math…).
type WorldArea struct{ ent.Schema }

func (WorldArea) Annotations() []schema.Annotation { return table("world_areas") }
func (WorldArea) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "area_id", StringLen: 64}}}
}
func (WorldArea) Fields() []ent.Field { return pointerFields() }

// WorldAreaVersion — game.world_area_versions (G): payload of specs/schemas/world-area.schema.json.
type WorldAreaVersion struct{ ent.Schema }

func (WorldAreaVersion) Annotations() []schema.Annotation { return table("world_area_versions") }
func (WorldAreaVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "area_version_id"}}}
}
func (WorldAreaVersion) Fields() []ent.Field {
	return append(versionFields(field.String("area_id").MaxLen(64).Immutable()),
		field.String("region_code").MaxLen(64).Immutable(),
		field.String("area_class").MaxLen(64).Immutable(),
		field.String("subject_code").MaxLen(64).Optional().Nillable().Immutable(),
	)
}
func (WorldAreaVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("area_id", "version_no").Unique().StorageKey("world_area_versions_no_uq")}
}

// StoryChapter — game.story_chapters (G): one chapter of "Thắp lại Tia Sáng" by chapter_code.
type StoryChapter struct{ ent.Schema }

func (StoryChapter) Annotations() []schema.Annotation { return table("story_chapters") }
func (StoryChapter) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "chapter_code", StringLen: 64}}}
}
func (StoryChapter) Fields() []ent.Field { return pointerFields() }

// StoryChapterVersion — game.story_chapter_versions (G): payload of specs/schemas/story-chapter.schema.json.
type StoryChapterVersion struct{ ent.Schema }

func (StoryChapterVersion) Annotations() []schema.Annotation { return table("story_chapter_versions") }
func (StoryChapterVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "chapter_version_id"}}}
}
func (StoryChapterVersion) Fields() []ent.Field {
	return append(versionFields(field.String("chapter_code").MaxLen(64).Immutable()),
		field.String("region_code").MaxLen(64).Immutable(),
		field.Int("order_no").Immutable(),
		field.String("subject_code").MaxLen(64).Optional().Nillable().Immutable(),
	)
}
func (StoryChapterVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("chapter_code", "version_no").Unique().StorageKey("story_chapter_versions_no_uq")}
}

// NpcDialogue — game.npc_dialogues (G, DATABASE_DESIGN §2.8): one dialogue graph of an NPC (several per NPC, one per
// grade range).
type NpcDialogue struct{ ent.Schema }

func (NpcDialogue) Annotations() []schema.Annotation { return table("npc_dialogues") }
func (NpcDialogue) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "dialogue_id"}}}
}
func (NpcDialogue) Fields() []ent.Field {
	return append([]ent.Field{field.String("npc_id").MaxLen(64).Immutable()}, pointerFields()...)
}
func (NpcDialogue) Indexes() []ent.Index {
	return []ent.Index{index.Fields("npc_id").StorageKey("npc_dialogues_npc_idx")}
}

// NpcDialogueVersion — game.npc_dialogue_versions (G): payload of specs/schemas/npc-dialogue.schema.json; grade_levels
// is copied out to filter by the grade of the student.
type NpcDialogueVersion struct{ ent.Schema }

func (NpcDialogueVersion) Annotations() []schema.Annotation { return table("npc_dialogue_versions") }
func (NpcDialogueVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "dialogue_version_id"}}}
}
func (NpcDialogueVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("dialogue_id", uuid.UUID{}).Immutable()),
		field.String("npc_id").MaxLen(64).Immutable(),
		field.JSON("grade_levels", []int{}).Immutable(),
	)
}
func (NpcDialogueVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("dialogue_id", "version_no").Unique().StorageKey("npc_dialogue_versions_no_uq")}
}

// LevelDefinition — game.level_definitions (G, DATABASE_DESIGN §2.8): the level package (zone bundle) of an area.
type LevelDefinition struct{ ent.Schema }

func (LevelDefinition) Annotations() []schema.Annotation { return table("level_definitions") }
func (LevelDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "level_id"}}}
}
func (LevelDefinition) Fields() []ent.Field {
	return append([]ent.Field{field.String("area_id").MaxLen(64).Immutable()}, pointerFields()...)
}
func (LevelDefinition) Indexes() []ent.Index {
	return []ent.Index{index.Fields("area_id").Unique().StorageKey("level_definitions_area_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// LevelVersion — game.level_versions (G): payload of specs/schemas/level-package.schema.json.
type LevelVersion struct{ ent.Schema }

func (LevelVersion) Annotations() []schema.Annotation { return table("level_versions") }
func (LevelVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "level_version_id"}}}
}
func (LevelVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("level_id", uuid.UUID{}).Immutable()),
		field.String("bundle_id").MaxLen(64).Immutable(),
		field.String("bundle_url").MaxLen(2048).Immutable(),
		field.String("bundle_sha256").MaxLen(64).Immutable(),
		field.Int64("size_bytes").Immutable(),
	)
}
func (LevelVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("level_id", "version_no").Unique().StorageKey("level_versions_no_uq")}
}

// ContentRelease — game.content_releases (GA, DATABASE_DESIGN §3.13, append-only): history of the release pointer of
// every game content item (F071 core; F078 adds rollback/unpublish rows).
type ContentRelease struct{ ent.Schema }

func (ContentRelease) Annotations() []schema.Annotation { return table("content_releases") }
func (ContentRelease) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.AppendOnly{PK: entx.PK{Column: "release_id"}}}
}
func (ContentRelease) Fields() []ent.Field {
	return []ent.Field{
		field.String("item_type").MaxLen(64).Immutable(),
		field.String("item_id").MaxLen(64).Immutable(),
		field.String("action").MaxLen(16).Immutable(),
		field.UUID("from_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("to_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(500).Optional().Nillable().Immutable(),
		field.UUID("released_by", uuid.UUID{}).Immutable(),
		field.Time("released_at").Default(nowUTC).Immutable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
	}
}
func (ContentRelease) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_type", "item_id", "idempotency_key").Unique().StorageKey("content_releases_idem_uq")}
}

// LevelSession — game.level_sessions (T, DATABASE_DESIGN §3.13): one entry of a student into a level with the content
// versions pinned at start (ADR-0043); one active level session per student in a tenant.
type LevelSession struct{ ent.Schema }

func (LevelSession) Annotations() []schema.Annotation { return table("level_sessions") }
func (LevelSession) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "level_session_id"}}}
}
func (LevelSession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("game_session_id", uuid.UUID{}).Immutable(),
		field.UUID("level_id", uuid.UUID{}).Immutable(),
		field.UUID("level_version_id", uuid.UUID{}).Immutable(),
		field.Int("level_version_no").Immutable(),
		field.String("area_id").MaxLen(64).Immutable(),
		field.Int64("manifest_version").Immutable(),
		field.JSON("content_versions", []map[string]any{}).Immutable(),
		field.String("status").MaxLen(32),
		field.Time("started_at").Default(nowUTC).Immutable(),
		field.Time("ended_at").Optional().Nillable(),
	}
}
func (LevelSession) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id").Unique().StorageKey("level_sessions_one_active_uq").
		Annotations(entsql.IndexWhere("status = 'active' AND deleted_at IS NULL"))}
}

// WorldAreaUnlock — game.world_area_unlocks (T, DATABASE_DESIGN §2.8a): an area unlocked for a student (once; the rule
// version that was met is kept). Surrogate key + partial unique (ent has no composite key).
type WorldAreaUnlock struct{ ent.Schema }

func (WorldAreaUnlock) Annotations() []schema.Annotation { return table("world_area_unlocks") }
func (WorldAreaUnlock) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "area_unlock_id"}}}
}
func (WorldAreaUnlock) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("area_id").MaxLen(64).Immutable(),
		field.Int("rule_version").Immutable(),
		field.String("unlocked_by").MaxLen(64).Immutable(),
		field.Time("unlocked_at").Default(nowUTC).Immutable(),
	}
}
func (WorldAreaUnlock) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "area_id").Unique().StorageKey("world_area_unlocks_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// ExplorationProgress — game.exploration_progress (T, DATABASE_DESIGN §2.8a): discoveries and collectibles a student
// found in an area and the completion percentage against the current area version.
type ExplorationProgress struct{ ent.Schema }

func (ExplorationProgress) Annotations() []schema.Annotation { return table("exploration_progress") }
func (ExplorationProgress) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "exploration_id"}}}
}
func (ExplorationProgress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("area_id").MaxLen(64).Immutable(),
		field.JSON("discoveries", []string{}),
		field.JSON("collectibles", []string{}),
		field.Float("completion_pct").Default(0).SchemaType(map[string]string{"postgres": "numeric(5,2)"}),
		field.Time("completed_at").Optional().Nillable(),
	}
}
func (ExplorationProgress) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "area_id").Unique().StorageKey("exploration_progress_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// StoryProgress — game.story_progress (T, DATABASE_DESIGN §2.8a): progress of a student in a chapter (no row = the
// status follows the chapter rules: locked or available).
type StoryProgress struct{ ent.Schema }

func (StoryProgress) Annotations() []schema.Annotation { return table("story_progress") }
func (StoryProgress) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "story_progress_id"}}}
}
func (StoryProgress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("chapter_code").MaxLen(64).Immutable(),
		field.UUID("chapter_version_id", uuid.UUID{}),
		field.String("status").MaxLen(32),
		field.Int("main_quests_completed").Default(0),
		field.JSON("completed_quest_ids", []string{}),
		field.Time("completed_at").Optional().Nillable(),
	}
}
func (StoryProgress) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "chapter_code").Unique().StorageKey("story_progress_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// WorldUnlockFact — game.world_unlock_facts (T, F071 [Confirmed 2026-11-21]): the facts the unlock rules of the areas
// read that no table of module game holds yet — quests completed (until F072), achievements (F075), event
// participation (F077) and the highest level reached (F074) — recorded once from their events.
type WorldUnlockFact struct{ ent.Schema }

func (WorldUnlockFact) Annotations() []schema.Annotation { return table("world_unlock_facts") }
func (WorldUnlockFact) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "fact_id"}}}
}
func (WorldUnlockFact) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("fact_kind").MaxLen(64).Immutable(),
		field.String("fact_ref").MaxLen(64).Immutable(),
		field.Int("fact_value").Default(0),
		field.Time("occurred_at").Default(nowUTC),
	}
}
func (WorldUnlockFact) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "fact_kind", "fact_ref").Unique().StorageKey("world_unlock_facts_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}
