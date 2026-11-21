package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
)

// AreaRow is the current (or a pinned) published version of an area (F071).
type AreaRow struct {
	VersionID uuid.UUID
	VersionNo int
	SHA256    string
	Area      world.Area
}

// ChapterRow is the current published version of a story chapter.
type ChapterRow struct {
	VersionID uuid.UUID
	VersionNo int
	SHA256    string
	Chapter   world.Chapter
}

// LevelRow is the current published version of a level package.
type LevelRow struct {
	LevelID   uuid.UUID
	VersionID uuid.UUID
	VersionNo int
	SHA256    string
	Level     world.Level
}

// DialogueRow is the current published version of an NPC dialogue.
type DialogueRow struct {
	DialogueID uuid.UUID
	VersionID  uuid.UUID
	VersionNo  int
	SHA256     string
	NPCID      string
	Grades     []int
	Payload    json.RawMessage
}

// WorldStudent is the projected student and the subjects its school turned on.
type WorldStudent struct {
	SchoolID        uuid.UUID
	Grade           int
	Status          string
	Deleted         bool
	EnabledSubjects []string // nil = every subject on
}

// Domain returns what the access rules read.
func (s WorldStudent) Domain() world.Student {
	return world.Student{Grade: s.Grade, EnabledSubjects: s.EnabledSubjects}
}

// ExplorationRow is the exploration_progress of one area.
type ExplorationRow struct {
	Exploration world.Exploration
	Pct         float64
	CompletedAt *time.Time
	UpdatedAt   time.Time
}

// WorldProgress is the world progress of one student in a tenant.
type WorldProgress struct {
	Facts       world.Facts
	Story       map[string]*world.Progress
	StoryDone   map[string]time.Time // chapter → completed_at
	Unlocks     map[string]time.Time
	Exploration map[string]*ExplorationRow
}

// StoryState is the chapter state the chapter rules read.
func (p *WorldProgress) StoryState() world.StoryState {
	s := world.StoryState{Completed: map[string]bool{}, SubjectChapters: p.Facts.SubjectChapters}
	for code, sp := range p.Story {
		if sp.Status == world.ChapterCompleted {
			s.Completed[code] = true
		}
	}
	return s
}

// WorldContent reads the published world content and the world progress of a student in the transaction of ctx (read
// or write): explicit tenant and soft-delete conditions (R27).
type WorldContent interface {
	CurrentAreas(ctx context.Context) ([]AreaRow, error)
	CurrentArea(ctx context.Context, areaID string) (*AreaRow, error)
	AreaVersion(ctx context.Context, versionID uuid.UUID) (*AreaRow, error)
	CurrentChapters(ctx context.Context) ([]ChapterRow, error)
	CurrentLevels(ctx context.Context) ([]LevelRow, error)
	CurrentLevel(ctx context.Context, levelID uuid.UUID) (*LevelRow, error)
	LevelOfArea(ctx context.Context, areaID string) (*LevelRow, error)
	CurrentDialogues(ctx context.Context, npcID string) ([]DialogueRow, error)
	Student(ctx context.Context, tenant, student uuid.UUID) (*WorldStudent, error)
	Progress(ctx context.Context, tenant, student uuid.UUID, chapters []ChapterRow) (*WorldProgress, error)
}

// Presence is the temporary presence of a student in an area (Valkey ew:game:presence:<student_id>, ADR-0004: never the
// only copy of important data — the area is re-entered after a loss).
type Presence struct {
	TenantID       uuid.UUID  `json:"tenant_id"`
	GameSessionID  uuid.UUID  `json:"game_session_id"`
	AreaID         string     `json:"area_id"`
	RoomID         uuid.UUID  `json:"room_id"`
	LevelSessionID *uuid.UUID `json:"level_session_id,omitempty"`
	EnteredAt      time.Time  `json:"entered_at"`
}

// PresenceStore keeps the presence of students (TTL = game_session_heartbeat_timeout; the heartbeat of the game session
// extends it). Get returns nil when there is none (expired or lost).
type PresenceStore interface {
	Set(ctx context.Context, student uuid.UUID, p Presence, ttl time.Duration) error
	Get(ctx context.Context, student uuid.UUID) (*Presence, error)
	Touch(ctx context.Context, student uuid.UUID, ttl time.Duration) error
}

// RateLimiter counts actions per student in fixed windows (discovery_report_rate); false with the wait when the limit is
// reached.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error)
}

// WorldRealtime pushes world messages to the connections of a student (best effort after commit).
type WorldRealtime interface {
	AreaUnlocked(ctx context.Context, student uuid.UUID, areaID string, name world.Name, unlockedBy string, at time.Time) error
}

// SnapshotCache keeps the immutable snapshots of published game content (ew:game:cv:<type>:<id>:<version>) and the
// current pointers (ew:game:current:<type>:<id>) — cache only (ADR-0004), PostgreSQL stays authoritative.
type SnapshotCache interface {
	PutSnapshot(ctx context.Context, key string, payload []byte) error
	SetCurrent(ctx context.Context, itemType, itemID string, versionID uuid.UUID) error
}

// HubPosition is where the room host of the hub (F091) last saw a student: area, room and position in metres.
type HubPosition struct {
	AreaID   string
	RoomID   string
	Position world.Vec3
}

// HubPositions reads the hub position of a student through the competition facade (getHubPosition, F071 T049); nil
// position when the student is in no hub instance. ErrHubUnavailable when the facade cannot answer: the caller then
// relies on the presence alone.
type HubPositions interface {
	Position(ctx context.Context, tenant, student uuid.UUID) (*HubPosition, error)
}
