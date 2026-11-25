// Package boss is the domain of the boss battles of module game (F103, CHALLENGE_BOSS_SYSTEM §3): the published boss
// version (payload of specs/schemas/boss.schema.json), its phases, requirements and completion rules, and the
// BossEncounter aggregate with its state machine (§3.3), contributions (§3.4) and eligibility. Pure: no I/O, the clock
// is passed in. The server decides everything from graded results of module learning (ADR-0016); wrong answers never
// take progress away (§3.5).
package boss

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// SchemaVersion is the boss.schema.json version the engine reads.
const SchemaVersion = 1

// Boss modes (boss.schema.json mode) and encounter modes.
const (
	ModeSolo          = "solo"
	ModePartyOptional = "party_optional"
	ModePartyRequired = "party_required"
	EncounterSolo     = "solo"
	EncounterParty    = "party"
)

// Completion rule types and on_fail policies (CHALLENGE_BOSS_SYSTEM §3.2).
const (
	RuleAll             = "all"
	RuleAnyN            = "any_n"
	RuleCollectiveCount = "collective_count"
	OnFailRetry         = "retry_phase"
	OnFailEnd           = "end_encounter"
)

// KindActivity is the only requirement kind served by F103: the questions of a learning activity graded by module
// learning (question plan, research R1). minigame, interaction and group_puzzle are refused at publish [Confirmed F103
// Clarifications 2026-11-24].
const KindActivity = "activity"

// Defaults of a boss version (boss.schema.json defaults).
const (
	DefaultMaxPhaseRetries = 3
	// DefaultPassRatio is the min_correct_ratio of a requirement without pass_rule: quest_pass_rule 0.7 (CONFIG_DEFAULTS).
	DefaultPassRatio = 0.7
)

// Domain errors; the application maps them to problem codes.
var (
	ErrInvalid               = errors.New("boss: invalid")                  // VALIDATION_FAILED at publish
	ErrTranslationIncomplete = errors.New("boss: translation incomplete")   // TRANSLATION_INCOMPLETE
	ErrNotMember             = errors.New("boss: not a member")             // ROOM_NOT_MEMBER
	ErrPhaseClosed           = errors.New("boss: phase closed")             // BOSS_PHASE_CLOSED
	ErrInvalidState          = errors.New("boss: invalid state transition") // INVALID_STATE_TRANSITION / BOSS_ENCOUNTER_FULL
)

// FieldError is a publish validation failure of one field.
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, a ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, a...)}
}

// Text is a bilingual text {vi, en}.
type Text struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Complete reports both languages present.
func (t Text) Complete() bool { return strings.TrimSpace(t.VI) != "" && strings.TrimSpace(t.EN) != "" }

// ActivityRef is the learning activity of a requirement (ADR-0048).
type ActivityRef struct {
	ActivityType    string `json:"activity_type"`
	ActivityID      string `json:"activity_id"`
	ActivityVersion *int   `json:"activity_version,omitempty"`
}

// GradeActivity is one per_grade entry.
type GradeActivity struct {
	GradeLevel  int         `json:"grade_level"`
	ActivityRef ActivityRef `json:"activity_ref"`
}

// PassRule is the pass rule of a requirement.
type PassRule struct {
	MinCorrectRatio *float64 `json:"min_correct_ratio,omitempty"`
	MinItems        *int     `json:"min_items,omitempty"`
}

// Requirement is one requirement of a phase.
type Requirement struct {
	RequirementID  string          `json:"requirement_id"`
	Kind           string          `json:"kind"`
	ActivityRef    *ActivityRef    `json:"activity_ref,omitempty"`
	InteractableID string          `json:"interactable_id,omitempty"`
	PerGrade       []GradeActivity `json:"per_grade,omitempty"`
	PassRule       *PassRule       `json:"pass_rule,omitempty"`
}

// ActivityFor returns the activity of the requirement for a grade: its per_grade entry, else the common activity_ref
// (edge case of the spec: a grade without per_grade uses the common activity, already checked against grade_levels).
func (r Requirement) ActivityFor(grade int) (ActivityRef, bool) {
	for _, g := range r.PerGrade {
		if g.GradeLevel == grade {
			return g.ActivityRef, true
		}
	}
	if r.ActivityRef != nil {
		return *r.ActivityRef, true
	}
	return ActivityRef{}, false
}

// Need is the number of correct answers a member needs on this requirement out of items questions: ceil(ratio × n)
// with n = min(min_items, items) (at least 1 when items > 0).
func (r Requirement) Need(items int) int {
	if items <= 0 {
		return math.MaxInt32
	}
	ratio, n := DefaultPassRatio, items
	if r.PassRule != nil {
		if r.PassRule.MinCorrectRatio != nil {
			ratio = *r.PassRule.MinCorrectRatio
		}
		if r.PassRule.MinItems != nil && *r.PassRule.MinItems < n {
			n = *r.PassRule.MinItems
		}
	}
	return max(1, int(math.Ceil(ratio*float64(n)-1e-9)))
}

// CompletionRule is completion_rule of a phase.
type CompletionRule struct {
	Type             string `json:"type"`
	N                int    `json:"n,omitempty"`
	CollectiveTarget int    `json:"collective_target,omitempty"`
	MinContribution  int    `json:"min_contribution,omitempty"`
}

// MinContribution is the correct answers a member must give in the phase to count as contributing (eligibility):
// min_contribution, else 1.
func (c CompletionRule) MinContributionOrDefault() int {
	if c.MinContribution > 0 {
		return c.MinContribution
	}
	return 1
}

// Presentation is the presentation data of a phase (client only).
type Presentation struct {
	IntroLine           *Text  `json:"intro_line,omitempty"`
	WrongAnswerReaction string `json:"wrong_answer_reaction,omitempty"`
	PhaseClearAnimation string `json:"phase_clear_animation,omitempty"`
}

// Phase is one phase of a boss.
type Phase struct {
	PhaseID          string         `json:"phase_id"`
	Order            int            `json:"order"`
	Title            *Text          `json:"title,omitempty"`
	EnergyShare      float64        `json:"energy_share"`
	Requirements     []Requirement  `json:"requirements"`
	CompletionRule   CompletionRule `json:"completion_rule"`
	TimeLimitSeconds int            `json:"time_limit_seconds"`
	OnFail           string         `json:"on_fail,omitempty"`
	Presentation     *Presentation  `json:"presentation,omitempty"`
}

// FailPolicy returns on_fail with its default retry_phase.
func (p Phase) FailPolicy() string {
	if p.OnFail == "" {
		return OnFailRetry
	}
	return p.OnFail
}

// Requirement returns a requirement of the phase.
func (p Phase) Requirement(id string) (Requirement, bool) {
	for _, r := range p.Requirements {
		if r.RequirementID == id {
			return r, true
		}
	}
	return Requirement{}, false
}

// PartySize is party_size.
type PartySize struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Reward is one reward of defeat_rewards / first_defeat_rewards (granted by the Reward Engine, never here).
type Reward map[string]any

// Version is a boss version (boss.schema.json v1). Server fields (boss_version_id, version, status) are set by the
// release core.
type Version struct {
	SchemaVersion         int        `json:"schema_version"`
	BossID                string     `json:"boss_id"`
	Code                  string     `json:"code"`
	OwnerScope            string     `json:"owner_scope"`
	Name                  Text       `json:"name"`
	Description           *Text      `json:"description,omitempty"`
	AreaID                string     `json:"area_id"`
	VisualRef             string     `json:"visual_ref,omitempty"`
	GradeLevels           []int      `json:"grade_levels"`
	SubjectID             string     `json:"subject_id"`
	ObjectiveIDs          []string   `json:"objective_ids,omitempty"`
	Mode                  string     `json:"mode"`
	PartySize             *PartySize `json:"party_size,omitempty"`
	TotalTimeLimitSeconds int        `json:"total_time_limit_seconds"`
	MaxPhaseRetries       *int       `json:"max_phase_retries,omitempty"`
	ReconnectGraceSeconds *int       `json:"reconnect_grace_seconds,omitempty"`
	Phases                []Phase    `json:"phases"`
	DefeatRewards         []Reward   `json:"defeat_rewards"`
	FirstDefeatRewards    []Reward   `json:"first_defeat_rewards,omitempty"`
}

// Parse reads a boss payload (lenient: the JSON Schema check is the publish validator's job).
func Parse(raw json.RawMessage) (Version, error) {
	var v Version
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, invalid("payload", "%v", err)
	}
	return v, nil
}

// SortedPhases returns the phases by order.
func (v Version) SortedPhases() []Phase {
	out := slices.Clone(v.Phases)
	slices.SortStableFunc(out, func(a, b Phase) int { return a.Order - b.Order })
	return out
}

// Phase returns the phase at index i of the order (false outside).
func (v Version) Phase(i int) (Phase, bool) {
	ps := v.SortedPhases()
	if i < 0 || i >= len(ps) {
		return Phase{}, false
	}
	return ps[i], true
}

// MaxRetries returns max_phase_retries with its default 3.
func (v Version) MaxRetries() int {
	if v.MaxPhaseRetries != nil {
		return *v.MaxPhaseRetries
	}
	return DefaultMaxPhaseRetries
}

// Grace returns reconnect_grace_seconds of the version, else fallback (boss_reconnect_grace).
func (v Version) Grace(fallback time.Duration) time.Duration {
	if v.ReconnectGraceSeconds != nil {
		return time.Duration(*v.ReconnectGraceSeconds) * time.Second
	}
	return fallback
}

// SizeMin is party_size.min (1 without party size).
func (v Version) SizeMin() int {
	if v.PartySize != nil && v.PartySize.Min > 0 {
		return v.PartySize.Min
	}
	return 1
}

// SizeMax is party_size.max (1 without party size).
func (v Version) SizeMax() int {
	if v.PartySize != nil && v.PartySize.Max > 0 {
		return v.PartySize.Max
	}
	return 1
}

// Solo reports a boss a student may play alone (mode solo or party_optional).
func (v Version) Solo() bool { return v.Mode != ModePartyRequired }

// Party reports a boss a party may play (party_optional or party_required).
func (v Version) Party() bool { return v.Mode == ModePartyOptional || v.Mode == ModePartyRequired }

// AdmitsGrade reports grade in grade_levels.
func (v Version) AdmitsGrade(grade int) bool { return slices.Contains(v.GradeLevels, grade) }

// MissingTranslations lists the pointers of player texts without vi or en (ADR-0056).
func (v Version) MissingTranslations() []string {
	var out []string
	check := func(p string, t *Text) {
		if t == nil {
			return
		}
		if strings.TrimSpace(t.VI) == "" {
			out = append(out, p+"/vi")
		}
		if strings.TrimSpace(t.EN) == "" {
			out = append(out, p+"/en")
		}
	}
	check("/name", &v.Name)
	check("/description", v.Description)
	for i, p := range v.Phases {
		check(fmt.Sprintf("/phases/%d/title", i), p.Title)
		if p.Presentation != nil {
			check(fmt.Sprintf("/phases/%d/presentation/intro_line", i), p.Presentation.IntroLine)
		}
	}
	return out
}

// Validate checks the domain rules the JSON Schema cannot express (publish, F103 data-model §3): the energy shares sum
// to 1, phase orders and ids are unique, the completion rule matches the requirements, every requirement is served
// (activity, with an activity for every grade of the boss), party_size is within 1..8 and min ≤ max.
func (v Version) Validate() error {
	if len(v.Phases) == 0 {
		return invalid("phases", "at least one phase")
	}
	if v.Party() && (v.PartySize == nil || v.PartySize.Min < 1 || v.PartySize.Max > 8 || v.PartySize.Min > v.PartySize.Max) {
		return invalid("party_size", "1 ≤ min ≤ max ≤ 8 for a party boss")
	}
	sum := 0.0
	ids, orders := map[string]bool{}, map[int]bool{}
	for i, p := range v.Phases {
		at := fmt.Sprintf("phases/%d", i)
		if ids[p.PhaseID] || orders[p.Order] {
			return invalid(at, "phase_id and order are unique")
		}
		ids[p.PhaseID], orders[p.Order] = true, true
		sum += p.EnergyShare
		reqs := map[string]bool{}
		for j, r := range p.Requirements {
			rat := fmt.Sprintf("%s/requirements/%d", at, j)
			if reqs[r.RequirementID] {
				return invalid(rat, "requirement_id is unique in the phase")
			}
			reqs[r.RequirementID] = true
			if r.Kind != KindActivity {
				return invalid(rat+"/kind", "only activity requirements are served in this release")
			}
			for _, g := range v.GradeLevels {
				if _, ok := r.ActivityFor(g); !ok {
					return invalid(rat, "no activity for grade %d", g)
				}
			}
		}
		switch p.CompletionRule.Type {
		case RuleAll:
		case RuleAnyN:
			if p.CompletionRule.N < 1 || p.CompletionRule.N > len(p.Requirements) {
				return invalid(at+"/completion_rule/n", "1 ≤ n ≤ number of requirements")
			}
		case RuleCollectiveCount:
			if p.CompletionRule.CollectiveTarget < 1 || p.CompletionRule.MinContribution < 1 {
				return invalid(at+"/completion_rule", "collective_target and min_contribution are required")
			}
		default:
			return invalid(at+"/completion_rule/type", "all, any_n or collective_count")
		}
		if p.TimeLimitSeconds > v.TotalTimeLimitSeconds {
			return invalid(at+"/time_limit_seconds", "at most total_time_limit_seconds")
		}
	}
	if math.Abs(sum-1) > 1e-6 {
		return invalid("phases", "the energy shares sum to 1 (got %.4f)", sum)
	}
	return nil
}
