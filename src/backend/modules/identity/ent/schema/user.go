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

// User — identity.users (G): the global identity (student or staff, ADR-0081). The unique index on
// lower(username) is an expression index written in the migration (ent cannot express it).
type User struct{ ent.Schema }

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "users"}}
}
func (User) Mixin() []ent.Mixin { return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "user_id"}}} }
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_type").MaxLen(64).Immutable(),
		field.String("status").MaxLen(64).Default("pending"),
		field.String("username").MaxLen(128).NotEmpty(),
		field.String("email").MaxLen(320).Optional().Nillable().Sensitive(),
		field.String("display_name").MaxLen(128).Optional().Nillable(),
		field.String("locale").MaxLen(64).Default("vi-VN"),
		field.UUID("primary_tenant_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("primary_school_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("deactivated_at").Optional().Nillable(),
		field.Time("anonymized_at").Optional().Nillable(),
		field.Time("provisioned_at").Default(nowUTC),
		field.Time("last_login_at").Optional().Nillable(),
	}
}
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("primary_tenant_id", "user_type").StorageKey("users_tenant_type_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// ExternalIdentity — identity.external_identities (G): (issuer, subject) → user; issuer
// platform_common holds the external_ref of common-tenant students (ACCOUNT_PROVISIONING §3.4).
type ExternalIdentity struct{ ent.Schema }

func (ExternalIdentity) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "external_identities"}}
}
func (ExternalIdentity) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "external_identity_id"}}}
}
func (ExternalIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.String("issuer").MaxLen(512).NotEmpty(),
		field.String("subject").MaxLen(255).NotEmpty(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.String("idp_alias").MaxLen(64).Optional().Nillable(),
		field.String("status").MaxLen(64).Default("active"),
		field.Time("superseded_at").Optional().Nillable(),
		field.UUID("superseded_by", uuid.UUID{}).Optional().Nillable(),
		field.Time("last_login_at").Optional().Nillable(),
	}
}
func (ExternalIdentity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("issuer", "subject").Unique().StorageKey("external_identities_iss_sub_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("user_id").StorageKey("external_identities_user_idx"),
		index.Fields("user_id", "issuer").Unique().StorageKey("external_identities_active_uq").
			Annotations(entsql.IndexWhere("status = 'active' AND deleted_at IS NULL")),
	}
}
