package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ConfigRow is one version of a game config item as the console reads it (F078).
type ConfigRow struct {
	ConfigID       uuid.UUID
	ItemType       string
	ItemKey        string
	VersionID      uuid.UUID
	VersionNo      int
	Status         string
	Title          *string
	Payload        json.RawMessage
	AuthorID       *uuid.UUID
	ReviewerID     *uuid.UUID
	ReviewNote     *string
	ChangeNote     *string
	BasedOnNo      *int
	PublishedAt    *time.Time
	PublishedBy    *uuid.UUID
	UpdatedAt      time.Time
	Revision       int64
	IsCurrent      bool
	ThumbnailSHA   *string // sha256 of a ready render_thumbnail of the version
	TurntableSHA   *string
	ScheduledAt    *time.Time // publish_at of a pending scheduled publication of this version
	CurrentVersion *int       // version number of the release pointer of the item
}

// ConfigFilter is listGameConfigs_game: one row per item (its newest version, or its newest version of Status),
// keyset after config_id.
type ConfigFilter struct {
	ItemType string
	Status   string
	After    *uuid.UUID
	Limit    int // rows to read (limit+1 to detect has_more)
}

// ReleaseRow is one entry of the release timeline (content_releases + pending/cancelled/failed schedules).
type ReleaseRow struct {
	ID           uuid.UUID
	Action       string
	FromNo       *int
	ToNo         *int
	Reason       *string
	ScheduledFor *time.Time
	Status       string
	FailureCode  *string
	By           uuid.UUID
	At           time.Time
}

// PreviewRow is a preview session found by its token hash.
type PreviewRow struct {
	ID         uuid.UUID
	ConfigType string
	ConfigID   uuid.UUID
	VersionID  uuid.UUID
	VersionNo  int
	CreatedBy  uuid.UUID
	ExpiresAt  time.Time
}

// MediaAssetRow is game.media_assets with the versions using it.
type MediaAssetRow struct {
	ID        uuid.UUID
	Kind      string
	MIME      string
	Filename  string
	Size      int64
	SHA256    string
	Bucket    string
	Status    string
	Reject    *string
	Variants  []map[string]any
	Width     *int
	Height    *int
	License   *string
	AltText   *string
	CreatedAt time.Time
	UsedIn    []MediaUse
}

// MediaUse is one version using an asset.
type MediaUse struct {
	ItemType string
	ConfigID uuid.UUID
	Version  int
}

// AssignedActivity is one entry of listAssignedGameActivities (published quest/mini-game assigned to a tenant).
type AssignedActivity struct {
	ItemType     string
	ConfigID     uuid.UUID
	VersionID    uuid.UUID
	VersionNo    int
	Payload      json.RawMessage
	PublishedAt  *time.Time
	ThumbnailSHA *string
}

// GameConfigReadModel is the read side of the console (explicit SQL, platform content, deleted_at filtered).
type GameConfigReadModel interface {
	List(ctx context.Context, f ConfigFilter) ([]ConfigRow, error)
	Version(ctx context.Context, itemType string, configID uuid.UUID, no int) (*ConfigRow, error)
	// Versions lists the versions newest first (keyset before version number), found = the item exists.
	Versions(ctx context.Context, itemType string, configID uuid.UUID, before *int, limit int) (rows []ConfigRow, current *int, found bool, err error)
	// Releases lists the release timeline newest first (keyset before (at, id)).
	Releases(ctx context.Context, itemType string, configID uuid.UUID, beforeAt *time.Time, beforeID uuid.UUID, limit int) ([]ReleaseRow, bool, error)
	Preview(ctx context.Context, tokenHash string) (*PreviewRow, error)
	MediaAsset(ctx context.Context, id uuid.UUID) (*MediaAssetRow, error)
	// Assigned lists the published quest/mini-game configs assigned to tenant, keyset after (item_type, config_id).
	Assigned(ctx context.Context, tenant uuid.UUID, afterType string, afterID *uuid.UUID, limit int) ([]AssignedActivity, error)
}
