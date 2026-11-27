package write

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
)

// ErrResultRecorded: the match already has its result (competition.match_results PK match_id, AC-CMP-005).
var ErrResultRecorded = errors.New("battle: match result already recorded")

// BattleParticipant is one student of a match.
type BattleParticipant struct {
	StudentID uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	TeamNo    int
}

// BattleMatch is a match as the battle engine reads it (competition.matches + match_participants of the tenant).
type BattleMatch struct {
	ID               uuid.UUID
	Mode             string
	Type             string
	SubjectID        uuid.UUID
	TopicID          *uuid.UUID
	Grade            int
	Status           string
	Ranked           bool
	IsBot            bool
	ScopeKind        string
	TournamentID     *uuid.UUID
	TournamentNodeID *uuid.UUID
	RulesetVersionID uuid.UUID
	TenantIDs        []uuid.UUID
	PlanID           *uuid.UUID
	Participants     []BattleParticipant
}

// BattleRoomRow is the summary record of a match room (multiplayer.rooms, room_id = match_id).
type BattleRoomRow struct {
	RoomID   uuid.UUID
	TenantID uuid.UUID
	RoomType string // battle | coop
	Mode     string
	SchoolID *uuid.UUID
	Capacity int
	MatchID  uuid.UUID
	At       time.Time
}

// MatchOutcome is what RecordResult writes: the result row, the participants and the match.
type MatchOutcome struct {
	Result     battle.Result
	RuleID     uuid.UUID
	TenantIDs  []uuid.UUID
	Summary    map[string]any
	RecordedBy string
	At         time.Time
}

// IntegritySignalRow is one row of competition.integrity_signals (ids and counters only, never a raw answer).
type IntegritySignalRow struct {
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	StudentID uuid.UUID
	MatchID   uuid.UUID
	Type      string
	Score     float64
	Evidence  map[string]any
	At        time.Time
}

// Restriction is a time-boxed restriction of competition (competition.competition_restrictions).
type Restriction struct {
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	StudentID uuid.UUID
	Kind      string
	Reason    string
	StartsAt  time.Time
	EndsAt    time.Time
}

// BattleRepository persists the battle engine in the transaction of ctx (TenantScope of the match; matches and
// match_results are G* tables reached through match_participants of the tenant, R27).
type BattleRepository interface {
	// BattleMatch returns a match of the tenant of ctx with its participants (nil when none).
	BattleMatch(ctx context.Context, id uuid.UUID) (*BattleMatch, error)
	// ConfigPayload returns the payload of a published configuration version (ruleset_version_id).
	ConfigPayload(ctx context.Context, versionID uuid.UUID) (json.RawMessage, error)
	// PinPlan writes the plan pinned for the match and its room id.
	PinPlan(ctx context.Context, match, plan uuid.UUID, version string, items json.RawMessage) error
	// OpenRoom inserts the room record (no-op when it exists).
	OpenRoom(ctx context.Context, r BattleRoomRow) error
	// MarkRunning moves the match from lobby to running.
	MarkRunning(ctx context.Context, match uuid.UUID, at time.Time) error
	// RecordResult writes match_results, the participants and the match once (ErrResultRecorded the second time).
	RecordResult(ctx context.Context, o MatchOutcome) error
	// AbortMatch moves a lobby/running match to aborted; false when it was over already.
	AbortMatch(ctx context.Context, match uuid.UUID, reason string, at time.Time) (bool, error)
	// CloseRoom closes the room record and the memberships still open (room_closed).
	CloseRoom(ctx context.Context, room uuid.UUID, at time.Time) error
	// JoinRoom records a membership (student, game session) of the room.
	JoinRoom(ctx context.Context, room uuid.UUID, roomType string, match uuid.UUID, school uuid.UUID, student, session uuid.UUID, at time.Time) error
	// LeaveRoom closes the open membership of the student (of that session when not Nil).
	LeaveRoom(ctx context.Context, room, student, session uuid.UUID, reason string, at time.Time) error
	// Disconnected counts a disconnection on the open membership.
	Disconnected(ctx context.Context, room, student uuid.UUID) error
	InsertSignal(ctx context.Context, s IntegritySignalRow) error
	// Abandons counts the ranked matches the student abandoned since (outcome abandoned).
	Abandons(ctx context.Context, student uuid.UUID, since time.Time) (int, error)
	// RecentOutcomes returns the outcomes of the latest completed matches of the student (newest first, at most limit;
	// no_contest left out): the loss streak of COMPETITION_SYSTEM §12.
	RecentOutcomes(ctx context.Context, student uuid.UUID, limit int) ([]string, error)
	// RestrictedUntil returns the end of a restriction of kind in force at (nil when none).
	RestrictedUntil(ctx context.Context, student uuid.UUID, kind string, at time.Time) (*time.Time, error)
	InsertRestriction(ctx context.Context, r Restriction) (uuid.UUID, error)
}
