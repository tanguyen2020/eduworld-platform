// Package achievement is the domain of the achievements of module game (F075, INVENTORY_ECONOMY §7): a versioned
// definition (trigger events, payload filters, progress metric, tiers, visibility), its validation (never play time,
// logins, messages or official scores — R19, NFR-SAFE-003), the progress of a student (never lowered, F075-AC-24) and the
// tiers a progress reaches. Pure: the evaluator in the application layer makes it idempotent and transactional.
package achievement

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalid is an achievement version that cannot be published (422 VALIDATION_FAILED naming the field, F075-AC-23).
var ErrInvalid = errors.New("achievement: invalid")

// FieldError names the invalid field (no value).
type FieldError struct {
	Field, Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// TriggerEvents is the trigger catalog (achievement.schema.json trigger.event_types): verified server events of progress
// and completion — never a login, a session, online time or a chat message.
var TriggerEvents = []string{"avatar.created", "quest.completed", "lesson.completed", "exercise.submitted", "learning_objective.mastered",
	"challenge.completed", "boss.encounter_completed", "world.discovery_found", "world.area_completed", "world.area_unlocked",
	"level.reached", "party.activity_completed", "match.completed", "collection.completed", "shop.item_purchased",
	"event.participation_started", "event.milestone_claimed", "season_track.tier_reached"} // F077 (per-student events)

// Categories of an achievement (AchievementCategory; event joined with F077).
var Categories = []string{"first_time", "progress", "learning", "exploration", "social", "team", "competition", "seasonal", "event", "hidden"}

// Tier codes in rank order.
var Tiers = []string{"single", "bronze", "silver", "gold", "platinum"}

// HiddenUntilUnlocked hides the name, conditions and progress of a locked achievement (F075-AC-22).
const HiddenUntilUnlocked = "hidden_until_unlocked"

// RewardTypes are the tier reward types (no xp: XP comes from reward rules — spec Clarifications).
var RewardTypes = []string{"currency", "item", "badge", "title", "collection_item"}

// MaxDistinct bounds a distinct_count threshold: the counted values live in progress_state (≤ 4096 bytes) [Proposed].
const MaxDistinct = 300

// bannedFields are payload fields an achievement may never count or sum (F075-AC-23, GAME_DESIGN §12): play time,
// logins/sessions, messages, official scores.
var bannedFields = []string{"time", "duration", "minute", "second", "login", "session", "online", "message", "chat", "streak", "score",
	"mark", "grade_point"}

// Text is a bilingual text.
type Text struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Filters select the trigger events by payload field (achievement.schema.json trigger.filters).
type Filters struct {
	SubjectIDs    []uuid.UUID `json:"subject_ids"`
	AreaIDs       []string    `json:"area_ids"`
	QuestTypes    []string    `json:"quest_types"`
	MinDifficulty string      `json:"min_difficulty"`
	Result        string      `json:"result"`
}

// Metric is the progress metric.
type Metric struct {
	Kind          string `json:"kind"`
	DistinctField string `json:"distinct_field"`
	SumField      string `json:"sum_field"`
}

// Tier is one tier of an achievement.
type Tier struct {
	Tier      string           `json:"tier"`
	Threshold int              `json:"threshold"`
	Rewards   []map[string]any `json:"rewards"`
}

// Definition is one achievement version.
type Definition struct {
	AchievementID uuid.UUID `json:"achievement_id"`
	Code          string    `json:"code"`
	Version       int       `json:"version"`
	Category      string    `json:"category"`
	Visibility    string    `json:"visibility"`
	Name          Text      `json:"name"`
	Description   *Text     `json:"description"`
	Trigger       struct {
		EventTypes []string `json:"event_types"`
		Filters    Filters  `json:"filters"`
	} `json:"trigger"`
	Metric Metric `json:"progress_metric"`
	Tiers  []Tier `json:"tiers"`
}

// Parse reads an achievement payload.
func Parse(raw []byte) (Definition, error) {
	var d Definition
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, invalid("payload", "not an achievement object")
	}
	return d, nil
}

// Hidden says whether the achievement is hidden until unlocked.
func (d Definition) Hidden() bool { return d.Visibility == HiddenUntilUnlocked }

func banned(field string) bool {
	f := strings.ToLower(field)
	for _, b := range bannedFields {
		if strings.Contains(f, b) {
			return true
		}
	}
	return false
}

// Validate checks what the JSON Schema leaves to the server (F075-AC-23): catalog triggers, a metric that never counts
// play time, logins, sessions, messages or official scores, tiers of increasing thresholds in rank order, rewards of the
// allowed types with a registered applier.
func (d Definition) Validate(hasApplier func(string) bool) error {
	if d.AchievementID == uuid.Nil {
		return invalid("achievement_id", "required")
	}
	if !slices.Contains(Categories, d.Category) {
		return invalid("category", "not an achievement category")
	}
	if len(d.Trigger.EventTypes) == 0 {
		return invalid("trigger/event_types", "at least one trigger")
	}
	for i, t := range d.Trigger.EventTypes {
		if !slices.Contains(TriggerEvents, t) {
			return invalid(fmt.Sprintf("trigger/event_types/%d", i), "not in the trigger catalog")
		}
	}
	switch d.Metric.Kind {
	case "count", "distinct_days_in_week":
	case "distinct_count":
		if d.Metric.DistinctField == "" || banned(d.Metric.DistinctField) {
			return invalid("progress_metric/distinct_field", "not a countable progress field")
		}
	case "sum":
		if d.Metric.SumField == "" || banned(d.Metric.SumField) {
			return invalid("progress_metric/sum_field", "not a summable progress field")
		}
	default:
		return invalid("progress_metric/kind", "unknown metric")
	}
	if banned(d.Trigger.Filters.Result) {
		return invalid("trigger/filters/result", "not a progress condition")
	}
	if len(d.Tiers) == 0 {
		return invalid("tiers", "at least one tier")
	}
	prevRank, prevThreshold := -1, 0
	for i, t := range d.Tiers {
		rank := slices.Index(Tiers, t.Tier)
		if rank < 0 {
			return invalid(fmt.Sprintf("tiers/%d/tier", i), "not a tier")
		}
		if rank <= prevRank || t.Threshold <= prevThreshold {
			return invalid(fmt.Sprintf("tiers/%d/threshold", i), "tiers go up in rank and threshold")
		}
		if d.Metric.Kind == "distinct_count" && t.Threshold > MaxDistinct {
			return invalid(fmt.Sprintf("tiers/%d/threshold", i), "at most %d distinct values", MaxDistinct)
		}
		if d.Metric.Kind == "distinct_days_in_week" && t.Threshold > 7 {
			return invalid(fmt.Sprintf("tiers/%d/threshold", i), "a week has 7 days")
		}
		for j, r := range t.Rewards {
			rt, _ := r["type"].(string)
			if !slices.Contains(RewardTypes, rt) || hasApplier == nil || !hasApplier(rt) {
				return invalid(fmt.Sprintf("tiers/%d/rewards/%d/type", i, j), "not a tier reward type")
			}
		}
		prevRank, prevThreshold = rank, t.Threshold
	}
	return nil
}

// Event is a verified server event as the evaluator sees it.
type Event struct {
	Type    string
	ID      uuid.UUID
	At      time.Time
	Payload map[string]any
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

var difficulties = []string{"easy", "normal", "hard", "expert", "legendary"}

// Matches says whether e counts for the definition: a trigger event type whose payload passes every filter.
func (d Definition) Matches(e Event) bool {
	if !slices.Contains(d.Trigger.EventTypes, e.Type) {
		return false
	}
	f := d.Trigger.Filters
	if len(f.SubjectIDs) > 0 {
		id, err := uuid.Parse(str(e.Payload["subject_id"]))
		if err != nil || !slices.Contains(f.SubjectIDs, id) {
			return false
		}
	}
	if len(f.AreaIDs) > 0 && !slices.Contains(f.AreaIDs, str(e.Payload["area_id"])) {
		return false
	}
	if len(f.QuestTypes) > 0 && !slices.Contains(f.QuestTypes, str(e.Payload["quest_type"])) {
		return false
	}
	if f.MinDifficulty != "" {
		got := slices.Index(difficulties, str(e.Payload["difficulty"]))
		if got < 0 || got < slices.Index(difficulties, f.MinDifficulty) {
			return false
		}
	}
	if f.Result != "" && str(e.Payload["result"]) != f.Result && str(e.Payload["status"]) != f.Result {
		return false
	}
	return true
}

// State is the progress of a student on one achievement (student_achievements.progress, progress_state, tier).
type State struct {
	Progress int      `json:"-"`
	Values   []string `json:"values,omitempty"` // fnv32 of the distinct values counted
	Week     string   `json:"week,omitempty"`   // ISO week of the distinct days (ICT)
	Days     []string `json:"days,omitempty"`
	Tier     string   `json:"-"` // highest tier unlocked ("" = none)
}

func hash(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return strconv.FormatUint(uint64(h.Sum32()), 16)
}

// Advance adds e to the progress (F075-AC-21): count +1; distinct_count a new value of distinct_field; sum the positive
// value of sum_field; distinct_days_in_week the days (zone loc) of the current ISO week. Progress never goes down
// (F075-AC-24). Returns whether the progress changed.
func (d Definition) Advance(st *State, e Event, loc *time.Location) bool {
	before := st.Progress
	switch d.Metric.Kind {
	case "count":
		st.Progress++
	case "distinct_count":
		v := str(e.Payload[d.Metric.DistinctField])
		if v == "" {
			return false
		}
		h := hash(v)
		if !slices.Contains(st.Values, h) && len(st.Values) < MaxDistinct {
			st.Values = append(st.Values, h)
		}
		st.Progress = max(st.Progress, len(st.Values))
	case "sum":
		n, ok := e.Payload[d.Metric.SumField].(float64)
		if !ok || n <= 0 || n > math.MaxInt32 {
			return false
		}
		st.Progress += int(n)
	case "distinct_days_in_week":
		at := e.At.In(loc)
		y, w := at.ISOWeek()
		week := fmt.Sprintf("%04d-W%02d", y, w)
		if st.Week != week {
			st.Week, st.Days = week, nil
		}
		day := at.Format(time.DateOnly)
		if !slices.Contains(st.Days, day) {
			st.Days = append(st.Days, day)
		}
		st.Progress = max(st.Progress, len(st.Days))
	}
	return st.Progress != before
}

// NewTiers are the tiers the progress reaches above the highest tier already unlocked, in order.
func (d Definition) NewTiers(st State) []Tier {
	have := slices.Index(Tiers, st.Tier)
	var out []Tier
	for _, t := range d.Tiers {
		if st.Progress >= t.Threshold && slices.Index(Tiers, t.Tier) > have {
			out = append(out, t)
		}
	}
	return out
}

// Target is the threshold of the next tier to unlock (nil when every tier is unlocked).
func (d Definition) Target(st State) *int {
	have := slices.Index(Tiers, st.Tier)
	for _, t := range d.Tiers {
		if slices.Index(Tiers, t.Tier) > have {
			v := t.Threshold
			return &v
		}
	}
	return nil
}
