package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/workflow"
)

// StatusChange is a workflow change of one version row (F025 review actions, F026 publish/archive): the status, and the
// review columns set (reviewer_id, audio_exempt_approved_by on approve; published_at on publish) or cleared (revise).
type StatusChange struct {
	Type             contentmodel.ContentType
	VersionID        uuid.UUID
	IfRow            int64
	Status           contentmodel.Status
	ReviewerID       *uuid.UUID
	ExemptApprovedBy *uuid.UUID
	PublishedAt      *time.Time
	PublishedBy      *uuid.UUID
	SnapshotKey      *string
	SnapshotSHA      *string
	ClearReview      bool
}

// VersionStatusStore changes the workflow columns of a version of any learning content type under the scope of ctx: a
// stale row version is VersionMismatch, an invisible row NotFound. It returns the new row version.
type VersionStatusStore interface {
	SetVersionStatus(ctx context.Context, c StatusChange) (int64, error)
}

// ReviewTask is a row of content.review_tasks.
type ReviewTask struct {
	ID                uuid.UUID
	Owner             contentmodel.Owner
	Lane              workflow.Lane
	ContentType       contentmodel.ContentType
	ContentID         uuid.UUID
	ContentVersionID  uuid.UUID
	VersionNo         int
	Title             string
	SchoolID          *uuid.UUID
	AuthorID          uuid.UUID
	AssigneeID        *uuid.UUID
	Status            workflow.TaskStatus
	Comments          json.RawMessage
	SubmittedAt       time.Time
	DueAt             *time.Time
	ResolvedAt        *time.Time
	ResolvedBy        *uuid.UUID
	AudioExemptReason string // not stored: the exemption of the version, for the queue view
}

// Transition is a row of content.content_transitions (append-only).
type Transition struct {
	Owner            contentmodel.Owner
	ContentType      contentmodel.ContentType
	ContentID        uuid.UUID
	ContentVersionID uuid.UUID
	Action           contentmodel.Action
	From, To         contentmodel.Status
	ActorID          uuid.UUID
	Note             string
	IdempotencyKey   string
	At               time.Time
}

// ReviewStore persists review tasks and workflow transitions under the scope of ctx (platform rows under PlatformScope).
type ReviewStore interface {
	// OpenTask returns the open task of a version.
	OpenTask(ctx context.Context, versionID uuid.UUID) (ReviewTask, bool, error)
	InsertTask(ctx context.Context, t ReviewTask) error
	// ResolveTask saves status, resolved_at and resolved_by of a task.
	ResolveTask(ctx context.Context, t ReviewTask) error
	// CancelOpenTasks cancels the open tasks of the versions of an item (the item or a version was soft-deleted).
	CancelOpenTasks(ctx context.Context, versionIDs []uuid.UUID, at time.Time) error
	InsertTransition(ctx context.Context, t Transition) error
}
