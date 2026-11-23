package reward

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
)

// TotalBucket is the daily counter of every XP of a student (xp_daily_cap).
const TotalBucket = "xp:total"

// ErrInvalid is a reward rule that cannot be published (F074-AC-23); FieldError says which field.
var ErrInvalid = errors.New("invalid reward rule")

// FieldError is a validation failure of one field (path, no value).
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// TriggerEvents are the server events a reward rule of F074 may react to (reward-rule.schema.json trigger.event_type
// of the F074 sources). None of them is a login, a session, online time, speed or a streak (F074-AC-09, NFR-SAFE-003).
var TriggerEvents = []string{"activity.graded", "lesson.completed", "quest.completed", "minigame.completed",
	"learning_objective.mastered", "world.discovery_found", "world.area_completed", "challenge.completed"}

// ChallengeStatuses are the end statuses of challenge.completed a rule may reward (F076): abandoned never is.
var ChallengeStatuses = []string{"succeeded", "failed"}

// SourceCodes is the catalog of XP sources of PROGRESSION_SYSTEM §3.1 (reward-rule.schema.json source_code).
var SourceCodes = []string{"quest.main_completed", "quest.side_completed", "quest.daily_completed", "quest.weekly_completed",
	"activity.completed", "mastery.objective_mastered", "lesson.completed", "exploration.discovery", "exploration.area_completed",
	"challenge.completed", "boss.defeated", "coop.contribution", "achievement.unlocked", "event.participation", "competition.participation"}

// Trigger is a verified server event as the Reward Engine sees it (never a client claim, R5).
type Trigger struct {
	EventType        string
	EventID          uuid.UUID
	OccurredAt       time.Time
	SourceType       string    // activity, quest, minigame, lesson, objective, world
	SourceID         uuid.UUID // attempt, quest_progress, lesson_version, objective, UUIDv5 of an interactable/area
	SourceCodes      []string  // catalog codes this event can earn (quest.completed main → quest.main_completed)
	ContentID        *uuid.UUID
	ContentVersionID *uuid.UUID // repeat counting key (nil: no diminishing)
	SubjectID        *uuid.UUID
	Mode             string // attempt mode (practice, assignment, quest, exam…); "" when not an attempt
	CorrectCount     *int
	ItemCount        *int
	FirstTryCorrect  *int
	RapidGuess       bool
	ContentGrades    []int
	RecommendationID *uuid.UUID
	// RuleID restricts the trigger to one reward rule (reward_rule_id of a mini-game config, F073); nil = every matching rule.
	RuleID *uuid.UUID
	// challenge.completed (F076): end status, reward_multiplier of the level, first clear of the student.
	ChallengeStatus  string
	RewardMultiplier float64
	FirstClear       bool
}

// Conditions are the rule conditions (reward-rule.schema.json conditions).
type Conditions struct {
	MinCorrectRatio     *float64    `json:"min_correct_ratio"`
	MinQuestions        *int        `json:"min_questions_answered"`
	FirstCompletionOnly bool        `json:"first_completion_only"`
	GradeLevels         []int       `json:"grade_levels"`
	SubjectIDs          []uuid.UUID `json:"subject_ids"`
	SourceIDs           []uuid.UUID `json:"source_ids"`
	MinPartySize        *int        `json:"min_party_size"`
	MinContribution     *int        `json:"min_individual_contribution"`
	ChallengeStatuses   []string    `json:"challenge_statuses"`
}

// Grant is one grant of a rule (xp, title in F074; currency, item, badge, collection_item by the appliers of F075).
type Grant struct {
	Type             string `json:"type"`
	Amount           int    `json:"amount,omitempty"`
	PerCorrect       int    `json:"per_correct_answer,omitempty"`
	TitleCode        string `json:"title_code,omitempty"`
	ItemID           string `json:"item_id,omitempty"`
	BadgeID          string `json:"badge_id,omitempty"`
	CurrencyCode     string `json:"currency_code,omitempty"`
	CollectionItemID string `json:"collection_item_id,omitempty"`
	// ScaleBy reward_multiplier: the XP (amount, per_correct_answer) and the Xu of the grant are multiplied by the
	// reward_multiplier of challenge.completed (F076), rounded down, before diminishing and caps.
	ScaleBy string `json:"scale_by,omitempty"`
}

// ScaleReward is the scale_by value of a challenge grant.
const ScaleReward = "reward_multiplier"

// Scaled returns the grant with its Xu scaled by the diminishing factor of the award, rounded down; ok is false when a
// currency grant scales to 0 (nothing to give).
func (g Grant) Scaled(factor float64) (Grant, bool) {
	if g.Type != "currency" {
		return g, true
	}
	g.Amount = int(math.Floor(float64(g.Amount)*factor + 1e-9))
	return g, g.Amount > 0
}

// Caps are the rule caps; Bucket "" counts in xp:src:<source_type>.
type Caps struct {
	XPPerDay int    `json:"xp_per_day"`
	Bucket   string `json:"bucket"`
}

// Diminishing is a repeat multiplier list and its window (the last multiplier applies to every later completion).
type Diminishing struct {
	Multipliers []float64 `json:"multipliers"`
	WindowDays  int       `json:"window_days"`
}

// Rule is one published reward rule version.
type Rule struct {
	RuleID       uuid.UUID
	VersionID    uuid.UUID
	Version      int
	Code         string
	SourceCode   string
	EventType    string
	Priority     int
	AttemptModes []string
	Conditions   Conditions
	Grants       []Grant
	Caps         Caps
	Diminishing  *Diminishing
}

// ruleDoc is the stored payload shape of reward-rule.schema.json the engine reads.
type ruleDoc struct {
	RewardRuleID uuid.UUID `json:"reward_rule_id"`
	Code         string    `json:"code"`
	SourceCode   string    `json:"source_code"`
	Priority     *int      `json:"priority"`
	Trigger      struct {
		EventType    string   `json:"event_type"`
		AttemptModes []string `json:"attempt_modes"`
	} `json:"trigger"`
	Conditions  Conditions   `json:"conditions"`
	Grants      []Grant      `json:"grants"`
	Caps        Caps         `json:"caps"`
	Diminishing *Diminishing `json:"diminishing_returns"`
}

// ParseRule reads a reward rule payload (authored or stored).
func ParseRule(raw []byte) (Rule, error) {
	var d ruleDoc
	if err := json.Unmarshal(raw, &d); err != nil {
		return Rule{}, invalid("payload", "not a reward rule object")
	}
	r := Rule{RuleID: d.RewardRuleID, Code: d.Code, SourceCode: d.SourceCode, EventType: d.Trigger.EventType, Priority: 100,
		AttemptModes: d.Trigger.AttemptModes, Conditions: d.Conditions, Grants: d.Grants, Caps: d.Caps, Diminishing: d.Diminishing}
	if d.Priority != nil {
		r.Priority = *d.Priority
	}
	return r, nil
}

// Validate checks what the JSON Schema leaves to the server (F074-AC-23): a catalog source code, an F074 trigger, a cap
// ≥ 1, an XP amount on every xp grant and a registered applier for every grant type.
func (r Rule) Validate(hasApplier func(grantType string) bool) error {
	if !slices.Contains(SourceCodes, r.SourceCode) {
		return invalid("source_code", "not in the XP source catalog")
	}
	if !slices.Contains(TriggerEvents, r.EventType) {
		return invalid("trigger/event_type", "not a trigger of the Reward Engine")
	}
	if r.Caps.XPPerDay < 1 {
		return invalid("caps/xp_per_day", "must be ≥ 1")
	}
	if len(r.Grants) == 0 {
		return invalid("grants", "at least one grant")
	}
	for i, g := range r.Grants {
		if hasApplier == nil || !hasApplier(g.Type) {
			return invalid(fmt.Sprintf("grants/%d/type", i), "no grant applier for %q", g.Type)
		}
		if g.Type == "xp" && g.Amount < 1 && g.PerCorrect < 1 {
			return invalid("grants", "an xp grant needs amount or per_correct_answer")
		}
		if g.Type == "currency" && (g.CurrencyCode != "xu" || g.Amount < 1) {
			return invalid(fmt.Sprintf("grants/%d", i), "a currency grant gives an amount of xu")
		}
		if g.ScaleBy != "" && (g.ScaleBy != ScaleReward || r.EventType != "challenge.completed") {
			return invalid(fmt.Sprintf("grants/%d/scale_by", i), "reward_multiplier scales challenge.completed grants only")
		}
	}
	for _, s := range r.Conditions.ChallengeStatuses {
		if r.EventType != "challenge.completed" || !slices.Contains(ChallengeStatuses, s) {
			return invalid("conditions/challenge_statuses", "succeeded or failed of challenge.completed only")
		}
	}
	if d := r.Diminishing; d != nil && (len(d.Multipliers) == 0 || d.WindowDays < 1) {
		return invalid("diminishing_returns", "multipliers and window_days ≥ 1 are required")
	}
	return nil
}

// Bucket is the cap counter of the rule for a trigger.
func (r Rule) Bucket(sourceType string) string {
	if r.Caps.Bucket != "" {
		return r.Caps.Bucket
	}
	return "xp:src:" + sourceType
}

// Policy holds the platform parameters (CONFIG_DEFAULTS: xp_daily_cap, xp_repeat_multipliers, xp_below_grade_multiplier).
type Policy struct {
	DailyCap             int
	Repeat               Diminishing
	BelowGradeMultiplier float64
	BelowGradeGap        int // content this many grades below the student or more gets the multiplier (2)
}

// RepeatWindowDays is the repeat window of a rule (its own diminishing_returns or xp_repeat_multipliers).
func (p Policy) RepeatWindowDays(r Rule) int {
	if r.Diminishing != nil {
		return r.Diminishing.WindowDays
	}
	return p.Repeat.WindowDays
}

func (p Policy) multipliers(r Rule) []float64 {
	if r.Diminishing != nil && len(r.Diminishing.Multipliers) > 0 {
		return r.Diminishing.Multipliers
	}
	return p.Repeat.Multipliers
}

// Student is what the engine reads of the student (projection ref.students).
type Student struct{ Grade int }

// History is the ledger history of one rule for a trigger: postings of the same content version and source code in the
// repeat window (Repeats) and ever (Ever, first_completion_only).
type History struct {
	Repeats int
	Ever    int
}

// Award is the outcome of one rule: the XP to post (after multipliers and caps), the base before the caps, the cap flag
// and the other grants (title…) to give through their appliers.
type Award struct {
	Rule     Rule
	Base     int
	Amount   int
	Capped   bool
	Bucket   string
	Rewarded []Grant // non-XP grants of the rule
	// Factor is the grade × repeat multiplier the XP got (1 without one); the Xu of a currency grant gets the same
	// diminishing returns (F075-AC-02, INVENTORY_ECONOMY §4.3) before the Xu daily cap.
	Factor float64
}

// Evaluate applies every matching rule to the trigger, highest priority first (then code), sharing the cap counters
// granted (bucket → XP already granted today; updated in place). Order of a rule: conditions → base XP (amount +
// per_correct_answer × first-try correct answers) → grade multiplier (waived for a recommendation or an assignment)
// → repeat multiplier → caps; rounded down (F074-AC-04…AC-07).
func (p Policy) Evaluate(t Trigger, rules []Rule, st Student, history func(Rule) History, granted map[string]int) []Award {
	matching := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.EventType == t.EventType && slices.Contains(t.SourceCodes, r.SourceCode) && p.modeAllowed(r, t.Mode) && (t.RuleID == nil || *t.RuleID == r.RuleID) {
			matching = append(matching, r)
		}
	}
	sort.SliceStable(matching, func(i, j int) bool {
		if matching[i].Priority != matching[j].Priority {
			return matching[i].Priority > matching[j].Priority
		}
		return matching[i].Code < matching[j].Code
	})
	var out []Award
	for _, r := range matching {
		h := history(r)
		if !conditionsHold(r.Conditions, t, st, h) {
			continue
		}
		a := Award{Rule: r, Bucket: r.Bucket(t.SourceType)}
		base := 0
		for _, g := range r.Grants {
			scale := 1.0
			if g.ScaleBy == ScaleReward && t.RewardMultiplier > 0 {
				scale = t.RewardMultiplier
			}
			if g.Type != "xp" {
				if g.Type == "currency" && scale != 1 {
					g.Amount = int(math.Floor(float64(g.Amount)*scale + 1e-9))
				}
				a.Rewarded = append(a.Rewarded, g)
				continue
			}
			base += int(math.Floor(float64(g.Amount+g.PerCorrect*firstTry(t))*scale + 1e-9))
		}
		a.Base = base
		factor := 1.0
		if p.belowGrade(t, st) {
			factor *= p.BelowGradeMultiplier
		}
		if t.ContentVersionID != nil {
			if ms := p.multipliers(r); len(ms) > 0 {
				factor *= ms[min(h.Repeats, len(ms)-1)]
			}
		}
		a.Factor = factor
		want := int(math.Floor(float64(base)*factor + 1e-9))
		remaining := min(p.DailyCap-granted[TotalBucket], r.Caps.XPPerDay-granted[a.Bucket])
		a.Amount = max(0, min(want, remaining))
		a.Capped = remaining <= 0 || a.Amount < want
		granted[TotalBucket] += a.Amount
		granted[a.Bucket] += a.Amount
		out = append(out, a)
	}
	return out
}

func (p Policy) modeAllowed(r Rule, mode string) bool {
	if mode == "" {
		return true
	}
	if len(r.AttemptModes) > 0 {
		return slices.Contains(r.AttemptModes, mode)
	}
	return mode != "exam" // an exam only earns the rules that list it (fixed "completion" XP, LEARNING_EXPERIENCE)
}

// belowGrade: content ≥ BelowGradeGap grades below the student, outside a recommendation or an assignment (FR-GAM-011).
func (p Policy) belowGrade(t Trigger, st Student) bool {
	if len(t.ContentGrades) == 0 || st.Grade < 1 || p.BelowGradeGap < 1 || t.RecommendationID != nil || t.Mode == "assignment" {
		return false
	}
	return st.Grade-slices.Max(t.ContentGrades) >= p.BelowGradeGap
}

// firstTry is the number of first-try correct answers (F074-AC-06): the event's count; without it the correct count
// unless the attempt is flagged rapid_guess (then 0).
func firstTry(t Trigger) int {
	switch {
	case t.FirstTryCorrect != nil:
		return max(0, *t.FirstTryCorrect)
	case t.RapidGuess || t.CorrectCount == nil:
		return 0
	}
	return max(0, *t.CorrectCount)
}

func conditionsHold(c Conditions, t Trigger, st Student, h History) bool {
	if c.MinPartySize != nil || c.MinContribution != nil {
		return false // party conditions: no F074 trigger carries a party (F103)
	}
	if c.MinCorrectRatio != nil {
		if t.ItemCount == nil || *t.ItemCount == 0 || t.CorrectCount == nil ||
			float64(*t.CorrectCount)/float64(*t.ItemCount) < *c.MinCorrectRatio {
			return false
		}
	}
	if c.MinQuestions != nil && (t.ItemCount == nil || *t.ItemCount < *c.MinQuestions) {
		return false
	}
	if c.FirstCompletionOnly && h.Ever > 0 {
		return false
	}
	if t.EventType == "challenge.completed" {
		statuses := c.ChallengeStatuses
		if len(statuses) == 0 {
			statuses = []string{"succeeded"}
		}
		if !slices.Contains(ChallengeStatuses, t.ChallengeStatus) || !slices.Contains(statuses, t.ChallengeStatus) {
			return false // abandoned is never rewarded; a failure only by a rule that lists it (never negative)
		}
		if c.FirstCompletionOnly && !t.FirstClear {
			return false // first_clear of the payload: the first succeeded attempt of the student on the challenge
		}
	}
	if len(c.GradeLevels) > 0 && !slices.Contains(c.GradeLevels, st.Grade) {
		return false
	}
	if len(c.SubjectIDs) > 0 && (t.SubjectID == nil || !slices.Contains(c.SubjectIDs, *t.SubjectID)) {
		return false
	}
	if len(c.SourceIDs) > 0 && (t.ContentID == nil || !slices.Contains(c.SourceIDs, *t.ContentID)) {
		return false
	}
	return true
}
