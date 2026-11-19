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

// Composed content types of F024 (specs/024-question-set-lesson-blueprint/data-model.md, CONTENT_MODEL §2, DATABASE_DESIGN
// §2.5): question sets, lessons, learning materials and assessment blueprints. Every type is an item table (H, release
// pointer) and a version table (H through the item) with the same columns: shared metadata in columns and the
// type-specific document in body (definition, blocks, sections…, validated by the use case and the JSON Schemas of
// specs/schemas). The generated search column search_tsv, the CHECKs, RLS and the immutability triggers are
// hand-written in the migration.

// docVersionFields are the columns shared by the four version tables; itemColumn is the id of the item.
func docVersionFields(itemColumn string) []ent.Field {
	return []ent.Field{
		field.UUID(itemColumn, uuid.UUID{}).Immutable(),
		field.JSON("title", json.RawMessage{}),
		field.JSON("summary", json.RawMessage{}).Optional(),
		field.UUID("subject_id", uuid.UUID{}).Optional().Nillable(),
		field.JSON("grade_levels", []int{}).Default([]int{}),
		field.JSON("topic_ids", []uuid.UUID{}).Default([]uuid.UUID{}),
		field.JSON("objective_ids", []uuid.UUID{}).Default([]uuid.UUID{}),
		field.JSON("media_refs", []uuid.UUID{}).Default([]uuid.UUID{}),
		field.JSON("worked_example_ids", []string{}).Default([]string{}),
		field.JSON("body", json.RawMessage{}),
	}
}

func docVersionIndexes(itemColumn, prefix string) []ent.Index {
	return []ent.Index{
		index.Fields(itemColumn, "version_no").Unique().StorageKey(prefix + "_no_uq"),
		index.Fields(itemColumn).Unique().StorageKey(prefix + "_open_uq").
			Annotations(entsql.IndexWhere("status IN ('draft','in_review') AND deleted_at IS NULL")),
		index.Fields("tenant_id", "subject_id").StorageKey(prefix + "_subject_idx").Annotations(live()),
	}
}

// QuestionSet — content.question_sets (H).
type QuestionSet struct{ ent.Schema }

func (QuestionSet) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "question_sets"}}
}
func (QuestionSet) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "question_set", PK: entx.PK{Column: "question_set_id"}}, ContentItemMixin{}}
}

// QuestionSetVersion — content.question_set_versions (H through the set).
type QuestionSetVersion struct{ ent.Schema }

func (QuestionSetVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "question_set_versions"}}
}
func (QuestionSetVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "question_set", PK: entx.PK{Column: "question_set_version_id"}, ResourceIDColumn: "question_set_id"},
		ContentVersionMixin{CurriculumOptional: true}, WorkflowFields{}}
}
func (QuestionSetVersion) Fields() []ent.Field { return docVersionFields("question_set_id") }
func (QuestionSetVersion) Indexes() []ent.Index {
	return docVersionIndexes("question_set_id", "question_set_versions")
}

// QuestionSetItem — content.question_set_items (H through the set): the fixed questions of a version, an immutable
// question_version_id each (F024-AC-02). A surrogate key replaces the composite key (soft-delete).
type QuestionSetItem struct{ ent.Schema }

func (QuestionSetItem) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "question_set_items"}}
}
func (QuestionSetItem) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "question_set", PK: entx.PK{Column: "question_set_item_id"}, ResourceIDColumn: "question_set_id"}}
}
func (QuestionSetItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("question_set_id", uuid.UUID{}).Immutable(),
		field.UUID("question_set_version_id", uuid.UUID{}).Immutable(),
		field.Int16("position").Immutable(),
		field.UUID("question_id", uuid.UUID{}).Immutable(),
		field.UUID("question_version_id", uuid.UUID{}).Immutable(),
		field.Float("max_score").SchemaType(map[string]string{"postgres": "numeric(7,2)"}).Optional().Nillable().Immutable(),
	}
}
func (QuestionSetItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("question_set_version_id", "question_version_id").Unique().StorageKey("question_set_items_uq").Annotations(live()),
		index.Fields("question_version_id").StorageKey("question_set_items_qv_idx").Annotations(live()),
	}
}

// Lesson — content.lessons (H).
type Lesson struct{ ent.Schema }

func (Lesson) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "lessons"}}
}
func (Lesson) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "lesson", PK: entx.PK{Column: "lesson_id"}}, ContentItemMixin{}}
}

// LessonVersion — content.lesson_versions (H through the lesson).
type LessonVersion struct{ ent.Schema }

func (LessonVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "lesson_versions"}}
}
func (LessonVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "lesson", PK: entx.PK{Column: "lesson_version_id"}, ResourceIDColumn: "lesson_id"},
		ContentVersionMixin{CurriculumOptional: true}, WorkflowFields{}}
}
func (LessonVersion) Fields() []ent.Field  { return docVersionFields("lesson_id") }
func (LessonVersion) Indexes() []ent.Index { return docVersionIndexes("lesson_id", "lesson_versions") }

// LearningMaterial — content.learning_materials (H).
type LearningMaterial struct{ ent.Schema }

func (LearningMaterial) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "learning_materials"}}
}
func (LearningMaterial) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "learning_material", PK: entx.PK{Column: "learning_material_id"}}, ContentItemMixin{}}
}

// LearningMaterialVersion — content.learning_material_versions (H through the material).
type LearningMaterialVersion struct{ ent.Schema }

func (LearningMaterialVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "learning_material_versions"}}
}
func (LearningMaterialVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "learning_material", PK: entx.PK{Column: "learning_material_version_id"},
		ResourceIDColumn: "learning_material_id"}, ContentVersionMixin{CurriculumOptional: true}, WorkflowFields{}}
}
func (LearningMaterialVersion) Fields() []ent.Field { return docVersionFields("learning_material_id") }
func (LearningMaterialVersion) Indexes() []ent.Index {
	return docVersionIndexes("learning_material_id", "learning_material_versions")
}

// AssessmentBlueprint — content.assessment_blueprints (H).
type AssessmentBlueprint struct{ ent.Schema }

func (AssessmentBlueprint) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "assessment_blueprints"}}
}
func (AssessmentBlueprint) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "assessment_blueprint", PK: entx.PK{Column: "assessment_blueprint_id"}}, ContentItemMixin{}}
}

// AssessmentBlueprintVersion — content.assessment_blueprint_versions (H through the blueprint).
type AssessmentBlueprintVersion struct{ ent.Schema }

func (AssessmentBlueprintVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "assessment_blueprint_versions"}}
}
func (AssessmentBlueprintVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: "assessment_blueprint", PK: entx.PK{Column: "assessment_blueprint_version_id"},
		ResourceIDColumn: "assessment_blueprint_id"}, ContentVersionMixin{CurriculumOptional: true}, WorkflowFields{}}
}
func (AssessmentBlueprintVersion) Fields() []ent.Field {
	return docVersionFields("assessment_blueprint_id")
}
func (AssessmentBlueprintVersion) Indexes() []ent.Index {
	return docVersionIndexes("assessment_blueprint_id", "assessment_blueprint_versions")
}
