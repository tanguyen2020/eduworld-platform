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

// GameSession — game.game_sessions (T, DATABASE_DESIGN §3.12, ADR-0031): the server-side play session of one
// student on one device. At most one active session per student (unique partial index); ended sessions keep
// their end reason and the session that replaced them. Never physically deleted (ADR-0028).
type GameSession struct{ ent.Schema }

func (GameSession) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "game_sessions"}}
}
func (GameSession) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "game_session_id"}}}
}
func (GameSession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("device_id", uuid.UUID{}).Immutable(),
		field.String("device_model").MaxLen(128).Optional().Nillable().Immutable(),
		field.String("client_platform").MaxLen(64).Immutable(),
		field.String("client_version").MaxLen(64).Immutable(),
		field.String("keycloak_sid").MaxLen(64).Optional().Nillable().Immutable(),
		field.Time("started_at").Default(nowUTC).Immutable(),
		field.Time("last_heartbeat_at").Default(nowUTC),
		field.Time("ended_at").Optional().Nillable(),
		field.String("end_reason").MaxLen(64).Optional().Nillable(),
		field.UUID("replaced_by_session_id", uuid.UUID{}).Optional().Nillable(),
	}
}
func (GameSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("student_id").Unique().StorageKey("game_sessions_one_active_uq").
			Annotations(entsql.IndexWhere("ended_at IS NULL AND deleted_at IS NULL")),
		index.Fields("last_heartbeat_at").StorageKey("game_sessions_heartbeat_idx").
			Annotations(entsql.IndexWhere("ended_at IS NULL")),
	}
}

// LoginEvent — game.login_events (TA, monthly range partitions on login_at, DATABASE_DESIGN §3.12): one row
// per entry into the game. Append-only except the single update that records logout_at/end_reason; no raw
// IP (only an HMAC), never returned with device_id or ip_hash (ADR-0031, AGENTS R23). The table is created by
// hand in the migration (composite key with the partition key); the ent schema describes its columns.
type LoginEvent struct{ ent.Schema }

func (LoginEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "game", Table: "login_events"}}
}
func (LoginEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "login_event_id"}}}
}
func (LoginEvent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("game_session_id", uuid.UUID{}).Immutable(),
		field.UUID("device_id", uuid.UUID{}).Immutable(),
		field.String("platform").MaxLen(64).Immutable(),
		field.String("device_model").MaxLen(128).Optional().Nillable().Immutable(),
		field.String("client_version").MaxLen(64).Optional().Nillable().Immutable(),
		field.Bytes("ip_hash").Optional().Nillable().Immutable(),
		field.Time("login_at").Default(nowUTC).Immutable(),
		field.Time("logout_at").Optional().Nillable(),
		field.String("end_reason").MaxLen(64).Optional().Nillable(),
		field.UUID("sis_connection_id", uuid.UUID{}).Optional().Nillable().Immutable(),
	}
}
