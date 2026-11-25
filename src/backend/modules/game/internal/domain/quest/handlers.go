package quest

import (
	"fmt"
	"slices"
)

// Place is what the server knows about where the student stands: the area of its presence and the NPCs and
// interactables of the current version of that area (F071 presence + world content).
type Place struct {
	AreaID        string
	NPCs          []string
	Interactables []string
}

// Report is what the client reports for an interaction step (QuestStepReport): never trusted alone (ADR-0016).
type Report struct {
	AreaID         string
	NPCID          string
	InteractableID string
}

// StepHandler is the behaviour of one step type (QUEST_SYSTEM §3). F072 registers talk_npc, go_to, collect, discover
// and activity; F073 (minigame), F076 (challenge) and F103 (party_activity, interdependent, boss) add theirs without
// changing the engine. A quest with a step type without handler cannot be published.
type StepHandler interface {
	// Type is the step type.
	Type() string
	// Learning reports a learning step: completed only from a server graded attempt, never reported by the client.
	Learning() bool
	// Validate checks the step definition at publish (structure only; references are checked by the validator).
	Validate(s Step) error
	// OnReport applies a client report to an active step at place: the count after the report, whether it changed,
	// ErrPrecondition when the report does not match the step or the place.
	OnReport(s Step, st StepState, r Report, at Place) (StepState, bool, error)
	// OnDiscovery applies a server discovery (world.discovery_found) to an active step: false when it does not match.
	OnDiscovery(s Step, st StepState, areaID, interactableID string) (StepState, bool)
}

// Registry maps step types to their handler.
type Registry map[string]StepHandler

// Handler returns the handler of a step type.
func (r Registry) Handler(stepType string) (StepHandler, bool) {
	h, ok := r[stepType]
	return h, ok
}

// Register adds handlers (later features).
func (r Registry) Register(hs ...StepHandler) {
	for _, h := range hs {
		r[h.Type()] = h
	}
}

// DefaultRegistry is the registry of F072 with the minigame step of F073 and the interdependent and boss steps of F103.
func DefaultRegistry() Registry {
	r := Registry{}
	r.Register(talkNPC{}, goTo{}, collect{}, discover{}, activity{}, MiniGameStep{}, InterdependentStep{}, BossStep{})
	return r
}

type frame struct{}

func (frame) Learning() bool { return false }
func (frame) OnDiscovery(_ Step, st StepState, _, _ string) (StepState, bool) {
	return st, false
}

// reportArea checks the area of a report: the presence area must be the step's area when the step names one, and a
// reported area_id must be the presence area.
func reportArea(s Step, r Report, at Place) error {
	if at.AreaID == "" {
		return fmt.Errorf("%w: no presence in an area", ErrPrecondition)
	}
	if r.AreaID != "" && r.AreaID != at.AreaID {
		return fmt.Errorf("%w: not in area %s", ErrPrecondition, r.AreaID)
	}
	if s.AreaID != "" && s.AreaID != at.AreaID {
		return fmt.Errorf("%w: the step is in area %s", ErrPrecondition, s.AreaID)
	}
	return nil
}

type talkNPC struct{ frame }

func (talkNPC) Type() string { return StepTalkNPC }
func (talkNPC) Validate(s Step) error {
	if s.NPCID == "" {
		return invalid("step %s: talk_npc needs npc_id", s.ID)
	}
	return nil
}
func (talkNPC) OnReport(s Step, st StepState, r Report, at Place) (StepState, bool, error) {
	if err := reportArea(s, r, at); err != nil {
		return st, false, err
	}
	if r.NPCID != s.NPCID || !slices.Contains(at.NPCs, s.NPCID) {
		return st, false, fmt.Errorf("%w: npc not of this step or not in this area", ErrPrecondition)
	}
	st.Count = 1
	return st, true, nil
}

type goTo struct{ frame }

func (goTo) Type() string { return StepGoTo }
func (goTo) Validate(s Step) error {
	if s.AreaID == "" || s.InteractableID == "" {
		return invalid("step %s: go_to needs area_id and interactable_id", s.ID)
	}
	return nil
}
func (goTo) OnReport(s Step, st StepState, r Report, at Place) (StepState, bool, error) {
	if err := reportArea(s, r, at); err != nil {
		return st, false, err
	}
	if r.InteractableID != s.InteractableID || !slices.Contains(at.Interactables, s.InteractableID) {
		return st, false, fmt.Errorf("%w: interactable not of this step", ErrPrecondition)
	}
	st.Count = 1
	return st, true, nil
}

type collect struct{ frame }

func (collect) Type() string { return StepCollect }
func (collect) Validate(s Step) error {
	if s.AreaID == "" || len(s.InteractableIDs) == 0 || s.Count < 1 || s.Count > len(s.InteractableIDs) {
		return invalid("step %s: collect needs area_id, interactable_ids and 1 ≤ count ≤ len(interactable_ids)", s.ID)
	}
	return nil
}
func (collect) OnReport(s Step, st StepState, r Report, at Place) (StepState, bool, error) {
	if err := reportArea(s, r, at); err != nil {
		return st, false, err
	}
	if !slices.Contains(s.InteractableIDs, r.InteractableID) || !slices.Contains(at.Interactables, r.InteractableID) {
		return st, false, fmt.Errorf("%w: interactable not of this step", ErrPrecondition)
	}
	if slices.Contains(st.Collected, r.InteractableID) {
		return st, false, nil // each interactable counts once (F072-AC-11)
	}
	st.Collected = append(slices.Clone(st.Collected), r.InteractableID)
	st.Count = len(st.Collected)
	return st, true, nil
}

type discover struct{}

func (discover) Type() string   { return StepDiscover }
func (discover) Learning() bool { return false }
func (discover) Validate(s Step) error {
	if s.AreaID == "" || len(s.InteractableIDs) == 0 || s.Count < 1 || s.Count > len(s.InteractableIDs) {
		return invalid("step %s: discover needs area_id, interactable_ids and 1 ≤ count ≤ len(interactable_ids)", s.ID)
	}
	return nil
}
func (discover) OnReport(_ Step, st StepState, _ Report, _ Place) (StepState, bool, error) {
	return st, false, fmt.Errorf("%w: a discover step advances from world.discovery_found only", ErrPrecondition)
}
func (discover) OnDiscovery(s Step, st StepState, areaID, interactableID string) (StepState, bool) {
	if areaID != s.AreaID || !slices.Contains(s.InteractableIDs, interactableID) || slices.Contains(st.Collected, interactableID) {
		return st, false
	}
	st.Collected = append(slices.Clone(st.Collected), interactableID)
	st.Count = len(st.Collected)
	return st, true
}

// ContentKindActivity is the only content_ref kind createAttempt of module learning opens (F030): question_set,
// assessment and lesson are refused at publish until createAttempt supports them.
const ContentKindActivity = "activity"

type activity struct{}

func (activity) Type() string   { return StepActivity }
func (activity) Learning() bool { return true }
func (activity) Validate(s Step) error {
	switch {
	case s.ContentRef == nil || s.PassRule == nil:
		return invalid("step %s: activity needs content_ref and pass_rule", s.ID)
	case s.ContentRef.Kind != ContentKindActivity:
		return invalid("step %s: content_ref.kind %q is not opened by module learning yet (only activity)", s.ID, s.ContentRef.Kind)
	}
	return nil
}
func (activity) OnReport(_ Step, st StepState, _ Report, _ Place) (StepState, bool, error) {
	return st, false, fmt.Errorf("%w: a learning step completes from the server graded attempt only", ErrPrecondition)
}
func (activity) OnDiscovery(_ Step, st StepState, _, _ string) (StepState, bool) { return st, false }
