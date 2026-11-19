// Package trash holds the trash read model and the restore policy of learning resources (ADR-0028,
// ADMIN_PORTAL §3.8): a soft-deleted item is listed with whether it can be restored; restoring an item whose
// unique key is taken again by a live item is a conflict, never an overwrite.
package trash

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// ErrConflict: restoring would violate a partial unique index (409 CONFLICT, F020-AC-26).
var ErrConflict = errors.New("trash: a live record already holds the unique key")

// ErrNotDeleted: the record is not in the trash (404 NOT_FOUND).
var ErrNotDeleted = errors.New("trash: record is not deleted")

// Entry is one soft-deleted record.
type Entry struct {
	ResourceType contentmodel.ResourceType
	ResourceID   uuid.UUID
	TenantID     *uuid.UUID
	Title        string
	DeletedAt    time.Time
	DeletedBy    *uuid.UUID
	DeleteReason string
	// Conflict: a live record holds the unique key of this one.
	Conflict bool
}

// Restorable reports whether the entry can be restored now.
func (e Entry) Restorable() bool { return !e.Conflict }

// CheckRestore applies the restore policy to an entry.
func CheckRestore(e Entry) error {
	if e.Conflict {
		return ErrConflict
	}
	return nil
}
