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

// FriendRequest — social.friend_requests (T, DATABASE_DESIGN §2.11, F095): a friend request between two students of one
// tenant. status pending/accepted/rejected/cancelled/expired; a rejected request still shows pending to the requester
// until expires_at (silent rejection, SOCIAL_SYSTEM §4.1). Never physically deleted (ADR-0028).
type FriendRequest struct{ ent.Schema }

func (FriendRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "social", Table: "friend_requests"}}
}
func (FriendRequest) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "request_id"}}}
}
func (FriendRequest) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("requester_id", uuid.UUID{}).Immutable(),
		field.UUID("addressee_id", uuid.UUID{}).Immutable(),
		field.String("status").MaxLen(32).Default("pending"),
		field.String("source").MaxLen(64).Default("direct").Immutable(),
		field.Time("expires_at").Immutable(),
		field.Time("responded_at").Optional().Nillable(),
	}
}
func (FriendRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("requester_id", "addressee_id").Unique().StorageKey("friend_requests_pending_uq").
			Annotations(entsql.IndexWhere("status = 'pending' AND deleted_at IS NULL")),
		index.Fields("tenant_id", "addressee_id", "status", "created_at").StorageKey("friend_requests_addressee_idx"),
		index.Fields("tenant_id", "requester_id", "created_at").StorageKey("friend_requests_requester_idx"),
		index.Fields("status", "expires_at").StorageKey("friend_requests_expiry_idx").
			Annotations(entsql.IndexWhere("status = 'pending'")),
	}
}

// Friendship — social.friendships (T, DATABASE_DESIGN §2.11): a symmetric friendship stored once per normalized pair
// (user_low_id < user_high_id). Removing or blocking soft-deletes the row; befriending again creates a new row.
type Friendship struct{ ent.Schema }

func (Friendship) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "social", Table: "friendships"}}
}
func (Friendship) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "friendship_id"}}}
}
func (Friendship) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_low_id", uuid.UUID{}).Immutable(),
		field.UUID("user_high_id", uuid.UUID{}).Immutable(),
	}
}
func (Friendship) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_low_id", "user_high_id").Unique().StorageKey("friendships_pair_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "user_low_id").StorageKey("friendships_low_idx"),
		index.Fields("tenant_id", "user_high_id").StorageKey("friendships_high_idx"),
	}
}

// FriendCode — social.friend_codes (T, SOCIAL_SYSTEM §4.1): the short-lived code of one student (one live code each);
// a new code or the expiry job soft-deletes the previous one.
type FriendCode struct{ ent.Schema }

func (FriendCode) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "social", Table: "friend_codes"}}
}
func (FriendCode) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "friend_code_id"}}}
}
func (FriendCode) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(12).Immutable(),
		field.UUID("owner_id", uuid.UUID{}).Immutable(),
		field.Time("expires_at").Immutable(),
	}
}
func (FriendCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").Unique().StorageKey("friend_codes_code_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("owner_id").Unique().StorageKey("friend_codes_owner_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// Block — social.blocks (T, SOCIAL_SYSTEM §5): one-way relation with a two-way effect on interactions.
type Block struct{ ent.Schema }

func (Block) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "social", Table: "blocks"}}
}
func (Block) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "block_id"}}}
}
func (Block) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("blocker_id", uuid.UUID{}).Immutable(),
		field.UUID("blocked_id", uuid.UUID{}).Immutable(),
	}
}
func (Block) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("blocker_id", "blocked_id").Unique().StorageKey("blocks_pair_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "blocked_id").StorageKey("blocks_blocked_idx"),
	}
}

// Mute — social.mutes (T, SOCIAL_SYSTEM §5): one-way relation that only affects the muter.
type Mute struct{ ent.Schema }

func (Mute) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "social", Table: "mutes"}}
}
func (Mute) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "mute_id"}}}
}
func (Mute) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("muter_id", uuid.UUID{}).Immutable(),
		field.UUID("muted_id", uuid.UUID{}).Immutable(),
	}
}
func (Mute) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("muter_id", "muted_id").Unique().StorageKey("mutes_pair_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "muted_id").StorageKey("mutes_muted_idx"),
	}
}
