package efficacy

import (
	"time"

	"github.com/google/uuid"
)

// Wave is a scheduled test wave of a study.
type Wave struct {
	ID          uuid.UUID
	Kind        string
	Form        string // validation only
	WindowStart time.Time
	WindowEnd   time.Time
	Status      string
}

// WaveRequest is a request of scheduleEfficacyWave.
type WaveRequest struct {
	Kind        string
	Form        string
	WindowStart time.Time
	WindowEnd   time.Time
	DurationSec int // 0 = the duration of the blueprint
}

// CheckWave applies F035-AC-12 and F035-AC-13: a valid kind and window in the future; validation names its form; pre and
// post only on an active study, once each (cancelled waves aside), post after pre and opening no earlier than the end
// of pre plus min_intervention_weeks. It returns ErrState for a transition the study does not allow.
func CheckWave(r WaveRequest, studyStatus string, minWeeks int, existing []Wave, now time.Time) ([]Violation, error) {
	var vs []Violation
	switch r.Kind {
	case WaveValidation:
		if r.Form != FormA && r.Form != FormB {
			vs = append(vs, Violation{"/form", "is required (a or b) for a validation wave"})
		}
		if studyStatus != StudyDraft && studyStatus != StudyActive {
			return nil, ErrState
		}
	case WavePre, WavePost:
		if studyStatus != StudyActive {
			return nil, ErrState
		}
	default:
		return []Violation{{"/kind", "must be validation, pre or post"}}, nil
	}
	if !r.WindowEnd.After(r.WindowStart) {
		vs = append(vs, Violation{"/window_end", "must be after window_start"})
	}
	if !r.WindowEnd.After(now) {
		vs = append(vs, Violation{"/window_end", "must be in the future"})
	}
	if r.DurationSec != 0 && (r.DurationSec < 60 || r.DurationSec > 14400) {
		vs = append(vs, Violation{"/duration_seconds", "must be between 60 and 14400"})
	}
	if len(vs) > 0 || r.Kind == WaveValidation {
		return vs, nil
	}
	var pre *Wave
	for i, w := range existing {
		if w.Status == WaveCancelled {
			continue
		}
		if w.Kind == r.Kind {
			return nil, ErrState
		}
		if w.Kind == WavePre {
			pre = &existing[i]
		}
	}
	if r.Kind == WavePost {
		if pre == nil {
			return nil, ErrState
		}
		earliest := pre.WindowEnd.Add(time.Duration(minWeeks) * Week)
		if r.WindowStart.Before(earliest) {
			return []Violation{{"/window_start", "the post wave opens at least min_intervention_weeks after the end of the pre wave"}}, nil
		}
	}
	return nil, nil
}

// FormOf is the form a cohort takes in a wave (F035-AC-12): pre the first form of its order, post the second one,
// validation the form of the request.
func FormOf(kind, order, requested string) string {
	switch kind {
	case WaveValidation:
		return requested
	case WavePre:
		if order == OrderBThenA {
			return FormB
		}
		return FormA
	}
	if order == OrderBThenA {
		return FormA
	}
	return FormB
}

// TargetsWave reports whether a cohort of an arm sits a wave of a kind: validation cohorts sit validation waves only,
// intervention and comparison cohorts sit pre and post only (data-model §3).
func TargetsWave(kind, arm string) bool {
	if kind == WaveValidation {
		return arm == ArmValidation
	}
	return arm == ArmIntervention || arm == ArmComparison
}
