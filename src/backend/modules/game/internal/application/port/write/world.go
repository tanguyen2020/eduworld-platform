package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
)

// Game content item types of the release core (F071; F072/F073 add quest, minigame_config).
const (
	ItemWorldArea    = "world_area"
	ItemStoryChapter = "story_chapter"
	ItemNPCDialogue  = "npc_dialogue"
	ItemLevel        = "level"
)

// ContentHead is the state of an item before a new version: its current version and hash and the next version number.
type ContentHead struct {
	CurrentVersionID *uuid.UUID
	CurrentSHA256    string
	NextVersionNo    int
}

// ContentVersion is a new published version of a game content item.
type ContentVersion struct {
	ItemType       string
	ItemID         string
	VersionID      uuid.UUID
	VersionNo      int
	Payload        json.RawMessage // stored payload (the type's JSON Schema)
	SHA256         string          // hash of the authored content (server fields excluded)
	Note           string
	SnapshotKey    string
	SnapshotSHA256 string
	At             time.Time
	By             uuid.UUID
	Columns        map[string]any // typed columns of the version table (region_code, grade_levels…)
}

// ContentRelease is one row of game.content_releases (append-only).
type ContentRelease struct {
	ReleaseID      uuid.UUID
	ItemType       string
	ItemID         string
	Action         string // publish (F078: rollback, unpublish)
	From, To       *uuid.UUID
	Reason         string
	By             uuid.UUID
	At             time.Time
	IdempotencyKey string
}

// GameContentStore writes the game content under the PlatformScope of the transaction (role game_platform).
type GameContentStore interface {
	// Head creates the identity row when missing (parent columns: npc_id of a dialogue, area_id of a level), locks it and
	// returns its head.
	Head(ctx context.Context, itemType, itemID string, parent map[string]any, at time.Time) (ContentHead, error)
	// Publish inserts the version as published, archives the previous published version and moves the pointer.
	Publish(ctx context.Context, v ContentVersion) error
	// Release appends a release row.
	Release(ctx context.Context, r ContentRelease) error
}

// LevelSession is game.level_sessions (ADR-0043: the versions pinned at start).
type LevelSession struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	StudentID       uuid.UUID
	GameSessionID   uuid.UUID
	LevelID         uuid.UUID
	LevelVersionID  uuid.UUID
	LevelVersionNo  int
	AreaID          string
	ManifestVersion int64
	ContentVersions []map[string]any
	Status          string
	StartedAt       time.Time
	EndedAt         *time.Time
}

// Level session statuses.
const (
	LevelActive    = "active"
	LevelCompleted = "completed"
	LevelAbandoned = "abandoned"
	LevelExpired   = "expired"
)

// WorldStore writes the world progress of a student in the tenant transaction of ctx (explicit tenant conditions).
type WorldStore interface {
	// LockStudent serializes the world progress writes of one student (pg_advisory_xact_lock).
	LockStudent(ctx context.Context, student uuid.UUID) error
	// RecordFact stores a fact once (kind quest_completed, achievement_unlocked, event_participation) or raises the
	// level fact; false when nothing changed.
	RecordFact(ctx context.Context, tenant, student uuid.UUID, kind, ref string, value int, at time.Time) (bool, error)
	// InsertUnlock records an unlock once; false when the area was already unlocked.
	InsertUnlock(ctx context.Context, tenant, student uuid.UUID, areaID string, ruleVersion int, by string, at time.Time) (bool, error)
	// SaveStory upserts the progress of a chapter.
	SaveStory(ctx context.Context, tenant, student uuid.UUID, chapter string, versionID uuid.UUID, p world.Progress, done int,
		completedAt *time.Time, at time.Time) error
	// SaveExploration upserts the exploration of an area.
	SaveExploration(ctx context.Context, tenant, student uuid.UUID, areaID string, e world.Exploration, pct float64,
		completedAt *time.Time, at time.Time) error
	// SetLastArea writes game_profiles.last_area_id (public areas only, WORLD_DESIGN §6).
	SetLastArea(ctx context.Context, tenant, student uuid.UUID, areaID string, at time.Time) error
	// ActiveLevelSession returns the active level session of the student (locked), nil when none.
	ActiveLevelSession(ctx context.Context, tenant, student uuid.UUID) (*LevelSession, error)
	// LevelSession returns a level session of tenant (locked), nil when unknown.
	LevelSession(ctx context.Context, tenant, id uuid.UUID) (*LevelSession, error)
	// CreateLevelSession inserts a new active level session.
	CreateLevelSession(ctx context.Context, s LevelSession) error
	// EndLevelSession sets the final status and ended_at.
	EndLevelSession(ctx context.Context, tenant, id uuid.UUID, status string, at time.Time) error
}
