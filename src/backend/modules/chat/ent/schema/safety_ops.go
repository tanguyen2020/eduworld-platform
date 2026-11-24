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

// F124 — operations of the Platform Safety Team: the on-call rotation of p1 cases and the break-glass grants of the
// System Admin (specs/124-safety-enforcement-oncall data-model §1, DATABASE_DESIGN §2.16, MODERATION §6.1, §6.3).

// ModerationOncallShift — chat.moderation_oncall_shifts (G, platform data without tenant): one on-call shift of a platform
// staff member (primary/secondary). No student data; soft-delete only (R21). Overlaps of one role are refused by the
// command under an advisory lock (409 ONCALL_SHIFT_OVERLAP).
type ModerationOncallShift struct{ ent.Schema }

func (ModerationOncallShift) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "moderation_oncall_shifts"}}
}
func (ModerationOncallShift) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "shift_id"}}}
}
func (ModerationOncallShift) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("moderator_user_id", uuid.UUID{}).Immutable(),
		field.String("oncall_role").MaxLen(16).Immutable(),
		field.Time("starts_at").Immutable(),
		field.Time("ends_at").Immutable(),
		field.String("note").MaxLen(500).Optional().Nillable(),
	}
}
func (ModerationOncallShift) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("oncall_role", "starts_at", "ends_at").StorageKey("moderation_oncall_shifts_role_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// BreakGlassGrant — chat.break_glass_grants (T): a break-glass request of a System Admin to read chat outside a case
// (MODERATION §6.1, CHILD_SAFETY §7): scope (channel or user) of the target tenant, message window, duration, reason,
// two-person decision (decided_by <> requested_by), expiry, revocation and the number of messages viewed (audit after the
// fact). Never message content.
type BreakGlassGrant struct{ ent.Schema }

func (BreakGlassGrant) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "break_glass_grants"}}
}
func (BreakGlassGrant) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "grant_id"}}}
}
func (BreakGlassGrant) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope_type").MaxLen(16).Immutable(),
		field.UUID("scope_id", uuid.UUID{}).Immutable(),
		field.Time("window_from").Immutable(),
		field.Time("window_to").Immutable(),
		field.Int("duration_minutes").Immutable(),
		field.String("reason").MaxLen(1000).Immutable(),
		field.String("status").MaxLen(16).Default("pending"),
		field.UUID("requested_by", uuid.UUID{}).Immutable(),
		field.Time("requested_at").Immutable(),
		field.UUID("decided_by", uuid.UUID{}).Optional().Nillable(),
		field.Time("decided_at").Optional().Nillable(),
		field.String("decision_note").MaxLen(1000).Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable(),
		field.UUID("revoked_by", uuid.UUID{}).Optional().Nillable(),
		field.Time("revoked_at").Optional().Nillable(),
		field.String("revoke_reason").MaxLen(1000).Optional().Nillable(),
		field.Int("messages_viewed_count").Default(0),
	}
}
func (BreakGlassGrant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "expires_at").StorageKey("break_glass_grants_active_idx").
			Annotations(entsql.IndexWhere("status = 'approved' AND deleted_at IS NULL")),
		index.Fields("tenant_id", "requested_at").StorageKey("break_glass_grants_tenant_idx"),
		index.Fields("requested_at").StorageKey("break_glass_grants_list_idx"),
	}
}
