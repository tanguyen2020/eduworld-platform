// Package challenge is the Challenge Engine of module game for the solo challenges (F076, CHALLENGE_BOSS_SYSTEM §2):
// the published challenge version (specs/schemas/challenge.schema.json), its five difficulty profiles mapped to the
// question difficulty band inside the grade of the student, the eligibility of a student, and the ChallengeAttempt
// aggregate whose score, lives and goal change only from the results module learning graded (ADR-0016). Pure: the
// clock and the platform parameters are given.
package challenge

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"
)

// Challenge types (challenge.schema.json challenge_type). F076 serves the solo ones; boss/coop/party → F103,
// tournament → F107.
const (
	TypeTime       = "time"
	TypePerfect    = "perfect"
	TypeSurvival   = "survival"
	TypePuzzle     = "puzzle"
	TypeKnowledge  = "knowledge"
	TypeWeekly     = "weekly"
	TypeSeasonal   = "seasonal"
	TypeBoss       = "boss"
	TypeCoop       = "coop"
	TypeParty      = "party"
	TypeTournament = "tournament"
)

// SoloTypes are the types F076 serves.
var SoloTypes = []string{TypeTime, TypePerfect, TypeSurvival, TypePuzzle, TypeKnowledge, TypeWeekly, TypeSeasonal}

// finiteTypes play one pass over their activities ("xong bộ câu"); the others cycle until the goal, the lives or the
// time end the challenge.
var finiteTypes = []string{TypePerfect, TypeKnowledge}

// Difficulty levels (difficulty_levels of CONFIG_DEFAULTS), easiest first.
const (
	Easy      = "easy"
	Normal    = "normal"
	Hard      = "hard"
	Expert    = "expert"
	Legendary = "legendary"
)

// Levels are the five levels in order.
var Levels = []string{Easy, Normal, Hard, Expert, Legendary}

// Rank is the order of a level (0 easy … 4 legendary, -1 unknown).
func Rank(level string) int { return slices.Index(Levels, level) }

// Modes (challenge.schema.json mode).
const (
	ModeSolo          = "solo"
	ModePartyOptional = "party_optional"
	ModePartyRequired = "party_required"
)

// Goal metrics (challenge.schema.json goal.metric).
const (
	MetricCorrect    = "correct_answers"
	MetricActivities = "completed_activities"
	MetricAccuracy   = "accuracy_ratio"
	MetricRounds     = "rounds_survived"
	MetricPuzzle     = "puzzle_solved"
	MetricCollective = "collective_correct_answers"
)

// Activity sources (challenge.schema.json activity_source.kind).
const (
	SourceActivities  = "activity_ref_list"
	SourceQuestionSet = "question_selection"
)

// ErrInvalid is a challenge that cannot be published (F076-AC-20); FieldError says which field.
var ErrInvalid = errors.New("invalid challenge")

// ErrTranslationIncomplete is a displayed text without its vi or en version (TRANSLATION_INCOMPLETE).
var ErrTranslationIncomplete = errors.New("translation incomplete")

// FieldError is a validation failure of one field (JSON pointer path, no value).
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// Text is a displayed text in vi and en (ADR-0056).
type Text struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Profile is one difficulty profile of a version (challenge.schema.json difficulty_profile).
type Profile struct {
	Level            string  `json:"level"`
	Min              int     `json:"question_difficulty_min"`
	Max              int     `json:"question_difficulty_max"`
	SecondsPerItem   int     `json:"seconds_per_item,omitempty"`
	Lives            int     `json:"lives,omitempty"`
	RewardMultiplier float64 `json:"reward_multiplier"`
	GradeLevels      []int   `json:"grade_levels,omitempty"`
}

// ActivityRef is one learning activity of the source (ADR-0048).
type ActivityRef struct {
	ActivityType    string `json:"activity_type"`
	ActivityID      string `json:"activity_id"`
	ActivityVersion int    `json:"activity_version,omitempty"`
}

// Source is activity_source.
type Source struct {
	Kind          string        `json:"kind"`
	ActivityRefs  []ActivityRef `json:"activity_refs,omitempty"`
	QuestionSetID string        `json:"question_set_id,omitempty"`
}

// Goal is the goal of the challenge.
type Goal struct {
	Metric          string  `json:"metric"`
	Target          float64 `json:"target"`
	MinContribution int     `json:"min_contribution,omitempty"`
}

// Retry is the retry policy of a version.
type Retry struct {
	MaxAttemptsPerDay        int    `json:"max_attempts_per_day,omitempty"`
	CooldownSecondsAfterFail *int   `json:"cooldown_seconds_after_fail,omitempty"`
	RetryPolicy              string `json:"retry_policy,omitempty"`
}

// Availability is the window of a version.
type Availability struct {
	StartAt    *time.Time `json:"start_at,omitempty"`
	EndAt      *time.Time `json:"end_at,omitempty"`
	Recurrence string     `json:"recurrence,omitempty"`
}

// Unlock is unlock_rule: server facts only (level, completed quests, open areas).
type Unlock struct {
	MinLevel          int      `json:"min_level,omitempty"`
	CompletedQuestIDs []string `json:"completed_quest_ids,omitempty"`
	UnlockedAreaIDs   []string `json:"unlocked_area_ids,omitempty"`
}

// Reward is one first-clear reward intent (granted by the Reward Engine, never by this package).
type Reward struct {
	Type             string `json:"type"`
	Amount           int    `json:"amount,omitempty"`
	CurrencyCode     string `json:"currency_code,omitempty"`
	ItemID           string `json:"item_id,omitempty"`
	BadgeID          string `json:"badge_id,omitempty"`
	TitleCode        string `json:"title_code,omitempty"`
	CollectionItemID string `json:"collection_item_id,omitempty"`
}

// Rewards are the reward intents of a version.
type Rewards struct {
	FirstClear     []Reward `json:"first_clear,omitempty"`
	PerClearRuleID string   `json:"per_clear_reward_rule_id,omitempty"`
}

// Version is a challenge version (the authored or stored payload of challenge.schema.json).
type Version struct {
	ChallengeID      string        `json:"challenge_id"`
	VersionID        string        `json:"challenge_version_id,omitempty"`
	Code             string        `json:"code"`
	Version          int           `json:"version,omitempty"`
	Type             string        `json:"challenge_type"`
	Name             Text          `json:"name"`
	Description      *Text         `json:"description,omitempty"`
	AreaID           string        `json:"area_id,omitempty"`
	GradeLevels      []int         `json:"grade_levels"`
	SubjectID        string        `json:"subject_id"`
	ObjectiveIDs     []string      `json:"objective_ids"`
	Mode             string        `json:"mode,omitempty"`
	BossID           string        `json:"boss_id,omitempty"`
	Difficulties     []Profile     `json:"difficulties"`
	Source           Source        `json:"activity_source"`
	Goal             Goal          `json:"goal"`
	TimeLimitSeconds int           `json:"time_limit_seconds,omitempty"`
	Retry            *Retry        `json:"retry,omitempty"`
	Availability     *Availability `json:"availability,omitempty"`
	Unlock           *Unlock       `json:"unlock_rule,omitempty"`
	Rewards          Rewards       `json:"rewards"`
}

// Parse reads a challenge payload (authored or stored); the default mode is solo.
func Parse(raw []byte) (Version, error) {
	var v Version
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, invalid("payload", "not a challenge object")
	}
	if v.Mode == "" {
		v.Mode = ModeSolo
	}
	return v, nil
}

// Profile returns the profile of a level.
func (v Version) Profile(level string) (Profile, bool) {
	for _, p := range v.Difficulties {
		if p.Level == level {
			return p, true
		}
	}
	return Profile{}, false
}

// Solo reports a version playable alone in F076: a solo type, not party_required.
func (v Version) Solo() bool {
	return slices.Contains(SoloTypes, v.Type) && v.Mode != ModePartyRequired
}

// Finite reports a type that plays one pass over its activities.
func (v Version) Finite() bool { return slices.Contains(finiteTypes, v.Type) }

// Window returns the availability window (nil bounds = open).
func (v Version) Window() (from, to *time.Time) {
	if v.Availability == nil {
		return nil, nil
	}
	return v.Availability.StartAt, v.Availability.EndAt
}

// MissingTranslations lists the displayed texts without vi or en.
func (v Version) MissingTranslations() []string {
	var out []string
	if v.Name.VI == "" || v.Name.EN == "" {
		out = append(out, "name")
	}
	if d := v.Description; d != nil && (d.VI == "" || d.EN == "") {
		out = append(out, "description")
	}
	return out
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Validate checks what the JSON Schema leaves to the server (F076-AC-20): translations first, then the mode of the group
// types, the profiles (one per level, min ≤ max, grades of the challenge, g1_3_challenge_rules), the time limit of a
// time challenge, the activity source (activity_ref_list of learning activities: createAttempt opens activities only;
// their publication is checked when an attempt opens them, F072 precedent), the goal of the type and the window.
func (v Version) Validate(r Rules) error {
	if m := v.MissingTranslations(); len(m) > 0 {
		return fmt.Errorf("%w: %v", ErrTranslationIncomplete, m)
	}
	group := slices.Contains([]string{TypeBoss, TypeCoop, TypeParty, TypeTournament}, v.Type)
	if group && (v.Mode == "" || v.Mode == ModeSolo) {
		return invalid("mode", "a %s challenge is never solo", v.Type)
	}
	seen := map[string]bool{}
	for i, p := range v.Difficulties {
		f := fmt.Sprintf("difficulties/%d/", i)
		switch {
		case Rank(p.Level) < 0 || seen[p.Level]:
			return invalid(f+"level", "one profile per level")
		case p.Min < 1 || p.Max > 5 || p.Min > p.Max:
			return invalid(f+"question_difficulty_min", "1 ≤ min ≤ max ≤ 5")
		}
		seen[p.Level] = true
		grades := p.GradeLevels
		if len(grades) == 0 {
			grades = v.GradeLevels
		}
		for _, g := range grades {
			if !slices.Contains(v.GradeLevels, g) {
				return invalid(f+"grade_levels", "grade %d is not a grade of the challenge", g)
			}
			if g <= 3 && Rank(p.Level) > Rank(r.g13Max()) {
				return invalid(f+"grade_levels", "grades 1–3 play up to %s (g1_3_challenge_rules)", r.g13Max())
			}
		}
	}
	if v.Type == TypeTime && v.TimeLimitSeconds <= 0 {
		return invalid("time_limit_seconds", "a time challenge needs a time limit")
	}
	switch v.Source.Kind {
	case SourceActivities:
		if len(v.Source.ActivityRefs) == 0 {
			return invalid("activity_source/activity_refs", "at least one activity")
		}
		for i, a := range v.Source.ActivityRefs {
			if !uuidRe.MatchString(a.ActivityID) {
				return invalid(fmt.Sprintf("activity_source/activity_refs/%d/activity_id", i), "a learning activity id")
			}
		}
	default:
		return invalid("activity_source/kind", "only activity_ref_list: createAttempt opens learning activities")
	}
	if !group {
		switch {
		case v.Goal.Metric == MetricCollective:
			return invalid("goal/metric", "a party metric for a solo challenge")
		case v.Goal.Metric == MetricAccuracy && (!v.Finite() || v.Goal.Target > 1):
			return invalid("goal/metric", "accuracy_ratio (≤ 1) is the goal of a perfect or knowledge challenge")
		}
	}
	if a := v.Availability; a != nil && a.StartAt != nil && a.EndAt != nil && !a.StartAt.Before(*a.EndAt) {
		return invalid("availability/end_at", "after start_at")
	}
	if v.Type == TypeSeasonal && (v.Availability == nil || v.Availability.StartAt == nil || v.Availability.EndAt == nil) {
		return invalid("availability", "a seasonal challenge has a start and an end")
	}
	return nil
}
