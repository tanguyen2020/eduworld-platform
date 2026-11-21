package port

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

// ErrModerationUnavailable: module chat did not answer the moderation of a display text in time (F100-AC-05: the
// command then stores the generated default name).
var ErrModerationUnavailable = errors.New("party: name moderation unavailable")

// NameModerator checks a name typed by a student with the moderation pipeline of module chat (facade
// moderateDisplayText, MODERATION §1): true = allowed. Never returns the matched words (R15).
type NameModerator interface {
	Check(ctx context.Context, tenant, author uuid.UUID, text string) (bool, error)
}

// PartyPresence tells which students are online (last presence signal of the realtime gateway, presence_ttl).
type PartyPresence interface {
	Online(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

// PartyRealtime pushes a realtime message to each student (ew:rt:user:<id>, research R1); after commit only.
type PartyRealtime interface {
	Send(ctx context.Context, to []uuid.UUID, typ string, payload any) error
}

// PartySnapshot is the realtime state of a party kept in Valkey (ew:party:*, VALKEY_DESIGN §3.2): rebuildable from
// PostgreSQL, never the source of truth (ADR-0004).
type PartySnapshot struct {
	PartyID      uuid.UUID
	LeaderID     uuid.UUID
	Type         string
	MaxMembers   int
	Status       string
	StateVersion int64
	Members      map[uuid.UUID]string // student → role
}

// PartyStateCache writes the realtime state of a party after commit (best effort).
type PartyStateCache interface {
	Sync(ctx context.Context, s PartySnapshot) error
	Drop(ctx context.Context, party uuid.UUID, students []uuid.UUID) error
}

// PartyMetrics observes the party use cases (eduworld_party_*); labels never carry ids.
type PartyMetrics interface {
	Command(command, result string)
	Invitation(result string)
	PolicyDenied(rule string)
}

// Card is the public card of a student (ref.player_profiles): nickname, level, avatar — never a real name, school or
// class (R16).
type Card struct {
	UserID   uuid.UUID
	Nickname *string
	Level    int
	Avatar   json.RawMessage
}
