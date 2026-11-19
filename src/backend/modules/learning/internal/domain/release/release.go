// Package release holds the release rules of learning content (F026, ADR-0015, ADR-0043, CONTENT_MODEL §5, §6): the
// release pointer current_version_id of an item only targets a published version of the same item; publish needs an
// approved version; rollback and unpublish need a reason; archive never touches the current release; a scheduled
// publication is applied once.
package release

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Action is content_releases.action.
type Action string

// Release actions.
const (
	ActionPublish   Action = "publish"
	ActionRollback  Action = "rollback"
	ActionUnpublish Action = "unpublish"
)

// MaxReason is the length limit of a rollback/unpublish reason (RollbackRequest, ReasonRequest).
const MaxReason = 500

// Errors of the release rules (422 INVALID_STATE_TRANSITION, 422 ROLLBACK_TARGET_INVALID, 422 VALIDATION_FAILED,
// 409 CONTENT_NOT_PUBLISHED).
var (
	ErrNotApproved     = errors.New("release: only an approved version can be published")
	ErrRollbackTarget  = errors.New("release: the target must be another published version of the item")
	ErrReasonRequired  = errors.New("release: a reason is required")
	ErrReasonTooLong   = errors.New("release: the reason is longer than 500 characters")
	ErrNotPublished    = errors.New("release: the item has no current release")
	ErrScheduleTooSoon = errors.New("release: publish_at is earlier than the minimum lead")
	ErrNotScheduled    = errors.New("release: the publication is no longer scheduled")
)

// Version is the release state of one version.
type Version struct {
	ID     uuid.UUID
	ItemID uuid.UUID
	Status contentmodel.Status
}

// CheckReason validates a rollback or unpublish reason.
func CheckReason(reason string) error {
	switch {
	case strings.TrimSpace(reason) == "":
		return ErrReasonRequired
	case utf8.RuneCountInString(reason) > MaxReason:
		return ErrReasonTooLong
	}
	return nil
}

// CheckPublish: the version is approved (F026-AC-02).
func CheckPublish(v Version) error {
	if v.Status != contentmodel.StatusApproved {
		return ErrNotApproved
	}
	return nil
}

// CheckRollback: the target is a published version of the same item other than the current release (F026-AC-09).
func CheckRollback(item uuid.UUID, current *uuid.UUID, target Version, reason string) error {
	if err := CheckReason(reason); err != nil {
		return err
	}
	if target.ItemID != item || target.Status != contentmodel.StatusPublished || (current != nil && *current == target.ID) {
		return ErrRollbackTarget
	}
	return nil
}

// CheckUnpublish: the item has a current release (F026-AC-10).
func CheckUnpublish(current *uuid.UUID, reason string) error {
	if err := CheckReason(reason); err != nil {
		return err
	}
	if current == nil {
		return ErrNotPublished
	}
	return nil
}

// CheckSchedule: publish_at is at least minLead after now (scheduled_publish_min_lead, F026-AC-17).
func CheckSchedule(publishAt, now time.Time, minLead time.Duration) error {
	if publishAt.Before(now.Add(minLead)) {
		return ErrScheduleTooSoon
	}
	return nil
}

// ScheduleStatus is scheduled_publications.status.
type ScheduleStatus string

// Schedule statuses.
const (
	ScheduleScheduled ScheduleStatus = "scheduled"
	ScheduleApplied   ScheduleStatus = "applied"
	ScheduleCancelled ScheduleStatus = "cancelled"
	ScheduleFailed    ScheduleStatus = "failed"
)

// Cancel moves a scheduled publication to cancelled (F026-AC-19).
func Cancel(s ScheduleStatus) (ScheduleStatus, error) {
	if s != ScheduleScheduled {
		return s, ErrNotScheduled
	}
	return ScheduleCancelled, nil
}

// Due reports a scheduled publication whose time has come.
func Due(s ScheduleStatus, publishAt, now time.Time) bool {
	return s == ScheduleScheduled && !publishAt.After(now)
}
