package lesson

import (
	"encoding/json"
	"slices"

	"github.com/google/uuid"
)

// Completion rules of a lesson version (Lesson.completion_rule; view_only needs a server-checked view heartbeat that no
// feature owns yet, so it never completes through activities — F031 spec, Out of scope).
const (
	RuleAllRequired = "all_required_activities"
	RuleAny         = "any_activity"
	RuleViewOnly    = "view_only"
)

// Body is the body of a lesson version (blocks, completion_rule, estimated_minutes).
type Body struct {
	Blocks           []Block `json:"blocks"`
	CompletionRule   string  `json:"completion_rule"`
	EstimatedMinutes *int    `json:"estimated_minutes,omitempty"`
}

// ParseBody reads a lesson version body; the completion rule defaults to all_required_activities.
func ParseBody(raw json.RawMessage) (Body, error) {
	var b Body
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &b); err != nil {
			return b, err
		}
	}
	if b.CompletionRule == "" {
		b.CompletionRule = RuleAllRequired
	}
	return b, nil
}

// ActivityRef is an activity block of a version.
type ActivityRef struct {
	ActivityID uuid.UUID
	Required   bool
}

// activityBlock reads the required flag of an activity block (default true, lesson.schema.json).
type activityBlock struct {
	Kind       string     `json:"kind"`
	ActivityID *uuid.UUID `json:"activity_id"`
	Required   *bool      `json:"required"`
}

// Activities lists the activity blocks of raw blocks in order (required defaults to true).
func Activities(raw json.RawMessage) []ActivityRef {
	var b struct {
		Blocks []activityBlock `json:"blocks"`
	}
	_ = json.Unmarshal(raw, &b)
	var out []ActivityRef
	for _, bl := range b.Blocks {
		if bl.Kind != KindActivity || bl.ActivityID == nil {
			continue
		}
		req := bl.Required == nil || *bl.Required
		out = append(out, ActivityRef{ActivityID: *bl.ActivityID, Required: req})
	}
	return out
}

// Contains reports whether the version has the activity.
func Contains(acts []ActivityRef, id uuid.UUID) bool {
	return slices.ContainsFunc(acts, func(a ActivityRef) bool { return a.ActivityID == id })
}

// Satisfied evaluates the completion rule of a version against the activities completed in a session (an activity is
// completed when one of its attempts was graded in the session — completed is not passed, ACTIVITY_ENGINE §7). It
// returns the required activities still missing. A session completes only once at least one activity is completed.
func Satisfied(rule string, acts []ActivityRef, completed []uuid.UUID) (bool, []uuid.UUID) {
	done := func(id uuid.UUID) bool { return slices.Contains(completed, id) }
	anyDone := slices.ContainsFunc(acts, func(a ActivityRef) bool { return done(a.ActivityID) })
	switch rule {
	case RuleAny:
		if anyDone {
			return true, nil
		}
		var all []uuid.UUID
		for _, a := range acts {
			all = append(all, a.ActivityID)
		}
		return false, all
	case RuleAllRequired:
		var missing []uuid.UUID
		for _, a := range acts {
			if a.Required && !done(a.ActivityID) {
				missing = append(missing, a.ActivityID)
			}
		}
		return len(missing) == 0 && anyDone, missing
	default:
		return false, nil
	}
}

// Session states.
const (
	SessionInProgress = "in_progress"
	SessionCompleted  = "completed"
)
