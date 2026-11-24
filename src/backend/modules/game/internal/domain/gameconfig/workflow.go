package gameconfig

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Status is the workflow status of a version (ContentStatus).
type Status string

// Statuses.
const (
	StatusDraft     Status = "draft"
	StatusInReview  Status = "in_review"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

// ValidStatus reports a ContentStatus value (list filter).
func ValidStatus(s string) bool {
	switch Status(s) {
	case StatusDraft, StatusInReview, StatusApproved, StatusRejected, StatusPublished, StatusArchived:
		return true
	}
	return false
}

// Action is a transition of transitionGameConfigVersion (publish/rollback/unpublish have their own endpoints).
type Action string

// Actions.
const (
	ActionSubmitReview Action = "submit_review"
	ActionApprove      Action = "approve"
	ActionReject       Action = "reject"
	ActionArchive      Action = "archive"
)

// Limits of the texts (OpenAPI GameConfigInput, transition body, ADR-0041).
const (
	MaxTitle      = 200
	MaxNote       = 2000
	MaxChangeNote = 500
	MaxReason     = 500
	// MaxPayloadBytes is the CHECK of the version payloads (octet_length ≤ 65536).
	MaxPayloadBytes = 65536
)

// Errors of the workflow (mapped by the use cases: 422 INVALID_STATE_TRANSITION, 403 PERMISSION_DENIED,
// 422 VALIDATION_FAILED, 409 CONTENT_NOT_EDITABLE).
var (
	ErrInvalidTransition = errors.New("gameconfig: action not allowed from this status")
	ErrSelfReview        = errors.New("gameconfig: the reviewer must not be the author")
	ErrNoteRequired      = errors.New("gameconfig: reject needs a note")
	ErrNoteTooLong       = errors.New("gameconfig: the note is longer than 2000 characters")
	ErrArchiveCurrent    = errors.New("gameconfig: the current release cannot be archived")
	ErrNotEditable       = errors.New("gameconfig: only a draft or rejected version can be edited")
	ErrUnknownAction     = errors.New("gameconfig: unknown action")
)

// Version is the workflow state of one version.
type Version struct {
	Status Status
	Author *uuid.UUID // nil for the versions published by the seed (system actor)
	// IsCurrent: the version is the release pointer of its item (archive is refused).
	IsCurrent bool
}

// Request is one transition by Actor.
type Request struct {
	Action Action
	Actor  uuid.UUID
	Note   string
}

// Transition applies a review action (CONTENT_MODEL §6, ADMIN_PORTAL §4, F078-AC-08): submit_review from draft or
// rejected; approve/reject from in_review by someone else than the author (also a System Admin holding game.*); reject
// needs a note; archive of a published or approved/rejected/draft version that is not the current release.
func Transition(v Version, r Request) (Status, error) {
	if utf8.RuneCountInString(r.Note) > MaxNote {
		return v.Status, ErrNoteTooLong
	}
	switch r.Action {
	case ActionSubmitReview:
		if v.Status != StatusDraft && v.Status != StatusRejected {
			return v.Status, ErrInvalidTransition
		}
		return StatusInReview, nil
	case ActionApprove, ActionReject:
		if v.Status != StatusInReview {
			return v.Status, ErrInvalidTransition
		}
		if v.Author != nil && *v.Author == r.Actor {
			return v.Status, ErrSelfReview
		}
		if r.Action == ActionReject {
			if strings.TrimSpace(r.Note) == "" {
				return v.Status, ErrNoteRequired
			}
			return StatusRejected, nil
		}
		return StatusApproved, nil
	case ActionArchive:
		if v.Status == StatusArchived || v.Status == StatusInReview {
			return v.Status, ErrInvalidTransition
		}
		if v.IsCurrent {
			return v.Status, ErrArchiveCurrent
		}
		return StatusArchived, nil
	}
	return v.Status, ErrUnknownAction
}

// Editable reports whether the payload/title of a version can change (updateGameConfigDraft): draft or rejected (a
// rejected version is reworked in place, then submitted again).
func Editable(s Status) bool { return s == StatusDraft || s == StatusRejected }
