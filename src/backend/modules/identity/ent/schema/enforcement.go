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

// F124 — account locks by source and staff email deliveries (specs/124-safety-enforcement-oncall data-model §1,
// DATABASE_DESIGN §2.2, ACCOUNT_PROVISIONING §6, MODERATION §6.3–§7).

// AccountLock — identity.account_locks (T*): one lock of an account by one source (school_admin, platform_admin, sis,
// moderation_sanction). identity.users.status = locked ⇔ at least one lock with released_at NULL (except deactivated or
// anonymized accounts). A lock of the moderation source references the sanction (source_ref_id) and the case (ids only,
// never content); tenant_id is NULL only for a platform staff member locked by the platform. Never deleted (R21).
type AccountLock struct{ ent.Schema }

func (AccountLock) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "account_locks"}}
}
func (AccountLock) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "lock_id"}}}
}
func (AccountLock) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.String("source").MaxLen(32).Immutable(),
		field.UUID("source_ref_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("case_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(1000).Optional().Nillable().Immutable(),
		field.Time("released_at").Optional().Nillable(),
		field.UUID("released_by", uuid.UUID{}).Optional().Nillable(),
		field.String("release_reason").MaxLen(1000).Optional().Nillable(),
		field.UUID("source_event_id", uuid.UUID{}).Optional().Nillable(),
	}
}
func (AccountLock) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").StorageKey("account_locks_active_idx").
			Annotations(entsql.IndexWhere("released_at IS NULL AND deleted_at IS NULL")),
		index.Fields("source", "source_ref_id").Unique().StorageKey("account_locks_source_ref_uq").
			Annotations(entsql.IndexWhere("source_ref_id IS NOT NULL AND deleted_at IS NULL")),
		index.Fields("user_id", "source").Unique().StorageKey("account_locks_admin_source_uq").
			Annotations(entsql.IndexWhere("source_ref_id IS NULL AND released_at IS NULL AND deleted_at IS NULL")),
	}
}

// EmailDelivery — identity.email_deliveries (T*): one email to one staff member by template (school safety
// notification/reminder, on-call page, break-glass request). It stores the recipient id only — the address is read when
// sending (R14, NFR-PRIV-002) — and params of ids and instants (no student name, case summary or chat content). One row
// per (source event, recipient, template); tenant_id NULL for platform staff emails. Status updates only, never deleted.
type EmailDelivery struct{ ent.Schema }

func (EmailDelivery) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "email_deliveries"}}
}
func (EmailDelivery) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "delivery_id"}}}
}
func (EmailDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("source_event_id", uuid.UUID{}).Immutable(),
		field.String("template_code").MaxLen(64).Immutable(),
		field.UUID("recipient_user_id", uuid.UUID{}).Immutable(),
		field.JSON("params", map[string]string{}).Immutable(),
		field.String("locale").MaxLen(8).Default("vi"),
		field.String("status").MaxLen(16).Default("pending"),
		field.Int16("attempts").Default(0),
		field.Time("next_attempt_at").Optional().Nillable(),
		field.Time("sent_at").Optional().Nillable(),
		field.String("last_error_code").MaxLen(64).Optional().Nillable(),
	}
}
func (EmailDelivery) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_event_id", "recipient_user_id", "template_code").Unique().StorageKey("email_deliveries_once_uq"),
		index.Fields("status", "next_attempt_at").StorageKey("email_deliveries_due_idx").
			Annotations(entsql.IndexWhere("status = 'pending'")),
	}
}
