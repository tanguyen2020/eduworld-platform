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

// Party — multiplayer.parties (T, DATABASE_DESIGN §2.9, §3.7, F100): a temporary play group of students of one
// tenant and school (PARTY_SYSTEM §3). Disbanding sets status, never deletes (ADR-0028); deleted_at is only for rows
// created by mistake. state_version grows by exactly one with every membership or settings change (expected_version
// of F101). The activity chosen in the Party Lobby (selected_activity_*) is durable (F101-AC-06); Ready is ephemeral.
type Party struct{ ent.Schema }

func (Party) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "multiplayer", Table: "parties"}}
}
func (Party) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "party_id"}}}
}
func (Party) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.String("name").MaxLen(128),
		field.UUID("leader_id", uuid.UUID{}),
		field.String("party_type").MaxLen(64).Immutable(),
		field.String("visibility").MaxLen(64).Default("private"),
		field.String("invitation_policy").MaxLen(64).Default("leader_only"),
		field.Int("max_members"),
		field.String("grade_band").MaxLen(64).Immutable(),
		field.String("status").MaxLen(64).Default("open"),
		field.UUID("current_activity_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("current_game_session_ref", uuid.UUID{}).Optional().Nillable(),
		field.String("selected_activity_type").MaxLen(64).Optional().Nillable(),
		field.String("selected_activity_ref").MaxLen(64).Optional().Nillable(),
		field.String("selected_game_mode").MaxLen(64).Optional().Nillable(),
		field.Int64("state_version").Default(0),
		field.Time("disbanded_at").Optional().Nillable(),
	}
}
func (Party) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "status").StorageKey("parties_tenant_status_idx").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("school_id", "status").StorageKey("parties_school_status_idx").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// PartyMember — multiplayer.party_members (T, DATABASE_DESIGN §3.7): one membership row per stay of a student in a
// party (joined_at → left_at). At most one active membership per student and one active leader per party (unique
// partial indexes, the last guard of FR-PTY-005). membership_id is the row key (ent needs one id column); the business
// key stays (party_id, student_id, joined_at). left_at/left_reason are set once.
type PartyMember struct{ ent.Schema }

func (PartyMember) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "multiplayer", Table: "party_members"}}
}
func (PartyMember) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "membership_id"}}}
}
func (PartyMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("party_id", uuid.UUID{}).Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.String("role").MaxLen(64),
		field.UUID("invitation_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("joined_at").Default(nowUTC).Immutable(),
		field.Time("left_at").Optional().Nillable(),
		field.String("left_reason").MaxLen(64).Optional().Nillable(),
	}
}
func (PartyMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("student_id").Unique().StorageKey("party_members_one_active_party_uq").
			Annotations(entsql.IndexWhere("left_at IS NULL AND deleted_at IS NULL")),
		index.Fields("party_id").Unique().StorageKey("party_members_one_leader_uq").
			Annotations(entsql.IndexWhere("left_at IS NULL AND role = 'leader' AND deleted_at IS NULL")),
		index.Fields("party_id", "student_id").Unique().StorageKey("party_members_active_uq").
			Annotations(entsql.IndexWhere("left_at IS NULL AND deleted_at IS NULL")),
		index.Fields("party_id", "joined_at").StorageKey("party_members_party_idx"),
		index.Fields("tenant_id", "student_id", "left_reason", "left_at").StorageKey("party_members_student_left_idx"),
	}
}

// PartyInvitation — multiplayer.party_invitations (T, DATABASE_DESIGN §2.9, F100): an invitation with a server-side
// expiry (party_invite_ttl); pending → accepted | rejected | expired | cancelled (PARTY_SYSTEM §13.2). end_reason says
// why it was cancelled or expired (event payload reason).
type PartyInvitation struct{ ent.Schema }

func (PartyInvitation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "multiplayer", Table: "party_invitations"}}
}
func (PartyInvitation) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "invitation_id"}}}
}
func (PartyInvitation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("party_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("inviter_id", uuid.UUID{}).Immutable(),
		field.UUID("invitee_id", uuid.UUID{}).Immutable(),
		field.String("status").MaxLen(64).Default("pending"),
		field.Time("expires_at").Immutable(),
		field.Time("responded_at").Optional().Nillable(),
		field.String("end_reason").MaxLen(64).Optional().Nillable(),
	}
}
func (PartyInvitation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("party_id", "invitee_id").Unique().StorageKey("party_invitations_pending_uq").
			Annotations(entsql.IndexWhere("status = 'pending' AND deleted_at IS NULL")),
		index.Fields("invitee_id", "status", "expires_at").StorageKey("party_invitations_invitee_idx"),
		index.Fields("expires_at").StorageKey("party_invitations_expiry_idx").
			Annotations(entsql.IndexWhere("status = 'pending' AND deleted_at IS NULL")),
		index.Fields("tenant_id", "inviter_id", "created_at").StorageKey("party_invitations_inviter_idx"),
	}
}

// PartyPolicy — multiplayer.party_policies (T, DATABASE_DESIGN §2.16): limits of a school or a class
// (allowed_party_types, cross_class, cross_grade_band, max_members_cap); a lower level only tightens. F100 reads them
// only (no administration API yet); the platform level comes from the configuration (no row). The policy version is
// the version column of the mixin.
type PartyPolicy struct{ ent.Schema }

func (PartyPolicy) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "multiplayer", Table: "party_policies"}}
}
func (PartyPolicy) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "policy_id"}}}
}
func (PartyPolicy) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope_type").MaxLen(64).Immutable(),
		field.UUID("scope_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.JSON("rules", map[string]any{}),
	}
}
func (PartyPolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("scope_type", "scope_id").Unique().StorageKey("party_policies_scope_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
