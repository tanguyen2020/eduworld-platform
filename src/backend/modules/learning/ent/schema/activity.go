package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Activity Engine of F030 (specs/030-activity-attempt-api/data-model.md, ACTIVITY_ENGINE §4–§7, DATABASE_DESIGN §2.6,
// §2.6a, §3.4): learning activities are a composed content type like the F024 types (item + version tables sharing
// docVersionFields; activity_type, config and the activity fields live in body, activity_type is also a generated
// column of the version table), attempts with their items and evaluations. The partitioned append-only tables
// assessment.attempt_answers and learning.learning_events, the CHECKs, RLS, triggers, _live views and grants are
// hand-written in the migration 20261130000001_f030_activity_attempt.sql.

// Activity — content.activities (H).
type Activity struct{ ent.Schema }

func (Activity) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "activities"}}
}
func (Activity) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "activity", PK: entx.PK{Column: "activity_id"}}, ContentItemMixin{}}
}

// ActivityVersion — content.activity_versions (H through the activity).
type ActivityVersion struct{ ent.Schema }

func (ActivityVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "activity_versions"}}
}
func (ActivityVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "activity", PK: entx.PK{Column: "activity_version_id"}, ResourceIDColumn: "activity_id"},
		ContentVersionMixin{CurriculumOptional: true}, WorkflowFields{}}
}
func (ActivityVersion) Fields() []ent.Field { return docVersionFields("activity_id") }
func (ActivityVersion) Indexes() []ent.Index {
	return docVersionIndexes("activity_id", "activity_versions")
}

// Attempt — assessment.attempts (T): one student working one pinned version (ACTIVITY_ENGINE §6, ADR-0043, ADR-0059).
// Times are server times; score, max_score and duration_ms are computed by the server only (ADR-0016).
type Attempt struct{ ent.Schema }

func (Attempt) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "assessment", Table: "attempts"}}
}
func (Attempt) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{PK: entx.PK{Column: "attempt_id"}}} }
func (Attempt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.String("context_type").MaxLen(64).Immutable(),
		field.UUID("context_id", uuid.UUID{}).Immutable(),
		field.UUID("activity_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("activity_type").MaxLen(32).Immutable(),
		// source_kind is the content type of source_version_id: activity or question_set; selection pins question
		// versions only (source_version_id = attempt id) [Proposed].
		field.String("source_kind").MaxLen(32).Immutable(),
		field.UUID("source_version_id", uuid.UUID{}).Immutable(),
		field.String("status").MaxLen(64).Default("in_progress"),
		field.Int64("seed").Immutable(),
		field.Time("started_at").Immutable(),
		field.Time("deadline_at").Optional().Nillable().Immutable(),
		field.Time("submitted_at").Optional().Nillable(),
		field.Time("graded_at").Optional().Nillable(),
		field.Time("paused_at").Optional().Nillable(),
		field.Int64("paused_ms").Default(0),
		field.Time("last_activity_at"),
		field.String("feedback_timing").MaxLen(64).Immutable(),
		field.String("display_language").MaxLen(64).Immutable(),
		field.String("submit_reason").MaxLen(64).Optional().Nillable(),
		field.Float("score").SchemaType(map[string]string{"postgres": "numeric(7,2)"}).Optional().Nillable(),
		field.Float("max_score").SchemaType(map[string]string{"postgres": "numeric(7,2)"}).Optional().Nillable(),
		field.Int64("duration_ms").Optional().Nillable(),
		field.Int64("client_elapsed_ms").Optional().Nillable(),
		field.Int("item_count").NonNegative().Immutable(),
		field.JSON("objective_ids", []uuid.UUID{}).Default([]uuid.UUID{}),
		field.UUID("recommendation_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
		// F042: the paper of an exam attempt, the device holding it (hash), the client platform and game session (EXAM_ENGINE §12).
		field.UUID("exam_paper_id", uuid.UUID{}).Optional().Nillable(),
		field.String("active_device_token").MaxLen(128).Optional().Nillable(),
		field.String("client_platform").MaxLen(64).Optional().Nillable(),
		field.UUID("game_session_id", uuid.UUID{}).Optional().Nillable(),
		// F073: the game score and current correct streak of a mini-game attempt (server computed, never the learning
		// score) and the pinned MiniGameConfigVersion (projection ref.minigame_config_versions).
		field.Int("game_score").SchemaType(map[string]string{"postgres": "integer"}).Optional().Nillable(),
		field.Int("game_streak").SchemaType(map[string]string{"postgres": "integer"}).Optional().Nillable(),
		field.UUID("minigame_config_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
	}
}
func (Attempt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "student_id", "started_at").StorageKey("attempts_student_idx").Annotations(live()),
		index.Fields("tenant_id", "context_type", "context_id").StorageKey("attempts_context_idx").Annotations(live()),
		index.Fields("student_id", "idempotency_key").Unique().StorageKey("attempts_idem_uq"),
		index.Fields("student_id", "source_version_id").Unique().StorageKey("attempts_practice_open_uq").
			Annotations(entsql.IndexWhere("status = 'in_progress' AND mode = 'practice' AND deleted_at IS NULL")),
		index.Fields("student_id", "context_type", "context_id").Unique().StorageKey("attempts_one_open_uq").
			Annotations(entsql.IndexWhere("status = 'in_progress' AND mode IN ('exam','assignment') AND deleted_at IS NULL")),
		index.Fields("status", "deadline_at").StorageKey("attempts_deadline_idx").
			Annotations(entsql.IndexWhere("status = 'in_progress' AND deadline_at IS NOT NULL AND deleted_at IS NULL")),
		index.Fields("status", "last_activity_at").StorageKey("attempts_activity_idx").
			Annotations(entsql.IndexWhere("status = 'in_progress' AND deleted_at IS NULL")),
	}
}

// AttemptItem — assessment.attempt_items (T): one delivered question with its pinned version, presentation (order of the
// shuffled lists, drawn parameters) and the result of its last scored answer (ACTIVITY_ENGINE §6.4, §7.2). A surrogate
// key replaces the composite key of DATABASE_DESIGN §2.6 (one entx mixin, soft-delete), unique (attempt_id, position).
type AttemptItem struct{ ent.Schema }

func (AttemptItem) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "assessment", Table: "attempt_items"}}
}
func (AttemptItem) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "attempt_item_id"}}}
}
func (AttemptItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("attempt_id", uuid.UUID{}).Immutable(),
		field.Int("position").Positive().Immutable(),
		field.UUID("question_version_id", uuid.UUID{}).Immutable(),
		field.String("question_type").MaxLen(64).Immutable(),
		field.JSON("objective_ids", []uuid.UUID{}).Default([]uuid.UUID{}).Immutable(),
		field.JSON("presentation", json.RawMessage{}).Immutable(),
		field.String("presented_payload_hash").MaxLen(64).Immutable(),
		field.Float("max_score").SchemaType(map[string]string{"postgres": "numeric(6,2)"}).Immutable(),
		field.Int("answer_count").Default(0).NonNegative(),
		field.Bool("final_is_correct").Optional().Nillable(),
		field.Float("final_score").SchemaType(map[string]string{"postgres": "numeric(6,2)"}).Optional().Nillable(),
		field.Int("hints_revealed").Default(0).NonNegative(),
	}
}
func (AttemptItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("attempt_id", "position").Unique().StorageKey("attempt_items_position_uq"),
	}
}

// Evaluation — assessment.evaluations (T): the grading of an attempt, exactly one effective per attempt (ACTIVITY_ENGINE
// §7.3). F030 writes source system, status final; re-evaluation (F031) adds rows and supersedes the old one.
type Evaluation struct{ ent.Schema }

func (Evaluation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "assessment", Table: "evaluations"}}
}
func (Evaluation) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "evaluation_id"}}}
}
func (Evaluation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("attempt_id", uuid.UUID{}).Immutable(),
		field.String("source").MaxLen(16).Immutable(),
		field.UUID("regrade_request_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("supersedes_evaluation_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.JSON("details", json.RawMessage{}).Optional().Immutable(),
		field.String("status").MaxLen(16).Default("final"),
		field.Float("score").SchemaType(map[string]string{"postgres": "numeric(7,2)"}).Immutable(),
		field.Float("max_score").SchemaType(map[string]string{"postgres": "numeric(7,2)"}).Immutable(),
		field.Bool("passed").Optional().Nillable().Immutable(),
		field.String("feedback").MaxLen(4000).Optional().Nillable().Immutable(),
		field.UUID("triggered_by", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(500).Optional().Nillable().Immutable(),
		field.Bool("effective").Default(true),
	}
}
func (Evaluation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("attempt_id").Unique().StorageKey("evaluations_effective_uq").
			Annotations(entsql.IndexWhere("effective AND deleted_at IS NULL")),
		index.Fields("tenant_id", "attempt_id").StorageKey("evaluations_attempt_idx").Annotations(live()),
	}
}
