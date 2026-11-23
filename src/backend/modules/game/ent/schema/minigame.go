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

// F073 — mini-game engine: mini-game configs are platform game content with versions on the F071 release core (G,
// ADR-0043), assigned to tenants (infra.tenant_resource_assignments resource_type minigame_config, ADR-0029); the plays
// of a student are a tenant table (T). A config holds no question: the questions come from the Question Bank of module
// learning through question_source (MINIGAME_ENGINE §1, M-1).

// MiniGameConfig — game.minigame_configs (G, DATABASE_DESIGN §2.8): identity of a platform mini-game config and its
// release pointer.
type MiniGameConfig struct{ ent.Schema }

func (MiniGameConfig) Annotations() []schema.Annotation { return table("minigame_configs") }
func (MiniGameConfig) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "minigame_config_id"}}}
}
func (MiniGameConfig) Fields() []ent.Field {
	return append([]ent.Field{
		field.String("code").MaxLen(64).Optional().Nillable(),
		field.String("owner_scope").MaxLen(64).Default("platform").Immutable(),
	}, pointerFields()...)
}
func (MiniGameConfig) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("minigame_configs_code_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// MiniGameConfigVersion — game.minigame_config_versions (G): payload of specs/schemas/minigame-config.schema.json plus the
// columns the start and the catalog filter on (type, grades, subject) and the question types the source can return
// (result of validateMinigameQuestionSource at publish).
type MiniGameConfigVersion struct{ ent.Schema }

func (MiniGameConfigVersion) Annotations() []schema.Annotation {
	return table("minigame_config_versions")
}
func (MiniGameConfigVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "minigame_config_version_id"}}}
}
func (MiniGameConfigVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("minigame_config_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.String("minigame_type").MaxLen(64).Immutable(),
		field.JSON("grade_levels", []int{}).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.JSON("question_types", []string{}).Immutable(),
	)
}
func (MiniGameConfigVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("minigame_config_id", "version_no").Unique().StorageKey("minigame_config_versions_no_uq")}
}

// MiniGamePlay — game.minigame_plays (T, data-model F073): one play of a student on a pinned config version; the
// learning attempt is graded by module learning, the result arrives through activity.graded (learning_score, game_score,
// completion_met) and is recorded once; play_date (ICT) counts limits.max_plays_per_day.
type MiniGamePlay struct{ ent.Schema }

func (MiniGamePlay) Annotations() []schema.Annotation { return table("minigame_plays") }
func (MiniGamePlay) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "play_id"}}}
}
func (MiniGamePlay) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("minigame_config_id", uuid.UUID{}).Immutable(),
		field.UUID("minigame_config_version_id", uuid.UUID{}).Immutable(),
		field.String("minigame_type").MaxLen(64).Immutable(),
		field.UUID("attempt_id", uuid.UUID{}).Immutable(),
		field.String("context_type").MaxLen(64).Immutable(),
		field.UUID("context_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("play_date").MaxLen(10).Immutable(),
		field.String("status").MaxLen(64),
		field.Float("learning_score").Optional().Nillable().SchemaType(map[string]string{"postgres": "numeric(7,2)"}),
		field.Float("max_score").Optional().Nillable().SchemaType(map[string]string{"postgres": "numeric(7,2)"}),
		field.Int("game_score").Optional().Nillable(),
		field.Int("correct_count").Optional().Nillable(),
		field.Int("item_count").Optional().Nillable(),
		field.Bool("completion_met").Optional().Nillable(),
		field.Bool("rewards_eligible").Optional().Nillable(),
		field.Time("started_at").Default(nowUTC).Immutable(),
		field.Time("completed_at").Optional().Nillable(),
	}
}
func (MiniGamePlay) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("tenant_id", "attempt_id").Unique().StorageKey("minigame_plays_attempt_uq").Annotations(live),
		index.Fields("tenant_id", "student_id", "minigame_config_id", "play_date").StorageKey("minigame_plays_day_idx").Annotations(live),
	}
}
