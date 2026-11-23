package liveops

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Run statuses (EventRunStatus).
const (
	StatusScheduled = "scheduled"
	StatusLive      = "live"
	StatusSuspended = "suspended"
	StatusEnded     = "ended"
	StatusCancelled = "cancelled"
	StatusArchived  = "archived"
)

// Operator actions on a run (game.event.operate; cancelSchoolEvent cancels too).
const (
	ActionSuspend = "suspend"
	ActionResume  = "resume"
	ActionCancel  = "cancel"
)

// ErrInvalidTransition is a transition the state machine refuses (422 INVALID_STATE_TRANSITION); ErrReason is a reason
// outside 3..1000 characters (422 VALIDATION_FAILED).
var (
	ErrInvalidTransition = errors.New("liveops: invalid state transition")
	ErrReason            = errors.New("liveops: reason must be 3 to 1000 characters")
)

// Run is the schedule and status of a run.
type Run struct {
	Status     string
	StartsAt   time.Time
	EndsAt     time.Time
	ClaimUntil time.Time
}

// Due returns the status the scheduler moves the run to at now ("" when nothing is due): scheduled → live at starts_at,
// live → ended at ends_at, ended → archived at claim_until, cancelled → archived at once (the reached milestones are
// granted first). A suspended run waits for an operator (resume or cancel).
func (r Run) Due(now time.Time) string {
	switch r.Status {
	case StatusScheduled:
		if !now.Before(r.StartsAt) {
			return StatusLive
		}
	case StatusLive:
		if !now.Before(r.EndsAt) {
			return StatusEnded
		}
	case StatusEnded:
		if !now.Before(r.ClaimUntil) {
			return StatusArchived
		}
	case StatusCancelled:
		return StatusArchived
	}
	return ""
}

// Operate returns the status after an operator action with its reason (F077-AC-03): suspend a live run, resume a
// suspended run, cancel a scheduled, live or suspended run.
func Operate(status, action, reason string) (string, error) {
	r := strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(r); n < 3 || n > 1000 {
		return "", ErrReason
	}
	switch {
	case action == ActionSuspend && status == StatusLive:
		return StatusSuspended, nil
	case action == ActionResume && status == StatusSuspended:
		return StatusLive, nil
	case action == ActionCancel && (status == StatusScheduled || status == StatusLive || status == StatusSuspended):
		return StatusCancelled, nil
	}
	return "", ErrInvalidTransition
}

// EventFor is the outbox event of a transition (event.started, event.suspended…).
func EventFor(from, to string) string {
	switch to {
	case StatusScheduled:
		return "event.scheduled"
	case StatusLive:
		if from == StatusSuspended {
			return "event.resumed"
		}
		return "event.started"
	case StatusSuspended:
		return "event.suspended"
	case StatusEnded:
		return "event.ended"
	case StatusCancelled:
		return "event.cancelled"
	}
	return ""
}

// Visibility says how a student sees a run at now (F077-AC-07): live and suspended runs, ended runs until claim_until
// (rewards can still be claimed), scheduled runs from upcoming before starts_at.
func Visible(r Run, now time.Time, upcoming time.Duration) bool {
	switch r.Status {
	case StatusLive, StatusSuspended:
		return true
	case StatusEnded:
		return now.Before(r.ClaimUntil)
	case StatusScheduled:
		return !now.Before(r.StartsAt.Add(-upcoming))
	}
	return false
}

// Earning reports a run that earns event points and contributions now: live (a suspended run gives no new reward,
// F077-AC-04; an ended run takes no new activity).
func Earning(status string) bool { return status == StatusLive }

// Claimable reports a run whose reached milestones can be claimed by the student (live or ended before claim_until).
func Claimable(r Run, now time.Time) bool {
	return r.Status == StatusLive || (r.Status == StatusEnded && now.Before(r.ClaimUntil))
}

// Audience is who an event is for (tenant, school, grade, subject — never a class, ADR-0095).
type Audience struct {
	Scope       string      `json:"scope"`
	TenantIDs   []uuid.UUID `json:"tenant_ids,omitempty"`
	SchoolIDs   []uuid.UUID `json:"school_ids,omitempty"`
	GradeLevels []int       `json:"grade_levels"`
	SubjectIDs  []uuid.UUID `json:"subject_ids,omitempty"`
}

// Validate checks the audience of an owner scope.
func (a Audience) Validate(owner string) error {
	switch a.Scope {
	case ScopePlatform:
		if owner != OwnerPlatform {
			return invalid("audience.scope", "a tenant event is for its tenant or its schools")
		}
	case ScopeTenant:
	case ScopeSchool:
		if len(a.SchoolIDs) == 0 {
			return invalid("audience.school_ids", "a school audience names its schools")
		}
	default:
		return invalid("audience.scope", "unknown scope")
	}
	if len(a.GradeLevels) == 0 {
		return invalid("audience.grade_levels", "at least one grade")
	}
	for _, g := range a.GradeLevels {
		if g < 1 || g > 9 {
			return invalid("audience.grade_levels", "grades 1..9")
		}
	}
	return nil
}

// Includes reports a student of tenant, school and grade in the audience of a run owned by owner (nil = platform). The
// subjects of the audience narrow the content, not the students (no subject enrolment exists, ADR-0095).
func (a Audience) Includes(owner *uuid.UUID, tenant, school uuid.UUID, grade int) bool {
	if owner != nil && *owner != tenant {
		return false
	}
	if owner == nil && len(a.TenantIDs) > 0 && !slices.Contains(a.TenantIDs, tenant) {
		return false
	}
	if len(a.SchoolIDs) > 0 && !slices.Contains(a.SchoolIDs, school) {
		return false
	}
	return slices.Contains(a.GradeLevels, grade)
}

// Tenants returns the tenants of the audience among active (the tenants whose manifest the run changes).
func (a Audience) Tenants(owner *uuid.UUID, active []uuid.UUID) []uuid.UUID {
	if owner != nil {
		return []uuid.UUID{*owner}
	}
	if len(a.TenantIDs) == 0 {
		return active
	}
	var out []uuid.UUID
	for _, t := range active {
		if slices.Contains(a.TenantIDs, t) {
			out = append(out, t)
		}
	}
	return out
}
