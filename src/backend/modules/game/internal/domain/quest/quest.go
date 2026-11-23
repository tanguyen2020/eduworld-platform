// Package quest is the domain of the quest engine of module game (F072, QUEST_SYSTEM): the published quest version
// (payload of specs/schemas/quest.schema.json v2), its availability for a student, the progress aggregate with its
// step state machine, the step handler registry, pass rules, completion rules, failure policy and the daily/weekly
// periods. Pure: no I/O, the clock is passed in.
package quest

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// SchemaVersion is the quest.schema.json version the engine reads.
const SchemaVersion = 2

// Quest types (QUEST_SYSTEM §2.3).
const (
	TypeMain         = "main"
	TypeSide         = "side"
	TypeDaily        = "daily"
	TypeWeekly       = "weekly"
	TypeLearning     = "learning"
	TypeExploration  = "exploration"
	TypeSocial       = "social"
	TypeParty        = "party"
	TypeChallenge    = "challenge"
	TypeSeasonal     = "seasonal"
	TypeSpecialEvent = "special_event"
	TypeBoss         = "boss"
)

// Types are the twelve quest types of schema v2.
var Types = []string{TypeMain, TypeSide, TypeDaily, TypeWeekly, TypeLearning, TypeExploration, TypeSocial, TypeParty,
	TypeChallenge, TypeSeasonal, TypeSpecialEvent, TypeBoss}

// objectiveTypes need at least one learning objective (QUEST_SYSTEM §2.1).
var objectiveTypes = []string{TypeLearning, TypeDaily, TypeWeekly, TypeChallenge, TypeParty, TypeBoss}

// Step types (QUEST_SYSTEM §3).
const (
	StepTalkNPC       = "talk_npc"
	StepGoTo          = "go_to"
	StepCollect       = "collect"
	StepDiscover      = "discover"
	StepActivity      = "activity"
	StepMinigame      = "minigame"
	StepPartyActivity = "party_activity"
	StepInterdep      = "interdependent"
	StepChallenge     = "challenge"
	StepBoss          = "boss"
)

// Start triggers (QUEST_SYSTEM §4).
const (
	TriggerNPC        = "npc"
	TriggerBoard      = "quest_board"
	TriggerAutoDaily  = "auto_daily"
	TriggerAutoWeekly = "auto_weekly"
	TriggerAssignment = "assignment"
	TriggerEvent      = "event"
	TriggerChain      = "chain"
	TriggerDiscovery  = "discovery"
	TriggerLevel      = "level_reached"
)

// Repeatability modes.
const (
	RepeatOnce       = "once"
	RepeatPerPeriod  = "per_period"
	RepeatRepeatable = "repeatable"
)

// Recurrences of availability.
const (
	RecurrenceNone   = "none"
	RecurrenceDaily  = "daily"
	RecurrenceWeekly = "weekly"
)

// Failure kinds and policies (QUEST_SYSTEM §2.5).
const (
	FailTimeLimit    = "time_limit_exceeded"
	FailAvailability = "availability_ended"
	OnFailReset      = "reset_step"
	OnFailQuest      = "fail_quest"
)

// Completion rule types.
const (
	CompleteAllRequired = "all_required_steps"
	CompleteAnyN        = "any_n_of"
)

// Domain errors; the application maps them to problem codes.
var (
	// ErrTranslationIncomplete: a text the player sees lacks vi or en (TRANSLATION_INCOMPLETE, ADR-0056).
	ErrTranslationIncomplete = errors.New("quest: translation incomplete")
	// ErrInvalid: the quest breaks a structural rule (VALIDATION_FAILED).
	ErrInvalid = errors.New("quest: invalid")
	// ErrRefInvalid: a reference to an area, NPC, interactable, chapter or quest is unknown (ITEM_REF_INVALID).
	ErrRefInvalid = errors.New("quest: reference invalid")
	// ErrPrecondition: a step report or start does not meet its preconditions (STEP_PRECONDITION_FAILED).
	ErrPrecondition = errors.New("quest: step precondition failed")
	// ErrInvalidState: the step or progress is not in a state that allows the action (INVALID_STATE_TRANSITION).
	ErrInvalidState = errors.New("quest: invalid state transition")
)

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Text is a bilingual text {vi, en} (localized_text of the schema).
type Text struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Complete reports a text with both languages.
func (t Text) Complete() bool { return strings.TrimSpace(t.VI) != "" && strings.TrimSpace(t.EN) != "" }

// ContentRef is the learning content a learning step opens (content_ref).
type ContentRef struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Version   *int   `json:"version,omitempty"`
	VersionID string `json:"version_id,omitempty"`
}

// PassRule is the pass rule of a learning step (QUEST_SYSTEM §3.1).
type PassRule struct {
	MinCorrectRatio   *float64 `json:"min_correct_ratio,omitempty"`
	MinQuestions      *int     `json:"min_questions,omitempty"`
	RetryPolicy       string   `json:"retry_policy,omitempty"`
	MaxAttemptsPerDay *int     `json:"max_attempts_per_day,omitempty"`
}

// Step is one step of a quest version.
type Step struct {
	ID              string      `json:"step_id"`
	Order           int         `json:"order"`
	Type            string      `json:"type"`
	Title           Text        `json:"title"`
	Hint            *Text       `json:"hint,omitempty"`
	DependsOn       []string    `json:"depends_on,omitempty"`
	Optional        bool        `json:"optional,omitempty"`
	AreaID          string      `json:"area_id,omitempty"`
	NPCID           string      `json:"npc_id,omitempty"`
	DialogueNodeID  string      `json:"dialogue_node_id,omitempty"`
	InteractableID  string      `json:"interactable_id,omitempty"`
	InteractableIDs []string    `json:"interactable_ids,omitempty"`
	Count           int         `json:"count,omitempty"`
	ContentRef      *ContentRef `json:"content_ref,omitempty"`
	PassRule        *PassRule   `json:"pass_rule,omitempty"`
	// MiniGameConfigID and MiniGameConfigVersion are the mini-game of a minigame step (F073; no version = current).
	MiniGameConfigID      string `json:"minigame_config_id,omitempty"`
	MiniGameConfigVersion *int   `json:"minigame_config_version,omitempty"`
}

// Targets are the interactables a step points the player at (go_to, collect, discover).
func (s Step) Targets() []string {
	if s.InteractableID != "" {
		return []string{s.InteractableID}
	}
	return s.InteractableIDs
}

// Target is the count a step needs (collect, discover: count; others 1).
func (s Step) Target() int {
	if (s.Type == StepCollect || s.Type == StepDiscover) && s.Count > 0 {
		return s.Count
	}
	return 1
}

// Trigger is start_trigger.
type Trigger struct {
	Kind            string   `json:"kind"`
	NPCID           string   `json:"npc_id,omitempty"`
	AreaID          string   `json:"area_id,omitempty"`
	EventID         string   `json:"event_id,omitempty"`
	InteractableIDs []string `json:"interactable_ids,omitempty"`
}

// Prerequisites are the start conditions of a quest.
type Prerequisites struct {
	MinLevel               int      `json:"min_level,omitempty"`
	CompletedQuestIDs      []string `json:"completed_quest_ids,omitempty"`
	UnlockedAreaIDs        []string `json:"unlocked_area_ids,omitempty"`
	RequiredSubjectEnabled *bool    `json:"required_subject_enabled,omitempty"`
}

// LearningRequirement is a mastery level a student must reach on an objective before accepting.
type LearningRequirement struct {
	ObjectiveID     string `json:"objective_id"`
	MinMasteryLevel string `json:"min_mastery_level"`
}

// Repeatability is repeatability.
type Repeatability struct {
	Mode           string `json:"mode"`
	CooldownHours  int    `json:"cooldown_hours,omitempty"`
	MaxCompletions int    `json:"max_completions,omitempty"`
}

// FailureCondition is one failure_conditions entry.
type FailureCondition struct {
	Kind   string `json:"kind"`
	StepID string `json:"step_id,omitempty"`
	OnFail string `json:"on_fail,omitempty"`
}

// Policy returns on_fail with its default reset_step.
func (f FailureCondition) Policy() string {
	if f.OnFail == "" {
		return OnFailReset
	}
	return f.OnFail
}

// Availability is availability.
type Availability struct {
	StartAt          *time.Time `json:"start_at,omitempty"`
	EndAt            *time.Time `json:"end_at,omitempty"`
	Recurrence       string     `json:"recurrence,omitempty"`
	TimeLimitMinutes *int       `json:"time_limit_minutes,omitempty"`
}

// CompletionRule is completion_rule.
type CompletionRule struct {
	Type    string   `json:"type"`
	StepIDs []string `json:"step_ids,omitempty"`
	N       int      `json:"n,omitempty"`
}

// Reward is one reward intent of rewards[] (granted by the Reward Engine, F074 — never by the quest engine).
type Reward map[string]any

// Version is a quest version (quest.schema.json v2). Server fields (quest_version_id, version, status) are set by the
// release core.
type Version struct {
	SchemaVersion        int                   `json:"schema_version"`
	QuestID              string                `json:"quest_id"`
	Code                 string                `json:"code"`
	OwnerScope           string                `json:"owner_scope"`
	QuestType            string                `json:"quest_type"`
	Name                 Text                  `json:"name"`
	Description          Text                  `json:"description"`
	Narrative            *Text                 `json:"narrative,omitempty"`
	GiverNPCID           string                `json:"giver_npc_id,omitempty"`
	AreaID               string                `json:"area_id,omitempty"`
	StoryChapterCode     string                `json:"story_chapter_code,omitempty"`
	GradeLevels          []int                 `json:"grade_levels"`
	SubjectID            string                `json:"subject_id"`
	SubjectCode          string                `json:"subject_code,omitempty"`
	ObjectiveIDs         []string              `json:"objective_ids"`
	EstimatedMinutes     int                   `json:"estimated_minutes,omitempty"`
	StartTrigger         Trigger               `json:"start_trigger"`
	Prerequisites        *Prerequisites        `json:"prerequisites,omitempty"`
	LearningRequirements []LearningRequirement `json:"learning_requirements,omitempty"`
	Repeatability        *Repeatability        `json:"repeatability,omitempty"`
	FailureConditions    []FailureCondition    `json:"failure_conditions,omitempty"`
	Availability         *Availability         `json:"availability,omitempty"`
	PartyMode            string                `json:"party_mode"`
	Steps                []Step                `json:"steps"`
	CompletionRule       CompletionRule        `json:"completion_rule"`
	Rewards              []Reward              `json:"rewards"`
}

// Parse reads a quest payload (lenient: the JSON Schema check is the publish validator's job).
func Parse(raw json.RawMessage) (Version, error) {
	var v Version
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, invalid("payload: %v", err)
	}
	return v, nil
}

// Step returns the step id.
func (v Version) Step(id string) (Step, bool) {
	for _, s := range v.Steps {
		if s.ID == id {
			return s, true
		}
	}
	return Step{}, false
}

// SortedSteps returns the steps by order (stable).
func (v Version) SortedSteps() []Step {
	out := slices.Clone(v.Steps)
	slices.SortStableFunc(out, func(a, b Step) int { return a.Order - b.Order })
	return out
}

// Deps returns the effective dependencies of a step: depends_on when declared, otherwise the steps of the previous
// order (sequential by order, QUEST_SYSTEM §3).
func (v Version) Deps(s Step) []string {
	if s.DependsOn != nil {
		return s.DependsOn
	}
	prev := 0
	for _, o := range v.Steps {
		if o.Order < s.Order && o.Order > prev {
			prev = o.Order
		}
	}
	if prev == 0 {
		return nil
	}
	var out []string
	for _, o := range v.Steps {
		if o.Order == prev {
			out = append(out, o.ID)
		}
	}
	return out
}

// RepeatMode returns repeatability.mode with its default: per_period for daily/weekly or a recurring availability,
// once otherwise.
func (v Version) RepeatMode() string {
	if v.Repeatability != nil && v.Repeatability.Mode != "" {
		return v.Repeatability.Mode
	}
	if v.Recurrence() != RecurrenceNone {
		return RepeatPerPeriod
	}
	return RepeatOnce
}

// Recurrence returns the period of a per-period quest: availability.recurrence, else daily for daily quests and
// weekly for weekly quests, else none.
func (v Version) Recurrence() string {
	if v.Availability != nil && v.Availability.Recurrence != "" && v.Availability.Recurrence != RecurrenceNone {
		return v.Availability.Recurrence
	}
	switch {
	case v.QuestType == TypeDaily || v.StartTrigger.Kind == TriggerAutoDaily:
		return RecurrenceDaily
	case v.QuestType == TypeWeekly || v.StartTrigger.Kind == TriggerAutoWeekly:
		return RecurrenceWeekly
	}
	return RecurrenceNone
}

// MandatoryLearning reports a quest of the learning loop that min_level never blocks (QUEST_SYSTEM §2.1, WORLD_DESIGN
// §7): learning, daily and weekly quests.
func (v Version) MandatoryLearning() bool {
	return v.QuestType == TypeLearning || v.QuestType == TypeDaily || v.QuestType == TypeWeekly
}

// AutoGranted reports a quest the server grants at the start of a period (never accepted by the player).
func (v Version) AutoGranted() bool {
	return v.StartTrigger.Kind == TriggerAutoDaily || v.StartTrigger.Kind == TriggerAutoWeekly
}

// Acceptable reports a start trigger the player may accept in F072 (assignment: F041/F078; event: F077).
func (v Version) Acceptable() bool {
	switch v.StartTrigger.Kind {
	case TriggerNPC, TriggerBoard, TriggerChain, TriggerDiscovery, TriggerLevel:
		return true
	}
	return false
}

// TimeLimit returns availability.time_limit_minutes (0 = none).
func (v Version) TimeLimit() time.Duration {
	if v.Availability == nil || v.Availability.TimeLimitMinutes == nil {
		return 0
	}
	return time.Duration(*v.Availability.TimeLimitMinutes) * time.Minute
}

// Failure returns the failure condition of kind; a time limit without a declared condition fails with the default
// reset_step (QUEST_SYSTEM §2.5: failure never loses progress by default).
func (v Version) Failure(kind string) (FailureCondition, bool) {
	for _, f := range v.FailureConditions {
		if f.Kind == kind {
			return f, true
		}
	}
	if kind == FailTimeLimit && v.TimeLimit() > 0 {
		return FailureCondition{Kind: FailTimeLimit, OnFail: OnFailReset}, true
	}
	return FailureCondition{}, false
}

// Window returns the availability window (nil bounds are open).
func (v Version) Window() (from, to *time.Time) {
	if v.Availability == nil {
		return nil, nil
	}
	return v.Availability.StartAt, v.Availability.EndAt
}

// InWindow reports now within availability.
func (v Version) InWindow(now time.Time) bool {
	from, to := v.Window()
	return (from == nil || !now.Before(*from)) && (to == nil || now.Before(*to))
}

// AdmitsGrade reports grade in grade_levels.
func (v Version) AdmitsGrade(grade int) bool { return slices.Contains(v.GradeLevels, grade) }

// Texts walks every text the player sees with its JSON pointer (name, description, narrative, steps' title and hint).
func (v Version) Texts(fn func(pointer string, t Text)) {
	fn("/name", v.Name)
	fn("/description", v.Description)
	if v.Narrative != nil {
		fn("/narrative", *v.Narrative)
	}
	for i, s := range v.Steps {
		fn(fmt.Sprintf("/steps/%d/title", i), s.Title)
		if s.Hint != nil {
			fn(fmt.Sprintf("/steps/%d/hint", i), *s.Hint)
		}
	}
}

// MissingTranslations lists the pointers of player texts without vi or en (ADR-0056).
func (v Version) MissingTranslations() []string {
	var out []string
	v.Texts(func(p string, t Text) {
		if strings.TrimSpace(t.VI) == "" {
			out = append(out, p+"/vi")
		}
		if strings.TrimSpace(t.EN) == "" {
			out = append(out, p+"/en")
		}
	})
	return out
}

// ExpiresOnGradeChange reports a quest that ends (expired, no penalty) when the student moves to grade (PROGRESSION_SYSTEM
// §5.1, FR-GAM-010, F074-AC-25): a learning, daily or weekly quest whose grade_levels do not hold the new grade.
// Exploration and social quests, and quests of every grade, stay.
func (v Version) ExpiresOnGradeChange(grade int) bool {
	return v.MandatoryLearning() && len(v.GradeLevels) > 0 && !slices.Contains(v.GradeLevels, grade)
}
