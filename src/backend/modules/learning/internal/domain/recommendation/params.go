package recommendation

import (
	"bytes"
	"encoding/json"
	"time"
)

// Params are the parameters of the rule engine a recommendation_rule experiment arm may change (F035-AC-21, AC-24,
// LEARNING_EFFICACY_PLAN §6). A nil field keeps the default of the engine (the control experience); no parameter
// adds play time or pressure (R19): the number of recommendations shown can only go down.
type Params struct {
	// ReviewThreshold: an objective whose mastery after decay is under it (with WeakMinEvidence evidence) is weak and
	// recommended for review (R2, R3, R8); default = level beginning or developing.
	ReviewThreshold *float64 `json:"review_threshold,omitempty"`
	// MaxRecommendations bounds the active recommendations of the student (shown at once ≤ ShownRecommendations).
	MaxRecommendations *int `json:"max_recommendations,omitempty"`
	// PrerequisiteDepth is the depth of the prerequisite walk of R3 (default 1: direct prerequisites).
	PrerequisiteDepth *int `json:"prerequisite_depth,omitempty"`
	// SpacedReviewDays: R4 recommends a decay review only when the last evidence is at least that many days old
	// (default 0: no spacing beyond the decay drop).
	SpacedReviewDays *int `json:"spaced_review_days,omitempty"`
	// PracticeDifficulty shifts the target difficulty of an adaptive practice by −1, 0 or +1 (default 0).
	PracticeDifficulty *int `json:"practice_difficulty,omitempty"`
	// DiagnosticThreshold: R7 counts an objective as not started while it has fewer evidence than this (default 1).
	DiagnosticThreshold *int `json:"diagnostic_threshold,omitempty"`
}

// Bounds of the parameters (an arm outside them is 422 VALIDATION_FAILED) [Confirmed, owner chốt 2026-11-20].
const (
	MinReviewThreshold     = 0.10
	MaxReviewThreshold     = 0.95
	MaxShownByArm          = 3
	MaxPrerequisiteDepth   = 3
	MaxSpacedReviewDays    = 60
	MaxDiagnosticThreshold = 10
)

// ParseParams decodes rule_params (unknown keys rejected); an empty value is the default.
func ParseParams(raw json.RawMessage) (Params, error) {
	var p Params
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return p, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Params{}, err
	}
	return p, nil
}

// Violations lists the parameters outside their bounds (JSON field name → reason).
func (p Params) Violations() map[string]string {
	out := map[string]string{}
	if v := p.ReviewThreshold; v != nil && (*v < MinReviewThreshold || *v > MaxReviewThreshold) {
		out["review_threshold"] = "must be between 0.10 and 0.95"
	}
	if v := p.MaxRecommendations; v != nil && (*v < 1 || *v > MaxShownByArm) {
		out["max_recommendations"] = "must be between 1 and 3"
	}
	if v := p.PrerequisiteDepth; v != nil && (*v < 1 || *v > MaxPrerequisiteDepth) {
		out["prerequisite_depth"] = "must be between 1 and 3"
	}
	if v := p.SpacedReviewDays; v != nil && (*v < 0 || *v > MaxSpacedReviewDays) {
		out["spaced_review_days"] = "must be between 0 and 60"
	}
	if v := p.PracticeDifficulty; v != nil && (*v < -1 || *v > 1) {
		out["practice_difficulty"] = "must be -1, 0 or 1"
	}
	if v := p.DiagnosticThreshold; v != nil && (*v < 1 || *v > MaxDiagnosticThreshold) {
		out["diagnostic_threshold"] = "must be between 1 and 10"
	}
	return out
}

// Merge returns p with the fields set in q (an experiment started later does not override an earlier one: the caller
// merges in start order and keeps the first value).
func (p Params) Merge(q Params) Params {
	if p.ReviewThreshold == nil {
		p.ReviewThreshold = q.ReviewThreshold
	}
	if p.MaxRecommendations == nil {
		p.MaxRecommendations = q.MaxRecommendations
	}
	if p.PrerequisiteDepth == nil {
		p.PrerequisiteDepth = q.PrerequisiteDepth
	}
	if p.SpacedReviewDays == nil {
		p.SpacedReviewDays = q.SpacedReviewDays
	}
	if p.PracticeDifficulty == nil {
		p.PracticeDifficulty = q.PracticeDifficulty
	}
	if p.DiagnosticThreshold == nil {
		p.DiagnosticThreshold = q.DiagnosticThreshold
	}
	return p
}

// Limit is the number of recommendations kept for the student: stored, bounded by MaxRecommendations.
func (p Params) Limit(stored int) int {
	if p.MaxRecommendations != nil && *p.MaxRecommendations < stored {
		return *p.MaxRecommendations
	}
	return stored
}

// DifficultyShift is the offset of the practice difficulty (0 by default).
func (p Params) DifficultyShift() int {
	if p.PracticeDifficulty == nil {
		return 0
	}
	return *p.PracticeDifficulty
}

func (p Params) depth() int {
	if p.PrerequisiteDepth == nil {
		return 1
	}
	return *p.PrerequisiteDepth
}

func (p Params) spacing() time.Duration {
	if p.SpacedReviewDays == nil {
		return 0
	}
	return time.Duration(*p.SpacedReviewDays) * 24 * time.Hour
}

func (p Params) notStarted(o Objective) bool {
	if p.DiagnosticThreshold == nil {
		return o.EvidenceCount == 0
	}
	return o.EvidenceCount < *p.DiagnosticThreshold
}
