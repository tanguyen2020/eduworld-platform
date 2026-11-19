package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// OwnerServiceGateway calls the platform-ops facade operations of the module owning a resource type
// (specs/openapi/modules/common/internal-common.v1.yaml, ADR-0074, AGENTS R29): in process through pkg/modcall
// with the original Principal, the scope and the Idempotency-Key of ctx. A module that is not wired in the
// binary, an infrastructure error or a missed deadline is 503 UNAVAILABLE (F045-AC-25); 4xx problems of the
// callee are returned as they are. The module argument comes from domain/platformadmin.Owner.
type OwnerServiceGateway interface {
	ListTrash(ctx context.Context, module string, q TrashQuery) (TrashPage, error)
	RestoreTrashItem(ctx context.Context, module, resourceType string, id uuid.UUID, reason string) error
	ApplyAssignments(ctx context.Context, module, resourceType string, id uuid.UUID, tenants []uuid.UUID) ([]ResourceAssignment, error)
	RevokeAssignments(ctx context.Context, module, resourceType string, id uuid.UUID, tenants []uuid.UUID, reason string) ([]ResourceAssignment, error)
	ListAssignments(ctx context.Context, module, resourceType string, id uuid.UUID, cursor string, limit int) (AssignmentPage, error)
	CloneStep(ctx context.Context, module string, req CloneStepRequest) (CloneStepResult, error)
	// MergeUserData moves the data of from into req.Into (mergeUserData); Status is done, blocked or nothing_to_do.
	MergeUserData(ctx context.Context, module string, from uuid.UUID, req MergeUserDataRequest) (MergeUserDataResult, error)
}

// TrashQuery filters a trash listing (tenant_ids empty = the callee's default for the Principal).
type TrashQuery struct {
	ResourceType string
	TenantIDs    []uuid.UUID
	Cursor       string
	Limit        int
}

// TrashEntry is one soft-deleted record (TrashItem).
type TrashEntry struct {
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	TenantID     *uuid.UUID `json:"tenant_id"`
	Title        string     `json:"title"`
	DeletedAt    time.Time  `json:"deleted_at"`
	DeletedBy    uuid.UUID  `json:"deleted_by"`
	DeleteReason string     `json:"delete_reason,omitempty"`
	Restorable   bool       `json:"restorable"`
}

// TrashPage is a page of trash entries.
type TrashPage struct {
	Data       []TrashEntry
	HasMore    bool
	NextCursor *string
}

// ResourceAssignment is one assignment of a platform resource to a tenant.
type ResourceAssignment struct {
	AssignmentID uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	TenantID     uuid.UUID
	AssignedBy   uuid.UUID
	AssignedAt   time.Time
	Active       bool
}

// AssignmentPage is a page of assignments.
type AssignmentPage struct {
	Data       []ResourceAssignment
	HasMore    bool
	NextCursor *string
}

// CloneStepRequest is one step of a clone job (executeTenantCloneStep).
type CloneStepRequest struct {
	JobID        uuid.UUID
	Source       uuid.UUID
	Target       uuid.UUID
	ResourceType string
	ResourceIDs  []uuid.UUID
	IDMap        map[uuid.UUID]uuid.UUID // source → copy of the previous steps
}

// ClonedPair maps a source record to its copy.
type ClonedPair struct{ SourceID, TargetID uuid.UUID }

// CloneStepResult is the answer of a module to a clone step.
type CloneStepResult struct {
	Status  string // succeeded | partially_failed | failed
	Cloned  []ClonedPair
	Skipped int
}

// MergeUserDataRequest is the body of mergeUserData (Idempotency-Key = merge_id).
type MergeUserDataRequest struct {
	MergeID uuid.UUID
	Into    uuid.UUID
	Reason  string
}

// MergeUserDataResult is the answer of a module to mergeUserData.
type MergeUserDataResult struct {
	Status        string // done | blocked | nothing_to_do
	BlockedReason string
}
