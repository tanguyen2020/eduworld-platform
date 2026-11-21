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

// QuestionPlan — assessment.question_plans (T, F105 data-model, DATABASE_DESIGN §2.6): the questions of one competition
// match chosen by module learning (createQuestionPlan, x-allowed-callers: competition). items pins the question versions
// with the presentation the room showed (seed, shuffled orders) and never copies an answer key: grading reads
// content.question_versions (ADR-0016, ADR-0043). One live plan per match (idempotent createQuestionPlan).
type QuestionPlan struct{ ent.Schema }

func (QuestionPlan) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "assessment", Table: "question_plans"}}
}
func (QuestionPlan) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "plan_id"}}}
}
func (QuestionPlan) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("match_id", uuid.UUID{}).Immutable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.Int16("grade_level").Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.String("plan_version").MaxLen(64).Immutable(),
		field.Int64("seed").Immutable(),
		field.JSON("items", json.RawMessage{}).Immutable(),
		field.JSON("participants", []uuid.UUID{}).Immutable(),
		field.Time("expires_at").Immutable(),
	}
}
func (QuestionPlan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("match_id").Unique().StorageKey("question_plans_match_uq").Annotations(live()),
	}
}
