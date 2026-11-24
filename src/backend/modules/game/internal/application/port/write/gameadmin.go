package write

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors of the console stores: a unique key taken (409) and a row that is gone (404).
var (
	ErrConflict = errors.New("write: conflict")
	ErrNotFound = errors.New("write: not found")
)

// ConfigItem is game.config_items: the console key of a game config item and its type-table key.
type ConfigItem struct {
	ConfigID        uuid.UUID
	ItemType        string
	ItemKey         string
	LatestVersionNo int
}

// ConfigVersion is one row of game.config_versions (the review workflow of a version, F078).
type ConfigVersion struct {
	ID          uuid.UUID
	ConfigID    uuid.UUID
	ItemType    string
	VersionNo   int
	Status      string
	Title       *string
	Payload     json.RawMessage
	SHA256      string
	AuthorID    *uuid.UUID
	ReviewerID  *uuid.UUID
	ReviewNote  *string
	ChangeNote  *string
	BasedOn     *uuid.UUID
	SubmittedAt *time.Time
	ReviewedAt  *time.Time
	PublishedAt *time.Time
	PublishedBy *uuid.UUID
	UpdatedAt   time.Time
	Revision    int64 // optimistic version of the row (ETag of a draft)
}

// Schedule is one row of game.scheduled_publications.
type Schedule struct {
	ID          uuid.UUID
	ConfigID    uuid.UUID
	ItemType    string
	VersionID   uuid.UUID
	VersionNo   int
	PublishAt   time.Time
	Status      string
	FailureCode *string
	ChangeNote  *string
	Reason      *string
	RequestedBy uuid.UUID
	CancelledBy *uuid.UUID
	FinishedAt  *time.Time
	CreatedAt   time.Time
}

// Usage is a place that still references a config item (unpublishGameConfig usages[], F078-AC-14).
type Usage struct {
	Kind     string // game_content, tenant_assignment
	ItemType string
	ItemID   string
	TenantID *uuid.UUID
}

// Pointer is the release state of an item in its type table.
type Pointer struct {
	Current *uuid.UUID // current_version_id (nil = not published / withdrawn)
}

// GameConfigStore writes the console tables under the PlatformScope of the transaction (role game_platform).
type GameConfigStore interface {
	// ItemByID locks and returns a config item (nil when missing or of another type).
	ItemByID(ctx context.Context, itemType string, id uuid.UUID) (*ConfigItem, error)
	// ItemByKey returns the config item of a type-table key (nil when missing).
	ItemByKey(ctx context.Context, itemType, key string) (*ConfigItem, error)
	// CreateItem inserts a config item.
	CreateItem(ctx context.Context, it ConfigItem, by uuid.UUID, at time.Time) error
	// NextVersionNo reserves the next version number of an item (config_items and the type identity counters).
	NextVersionNo(ctx context.Context, it ConfigItem) (int, error)
	// InsertVersion inserts a version.
	InsertVersion(ctx context.Context, v ConfigVersion, by uuid.UUID, at time.Time) error
	// Version locks and returns a version of an item by number (nil when missing).
	Version(ctx context.Context, configID uuid.UUID, no int) (*ConfigVersion, error)
	// UpdateDraft changes title/payload/status of a draft when its revision is still expected; false = changed meanwhile.
	UpdateDraft(ctx context.Context, v ConfigVersion, expected int64, by uuid.UUID, at time.Time) (bool, error)
	// SetWorkflow writes status, reviewer, note and timestamps of a version (expected revision, false = changed meanwhile).
	SetWorkflow(ctx context.Context, v ConfigVersion, expected int64, by uuid.UUID, at time.Time) (bool, error)
	// Pointer returns the current version of the item in its type table (locked).
	Pointer(ctx context.Context, itemType, key string) (Pointer, error)
	// MovePointer sets the release pointer of the type table (nil = withdrawn); the target must be a published version.
	MovePointer(ctx context.Context, itemType, key string, to *uuid.UUID, by uuid.UUID, at time.Time) error
	// TypeVersion returns the stored payload of a published/archived version in its type table (nil when missing).
	TypeVersion(ctx context.Context, itemType string, versionID uuid.UUID) (json.RawMessage, error)
	// SetTypeStatus mirrors an explicit archive of a published version into its type table.
	SetTypeStatus(ctx context.Context, itemType string, versionID uuid.UUID, status string, at time.Time) error
	// InsertSchedule, ScheduleByID (locked), ActiveSchedule (locked; nil when none), DueSchedules (locked, skip locked) and
	// FinishSchedule manage game.scheduled_publications.
	InsertSchedule(ctx context.Context, s Schedule, at time.Time) error
	ScheduleByID(ctx context.Context, configID, id uuid.UUID) (*Schedule, error)
	ActiveSchedule(ctx context.Context, configID uuid.UUID) (*Schedule, error)
	DueSchedules(ctx context.Context, now time.Time, limit int) ([]Schedule, error)
	FinishSchedule(ctx context.Context, s Schedule, at time.Time) error
	// InsertPreview stores a preview session (token hash only).
	InsertPreview(ctx context.Context, id uuid.UUID, tokenHash, configType string, configID, versionID uuid.UUID, versionNo int, by uuid.UUID,
		at, expires time.Time) error
	// Usages lists the published game content referencing the item and its active tenant assignments.
	Usages(ctx context.Context, itemType, key string, configID uuid.UUID, limit int) ([]Usage, error)
	// LatestClientManifests returns the manifest of the newest Godot build of each platform (empty when none registered).
	LatestClientManifests(ctx context.Context) ([]json.RawMessage, error)
	// MediaStatuses returns the status and kind of game media assets by id (missing ids are absent).
	MediaStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]MediaRef, error)
	// LinkMedia records the content media of a version (media_usages usage content; idempotent).
	LinkMedia(ctx context.Context, assets []uuid.UUID, itemType string, configID, versionID uuid.UUID, by uuid.UUID, at time.Time) error
}

// MediaRef is the state of a referenced game media asset.
type MediaRef struct {
	Kind   string
	Status string
}
