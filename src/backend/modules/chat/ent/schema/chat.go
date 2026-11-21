package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Channel — chat.channels (T, DATABASE_DESIGN §2.12, F115): a conversation space of one tenant. channel_type is one of
// world/area/match/party/team/direct (no class/study_group — ADR-0095); is_gameplay is true for match, party and team
// bound to a match (CHECK in the migration). last_seq only grows (chat-persister, GREATEST); seq itself is allocated in Valkey before the
// JetStream publish (ADR-0035). Gameplay channels are purged by gameplay-chat-purge only (ADR-0033, F117).
type Channel struct{ ent.Schema }

func (Channel) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "channels"}}
}
func (Channel) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "channel_id"}}}
}
func (Channel) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.String("channel_type").MaxLen(64).Immutable(),
		field.Bool("is_gameplay").Default(false).Immutable(),
		field.JSON("scope_ref", map[string]string{}).Optional(),
		field.UUID("policy_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("retention_days").Optional().Nillable(),
		field.Int64("last_seq").Default(0),
		field.String("status").MaxLen(64).Default("open"),
		field.Time("opened_at"),
		field.Time("closed_at").Optional().Nillable(),
		field.Time("purge_after").Optional().Nillable(),
	}
}
func (Channel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "channel_type", "status").StorageKey("channels_type_status_idx"),
	}
}

// ChannelMember — chat.channel_members (T, DATABASE_DESIGN §2.12, CHAT_SYSTEM §4): one student in one channel. A live
// row per (channel_id, user_id); leaving sets left_at (history stays readable from joined_at only while a member).
// Cursors only grow. Staff is never a member (ADR-0031, ADR-0072).
type ChannelMember struct{ ent.Schema }

func (ChannelMember) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "channel_members"}}
}
func (ChannelMember) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "channel_member_id"}}}
}
func (ChannelMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("channel_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.String("role").MaxLen(64).Default("member"),
		field.Time("joined_at"),
		field.Time("left_at").Optional().Nillable(),
		field.Time("muted_until").Optional().Nillable(),
		field.Int64("last_delivered_seq").Default(0),
		field.Int64("last_read_seq").Default(0),
		field.Int("unread_mentions").Default(0),
		field.Bool("show_read_receipts").Default(true),
		field.String("notification_level").MaxLen(64).Default("all"),
	}
}
func (ChannelMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "user_id").Unique().StorageKey("channel_members_pair_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "user_id").StorageKey("channel_members_user_idx"),
	}
}

// ChatPolicy — chat.chat_policies (T*, DATABASE_DESIGN §2.12, CHAT_SYSTEM §3): the policy document of one level
// (platform → tenant → school, ADR-0095) validated against specs/schemas/chat-policy.schema.json v5. Platform rows have no
// tenant (written by chat_platform only: the seed migration, F120). No grade condition exists (ADR-0020, R15).
type ChatPolicy struct{ ent.Schema }

func (ChatPolicy) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "chat_policies"}}
}
func (ChatPolicy) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "policy_id"}}}
}
func (ChatPolicy) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope_type").MaxLen(64).Immutable(),
		field.UUID("scope_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.JSON("rules", json.RawMessage{}),
	}
}
func (ChatPolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "scope_type").StorageKey("chat_policies_tenant_scope_idx"),
	}
}
