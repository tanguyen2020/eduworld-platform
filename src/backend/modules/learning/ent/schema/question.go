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

// Question bank tables (F022, specs/022-question-bank-core/data-model.md, QUESTION_BANK §2, DATABASE_DESIGN §2.5,
// §3.3). Hybrid content (H) visible to a tenant through the assignment of the question (resource_type question);
// size CHECKs of the jsonb columns, the immutability and release-target triggers and the delivery view
// content.question_delivery_v (no answer column) are hand-written in the migration.

const questionResource = "question"

// Question — content.questions (H): the question item and its release pointer.
type Question struct{ ent.Schema }

func (Question) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "questions"}}
}
func (Question) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entx.HybridContent{ResourceType: questionResource, PK: entx.PK{Column: "question_id"}},
		ContentItemMixin{},
	}
}
func (Question) Indexes() []ent.Index {
	return []ent.Index{index.Fields("owner_scope", "tenant_id").StorageKey("questions_owner_idx").Annotations(live())}
}

// QuestionVersion — content.question_versions (H): one version; render_payload never holds answer data, the
// server-only parts are answer_key, scoring_rule, explanation, option_feedback, hints, min_response_ms.
type QuestionVersion struct{ ent.Schema }

func (QuestionVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "question_versions"}}
}
func (QuestionVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entx.HybridContent{ResourceType: questionResource, PK: entx.PK{Column: "question_version_id"}, ResourceIDColumn: "question_id"},
		ContentVersionMixin{},
	}
}
func (QuestionVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("question_id", uuid.UUID{}).Immutable(),
		field.String("type").MaxLen(64),
		field.JSON("stem", json.RawMessage{}),
		field.JSON("render_payload", json.RawMessage{}),
		field.JSON("answer_key", json.RawMessage{}),
		field.JSON("scoring_rule", json.RawMessage{}),
		field.JSON("explanation", json.RawMessage{}).Optional(),
		field.JSON("option_feedback", json.RawMessage{}).Optional(),
		field.JSON("hints", json.RawMessage{}).Optional(),
		field.JSON("worked_example_ref", json.RawMessage{}).Optional(),
		field.Int32("min_response_ms").Optional().Nillable(),
		field.Float("max_score").SchemaType(map[string]string{"postgres": "numeric(7,2)"}).Default(1),
		field.Bool("exam_only").Default(false),
		field.Bool("efficacy_instrument").Default(false),
		field.JSON("math_assets", json.RawMessage{}).Optional(),
		field.JSON("tags", []string{}).Default([]string{}),
		field.Int16("difficulty"),
		field.JSON("grade_levels", []int{}),
		field.UUID("subject_id", uuid.UUID{}),
		field.UUID("topic_id", uuid.UUID{}),
		field.JSON("media_refs", []uuid.UUID{}).Default([]uuid.UUID{}),
		field.Bytes("content_hash").SchemaType(map[string]string{"postgres": "bytea"}).Optional(),
	}
}
func (QuestionVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("question_id", "version_no").Unique().StorageKey("question_versions_no_uq"),
		index.Fields("question_id").Unique().StorageKey("question_versions_open_uq").
			Annotations(entsql.IndexWhere("status IN ('draft','in_review') AND deleted_at IS NULL")),
		index.Fields("tenant_id", "subject_id", "topic_id", "difficulty").StorageKey("question_versions_delivery_idx").
			Annotations(entsql.IndexWhere("status = 'published' AND deleted_at IS NULL")),
	}
}

// QuestionVersionObjective — content.question_version_objectives (H): LOs assessed by a version (CONTENT_MODEL §3).
// A surrogate key replaces the composite key of DATABASE_DESIGN (soft-delete), question_id carries the assignment.
type QuestionVersionObjective struct{ ent.Schema }

func (QuestionVersionObjective) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "question_version_objectives"}}
}
func (QuestionVersionObjective) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entx.HybridContent{ResourceType: questionResource, PK: entx.PK{Column: "question_version_objective_id"}, ResourceIDColumn: "question_id"},
	}
}
func (QuestionVersionObjective) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("question_id", uuid.UUID{}).Immutable(),
		field.UUID("question_version_id", uuid.UUID{}).Immutable(),
		field.UUID("objective_id", uuid.UUID{}).Immutable(),
	}
}
func (QuestionVersionObjective) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("question_version_id", "objective_id").Unique().StorageKey("question_version_objectives_uq").Annotations(live()),
		index.Fields("objective_id").StorageKey("question_version_objectives_lo_idx").Annotations(live()),
	}
}
