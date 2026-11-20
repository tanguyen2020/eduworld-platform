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

// Room — multiplayer.rooms (T, DATABASE_DESIGN §2.9, F091): summary record of a room; the live state lives in the
// memory of the room host and in Valkey (ADR-0004). F091 writes room_type hub_instance only, with the partition
// (school_id, grade_band) of its members (ADR-0054). Never physically deleted (ADR-0028).
type Room struct{ ent.Schema }

func (Room) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "multiplayer", Table: "rooms"}}
}
func (Room) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "room_id"}}}
}
func (Room) Fields() []ent.Field {
	return []ent.Field{
		field.String("room_type").MaxLen(64).Immutable(),
		field.String("area_id").MaxLen(64).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("grade_band").MaxLen(64).Optional().Nillable().Immutable(),
		field.Int("capacity").Immutable(),
		field.String("status").MaxLen(64).Default("open"),
		field.Time("closed_at").Optional().Nillable(),
	}
}
func (Room) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "area_id", "school_id", "grade_band").StorageKey("rooms_partition_open_idx").
			Annotations(entsql.IndexWhere("status = 'open' AND deleted_at IS NULL")),
	}
}

// SessionMembership — multiplayer.session_memberships (T, DATABASE_DESIGN §2.9, MULTIPLAYER_GAMEPLAY §10): who was in
// which room and when, to investigate behaviour reports. For hub_instance it is written only while
// audit_hub_membership is on for the tenant (F091-AC-11). left_at and leave_reason are set once.
type SessionMembership struct{ ent.Schema }

func (SessionMembership) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "multiplayer", Table: "session_memberships"}}
}
func (SessionMembership) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "membership_id"}}}
}
func (SessionMembership) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("room_id", uuid.UUID{}).Immutable(),
		field.String("room_type").MaxLen(64).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("game_session_id", uuid.UUID{}).Immutable(),
		field.UUID("party_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("match_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("encounter_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("joined_at").Default(nowUTC).Immutable(),
		field.Time("left_at").Optional().Nillable(),
		field.String("leave_reason").MaxLen(64).Optional().Nillable(),
		field.Int("disconnects").Default(0),
	}
}
func (SessionMembership) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("room_id", "student_id").Unique().StorageKey("session_memberships_active_uq").
			Annotations(entsql.IndexWhere("left_at IS NULL AND deleted_at IS NULL")),
		index.Fields("tenant_id", "student_id", "joined_at").StorageKey("session_memberships_student_idx"),
	}
}
