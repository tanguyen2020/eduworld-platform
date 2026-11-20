// Package read holds the read-context ports of module chat (ADR-0087): read models only, under entx.ReadSession; no
// writes, no events.
package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// After is the keyset position of a page (created_at, row id), nil for the first page.
type After struct {
	At time.Time
	ID uuid.UUID
}

// Relation is one row of listFriends, listBlocks or listMutes: the other student and the public card (R16).
type Relation struct {
	RowID     uuid.UUID // friendship_id, block_id or mute_id (cursor)
	UserID    uuid.UUID
	Nickname  *string
	Level     int
	Avatar    json.RawMessage
	CreatedAt time.Time
}

// Request is one row of listFriendRequests with the cards of both students.
type Request struct {
	ID          uuid.UUID
	RequesterID uuid.UUID
	AddresseeID uuid.UUID
	Status      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Requester   Relation
	Addressee   Relation
}

// SocialReader reads the relations of one student in the session tenant (RLS plus explicit tenant_id, deleted_at).
type SocialReader interface {
	// Friends lists the live friends of user whose student is not cleaned up, newest first.
	Friends(ctx context.Context, tenant, user uuid.UUID, after *After, limit int) ([]Relation, error)
	// Requests lists the requests addressed to (incoming: live pending) or sent by (outgoing: pending or rejected,
	// not expired) user, newest first, at most limit.
	Requests(ctx context.Context, tenant, user uuid.UUID, incoming bool, now time.Time, limit int) ([]Request, error)
	// Blocks lists the live blocks created by user, newest first.
	Blocks(ctx context.Context, tenant, user uuid.UUID, after *After, limit int) ([]Relation, error)
	// Mutes lists the live mutes created by user, newest first.
	Mutes(ctx context.Context, tenant, user uuid.UUID, after *After, limit int) ([]Relation, error)
	// FriendIDs returns the live friends of user without a block in either direction (presence fan-out).
	FriendIDs(ctx context.Context, tenant, user uuid.UUID) ([]uuid.UUID, error)
}
