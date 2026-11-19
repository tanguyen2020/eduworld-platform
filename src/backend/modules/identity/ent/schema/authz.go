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

// Permission — identity.permissions (G): read-only copy of the code catalog (pkg/authz) written only by
// identity_migrator (cmd/migrate), with the scope kind of each code (ADR-0066, DATABASE_DESIGN §3.1).
type Permission struct{ ent.Schema }

func (Permission) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "permissions"}}
}
func (Permission) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "permission_code", StringLen: 64}}}
}
func (Permission) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope_kind").MaxLen(64),
		field.String("group_code").MaxLen(64),
		field.String("name_vi").MaxLen(255),
		field.String("name_en").MaxLen(255),
		field.String("catalog_version").MaxLen(64),
		field.Time("synced_at").Default(nowUTC),
	}
}

// Role — identity.roles (T*): system roles and shared roles have tenant_id NULL; custom roles belong to a
// tenant and are always tenant roles (DATABASE_DESIGN §3.1, ADR-0066).
type Role struct{ ent.Schema }

func (Role) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "roles"}}
}
func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "role_id"}}}
}
func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64),
		field.String("name_vi").MaxLen(255),
		field.String("name_en").MaxLen(255),
		field.String("description").MaxLen(1000).Optional().Nillable(),
		field.String("role_scope_kind").MaxLen(64).Immutable(),
		field.Bool("is_system").Default(false).Immutable(),
	}
}
func (Role) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique().StorageKey("roles_shared_code_uq").
			Annotations(entsql.IndexWhere("tenant_id IS NULL AND deleted_at IS NULL")),
		index.Fields("tenant_id", "code").Unique().StorageKey("roles_tenant_code_uq").
			Annotations(entsql.IndexWhere("tenant_id IS NOT NULL AND deleted_at IS NULL")),
	}
}

// RolePermission — identity.role_permissions (T*, tenant_id = roles.tenant_id): the permissions of a role. The
// composite key of the DDL is a surrogate id plus a unique index; removing a permission soft-deletes the row.
type RolePermission struct{ ent.Schema }

func (RolePermission) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "role_permissions"}}
}
func (RolePermission) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "role_permission_id"}}}
}
func (RolePermission) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("role_id", uuid.UUID{}).Immutable(),
		field.String("permission_code").MaxLen(64).Immutable(),
	}
}
func (RolePermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id", "permission_code").Unique().StorageKey("role_permissions_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// UserRole — identity.user_roles (T*): a role granted to a user at a scope (platform rows have tenant_id NULL;
// no class scope, ADR-0072). Revocation is a soft-delete; expiry is valid_to.
type UserRole struct{ ent.Schema }

func (UserRole) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "user_roles"}}
}
func (UserRole) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "user_role_id"}}}
}
func (UserRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("role_id", uuid.UUID{}).Immutable(),
		field.String("scope_type").MaxLen(64).Immutable(),
		field.UUID("scope_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("valid_from").Default(nowUTC),
		field.Time("valid_to").Optional().Nillable(),
		field.UUID("granted_by", uuid.UUID{}).Immutable(),
		field.String("source").MaxLen(64).Default("manual").Immutable(),
		field.Time("training_completed_at").Optional().Nillable(),
	}
}
func (UserRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "role_id", "scope_type", "scope_id").Unique().StorageKey("user_roles_active_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL AND valid_to IS NULL")),
		index.Fields("user_id", "tenant_id").StorageKey("user_roles_user_tenant_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "role_id").StorageKey("user_roles_tenant_role_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// PortalPasswordResetToken — identity.portal_password_reset_tokens (G): one-time reset of a staff password
// (ADR-0091); only the SHA-256 of the token is stored. A new token soft-deletes the unused one (superseded).
type PortalPasswordResetToken struct{ ent.Schema }

func (PortalPasswordResetToken) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "portal_password_reset_tokens"}}
}
func (PortalPasswordResetToken) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "reset_token_id"}}}
}
func (PortalPasswordResetToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.String("token_hash").MaxLen(64).Immutable(),
		field.String("locale").MaxLen(8).Default("vi"),
		field.Time("expires_at").Immutable(),
		field.Time("used_at").Optional().Nillable(),
		field.String("requested_device_hash").MaxLen(64).Optional().Nillable().Immutable(),
	}
}
func (PortalPasswordResetToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_hash").Unique().StorageKey("portal_password_reset_tokens_hash_uq"),
		index.Fields("user_id").Unique().StorageKey("portal_password_reset_tokens_active_uq").
			Annotations(entsql.IndexWhere("used_at IS NULL AND deleted_at IS NULL")),
		index.Fields("expires_at").StorageKey("portal_password_reset_tokens_expires_idx"),
	}
}
