package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MergeLink links the original student_id of a merged account to the target user (IDENTITY_MAPPING §7, OQ F016-2,
// F045-3): the attempts, learning events and mastery stay under the original student_id.
type MergeLink struct {
	MergeID   uuid.UUID
	TenantID  uuid.UUID
	SourceID  uuid.UUID
	TargetID  uuid.UUID
	Reason    string
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

// MergeLinkStore stores the merge links of the tenant of ctx (learning.student_merge_links, RLS T).
type MergeLinkStore interface {
	// MergeLinkByID returns the link of a merge (nil when absent).
	MergeLinkByID(ctx context.Context, mergeID uuid.UUID) (*MergeLink, error)
	// MergeLinkOfSource returns the live link of a source student (nil when absent).
	MergeLinkOfSource(ctx context.Context, sourceID uuid.UUID) (*MergeLink, error)
	// HasStudentData reports whether the student has learning data in the tenant (attempts, mastery, lesson sessions).
	HasStudentData(ctx context.Context, studentID uuid.UUID) (bool, error)
	InsertMergeLink(ctx context.Context, l MergeLink) error
}
