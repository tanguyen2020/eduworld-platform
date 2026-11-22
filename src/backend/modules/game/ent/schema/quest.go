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

// F072 — quest engine: quests are game content with versions on the F071 release core (G, ADR-0043), assigned to
// tenants (infra.tenant_resource_assignments, ADR-0029); the progress of a student on a pinned version, the link
// between a learning attempt and a quest step and the daily/weekly grants are tenant tables (T).

// QuestDefinition — game.quest_definitions (G, DATABASE_DESIGN §2.8): identity of a platform quest and its release
// pointer.
type QuestDefinition struct{ ent.Schema }

func (QuestDefinition) Annotations() []schema.Annotation { return table("quest_definitions") }
func (QuestDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "quest_id"}}}
}
func (QuestDefinition) Fields() []ent.Field {
	return append([]ent.Field{
		field.String("code").MaxLen(64).Optional().Nillable(),
		field.String("owner_scope").MaxLen(64).Default("platform").Immutable(),
	}, pointerFields()...)
}
func (QuestDefinition) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("quest_definitions_code_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// QuestVersion — game.quest_versions (G): payload of specs/schemas/quest.schema.json v2 plus the columns the player
// queries filter on (quest type, grades, subject, area, availability).
type QuestVersion struct{ ent.Schema }

func (QuestVersion) Annotations() []schema.Annotation { return table("quest_versions") }
func (QuestVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "quest_version_id"}}}
}
func (QuestVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("quest_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.String("quest_type").MaxLen(64).Immutable(),
		field.JSON("grade_levels", []int{}).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.String("subject_code").MaxLen(64).Optional().Nillable().Immutable(),
		field.String("area_id").MaxLen(64).Optional().Nillable().Immutable(),
		field.String("giver_npc_id").MaxLen(64).Optional().Nillable().Immutable(),
		field.String("story_chapter_code").MaxLen(64).Optional().Nillable().Immutable(),
		field.String("start_trigger").MaxLen(64).Immutable(),
		field.JSON("objective_ids", []string{}).Immutable(),
		field.Time("available_from").Optional().Nillable().Immutable(),
		field.Time("available_to").Optional().Nillable().Immutable(),
	)
}
func (QuestVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("quest_id", "version_no").Unique().StorageKey("quest_versions_no_uq")}
}

// QuestProgress — game.quest_progress (T, DATABASE_DESIGN §2.8, QUEST_SYSTEM §2.2): the progress of a student on the
// quest version pinned at accept (ADR-0043). One active progress per quest and student; one active or completed
// progress per period of a periodic quest.
type QuestProgress struct{ ent.Schema }

func (QuestProgress) Annotations() []schema.Annotation { return table("quest_progress") }
func (QuestProgress) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "quest_progress_id"}}}
}
func (QuestProgress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("quest_id", uuid.UUID{}).Immutable(),
		field.UUID("quest_version_id", uuid.UUID{}).Immutable(),
		field.String("quest_type").MaxLen(64).Immutable(),
		field.String("status").MaxLen(64),
		field.JSON("step_states", map[string]any{}),
		field.String("start_trigger").MaxLen(64).Immutable(),
		field.String("period_key").MaxLen(16).Optional().Nillable().Immutable(),
		field.Int("completion_no").Default(1).Immutable(),
		field.UUID("party_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("started_at").Default(nowUTC).Immutable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable(),
		field.Time("deadline_at").Optional().Nillable(),
		field.Time("failed_at").Optional().Nillable(),
		field.Time("ended_at").Optional().Nillable(),
	}
}
func (QuestProgress) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "student_id", "quest_id").Unique().StorageKey("quest_progress_one_active_uq").
			Annotations(entsql.IndexWhere("status = 'active' AND deleted_at IS NULL")),
		index.Fields("tenant_id", "student_id", "quest_id", "period_key").Unique().StorageKey("quest_progress_period_uq").
			Annotations(entsql.IndexWhere("period_key IS NOT NULL AND status IN ('active', 'completed') AND deleted_at IS NULL")),
		index.Fields("tenant_id", "student_id", "status").StorageKey("quest_progress_student_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// QuestStepAttempt — game.quest_step_attempts (T): a learning attempt opened for a quest step (createAttempt of module
// learning) and its pass_rule result; started_on (ICT) counts max_attempts_per_day.
type QuestStepAttempt struct{ ent.Schema }

func (QuestStepAttempt) Annotations() []schema.Annotation { return table("quest_step_attempts") }
func (QuestStepAttempt) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "quest_step_attempt_id"}}}
}
func (QuestStepAttempt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("quest_progress_id", uuid.UUID{}).Immutable(),
		field.String("step_id").MaxLen(64).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("attempt_id", uuid.UUID{}).Immutable(),
		field.UUID("content_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("started_on").MaxLen(10).Immutable(),
		field.String("status").MaxLen(64),
		field.Int("correct_count").Optional().Nillable(),
		field.Int("item_count").Optional().Nillable(),
		field.Time("graded_at").Optional().Nillable(),
	}
}
func (QuestStepAttempt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "attempt_id").Unique().StorageKey("quest_step_attempts_attempt_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "quest_progress_id", "step_id", "started_on").StorageKey("quest_step_attempts_day_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// PeriodicQuestGrant — game.periodic_quest_grants (T, QUEST_SYSTEM §2.4): the daily or weekly quests granted to a
// student for one period (one row per student, kind and period).
type PeriodicQuestGrant struct{ ent.Schema }

func (PeriodicQuestGrant) Annotations() []schema.Annotation { return table("periodic_quest_grants") }
func (PeriodicQuestGrant) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "grant_id"}}}
}
func (PeriodicQuestGrant) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("kind").MaxLen(64).Immutable(),
		field.String("period_key").MaxLen(16).Immutable(),
		field.JSON("quest_ids", []string{}).Immutable(),
		field.Time("granted_at").Default(nowUTC).Immutable(),
	}
}
func (PeriodicQuestGrant) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "kind", "period_key").Unique().StorageKey("periodic_quest_grants_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// TenantResourceAssignment — infra.tenant_resource_assignments (DATABASE_DESIGN §3.0, ADR-0029) of eduworld_game: a
// platform resource of module game (quest; F073 minigame_config, item) assigned to a tenant. Soft-delete = revoked.
// No RLS (schema infra, ADR-0089 §4): game_app only reads it with an explicit tenant condition; game_platform writes it.
type TenantResourceAssignment struct{ ent.Schema }

func (TenantResourceAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "infra", Table: "tenant_resource_assignments"}}
}
func (TenantResourceAssignment) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "assignment_id"}}}
}
func (TenantResourceAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.String("resource_type").MaxLen(64).Immutable(),
		field.UUID("resource_id", uuid.UUID{}).Immutable(),
		field.UUID("assigned_by", uuid.UUID{}).Immutable(),
		field.Time("assigned_at").Default(nowUTC).Immutable(),
	}
}
func (TenantResourceAssignment) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("resource_type", "resource_id", "tenant_id").Unique().StorageKey("tra_active_uq").Annotations(live),
		index.Fields("tenant_id", "resource_type", "resource_id").StorageKey("tra_tenant_idx").Annotations(live),
	}
}
