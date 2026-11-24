package gameconfig

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ReleaseAction is content_releases.action.
type ReleaseAction string

// Release actions.
const (
	ReleasePublish   ReleaseAction = "publish"
	ReleaseRollback  ReleaseAction = "rollback"
	ReleaseUnpublish ReleaseAction = "unpublish"
)

// ScheduleStatus is scheduled_publications.status (ContentRelease.status).
type ScheduleStatus string

// Schedule statuses.
const (
	ScheduleScheduled ScheduleStatus = "scheduled"
	ScheduleApplied   ScheduleStatus = "applied"
	ScheduleCancelled ScheduleStatus = "cancelled"
	ScheduleFailed    ScheduleStatus = "failed"
)

// Errors of the release rules (409 CONTENT_NOT_EDITABLE/INVALID_STATE_TRANSITION, 422 ROLLBACK_TARGET_INVALID,
// 422 VALIDATION_FAILED, 409 CONTENT_NOT_PUBLISHED).
var (
	ErrNotApproved        = errors.New("gameconfig: only an approved version can be published")
	ErrRollbackTarget     = errors.New("gameconfig: the target must be another published version of the item")
	ErrRollbackNotAllowed = errors.New("gameconfig: the runtime of this type does not follow the release pointer")
	ErrReasonRequired     = errors.New("gameconfig: a reason is required")
	ErrReasonTooLong      = errors.New("gameconfig: the reason is longer than 500 characters")
	ErrNotPublished       = errors.New("gameconfig: the item has no current release")
	ErrScheduleTooSoon    = errors.New("gameconfig: publish_at is earlier than the minimum lead")
	ErrNotScheduled       = errors.New("gameconfig: the publication is no longer scheduled")
	ErrAlreadyScheduled   = errors.New("gameconfig: the item already has a scheduled publication")
)

// Target is the release state of one version of the item.
type Target struct {
	ID     uuid.UUID
	Status Status
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

// CheckPublish: only an approved version is published (now or scheduled).
func CheckPublish(s Status) error {
	if s != StatusApproved {
		return ErrNotApproved
	}
	return nil
}

// CheckRollback: the type follows the pointer and the target is a published version of the item other than the current
// release (F078-AC-13). No version is created.
func CheckRollback(t Type, current *uuid.UUID, target Target, reason string) error {
	if err := CheckReason(reason); err != nil {
		return err
	}
	if !t.Rollback {
		return ErrRollbackNotAllowed
	}
	if target.Status != StatusPublished || (current != nil && *current == target.ID) {
		return ErrRollbackTarget
	}
	return nil
}

// CheckUnpublish: the type can be withdrawn and the item has a current release (F078-AC-14).
func CheckUnpublish(t Type, current *uuid.UUID, reason string) error {
	if err := CheckReason(reason); err != nil {
		return err
	}
	if !t.Unpublish {
		return ErrRollbackNotAllowed
	}
	if current == nil {
		return ErrNotPublished
	}
	return nil
}

// CheckSchedule: publish_at is at least minLead after now (scheduled_publish_min_lead, F078-AC-12).
func CheckSchedule(publishAt, now time.Time, minLead time.Duration) error {
	if publishAt.Before(now.Add(minLead)) {
		return ErrScheduleTooSoon
	}
	return nil
}

// Cancel moves a scheduled publication to cancelled.
func Cancel(s ScheduleStatus) (ScheduleStatus, error) {
	if s != ScheduleScheduled {
		return s, ErrNotScheduled
	}
	return ScheduleCancelled, nil
}

// Due reports a scheduled publication whose time has come (the job applies it once, ±1 minute of its tick).
func Due(s ScheduleStatus, publishAt, now time.Time) bool {
	return s == ScheduleScheduled && !publishAt.After(now)
}
