package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

func nowUTC() time.Time { return time.Now().UTC() }

// ContentRelease — content.content_releases (TA*, DATABASE_DESIGN §3.13, ADR-0043): history of the release
// pointer of every learning item. Append-only; rows with tenant_id NULL (platform items) are written only by
// learning_platform under PlatformScope (ADR-0089 §2). F020 creates the table; F021/F026 write it.
type ContentRelease struct{ ent.Schema }

func (ContentRelease) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "content_releases"}}
}
func (ContentRelease) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptionalAppendOnly{PK: entx.PK{Column: "release_id"}}}
}
func (ContentRelease) Fields() []ent.Field {
	return []ent.Field{
		field.String("item_type").MaxLen(64).Immutable(),
		field.UUID("item_id", uuid.UUID{}).Immutable(),
		field.String("action").MaxLen(16).Immutable(),
		field.UUID("from_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("to_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(500).Optional().Nillable().Immutable(),
		field.Time("scheduled_for").Optional().Nillable().Immutable(),
		field.UUID("released_by", uuid.UUID{}).Immutable(),
		field.Time("released_at").Default(nowUTC).Immutable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
	}
}
func (ContentRelease) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_type", "item_id", "idempotency_key").Unique().StorageKey("content_releases_idem_uq"),
		index.Fields("item_type", "item_id", "released_at").StorageKey("content_releases_item_idx"),
	}
}

// ContentTransition — content.content_transitions (TA*, DATABASE_DESIGN §2.5): workflow history of content
// versions (CONTENT_MODEL §6), the audit trail of the CMS. Append-only.
type ContentTransition struct{ ent.Schema }

func (ContentTransition) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "content_transitions"}}
}
func (ContentTransition) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptionalAppendOnly{PK: entx.PK{Column: "transition_id"}}}
}
func (ContentTransition) Fields() []ent.Field {
	return []ent.Field{
		field.String("content_type").MaxLen(64).Immutable(),
		field.UUID("content_version_id", uuid.UUID{}).Immutable(),
		field.String("from_status").MaxLen(64).Optional().Nillable().Immutable(),
		field.String("to_status").MaxLen(64).Immutable(),
		field.UUID("actor_id", uuid.UUID{}).Immutable(),
		field.Time("at").Default(nowUTC).Immutable(),
		field.String("note").MaxLen(2000).Optional().Nillable().Immutable(),
	}
}
func (ContentTransition) Indexes() []ent.Index {
	return []ent.Index{index.Fields("content_version_id").StorageKey("content_transitions_version_idx")}
}

// TenantResourceAssignment — infra.tenant_resource_assignments (DATABASE_DESIGN §3.0, ADR-0029): a platform
// resource of module learning assigned to a tenant. Soft-delete = revoked. No RLS (schema infra, ADR-0089 §4);
// learning_app may only read it (H policies and the entx hybrid filter), learning_platform writes it.
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
