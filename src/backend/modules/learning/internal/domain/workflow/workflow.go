// Package workflow is the review workflow of every learning content type (F025, CONTENT_MODEL §6, ADMIN_PORTAL §4):
// the review actions of a version (submit_review, approve, reject, revise) on top of the transition table of
// contentmodel, the review lane of a version and the status of its review task.
package workflow

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Lane is review_lane (ADMIN_PORTAL §3.5, §4): tenant content is reviewed in the tenant lane, platform content in the
// platform_learning lane.
type Lane string

// Lanes.
const (
	LaneTenant   Lane = "tenant"
	LanePlatform Lane = "platform_learning"
)

// LaneOf returns the lane of content owned by o.
func LaneOf(o contentmodel.Owner) Lane {
	if o.Platform() {
		return LanePlatform
	}
	return LaneTenant
}

// TaskStatus is the status of a review task.
type TaskStatus string

// Task statuses.
const (
	TaskOpen      TaskStatus = "open"
	TaskApproved  TaskStatus = "approved"
	TaskRejected  TaskStatus = "rejected"
	TaskCancelled TaskStatus = "cancelled"
)

// MaxNoteLen is the length limit of a transition note (content_transitions.note, transitionContentVersion body).
const MaxNoteLen = 2000

// Errors of the review actions (mapped by the use case: 422 INVALID_STATE_TRANSITION, 403 PERMISSION_DENIED,
// 422 VALIDATION_FAILED).
var (
	ErrInvalidTransition        = errors.New("workflow: action not allowed from this status")
	ErrSelfReview               = errors.New("workflow: the reviewer must not be the author")
	ErrNoteRequired             = errors.New("workflow: reject needs a note")
	ErrNoteTooLong              = errors.New("workflow: the note is longer than 2000 characters")
	ErrExemptionNotAcknowledged = errors.New("workflow: the audio exemption must be acknowledged")
)

var reviewActions = []contentmodel.Action{contentmodel.ActionSubmitReview, contentmodel.ActionApprove, contentmodel.ActionReject,
	contentmodel.ActionRevise}

// ReviewAction reports one of the four review actions of F025.
func ReviewAction(a contentmodel.Action) bool { return slices.Contains(reviewActions, a) }

// Supported reports an action of transitionContentVersion: the review actions and archive (F026-AC-28); publish has its
// own endpoint.
func Supported(a contentmodel.Action) bool { return ReviewAction(a) || a == contentmodel.ActionArchive }

// Version is the workflow state of a content version.
type Version struct {
	Status            contentmodel.Status
	Author            uuid.UUID
	AudioExemptReason string
	// IsCurrent: the version is the release pointer of its item (archive is refused, F026-AC-28).
	IsCurrent bool
}

// Request is a review action by Actor.
type Request struct {
	Action                     contentmodel.Action
	Actor                      uuid.UUID
	Note                       string
	AudioExemptionAcknowledged bool
}

// Result is the effect of an action.
type Result struct {
	From, To contentmodel.Status
	// Task is the status the review task of the version gets: open for submit_review, approved/rejected when the
	// reviewer decides, "" when no task changes (revise).
	Task TaskStatus
	// Reviewer is set by approve (reviewer_id); ExemptionApproved records audio_exempt_approved_by.
	Reviewer          bool
	ExemptionApproved bool
}

// Apply applies a review action (data-model §2): approve needs a reviewer other than the author and the acknowledgement
// of a declared audio exemption, reject a note of 1–2000 characters.
func Apply(v Version, r Request) (Result, error) {
	if !Supported(r.Action) {
		return Result{}, ErrInvalidTransition
	}
	if utf8.RuneCountInString(r.Note) > MaxNoteLen {
		return Result{}, ErrNoteTooLong
	}
	to, err := contentmodel.Transition(v.Status, r.Action, contentmodel.TransitionInput{Actor: r.Actor, Author: v.Author, Note: r.Note, IsCurrent: v.IsCurrent})
	switch {
	case errors.Is(err, contentmodel.ErrReviewerIsAuthor):
		return Result{}, ErrSelfReview
	case errors.Is(err, contentmodel.ErrReasonRequired):
		return Result{}, ErrNoteRequired
	case err != nil:
		return Result{}, ErrInvalidTransition
	}
	res := Result{From: v.Status, To: to}
	switch r.Action {
	case contentmodel.ActionSubmitReview:
		res.Task = TaskOpen
	case contentmodel.ActionApprove:
		exempt := strings.TrimSpace(v.AudioExemptReason) != ""
		if exempt && !r.AudioExemptionAcknowledged {
			return Result{}, ErrExemptionNotAcknowledged
		}
		res.Task, res.Reviewer, res.ExemptionApproved = TaskApproved, true, exempt
	case contentmodel.ActionReject:
		res.Task = TaskRejected
	}
	return res, nil
}
