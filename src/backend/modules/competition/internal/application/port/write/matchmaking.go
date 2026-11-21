package write

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
)

// MatchParticipant is one student of a new match.
type MatchParticipant struct {
	StudentID uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	TeamNo    int
	PartyID   *uuid.UUID
	Rating    *mm.Rating // internal: rating at match time (never returned to a client)
}

// NewMatch is a match to create (competition.matches + match_participants); its id is the proposal id, the duel or
// the bot ticket derived id, so a retry creates nothing (F104-AC-05, edge case).
type NewMatch struct {
	ID               uuid.UUID
	Mode             string
	Type             string
	SubjectID        uuid.UUID
	TopicID          *uuid.UUID
	Grade            int
	Ranked           bool
	IsBot            bool
	ScopeKind        string
	TenantIDs        []uuid.UUID
	TournamentNodeID *uuid.UUID
	DuelID           *uuid.UUID
	RulesetVersionID uuid.UUID
	Participants     []MatchParticipant
	At               time.Time
}

// Match is a match as one of its participants sees it.
type Match struct {
	ID        uuid.UUID
	Mode      string
	Type      string
	SubjectID uuid.UUID
	Grade     int
	Status    string
	Ranked    bool
	IsBot     bool
	Teams     map[uuid.UUID]int // student → team
	CreatedAt time.Time
}

// DuelRef identifies a pending duel of any tenant (job duel-expiry).
type DuelRef struct {
	TenantID uuid.UUID
	DuelID   uuid.UUID
}

// ErrDuelPending: a pending duel from the challenger to the opponent exists (duels_pending_uq).
var ErrDuelPending = errors.New("matchmaking: pending duel exists")

// Pair is an unordered pair of students (smaller uuid string first).
type Pair [2]uuid.UUID

// PairOf returns the pair of a and b.
func PairOf(a, b uuid.UUID) Pair {
	if a.String() > b.String() {
		a, b = b, a
	}
	return Pair{a, b}
}

// MatchRepository reads the projections matchmaking needs and persists matches and duels (competition.*) in the
// transaction of ctx with explicit tenant and deleted_at conditions (R27). Data of other modules come from ref.*.
type MatchRepository interface {
	// ActiveConfig returns the published matchmaking configuration in effect at (latest effective_from ≤ at).
	ActiveConfig(ctx context.Context, at time.Time) (mm.Config, error)
	// Players returns the matchmaking view of the students of tenant at now (missing: not projected in tenant).
	Players(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, now time.Time) (map[uuid.UUID]mm.Player, error)
	// BlockedPairs returns the pairs among ids with a live block, either direction (H6).
	BlockedPairs(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) (map[Pair]bool, error)
	// RecentPairs returns the pairs among ids who played the same match since (soft scoring).
	RecentPairs(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, since time.Time) (map[Pair]bool, error)
	// CreateMatch inserts the match and its participants; false when the match id exists (nothing written).
	CreateMatch(ctx context.Context, m NewMatch) (bool, error)
	// MatchOf returns a match of tenant in which student takes part, nil otherwise.
	MatchOf(ctx context.Context, tenant, id, student uuid.UUID) (*Match, error)
	// CreateDuel inserts a pending duel (ErrDuelPending on the pending unique index).
	CreateDuel(ctx context.Context, d mm.Duel) error
	// Duel returns a duel of tenant (locked FOR UPDATE when lock), nil when unknown.
	Duel(ctx context.Context, tenant, id uuid.UUID, lock bool) (*mm.Duel, error)
	// SaveDuel writes status, match_id and responded_at.
	SaveDuel(ctx context.Context, d mm.Duel) error
	// DuelsSince counts the duels sent by challenger since (duel rate).
	DuelsSince(ctx context.Context, tenant, challenger uuid.UUID, since time.Time) ([]time.Time, error)
}

// DuelExpiryRepository is the platform side of the job duel-expiry (SystemPlatformScope: every tenant).
type DuelExpiryRepository interface {
	ExpiredDuels(ctx context.Context, now time.Time, limit int) ([]DuelRef, error)
}
