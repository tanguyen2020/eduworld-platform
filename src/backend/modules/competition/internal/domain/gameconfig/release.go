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

// Errors of the release rules (422 INVALID_STATE_TRANSITION, 422 ROLLBACK_TARGET_INVALID, 422 VALIDATION_FAILED,
// 409 CONTENT_NOT_PUBLISHED).
var (
	ErrNotApproved         = errors.New("gameconfig: only an approved version can be published")
	ErrRollbackTarget      = errors.New("gameconfig: the target must be another version of the configuration published before")
	ErrUnpublishNotAllowed = errors.New("gameconfig: the runtime needs this catalog: it cannot be withdrawn")
	ErrReasonRequired      = errors.New("gameconfig: a reason is required")
	ErrReasonTooLong       = errors.New("gameconfig: the reason is longer than 500 characters")
	ErrNotPublished        = errors.New("gameconfig: the configuration has no current release")
	ErrRetroactive         = errors.New("gameconfig: effective_from is in the past")
)

// Target is the release state of the version a rollback points to.
type Target struct {
	ID        uuid.UUID
	Status    Status
	Published bool // published_at is set: the version has been published once
}

// CheckReason validates a rollback or unpublish reason (required, ≤ 500 characters — F108-AC-05).
func CheckReason(reason string) error {
	switch {
	case strings.TrimSpace(reason) == "":
		return ErrReasonRequired
	case utf8.RuneCountInString(reason) > MaxReason:
		return ErrReasonTooLong
	}
	return nil
}

// CheckPublish: only an approved version is published.
func CheckPublish(s Status) error {
	if s != StatusApproved {
		return ErrNotApproved
	}
	return nil
}

// EffectiveFrom resolves the effective time of a publish (F108-AC-05): absent → now; a time before now is refused (a
// configuration never applies retroactively to matches already played).
func EffectiveFrom(requested *time.Time, now time.Time) (time.Time, error) {
	if requested == nil {
		return now, nil
	}
	at := requested.UTC()
	if at.Before(now) {
		return at, ErrRetroactive
	}
	return at, nil
}

// CheckRollback: the target is a version of the configuration published before (published_at set, not archived) other
// than the current release (F108-AC-05: otherwise ROLLBACK_TARGET_INVALID). Rollback only moves the pointer: no version
// is created and no version changes (ADR-0043).
func CheckRollback(current *uuid.UUID, target *Target, reason string) error {
	if err := CheckReason(reason); err != nil {
		return err
	}
	if target == nil || !target.Published || target.Status != StatusPublished || (current != nil && *current == target.ID) {
		return ErrRollbackTarget
	}
	return nil
}

// CheckUnpublish: the type can be withdrawn and the configuration has a current release.
func CheckUnpublish(t Type, current *uuid.UUID, reason string) error {
	if err := CheckReason(reason); err != nil {
		return err
	}
	if !t.Unpublish {
		return ErrUnpublishNotAllowed
	}
	if current == nil {
		return ErrNotPublished
	}
	return nil
}
