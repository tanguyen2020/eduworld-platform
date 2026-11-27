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

// Tables of the curriculum structure (F021, specs/021-curriculum/data-model.md, CURRICULUM_MODEL §2–§6). Every
// table of a curriculum version is hybrid content (H) visible to a tenant through the assignment of its
// curriculum (resource_type curriculum): each one carries curriculum_id for the entx filter and the RLS policy
// (F021 clarification 2026-11-19). The subject catalog and the textbook tables are global (G) platform catalogs.
// Expression unique indexes (coalesce of tenant_id/to_lo_id), CHECKs, RLS and triggers are hand-written in the
// migration.

const curriculumResource = "curriculum"

var dateType = map[string]string{"postgres": "date"}

func live() schema.Annotation { return entsql.IndexWhere("deleted_at IS NULL") }

func versionKeys() []ent.Field {
	return []ent.Field{
		field.UUID("curriculum_id", uuid.UUID{}).Immutable(),
		field.UUID("curriculum_version_id", uuid.UUID{}).Immutable(),
	}
}

// Curriculum — curriculum.curricula (H): the curriculum and its release pointer (ContentItemMixin).
type Curriculum struct{ ent.Schema }

func (Curriculum) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "curricula"}}
}
func (Curriculum) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entx.HybridContent{ResourceType: curriculumResource, PK: entx.PK{Column: "curriculum_id"}},
		ContentItemMixin{},
	}
}
func (Curriculum) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).Immutable(),
		field.JSON("name", map[string]string{}),
		field.String("framework").MaxLen(64).Optional().Nillable(),
	}
}

// CurriculumVersion — curriculum.curriculum_versions (H): the unit of version and release of a curriculum. Its
// own id is curriculum_version_id, so it declares the version columns itself instead of ContentVersionMixin
// (whose curriculum_version_id column would collide).
type CurriculumVersion struct{ ent.Schema }

func (CurriculumVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "curriculum_versions"}}
}
func (CurriculumVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: curriculumResource, PK: entx.PK{Column: "curriculum_version_id"}, ResourceIDColumn: "curriculum_id"}}
}
func (CurriculumVersion) Fields() []ent.Field {
	return append([]ent.Field{
		field.UUID("curriculum_id", uuid.UUID{}).Immutable(),
		field.Int("version_no").Positive().Immutable(),
		field.String("version_label").MaxLen(16),
		field.String("status").MaxLen(64).Default("draft"),
		field.Time("effective_from").SchemaType(dateType).Optional().Nillable(),
		field.Time("effective_to").SchemaType(dateType).Optional().Nillable(),
		field.UUID("supersedes_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("based_on_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("author_id", uuid.UUID{}).Immutable(),
		field.UUID("reviewer_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("published_at").Optional().Nillable(),
		field.String("change_note").MaxLen(1000).Optional().Nillable(),
		field.String("content_language").MaxLen(64).Default("vi"),
	}, cloneFields()...)
}
func (CurriculumVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("curriculum_id", "version_no").Unique().StorageKey("curriculum_versions_no_uq"),
		index.Fields("curriculum_id", "version_label").Unique().StorageKey("curriculum_versions_label_uq").Annotations(live()),
	}
}

// Subject — curriculum.subjects (G): the platform subject catalog.
type Subject struct{ ent.Schema }

func (Subject) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "subjects"}}
}
func (Subject) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "subject_id"}}}
}
func (Subject) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).Immutable(),
		field.JSON("name", map[string]string{}),
		field.JSON("grade_levels", []int{}),
		field.String("icon_key").MaxLen(64).Optional().Nillable(),
	}
}
func (Subject) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("subjects_code_uq").Annotations(live())}
}

// SubjectGrade — curriculum.subject_grades (H): a subject × grade pair of a version.
type SubjectGrade struct{ ent.Schema }

func (SubjectGrade) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "subject_grades"}}
}
func (SubjectGrade) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: curriculumResource, PK: entx.PK{Column: "subject_grade_id"}, ResourceIDColumn: "curriculum_id"}}
}
func (SubjectGrade) Fields() []ent.Field {
	return append(versionKeys(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.Int16("grade_level").Immutable(),
		field.Int16("periods_per_year").Optional().Nillable(),
	)
}
func (SubjectGrade) Indexes() []ent.Index {
	return []ent.Index{index.Fields("curriculum_version_id", "subject_id", "grade_level").Unique().
		StorageKey("subject_grades_uq").Annotations(live())}
}

// Topic — curriculum.topics (H): the topic tree of a subject × grade of a version.
type Topic struct{ ent.Schema }

func (Topic) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "topics"}}
}
func (Topic) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: curriculumResource, PK: entx.PK{Column: "topic_id"}, ResourceIDColumn: "curriculum_id"}}
}
func (Topic) Fields() []ent.Field {
	return append(versionKeys(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.Int16("grade_level").Immutable(),
		field.UUID("parent_topic_id", uuid.UUID{}).Optional().Nillable(),
		field.String("code").MaxLen(64),
		field.String("name").MaxLen(255),
		field.Int32("sort_order").Default(1),
	)
}
func (Topic) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("curriculum_version_id", "code").Unique().StorageKey("topics_code_uq").Annotations(live()),
		index.Fields("curriculum_version_id", "subject_id", "grade_level", "sort_order").StorageKey("topics_tree_idx").Annotations(live()),
	}
}

// LearningObjective — curriculum.learning_objectives (H): the unit of mastery (CURRICULUM_MODEL §3.3).
// competency_ids and prerequisite_ids are jsonb arrays of ids (F021 clarification 2026-11-19).
type LearningObjective struct{ ent.Schema }

func (LearningObjective) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "learning_objectives"}}
}
func (LearningObjective) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: curriculumResource, PK: entx.PK{Column: "objective_id"}, ResourceIDColumn: "curriculum_id"}}
}
func (LearningObjective) Fields() []ent.Field {
	return append(versionKeys(),
		field.UUID("topic_id", uuid.UUID{}),
		field.String("code").MaxLen(64),
		field.String("stable_key").MaxLen(64),
		field.String("description").MaxLen(1000),
		field.String("bloom_level").MaxLen(64).Optional().Nillable(),
		field.String("yccd_ref").MaxLen(64).Optional().Nillable(),
		field.JSON("competency_ids", []uuid.UUID{}).Default([]uuid.UUID{}),
		field.JSON("prerequisite_ids", []uuid.UUID{}).Default([]uuid.UUID{}),
		field.Int32("sort_order").Default(1),
	)
}
func (LearningObjective) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("curriculum_version_id", "code").Unique().StorageKey("learning_objectives_code_uq").Annotations(live()),
		index.Fields("curriculum_version_id", "topic_id").StorageKey("learning_objectives_topic_idx").Annotations(live()),
	}
}

// Competency — curriculum.competencies (H): competencies and qualities of a version (CURRICULUM_MODEL §3.4).
type Competency struct{ ent.Schema }

func (Competency) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "competencies"}}
}
func (Competency) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: curriculumResource, PK: entx.PK{Column: "competency_id"}, ResourceIDColumn: "curriculum_id"}}
}
func (Competency) Fields() []ent.Field {
	return append(versionKeys(),
		field.String("code").MaxLen(64),
		field.JSON("name", map[string]string{}),
		field.String("kind").MaxLen(64),
		field.UUID("subject_id", uuid.UUID{}).Optional().Nillable(),
	)
}
func (Competency) Indexes() []ent.Index {
	return []ent.Index{index.Fields("curriculum_version_id", "code").Unique().StorageKey("competencies_code_uq").Annotations(live())}
}

// LoVersionMapping — curriculum.lo_version_mappings (H): LO of the superseded version → LO of this version
// (CURRICULUM_MODEL §5). The unique key (coalesce of to_lo_id) is hand-written.
type LoVersionMapping struct{ ent.Schema }

func (LoVersionMapping) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "lo_version_mappings"}}
}
func (LoVersionMapping) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: curriculumResource, PK: entx.PK{Column: "mapping_id"}, ResourceIDColumn: "curriculum_id"}}
}
func (LoVersionMapping) Fields() []ent.Field {
	return append(versionKeys(),
		field.UUID("from_lo_id", uuid.UUID{}).Immutable(),
		field.UUID("to_lo_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("relation").MaxLen(64).Immutable(),
	)
}

// TextbookSeries — curriculum.textbook_series (G): a textbook series, metadata only (CURRICULUM_MODEL §6).
type TextbookSeries struct{ ent.Schema }

func (TextbookSeries) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "textbook_series"}}
}
func (TextbookSeries) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "series_id"}}}
}
func (TextbookSeries) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).Immutable(),
		field.String("name").MaxLen(255),
		field.String("publisher").MaxLen(255).Optional().Nillable(),
	}
}
func (TextbookSeries) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("textbook_series_code_uq").Annotations(live())}
}

// Textbook — curriculum.textbooks (G): one volume of a series for a subject × grade.
type Textbook struct{ ent.Schema }

func (Textbook) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "textbooks"}}
}
func (Textbook) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "textbook_id"}}}
}
func (Textbook) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("series_id", uuid.UUID{}).Immutable(),
		field.UUID("subject_id", uuid.UUID{}).Immutable(),
		field.Int16("grade_level").Immutable(),
		field.Int16("volume").Immutable(),
	}
}
func (Textbook) Indexes() []ent.Index {
	return []ent.Index{index.Fields("series_id", "subject_id", "grade_level", "volume").Unique().StorageKey("textbooks_uq").Annotations(live())}
}

// TextbookSection — curriculum.textbook_sections (G): a lesson of a textbook (no text of the book).
type TextbookSection struct{ ent.Schema }

func (TextbookSection) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "textbook_sections"}}
}
func (TextbookSection) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "section_id"}}}
}
func (TextbookSection) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("textbook_id", uuid.UUID{}).Immutable(),
		field.String("chapter").MaxLen(128).Optional().Nillable(),
		field.Int16("lesson_no"),
		field.String("title").MaxLen(255),
		field.String("page_range").MaxLen(16).Optional().Nillable(),
	}
}
func (TextbookSection) Indexes() []ent.Index {
	return []ent.Index{index.Fields("textbook_id", "lesson_no").StorageKey("textbook_sections_textbook_idx").Annotations(live())}
}

// TextbookSectionObjective — curriculum.textbook_section_objectives (G): section ↔ LO (n–n). A surrogate key
// replaces the composite key of DATABASE_DESIGN §2.16 so the link can be soft-deleted (F021 clarification).
type TextbookSectionObjective struct{ ent.Schema }

func (TextbookSectionObjective) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "curriculum", Table: "textbook_section_objectives"}}
}
func (TextbookSectionObjective) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "section_objective_id"}}}
}
func (TextbookSectionObjective) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("section_id", uuid.UUID{}).Immutable(),
		field.UUID("objective_id", uuid.UUID{}).Immutable(),
	}
}
func (TextbookSectionObjective) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("section_id", "objective_id").Unique().StorageKey("textbook_section_objectives_uq").Annotations(live()),
		index.Fields("objective_id").StorageKey("textbook_section_objectives_lo_idx").Annotations(live()),
	}
}

// ImportJob — content.import_jobs (T*): a content import job with its row report (F021 frame, F024 adds kinds).
// Tenant rows for tenant imports, tenant_id NULL for platform imports (written under PlatformScope).
type ImportJob struct{ ent.Schema }

func (ImportJob) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "import_jobs"}}
}
func (ImportJob) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "import_id"}}}
}
func (ImportJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("owner_scope").MaxLen(64).Immutable(),
		field.String("kind").MaxLen(64).Immutable(),
		field.UUID("asset_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("source_ref").MaxLen(512).Optional().Nillable().Immutable(),
		field.Bool("dry_run").Default(false).Immutable(),
		field.String("status").MaxLen(64).Default("queued"),
		field.Int32("created_count").Default(0),
		field.Int32("rejected_count").Default(0),
		field.JSON("errors", []map[string]any{}).Default([]map[string]any{}),
		field.JSON("result_refs", map[string]any{}).Default(map[string]any{}),
		field.String("idempotency_key").MaxLen(64).Immutable(),
		field.Time("finished_at").Optional().Nillable(),
	}
}
func (ImportJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_by", "idempotency_key").Unique().StorageKey("import_jobs_idem_uq"),
		index.Fields("status", "created_at").StorageKey("import_jobs_status_idx"),
	}
}

// ImportItem — content.import_items (T*, F028): one question of a Word file of a lessons_docx job with its analysis
// (status, issues, formulas, pictures, parsed authoring fields) and the draft created from it. Same owner as its job.
type ImportItem struct{ ent.Schema }

func (ImportItem) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "import_items"}}
}
func (ImportItem) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "import_item_id"}}}
}
func (ImportItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("import_id", uuid.UUID{}).Immutable(),
		field.Int32("item_no").Immutable(),
		field.String("source_label").MaxLen(32).Immutable(),
		field.String("status").MaxLen(32),
		field.String("question_type").MaxLen(32).Optional().Nillable().Immutable(),
		field.String("stem_preview").MaxLen(200).Immutable(),
		field.JSON("issues", []map[string]any{}).Default([]map[string]any{}),
		field.JSON("formulas", []map[string]any{}).Default([]map[string]any{}).Immutable(),
		field.JSON("images", []map[string]any{}).Default([]map[string]any{}).Immutable(),
		field.JSON("payload", map[string]any{}).Default(map[string]any{}).Immutable(),
		field.UUID("question_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("question_version_id", uuid.UUID{}).Optional().Nillable(),
	}
}
func (ImportItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("import_id", "item_no").Unique().StorageKey("import_items_no_uq").Annotations(live()),
		index.Fields("import_id", "status", "item_no").StorageKey("import_items_status_idx").Annotations(live()),
	}
}
