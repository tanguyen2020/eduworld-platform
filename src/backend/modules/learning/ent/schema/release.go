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

// ScheduledPublication — content.scheduled_publications (T*, F026 US4, specs/026-content-release/data-model.md): a publish
// of an approved version at publish_at; content_releases is written only when the pointer really moves. Platform rows
// (tenant_id NULL) are written under PlatformScope. CHECKs and RLS are hand-written in the migration.
type ScheduledPublication struct{ ent.Schema }

func (ScheduledPublication) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "scheduled_publications"}}
}
func (ScheduledPublication) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "schedule_id"}}}
}
func (ScheduledPublication) Fields() []ent.Field {
	return []ent.Field{
		field.String("item_type").MaxLen(64).Immutable(),
		field.UUID("item_id", uuid.UUID{}).Immutable(),
		field.UUID("version_id", uuid.UUID{}).Immutable(),
		field.Int("version_no").Positive().Immutable(),
		field.Time("publish_at").Immutable(),
		field.String("status").MaxLen(64).Default("scheduled"),
		field.String("failure_code").MaxLen(64).Optional().Nillable(),
		field.UUID("applied_release_id", uuid.UUID{}).Optional().Nillable(),
		field.String("change_note").MaxLen(500).Optional().Nillable().Immutable(),
		field.UUID("requested_by", uuid.UUID{}).Immutable(),
		field.UUID("cancelled_by", uuid.UUID{}).Optional().Nillable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
	}
}
func (ScheduledPublication) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_type", "item_id", "idempotency_key").Unique().StorageKey("scheduled_publications_idem_uq"),
		index.Fields("item_type", "item_id").Unique().StorageKey("scheduled_publications_pending_uq").
			Annotations(entsql.IndexWhere("status = 'scheduled' AND deleted_at IS NULL")),
		index.Fields("status", "publish_at").StorageKey("scheduled_publications_due_idx").
			Annotations(entsql.IndexWhere("status = 'scheduled' AND deleted_at IS NULL")),
	}
}
