package contentmodel

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Status of a content version (content_status, CONTENT_MODEL §6).
type Status string

// Workflow statuses.
const (
	StatusDraft     Status = "draft"
	StatusInReview  Status = "in_review"
	StatusApproved  Status = "approved"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
	StatusRejected  Status = "rejected"
)

// Valid reports a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusInReview, StatusApproved, StatusPublished, StatusArchived, StatusRejected:
		return true
	}
	return false
}

// AtLeastApproved reports approved, published or archived (a reviewer is recorded).
func (s Status) AtLeastApproved() bool {
	return s == StatusApproved || s == StatusPublished || s == StatusArchived
}

// Immutable reports a version whose content may no longer change (CONTENT_MODEL §5 rule 1).
func (s Status) Immutable() bool { return s == StatusPublished || s == StatusArchived }

// Action of the workflow (CONTENT_MODEL §6).
type Action string

// Workflow actions.
const (
	ActionSubmitReview Action = "submit_review"
	ActionApprove      Action = "approve"
	ActionReject       Action = "reject"
	ActionRevise       Action = "revise"
	ActionPublish      Action = "publish"
	ActionArchive      Action = "archive"
)

// Workflow errors (mapped to 409/422 by the use cases of F025/F026).
var (
	ErrTransition       = errors.New("contentmodel: transition not allowed from this status")
	ErrReviewerIsAuthor = errors.New("contentmodel: the reviewer must not be the author")
	ErrReasonRequired   = errors.New("contentmodel: a reason is required")
	ErrCurrentVersion   = errors.New("contentmodel: the version is the current release")
)

// TransitionInput is the context of a transition.
type TransitionInput struct {
	Actor     uuid.UUID
	Author    uuid.UUID
	Note      string
	IsCurrent bool // the version is the item's release pointer (current_version_id)
}

var transitions = map[Status]map[Action]Status{
	StatusDraft:     {ActionSubmitReview: StatusInReview},
	StatusInReview:  {ActionApprove: StatusApproved, ActionReject: StatusRejected},
	StatusRejected:  {ActionRevise: StatusDraft},
	StatusApproved:  {ActionRevise: StatusDraft, ActionPublish: StatusPublished},
	StatusPublished: {ActionArchive: StatusArchived},
}

// Transition returns the status reached by action from from (data-model §2): approve needs a reviewer
// other than the author, reject a reason, archive a version that is not the current release. The full
// publish conditions (children published, snapshot, audio) belong to F025/F026.
func Transition(from Status, action Action, in TransitionInput) (Status, error) {
	to, ok := transitions[from][action]
	if !ok {
		return from, ErrTransition
	}
	switch action {
	case ActionApprove:
		if in.Actor == uuid.Nil || in.Actor == in.Author {
			return from, ErrReviewerIsAuthor
		}
	case ActionReject:
		if strings.TrimSpace(in.Note) == "" {
			return from, ErrReasonRequired
		}
	case ActionArchive:
		if in.IsCurrent {
			return from, ErrCurrentVersion
		}
	}
	return to, nil
}

// Deletion errors (ADR-0028, CONTENT_MODEL §5 rule 10).
var (
	ErrVersionNotDeletable = errors.New("contentmodel: only draft or rejected versions can be deleted")
	ErrItemPublished       = errors.New("contentmodel: an item with a current release cannot be deleted")
)

// CanDeleteVersion: only draft and rejected versions are soft-deleted; published ones are archived.
func CanDeleteVersion(s Status) error {
	if s == StatusDraft || s == StatusRejected {
		return nil
	}
	return ErrVersionNotDeletable
}

// CanDeleteItem: an item is soft-deleted only when it is not released (current_version_id IS NULL).
func CanDeleteItem(currentVersionID *uuid.UUID) error {
	if currentVersionID != nil && *currentVersionID != uuid.Nil {
		return ErrItemPublished
	}
	return nil
}
