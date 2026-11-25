package quest

import (
	"fmt"
	"regexp"
	"time"
)

// F103 step handlers (QUEST_SYSTEM §3, §3.2): interdependent (each member answers the final question in its own
// attempt; roles hold the clue cards) and boss (completes from boss.encounter_completed defeated for an eligible
// member). Neither completes from a client report.

// Limits of an interdependent step (interdependent_roles 2–4, CONFIG_DEFAULTS).
const (
	MinRoles = 2
	MaxRoles = 4
)

var roleRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// InterdependentStep is the handler of interdependent: a learning step (startQuestActivity opens the final question).
type InterdependentStep struct{}

func (InterdependentStep) Type() string   { return StepInterdep }
func (InterdependentStep) Learning() bool { return true }
func (InterdependentStep) Validate(s Step) error {
	switch {
	case len(s.Roles) < MinRoles || len(s.Roles) > MaxRoles:
		return invalid("step %s: interdependent needs %d to %d roles", s.ID, MinRoles, MaxRoles)
	case s.ContentRef == nil || s.PassRule == nil:
		return invalid("step %s: interdependent needs content_ref (final question) and pass_rule", s.ID)
	case s.ContentRef.Kind != ContentKindActivity:
		return invalid("step %s: content_ref.kind %q is not opened by module learning yet (only activity)", s.ID, s.ContentRef.Kind)
	case s.RoleAssignment != "" && s.RoleAssignment != "auto" && s.RoleAssignment != "leader_assigns":
		return invalid("step %s: role_assignment", s.ID)
	case s.Completion != "" && s.Completion != "all_members" && s.Completion != "min_members":
		return invalid("step %s: completion", s.ID)
	case s.Completion == "min_members" && (s.MinMembersCount < 1 || s.MinMembersCount > 8):
		return invalid("step %s: min_members needs 1 ≤ min_members_count ≤ 8", s.ID)
	}
	seen := map[string]bool{}
	for _, r := range s.Roles {
		if !roleRe.MatchString(r.Role) || seen[r.Role] {
			return invalid("step %s: role %q duplicated or invalid", s.ID, r.Role)
		}
		seen[r.Role] = true
		if r.Clue.Kind != "learning_material" || !uuidRe.MatchString(r.Clue.ID) {
			return invalid("step %s: role %s needs a learning_material clue_content_ref", s.ID, r.Role)
		}
	}
	return nil
}
func (InterdependentStep) OnReport(_ Step, st StepState, _ Report, _ Place) (StepState, bool, error) {
	return st, false, fmt.Errorf("%w: an interdependent step completes from the graded final question only", ErrPrecondition)
}
func (InterdependentStep) OnDiscovery(_ Step, st StepState, _, _ string) (StepState, bool) {
	return st, false
}

// BossStep is the handler of boss: completes from boss.encounter_completed (defeated, member eligible, F103-AC-18).
type BossStep struct{}

func (BossStep) Type() string   { return StepBoss }
func (BossStep) Learning() bool { return false }
func (BossStep) Validate(s Step) error {
	if !uuidRe.MatchString(s.BossID) {
		return invalid("step %s: boss needs boss_id", s.ID)
	}
	return nil
}
func (BossStep) OnReport(_ Step, st StepState, _ Report, _ Place) (StepState, bool, error) {
	return st, false, fmt.Errorf("%w: a boss step completes from the defeated boss encounter only", ErrPrecondition)
}
func (BossStep) OnDiscovery(_ Step, st StepState, _, _ string) (StepState, bool) { return st, false }

// validateParty checks the party fields of a version (QUEST_SYSTEM §6): a party quest has party_size 1 ≤ min ≤ max ≤ 8;
// collective_target only on learning steps of a collective_count quest; a solo quest declares no party field.
func validateParty(v Version, reg Registry) error {
	switch v.SharedProgressPolicy {
	case "", "individual", "shared_steps", "collective_count":
	default:
		return invalid("shared_progress_policy %q", v.SharedProgressPolicy)
	}
	if v.PartyMode == "party_optional" || v.PartyMode == "party_required" {
		if v.PartySize == nil || v.PartySize.Min < 1 || v.PartySize.Max > 8 || v.PartySize.Min > v.PartySize.Max {
			return invalid("party_size: 1 ≤ min ≤ max ≤ 8 for a party quest")
		}
	}
	for _, s := range v.Steps {
		if s.CollectiveTarget == nil {
			continue
		}
		h, _ := reg.Handler(s.Type)
		switch {
		case h == nil || !h.Learning():
			return invalid("step %s: collective_target only on a learning step", s.ID)
		case v.SharedProgressPolicy != "collective_count":
			return invalid("step %s: collective_target needs shared_progress_policy collective_count", s.ID)
		case s.CollectiveTarget.Count < 1 || s.CollectiveTarget.MinContribution < 1 ||
			(s.CollectiveTarget.Metric != "correct_answers" && s.CollectiveTarget.Metric != "completed_items"):
			return invalid("step %s: collective_target needs metric, count ≥ 1 and min_contribution ≥ 1", s.ID)
		}
	}
	return nil
}

// CompleteStep completes an active step of the progress for the member (a shared frame step completed by another
// member of the party, a collective step whose target the party reached, an interdependent step the party completed,
// a boss step of a defeated encounter — F103-AC-14, AC-15, AC-18, AC-23). A step not active changes nothing.
func (p *Progress) CompleteStep(v Version, stepID string, now time.Time) Outcome {
	s, st, err := p.activeStep(v, stepID)
	if err != nil {
		return Outcome{StepID: stepID}
	}
	st.Count = s.Target()
	return p.apply(v, s, st, now)
}

// BossDefeated completes the active boss steps of bossID (F103-AC-18: called only for an eligible member).
func (p *Progress) BossDefeated(v Version, bossID string, now time.Time) []Outcome {
	if !p.Active() {
		return nil
	}
	var out []Outcome
	for _, s := range v.SortedSteps() {
		if s.Type != StepBoss || s.BossID != bossID || p.Steps[s.ID].Status != StepActive {
			continue
		}
		out = append(out, p.CompleteStep(v, s.ID, now))
		if !p.Active() {
			break
		}
	}
	return out
}
