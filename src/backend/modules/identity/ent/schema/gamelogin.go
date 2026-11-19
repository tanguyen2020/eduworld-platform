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

// LoginEvent — identity.login_events (TA*, partitioned by month on occurred_at): one row per login
// attempt (game and portal). Rows with a tenant are written by identity_app under the TenantScope, rows
// without (failure before the tenant is known) by identity_platform under the PlatformScope (ADR-0089 §2).
// No username, password, token or raw IP (only an HMAC of the IP). The table is created by hand in the
// migration (composite key with the partition key); the ent schema describes its columns.
type LoginEvent struct{ ent.Schema }

func (LoginEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "login_events"}}
}
func (LoginEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptionalAppendOnly{PK: entx.PK{Column: "login_event_id"}}}
}
func (LoginEvent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("occurred_at").Default(nowUTC).Immutable(),
		field.String("client").MaxLen(64).Immutable(),
		field.Bytes("ip_hash").Optional().Nillable().Immutable(),
		field.UUID("sis_connection_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("device_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("result").MaxLen(64).Immutable(),
		field.String("tenant_resolution").MaxLen(64).Optional().Nillable().Immutable(),
	}
}

// GameAuthSession — identity.game_auth_sessions (T, ADR-0086): one game login session bound to a device,
// with the SHA-256 of the current refresh token and up to 20 rotated hashes (reuse detection). No raw token.
type GameAuthSession struct{ ent.Schema }

func (GameAuthSession) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "game_auth_sessions"}}
}
func (GameAuthSession) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "game_auth_session_id"}}}
}
func (GameAuthSession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.String("keycloak_sid").MaxLen(64).Immutable(),
		field.UUID("device_id", uuid.UUID{}).Immutable(),
		field.String("platform").MaxLen(16).Immutable(),
		field.String("refresh_token_hash").MaxLen(64),
		field.JSON("previous_token_hashes", []string{}).Default([]string{}),
		field.Time("last_refreshed_at").Default(nowUTC),
		field.Time("revoked_at").Optional().Nillable(),
		field.String("revoke_reason").MaxLen(32).Optional().Nillable(),
	}
}
func (GameAuthSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("keycloak_sid").Unique().StorageKey("game_auth_sessions_sid_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("refresh_token_hash").Unique().StorageKey("game_auth_sessions_token_uq").
			Annotations(entsql.IndexWhere("revoked_at IS NULL AND deleted_at IS NULL")),
		index.Fields("tenant_id", "user_id").StorageKey("game_auth_sessions_user_idx").
			Annotations(entsql.IndexWhere("revoked_at IS NULL AND deleted_at IS NULL")),
		index.Fields("device_id").StorageKey("game_auth_sessions_device_idx").
			Annotations(entsql.IndexWhere("revoked_at IS NULL AND deleted_at IS NULL")),
	}
}
