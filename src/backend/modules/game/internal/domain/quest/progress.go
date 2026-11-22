package quest

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Progress statuses (QUEST_SYSTEM §5).
const (
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusExpired   = "expired"
	StatusAbandoned = "abandoned"
	StatusFailed    = "failed"
)

// Step statuses.
const (
	StepLocked    = "locked"
	StepActive    = "active"
	StepCompleted = "completed"
)

// StepState is the state of one step of a progress (step_states[step_id]).
type StepState struct {
	Status      string     `json:"status"`
	Count       int        `json:"count"`
	Collected   []string   `json:"collected_ids,omitempty"`
	AttemptIDs  []string   `json:"attempt_ids,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Progress is the QuestProgress aggregate: the progress of a student on the quest version pinned when it started
// (ADR-0043). It changes only through its methods, from server-verified facts (ADR-0016).
type Progress struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	StudentID    uuid.UUID
	SchoolID     uuid.UUID
	QuestID      uuid.UUID
	VersionID    uuid.UUID
	QuestType    string
	Status       string
	Steps        map[string]StepState
	StartTrigger string
	PeriodKey    string
	CompletionNo int
	PartyID      *uuid.UUID
	StartedAt    time.Time
	CompletedAt  *time.Time
	ExpiresAt    *time.Time
	DeadlineAt   *time.Time
	FailedAt     *time.Time
	EndedAt      *time.Time
	Version      int // optimistic lock
}

// Start is what a new progress needs.
type Start struct {
	ID, TenantID, StudentID, SchoolID uuid.UUID
	QuestID, VersionID                uuid.UUID
	Trigger                           string
	PeriodKey                         string
	ExpiresAt                         *time.Time
	CompletionNo                      int
	PartyID                           *uuid.UUID
	At                                time.Time
}

// NewProgress starts a progress on version v: the steps whose dependencies are met are active, the others locked
// (F072-AC-04); the time limit sets the deadline.
func NewProgress(v Version, s Start) *Progress {
	p := &Progress{ID: s.ID, TenantID: s.TenantID, StudentID: s.StudentID, SchoolID: s.SchoolID, QuestID: s.QuestID, VersionID: s.VersionID,
		QuestType: v.QuestType, Status: StatusActive, Steps: map[string]StepState{}, StartTrigger: s.Trigger, PeriodKey: s.PeriodKey,
		CompletionNo: max(1, s.CompletionNo), PartyID: s.PartyID, StartedAt: s.At, ExpiresAt: s.ExpiresAt, Version: 1}
	for _, st := range v.Steps {
		p.Steps[st.ID] = StepState{Status: StepLocked}
	}
	if d := v.TimeLimit(); d > 0 {
		dl := s.At.Add(d)
		p.DeadlineAt = &dl
	}
	p.unlock(v)
	return p
}

// Active reports an active progress.
func (p *Progress) Active() bool { return p.Status == StatusActive }

// State returns the state of a step.
func (p *Progress) State(id string) StepState { return p.Steps[id] }

// depMet reports a dependency that no longer blocks: completed, or optional and itself unlocked (an optional step
// never gates the steps after it, F072-AC-19).
func (p *Progress) depMet(v Version, id string) bool {
	st := p.Steps[id]
	if st.Status == StepCompleted {
		return true
	}
	s, ok := v.Step(id)
	return ok && s.Optional && st.Status != StepLocked
}

// unlock activates the locked steps whose dependencies are met (until a fixed point); returns the steps activated.
func (p *Progress) unlock(v Version) []string {
	var out []string
	for changed := true; changed; {
		changed = false
		for _, s := range v.SortedSteps() {
			if p.Steps[s.ID].Status != StepLocked {
				continue
			}
			met := true
			for _, d := range v.Deps(s) {
				if !p.depMet(v, d) {
					met = false
					break
				}
			}
			if met {
				st := p.Steps[s.ID]
				st.Status = StepActive
				p.Steps[s.ID] = st
				out = append(out, s.ID)
				changed = true
			}
		}
	}
	return out
}

// Outcome is what a step change did.
type Outcome struct {
	StepID        string
	Changed       bool     // the step state changed
	StepCompleted bool     // the step completed now
	Activated     []string // steps activated by the completion
	Completed     bool     // the quest completed now
}

// activeStep returns an active step or ErrInvalidState (locked or completed step, ended progress).
func (p *Progress) activeStep(v Version, id string) (Step, StepState, error) {
	s, ok := v.Step(id)
	if !ok {
		return s, StepState{}, fmt.Errorf("%w: unknown step", ErrPrecondition)
	}
	if !p.Active() {
		return s, StepState{}, fmt.Errorf("%w: the quest is %s", ErrInvalidState, p.Status)
	}
	st := p.Steps[id]
	if st.Status != StepActive {
		return s, st, fmt.Errorf("%w: the step is %s", ErrInvalidState, st.Status)
	}
	return s, st, nil
}

// apply stores a new step state, completes the step when its target is met, unlocks and completes the quest.
func (p *Progress) apply(v Version, s Step, st StepState, at time.Time) Outcome {
	o := Outcome{StepID: s.ID, Changed: true}
	if st.Count >= s.Target() {
		st.Status, st.CompletedAt = StepCompleted, &at
		o.StepCompleted = true
	}
	p.Steps[s.ID] = st
	if o.StepCompleted {
		o.Activated = p.unlock(v)
		o.Completed = p.completeIfDone(v, at)
	}
	return o
}

// Report applies a client report of an interaction step after the server checked the place (F072-AC-09…AC-11).
func (p *Progress) Report(v Version, reg Registry, stepID string, r Report, at Place, now time.Time) (Outcome, error) {
	s, st, err := p.activeStep(v, stepID)
	if err != nil {
		return Outcome{StepID: stepID}, err
	}
	h, ok := reg.Handler(s.Type)
	if !ok {
		return Outcome{StepID: stepID}, fmt.Errorf("%w: no handler for %s", ErrPrecondition, s.Type)
	}
	next, changed, err := h.OnReport(s, st, r, at)
	if err != nil || !changed {
		return Outcome{StepID: stepID}, err
	}
	return p.apply(v, s, next, now), nil
}

// Discovery applies a server discovery to every active step it matches (F072-AC-12).
func (p *Progress) Discovery(v Version, reg Registry, areaID, interactableID string, now time.Time) []Outcome {
	if !p.Active() {
		return nil
	}
	var out []Outcome
	for _, s := range v.SortedSteps() {
		st := p.Steps[s.ID]
		if st.Status != StepActive {
			continue
		}
		h, ok := reg.Handler(s.Type)
		if !ok {
			continue
		}
		if next, ok := h.OnDiscovery(s, st, areaID, interactableID); ok {
			out = append(out, p.apply(v, s, next, now))
			if !p.Active() {
				break
			}
		}
	}
	return out
}

// CanStartActivity checks that stepID is an active learning step.
func (p *Progress) CanStartActivity(v Version, reg Registry, stepID string) (Step, error) {
	s, _, err := p.activeStep(v, stepID)
	if err != nil {
		return s, err
	}
	if h, ok := reg.Handler(s.Type); !ok || !h.Learning() {
		return s, fmt.Errorf("%w: not a learning step", ErrPrecondition)
	}
	return s, nil
}

// AttemptStarted records an attempt opened for a learning step.
func (p *Progress) AttemptStarted(stepID, attemptID string) {
	st := p.Steps[stepID]
	if !slices.Contains(st.AttemptIDs, attemptID) {
		st.AttemptIDs = append(slices.Clone(st.AttemptIDs), attemptID)
	}
	p.Steps[stepID] = st
}

// ActivityResult applies the pass rule result of a graded attempt to its learning step (F072-AC-15): passed completes
// the step; not passed leaves it active (retry with new questions, Q-5). An ended progress or a step no longer active
// changes nothing (edge cases of the spec).
func (p *Progress) ActivityResult(v Version, stepID string, passed bool, now time.Time) Outcome {
	s, st, err := p.activeStep(v, stepID)
	if err != nil || !passed {
		return Outcome{StepID: stepID}
	}
	st.Count = s.Target()
	return p.apply(v, s, st, now)
}

// Done reports the completion rule met: all_required_steps (optional steps never block) or any_n_of.
func (p *Progress) Done(v Version) bool {
	switch v.CompletionRule.Type {
	case CompleteAnyN:
		n := 0
		for _, id := range v.CompletionRule.StepIDs {
			if p.Steps[id].Status == StepCompleted {
				n++
			}
		}
		return v.CompletionRule.N > 0 && n >= v.CompletionRule.N
	default:
		for _, s := range v.Steps {
			if !s.Optional && p.Steps[s.ID].Status != StepCompleted {
				return false
			}
		}
		return len(v.Steps) > 0
	}
}

func (p *Progress) completeIfDone(v Version, at time.Time) bool {
	if !p.Active() || !p.Done(v) {
		return false
	}
	p.Status, p.CompletedAt, p.EndedAt = StatusCompleted, &at, &at
	return true
}

// end moves an active progress to a final status.
func (p *Progress) end(status string, at time.Time) error {
	if !p.Active() {
		return fmt.Errorf("%w: the quest is %s", ErrInvalidState, p.Status)
	}
	p.Status, p.EndedAt = status, &at
	if status == StatusFailed {
		p.FailedAt = &at
	}
	return nil
}

// Abandon is abandonQuest (F072-AC-07): attempts and learning records stay as they are.
func (p *Progress) Abandon(at time.Time) error { return p.end(StatusAbandoned, at) }

// Expire ends a progress whose period or availability is over (F072-AC-23): nothing is taken from the student.
func (p *Progress) Expire(at time.Time) error { return p.end(StatusExpired, at) }

// Fail ends a progress on a failure condition with on_fail = fail_quest (F072-AC-26).
func (p *Progress) Fail(at time.Time) error { return p.end(StatusFailed, at) }

// ResetSteps applies on_fail = reset_step (F072-AC-26): the step of the condition (or, without step_id, every active
// step) goes back to active with no progress; the other steps keep theirs; the time limit restarts. Returns the steps
// reset.
func (p *Progress) ResetSteps(v Version, stepID string, at time.Time) []string {
	if !p.Active() {
		return nil
	}
	var reset []string
	for _, s := range v.SortedSteps() {
		st := p.Steps[s.ID]
		if (stepID != "" && s.ID != stepID) || st.Status != StepActive {
			continue // completed steps keep their progress; locked steps have none
		}
		p.Steps[s.ID] = StepState{Status: StepActive, AttemptIDs: st.AttemptIDs}
		reset = append(reset, s.ID)
	}
	if d := v.TimeLimit(); d > 0 {
		dl := at.Add(d)
		p.DeadlineAt = &dl
	}
	return reset
}
