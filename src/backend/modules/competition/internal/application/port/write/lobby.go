package write

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// ErrActivityRunning: the party already has a running activity (unique party_activities_one_running_uq).
var ErrActivityRunning = errors.New("party: an activity is already running")

// SessionRoom is the summary record of the room of a party session (multiplayer.rooms): private_instance of module
// competition, or the boss encounter of module game (room_id = encounter_id).
type SessionRoom struct {
	RoomID    uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	GradeBand string
	RoomType  string
	AreaID    string
	Capacity  int
	CreatedAt time.Time
}

// PartyRoom is the open session room of a student (open membership of a running party activity).
type PartyRoom struct {
	RoomID     uuid.UUID
	RoomType   string
	AreaID     string
	PartyID    uuid.UUID
	ActivityID uuid.UUID
	SchoolID   uuid.UUID
	GradeBand  string
	Capacity   int
	Members    []uuid.UUID // students with an open membership of the activity
}

// ActivityRepository persists party activities, their rooms and session memberships (multiplayer.party_activities,
// rooms, session_memberships) in the transaction of ctx with explicit tenant_id and deleted_at conditions (R27).
type ActivityRepository interface {
	// Running returns the running activity of a party (locked FOR UPDATE), nil when none.
	Running(ctx context.Context, tenant, partyID uuid.UUID) (*party.Activity, error)
	// Get returns an activity of the tenant (locked FOR UPDATE), nil when unknown.
	Get(ctx context.Context, tenant, id uuid.UUID) (*party.Activity, error)
	// Create inserts a running activity and points the party at it (ErrActivityRunning).
	Create(ctx context.Context, a *party.Activity) error
	// End writes the end of an activity and clears the current activity of the party.
	End(ctx context.Context, a *party.Activity) error
	// CreateRoom inserts the room summary (idempotent on room_id).
	CreateRoom(ctx context.Context, r SessionRoom) error
	// CloseRoom closes the room summary (idempotent).
	CloseRoom(ctx context.Context, tenant, room uuid.UUID, at time.Time) error
	// OpenMemberships inserts one open session membership per participant (game_session_id when known).
	OpenMemberships(ctx context.Context, a *party.Activity, students []uuid.UUID, sessions map[uuid.UUID]uuid.UUID, at time.Time) error
	// OpenMembers returns the participants of an activity whose membership is open.
	OpenMembers(ctx context.Context, tenant, activity uuid.UUID) (map[uuid.UUID]bool, error)
	// CloseMembership ends the open membership of a student in an activity; false when none was open.
	CloseMembership(ctx context.Context, tenant, activity, student uuid.UUID, reason string, at time.Time) (bool, error)
	// Participants returns every student who had a membership of the activity (open or closed).
	Participants(ctx context.Context, tenant, activity uuid.UUID) ([]uuid.UUID, error)
	// CloseMemberships ends every open membership of an activity.
	CloseMemberships(ctx context.Context, tenant, activity uuid.UUID, reason string, at time.Time) error
	// RoomOf returns the open party room of a student in an area ("" = any area), nil when none.
	RoomOf(ctx context.Context, tenant, student uuid.UUID, area string) (*PartyRoom, error)
	// Member reports an open membership of the student in the room.
	Member(ctx context.Context, tenant, room, student uuid.UUID) (bool, error)
	// SetGameSession records the game session of the student entering the room (first entry or another device).
	SetGameSession(ctx context.Context, tenant, room, student, session uuid.UUID) error
}

// AreaDirectory reads the world areas and unlocks projected from module game (ref.world_areas, ref.area_unlocks).
type AreaDirectory interface {
	// Areas returns the active areas (at most limit, ordered by id).
	Areas(ctx context.Context, limit int) ([]party.AreaRule, error)
	// Area returns one area, nil when unknown.
	Area(ctx context.Context, id string) (*party.AreaRule, error)
	// Unlocked returns, per student, whether the area is unlocked for them.
	Unlocked(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, area string) (map[uuid.UUID]bool, error)
	// UnlockedAreas returns the unlocked areas of each student.
	UnlockedAreas(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]map[string]bool, error)
}

// SweepRepository is the platform side of the presence sweeper (SystemPlatformScope): the active parties of every
// tenant to keep ew:party:active complete after a Valkey loss (ADR-0004).
type SweepRepository interface {
	ActiveParties(ctx context.Context, after uuid.UUID, limit int) ([]port.PartyRef, error)
}
