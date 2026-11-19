package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Review workflow of F025 (specs/025-content-review-workflow/data-model.md, CONTENT_MODEL §6, §8, §8.1, DATABASE_DESIGN
// §2.5): review tasks by lane, translation jobs and the translation/audio columns of every learning content version.
// CHECKs, RLS, _live views and grants are hand-written in the migration 20261128000001_f025_workflow.sql.

// WorkflowFields adds the bilingual and g1_3 audio columns to a learning content version table (question_versions and
// the four F024 version tables; the activity versions of F030 too). translation_status is computed on every draft
// save (CONTENT_MODEL §8), translation_meta keeps per JSON pointer the hash of the source text a translation was made
// from; the audio exemption (CONTENT_MODEL §8.1) is confirmed by the reviewer on approve [Proposed: the columns exist
// on every version table so the generated repositories of F024 stay uniform].
type WorkflowFields struct{ mixin.Schema }

// Fields of the workflow mixin.
func (WorkflowFields) Fields() []ent.Field {
	return []ent.Field{
		field.String("translation_status").MaxLen(64).Optional().Nillable(),
		field.JSON("translation_meta", json.RawMessage{}).Optional(),
		field.String("audio_exempt_reason").MaxLen(255).Optional().Nillable(),
		field.UUID("audio_exempt_approved_by", uuid.UUID{}).Optional().Nillable(),
		// F026: who published the version (ContentVersionSummary.published_by).
		field.UUID("published_by", uuid.UUID{}).Optional().Nillable(),
	}
}

// ReviewTask — content.review_tasks (T*): the review of one submitted version in a lane (ADMIN_PORTAL §4). Lane tenant
// rows carry the tenant; lane platform_learning rows have tenant_id NULL and are written under PlatformScope only
// [Proposed: T* rather than H of the data-model — a review task is never assigned to a tenant].
type ReviewTask struct{ ent.Schema }

func (ReviewTask) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "review_tasks"}}
}
func (ReviewTask) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "task_id"}}}
}
func (ReviewTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("review_lane").MaxLen(64).Immutable(),
		field.String("content_type").MaxLen(64).Immutable(),
		field.UUID("content_id", uuid.UUID{}).Immutable(),
		field.UUID("content_version_id", uuid.UUID{}).Immutable(),
		field.Int("version_no").Positive().Immutable(),
		field.String("title").MaxLen(255).Optional().Nillable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("author_id", uuid.UUID{}).Immutable(),
		field.UUID("assignee_id", uuid.UUID{}).Optional().Nillable(),
		field.String("status").MaxLen(64).Default("open"),
		field.JSON("comments", json.RawMessage{}).Optional(),
		field.Time("submitted_at").Default(nowUTC).Immutable(),
		field.Time("due_at").Optional().Nillable(),
		field.Time("resolved_at").Optional().Nillable(),
		field.UUID("resolved_by", uuid.UUID{}).Optional().Nillable(),
	}
}
func (ReviewTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_version_id").Unique().StorageKey("review_tasks_open_uq").
			Annotations(entsql.IndexWhere("status = 'open' AND deleted_at IS NULL")),
		index.Fields("tenant_id", "review_lane", "status", "submitted_at").StorageKey("review_tasks_queue_idx").Annotations(live()),
		index.Fields("assignee_id", "status").StorageKey("review_tasks_assignee_idx").Annotations(live()),
	}
}

// TranslationJob — content.translation_jobs (T*): an export or import of translation files (XLIFF 2.1 / CSV,
// ADMIN_PORTAL §1.5) with its report. Platform jobs have tenant_id NULL (PlatformScope).
type TranslationJob struct{ ent.Schema }

func (TranslationJob) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "translation_jobs"}}
}
func (TranslationJob) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "job_id"}}}
}
func (TranslationJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("owner_scope").MaxLen(64).Immutable(),
		field.String("kind").MaxLen(64).Immutable(),
		field.String("status").MaxLen(64).Default("queued"),
		field.String("format").MaxLen(64).Immutable(),
		field.String("source_language").MaxLen(64).Immutable(),
		field.String("target_language").MaxLen(64).Immutable(),
		field.JSON("request", json.RawMessage{}).Immutable(),
		field.UUID("input_asset_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("result_storage_key").MaxLen(512).Optional().Nillable(),
		field.JSON("report", json.RawMessage{}).Optional(),
		field.UUID("requested_by", uuid.UUID{}).Immutable(),
		field.Time("finished_at").Optional().Nillable(),
	}
}
func (TranslationJob) Indexes() []ent.Index {
	return []ent.Index{index.Fields("status", "created_at").StorageKey("translation_jobs_status_idx").Annotations(live())}
}
