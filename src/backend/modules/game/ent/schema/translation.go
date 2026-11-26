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

// F082 — translation of game content (specs/082-game-studio-chat-assets-translations: data-model §1; DATABASE_DESIGN
// §2.16; CONTENT_MODEL §8; ADR-0043, ADR-0056). Platform data (G, no tenant): the translation state of each console
// version and the jobs exporting/importing translation files.

// ConfigTranslationState — game.config_translation_states (G): translation_status of one game.config_versions row, the
// pointers missing a required language and the meta of the last translation of each text (research R1, R3).
type ConfigTranslationState struct{ ent.Schema }

func (ConfigTranslationState) Annotations() []schema.Annotation {
	return table("config_translation_states")
}
func (ConfigTranslationState) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "config_version_id"}}}
}
func (ConfigTranslationState) Fields() []ent.Field {
	return []ent.Field{
		field.String("config_type").MaxLen(64).Immutable(),
		field.UUID("config_id", uuid.UUID{}).Immutable(),
		field.String("translation_status").MaxLen(64),
		field.JSON("missing_pointers", []string{}),
		field.JSON("outdated_pointers", []map[string]any{}),
		field.JSON("translation_meta", map[string]any{}),
		field.Time("computed_at"),
	}
}
func (ConfigTranslationState) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("config_type", "translation_status").StorageKey("config_translation_states_status_idx").Annotations(live),
		index.Fields("config_id").StorageKey("config_translation_states_item_idx").Annotations(live),
	}
}

// TranslationJob — game.translation_jobs (G): one export or import of a translation file (XLIFF 2.1 / CSV), run by the
// worker (profile jobs); only its requester reads it (System Admin: every job, F082-AC-16).
type TranslationJob struct{ ent.Schema }

func (TranslationJob) Annotations() []schema.Annotation { return table("translation_jobs") }
func (TranslationJob) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "job_id"}}}
}
func (TranslationJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("kind").MaxLen(64).Immutable(),
		field.String("status").MaxLen(64),
		field.String("format").MaxLen(64).Immutable(),
		field.String("source_language").MaxLen(8).Immutable(),
		field.String("target_language").MaxLen(8).Immutable(),
		field.Bool("only_missing").Default(true).Immutable(),
		field.Bool("dry_run").Default(false).Immutable(),
		field.JSON("request", map[string]any{}).Immutable(),
		field.UUID("input_asset_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("result_storage_key").MaxLen(512).Optional().Nillable(),
		field.JSON("report", map[string]any{}).Optional(),
		field.String("error_message").MaxLen(1000).Optional().Nillable(),
		field.UUID("requested_by", uuid.UUID{}).Immutable(),
		field.String("idempotency_key").MaxLen(64).Optional().Nillable().Immutable(),
		field.Int("attempts").Default(0),
		field.Time("started_at").Optional().Nillable(),
		field.Time("finished_at").Optional().Nillable(),
	}
}
func (TranslationJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("requested_by", "idempotency_key").Unique().StorageKey("translation_jobs_idem_uq").
			Annotations(entsql.IndexWhere("idempotency_key IS NOT NULL AND deleted_at IS NULL")),
		index.Fields("status", "created_at").StorageKey("translation_jobs_queue_idx").
			Annotations(entsql.IndexWhere("status IN ('queued', 'running') AND deleted_at IS NULL")),
	}
}
