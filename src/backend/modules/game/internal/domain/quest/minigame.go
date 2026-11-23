package quest

import "fmt"

// MiniGameStep is the handler of the step type minigame (F073-AC-16, QUEST_SYSTEM §3): a learning step that opens an
// attempt of mode quest through the plugin minigame of module learning (createAttempt, content minigame_config, the
// config version pinned at start) and completes from the server graded attempt when its completion rule is met
// (completion_met of activity.graded), never from a client report.
type MiniGameStep struct{}

// Type implements StepHandler.
func (MiniGameStep) Type() string { return StepMinigame }

// Learning implements StepHandler.
func (MiniGameStep) Learning() bool { return true }

// Validate implements StepHandler.
func (MiniGameStep) Validate(s Step) error {
	if s.MiniGameConfigID == "" || s.PassRule == nil {
		return invalid("step %s: minigame needs minigame_config_id and pass_rule", s.ID)
	}
	return nil
}

// OnReport implements StepHandler.
func (MiniGameStep) OnReport(_ Step, st StepState, _ Report, _ Place) (StepState, bool, error) {
	return st, false, fmt.Errorf("%w: a minigame step completes from the server graded attempt only", ErrPrecondition)
}

// OnDiscovery implements StepHandler.
func (MiniGameStep) OnDiscovery(_ Step, st StepState, _, _ string) (StepState, bool) {
	return st, false
}
