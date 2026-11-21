package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// LobbyStore keeps the ephemeral Party Lobby state in Valkey (ew:party:<party_id>:lobby, ew:party:active,
// ew:rl:party_lobby:<user_id>, VALKEY_DESIGN §3.2): rebuildable, never the source of truth (ADR-0004). A lost state
// reads as empty: Ready false, timers restart.
type LobbyStore interface {
	// Load returns the lobby state of a party (empty when lost or never written).
	Load(ctx context.Context, party uuid.UUID) (party.Lobby, error)
	// SetReady sets the Ready flag of a member.
	SetReady(ctx context.Context, party, student uuid.UUID, ready bool) error
	// ResetReady clears every Ready flag of the party.
	ResetReady(ctx context.Context, party uuid.UUID) error
	// Save writes the whole lobby state (presence sweeper).
	Save(ctx context.Context, party uuid.UUID, l party.Lobby) error
	// Drop deletes the lobby state of a disbanded party and stops tracking it.
	Drop(ctx context.Context, tenant, party uuid.UUID) error
	// Track registers an active party for the presence sweeper (ew:party:active).
	Track(ctx context.Context, tenant, party uuid.UUID, at time.Time) error
	// Tracked returns up to limit tracked parties.
	Tracked(ctx context.Context, limit int) ([]PartyRef, error)
	// Allow counts one lobby command of student against party_lobby_cmd_rate (per second); false with the wait when over.
	Allow(ctx context.Context, student uuid.UUID, perSecond int) (bool, time.Duration, error)
}

// PartyRef identifies a party of a tenant (sweeper).
type PartyRef struct {
	TenantID uuid.UUID
	PartyID  uuid.UUID
}

// JobLock is a lease taken by one replica for one run of a job (ew:lock:job:<name>).
type JobLock interface {
	Acquire(ctx context.Context, name string, ttl time.Duration) (bool, error)
}

// PresenceStatuses returns the raw presence status of students (PresenceStatus of the realtime gateway, "" when no
// signal or expired) from the party presence store.
type PresenceStatuses interface {
	Statuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// ErrGameUnavailable: module game did not answer (no facade yet, error or deadline) — 503 UNAVAILABLE with Retry-After.
var ErrGameUnavailable = errors.New("party: module game unavailable")

// ErrGameRefused: module game refused the activity (unknown activity, conflict) — the request is not retryable.
var ErrGameRefused = errors.New("party: module game refused the activity")

// GameCandidate is a quest/boss activity offered by module game for the members of a party.
type GameCandidate struct {
	Type        string
	Ref         string
	Title       map[string]string
	Eligibility party.Eligibility
}

// GameStart is the request of startPartyActivity (Idempotency-Key = party_activity_id).
type GameStart struct {
	ActivityID uuid.UUID
	PartyID    uuid.UUID
	LeaderID   uuid.UUID
	Type       string
	Ref        string
	GameMode   string
	Members    []uuid.UUID
	Sessions   map[uuid.UUID]uuid.UUID // known game sessions
}

// GameStarted is the session created by module game: the encounter (boss) or the shared quest session (quest).
type GameStarted struct {
	ExternalRef uuid.UUID
	RoomType    string
	AreaID      string
	SceneRef    string
}

// GameActivities is the facade game for quest and boss party activities (listPartyActivityCandidates,
// checkPartyActivityEligibility, startPartyActivity — research R1); called outside any transaction of module
// competition with the TenantScope of the party (R29). Errors: ErrGameUnavailable, ErrGameRefused.
type GameActivities interface {
	Candidates(ctx context.Context, partyID uuid.UUID, members []uuid.UUID) ([]GameCandidate, error)
	Check(ctx context.Context, partyID uuid.UUID, typ, ref string, members []uuid.UUID) (party.Eligibility, error)
	Start(ctx context.Context, s GameStart) (GameStarted, error)
}

// LobbyMetrics observes the lobby and the party sessions (eduworld_party_*; labels never carry ids).
type LobbyMetrics interface {
	LobbyUpdate(d time.Duration)
	SessionStart(activityType, result string)
	LeaderAutoTransfer()
	ActivityEnd(reason string)
}
