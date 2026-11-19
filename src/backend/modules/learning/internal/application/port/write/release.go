package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/release"
)

// ReleaseRow is a row of content.content_releases (append-only, F026).
type ReleaseRow struct {
	ID             uuid.UUID
	Owner          contentmodel.Owner
	ItemType       contentmodel.ContentType
	ItemID         uuid.UUID
	Action         release.Action
	FromVersionID  *uuid.UUID
	ToVersionID    *uuid.UUID
	Reason         string
	ScheduledFor   *time.Time
	ReleasedBy     uuid.UUID
	ReleasedAt     time.Time
	IdempotencyKey string
}

// ReleaseStore moves release pointers and records releases under the scope of ctx (F026).
type ReleaseStore interface {
	// SetCurrentVersion sets current_version_id of an item (nil = unpublished) when its row version is ifRow: a lost
	// race is VersionMismatch (409 CONTENT_VERSION_CONFLICT for the caller), a target that is not a published version of
	// the item is RollbackTargetInvalid (trigger <item>_release_target).
	SetCurrentVersion(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, current *uuid.UUID, ifRow int64) error
	InsertRelease(ctx context.Context, r ReleaseRow) error
	// AssignedTenants lists the tenants a platform resource is assigned to (release fan-out of a platform item).
	AssignedTenants(ctx context.Context, rt contentmodel.ResourceType, id uuid.UUID) ([]uuid.UUID, error)
}

// Schedule is a row of content.scheduled_publications (F026 US4).
type Schedule struct {
	ID               uuid.UUID
	Owner            contentmodel.Owner
	ItemType         contentmodel.ContentType
	ItemID           uuid.UUID
	VersionID        uuid.UUID
	VersionNo        int
	PublishAt        time.Time
	Status           release.ScheduleStatus
	FailureCode      *string
	AppliedReleaseID *uuid.UUID
	ChangeNote       *string
	RequestedBy      uuid.UUID
	CancelledBy      *uuid.UUID
	IdempotencyKey   string
	RowVersion       int64
	CreatedAt        time.Time
}

// ScheduleStore persists scheduled publications under the scope of ctx (platform rows under PlatformScope).
type ScheduleStore interface {
	InsertSchedule(ctx context.Context, s Schedule) error
	Schedule(ctx context.Context, id uuid.UUID) (Schedule, error)
	// UpdateSchedule saves status, failure_code, applied_release_id and cancelled_by when the row version is ifRow
	// (VersionMismatch otherwise: another worker or user took it).
	UpdateSchedule(ctx context.Context, s Schedule, ifRow int64) error
	// DueSchedules lists scheduled rows with publish_at ≤ now of every owner (ctx carries tenancy.Bypass(platform_job)).
	DueSchedules(ctx context.Context, now time.Time, limit int) ([]Schedule, error)
}
