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

// PartyActivity — multiplayer.party_activities (T, DATABASE_DESIGN §2.9, F101): one play session of a party started
// from the Party Lobby (PARTY_SYSTEM §6). running → ended once; at most one running activity per party. result_summary
// holds the shared progress only (≤ 4 KB), never a score, result or mastery of a student (FR-PTY-013). room_id is the
// private_instance room of module competition (explore, quest) or the boss encounter of module game (no FK).
type PartyActivity struct{ ent.Schema }

func (PartyActivity) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "multiplayer", Table: "party_activities"}}
}
func (PartyActivity) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "party_activity_id"}}}
}
func (PartyActivity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("party_id", uuid.UUID{}).Immutable(),
		field.String("activity_type").MaxLen(64).Immutable(),
		field.String("activity_ref").MaxLen(64).Immutable(),
		field.String("game_mode").MaxLen(64).Optional().Nillable().Immutable(),
		field.UUID("room_id", uuid.UUID{}).Immutable(),
		field.String("room_type").MaxLen(64).Immutable(),
		field.UUID("external_ref", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("status").MaxLen(64).Default("running"),
		field.Int("min_members").Immutable(),
		field.Int("max_members").Immutable(),
		field.Bool("allow_continue_on_disconnect").Immutable(),
		field.Time("started_at").Immutable(),
		field.Time("ended_at").Optional().Nillable(),
		field.String("end_reason").MaxLen(64).Optional().Nillable(),
		field.JSON("result_summary", map[string]any{}).Optional(),
	}
}
func (PartyActivity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("party_id").Unique().StorageKey("party_activities_one_running_uq").
			Annotations(entsql.IndexWhere("status = 'running' AND deleted_at IS NULL")),
		index.Fields("room_id").StorageKey("party_activities_room_idx"),
		index.Fields("tenant_id", "party_id", "started_at").StorageKey("party_activities_party_idx"),
	}
}
