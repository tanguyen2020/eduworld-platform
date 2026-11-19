package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/google/uuid"
)

// ContentItemMixin adds the item columns of the shared content model (DATABASE_DESIGN §2 "H", §3.13,
// CONTENT_MODEL §5, ADR-0029, ADR-0043) to an item entity (questions, question_sets, curricula…). It is
// not an entx mixin: the entity declares exactly one entx mixin, entx.HybridContent (tenant_id NULL =
// platform, owner_scope, soft-delete), and adds this one for the release pointer and the clone origin.
// The CHECK (owner_scope = 'platform') = (tenant_id IS NULL) is written by hand in the migration of the
// table (F020-AC-08).
type ContentItemMixin struct{ mixin.Schema }

// Fields of a content item.
func (ContentItemMixin) Fields() []ent.Field {
	return append([]ent.Field{
		// Release pointer: the published version new attempts start with; NULL = never/no longer published.
		field.UUID("current_version_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("latest_version_no").Default(0).NonNegative(),
	}, cloneFields()...)
}

// ContentVersionMixin adds the version columns of the shared content model (CONTENT_MODEL §3, §5, §6;
// DATABASE_DESIGN §3.3, §3.13) to a version entity whose single entx mixin is entx.HybridContent with
// ResourceIDColumn = its item id column (a version is visible to a tenant when its item is assigned).
// The version number is version_no: entx owns "version" (optimistic concurrency, ADR-0039) [Proposed,
// F020 clarification 2026-11-19]. Without CurriculumOptional, curriculum_version_id is NOT NULL
// (media is the only exception, CONTENT_MODEL §3).
type ContentVersionMixin struct {
	mixin.Schema
	CurriculumOptional bool
}

// Fields of a content version.
func (m ContentVersionMixin) Fields() []ent.Field {
	curriculum := field.UUID("curriculum_version_id", uuid.UUID{})
	if m.CurriculumOptional {
		curriculum = curriculum.Optional().Nillable()
	}
	return append([]ent.Field{
		field.Int("version_no").Positive().Immutable(),
		field.String("status").MaxLen(64).Default("draft"),
		field.UUID("author_id", uuid.UUID{}).Immutable(),
		field.UUID("reviewer_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("published_at").Optional().Nillable(),
		field.String("content_language").MaxLen(64).Default("vi"),
		curriculum,
		field.String("change_note").MaxLen(500).Optional().Nillable(),
		field.Time("publish_at").Optional().Nillable(),
		field.String("snapshot_key").MaxLen(512).Optional().Nillable(),
		field.String("snapshot_sha256").MaxLen(64).Optional().Nillable(),
		field.UUID("based_on_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
	}, cloneFields()...)
}

// cloneFields are the clone origin columns (ADR-0029, MULTITENANCY §10): set once by the clone step.
func cloneFields() []ent.Field {
	return []ent.Field{
		field.UUID("cloned_from_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("cloned_from_tenant_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("cloned_at").Optional().Nillable().Immutable(),
		field.UUID("cloned_by", uuid.UUID{}).Optional().Nillable().Immutable(),
	}
}
