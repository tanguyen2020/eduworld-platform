package world

import (
	"errors"
	"regexp"
	"slices"
)

// Requirement kinds (UnlockRequirement.kind, AsyncAPI world.area.unlocked unlocked_by).
const (
	ReqQuest     = "quest"
	ReqChapters  = "chapters_completed"
	ReqObjective = "learning_objective"
	ReqLevel     = "level"
	ReqAchieve   = "achievement"
	ReqEvent     = "event"
)

// Mastery levels in order (LEARNING_ANALYTICS): a condition "≥ developing" is met by developing and above.
var masteryOrder = []string{"not_started", "beginning", "developing", "proficient", "mastered"}

func masteryRank(level string) int { return slices.Index(masteryOrder, level) }

// Rule is WorldArea.unlock_rule (WORLD_DESIGN §7.2): exactly one of all/any or a condition. There is no payment
// condition and the decoder rejects any other key.
type Rule struct {
	All                 []Rule           `json:"all,omitempty"`
	Any                 []Rule           `json:"any,omitempty"`
	QuestCompleted      *QuestCond       `json:"quest_completed,omitempty"`
	ChaptersCompleted   *ChaptersCond    `json:"chapters_completed,omitempty"`
	ObjectiveProgress   *ObjectiveCond   `json:"objective_progress,omitempty"`
	MinLevel            *LevelCond       `json:"min_level,omitempty"`
	AchievementUnlocked *AchievementCond `json:"achievement_unlocked,omitempty"`
	EventParticipation  *EventCond       `json:"event_participation,omitempty"`
}

// QuestCond is quest_completed.
type QuestCond struct {
	QuestID string `json:"quest_id"`
	Label   *Name  `json:"label,omitempty"`
}

// ChaptersCond is chapters_completed.
type ChaptersCond struct {
	Min             int   `json:"min"`
	SubjectChapters bool  `json:"subject_chapters_only,omitempty"`
	Label           *Name `json:"label,omitempty"`
}

// ObjectiveCond is objective_progress (read from ref.mastery_summary).
type ObjectiveCond struct {
	ObjectiveIDs []string `json:"objective_ids"`
	MinCount     int      `json:"min_count"`
	MinLevel     string   `json:"min_level"`
	Label        *Name    `json:"label,omitempty"`
}

// LevelCond is min_level (not for learning areas).
type LevelCond struct {
	Level int   `json:"level"`
	Label *Name `json:"label,omitempty"`
}

// AchievementCond is achievement_unlocked.
type AchievementCond struct {
	AchievementID string `json:"achievement_id"`
	Label         *Name  `json:"label,omitempty"`
}

// EventCond is event_participation (not for learning areas).
type EventCond struct {
	EventID string `json:"event_id"`
	Label   *Name  `json:"label,omitempty"`
}

// Facts are what the server knows of a student when it evaluates the rules (never sent by the client, ADR-0016).
type Facts struct {
	Quests          map[string]bool   // quest ids completed
	Achievements    map[string]bool   // achievement ids unlocked
	Events          map[string]bool   // event ids the student took part in
	Level           int               // highest level reached
	Chapters        int               // story chapters completed
	SubjectChapters int               // subject chapters (1–4) completed
	Mastery         map[string]string // objective id → mastery level
}

// Requirement is one condition of a rule and whether the student meets it (UnlockRequirement).
type Requirement struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref,omitempty"`
	Label   *Name  `json:"label,omitempty"`
	Met     bool   `json:"met"`
	Current int    `json:"current"`
	Target  int    `json:"target"`
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (r Rule) count() int {
	n := 0
	for _, set := range []bool{len(r.All) > 0, len(r.Any) > 0, r.QuestCompleted != nil, r.ChaptersCompleted != nil,
		r.ObjectiveProgress != nil, r.MinLevel != nil, r.AchievementUnlocked != nil, r.EventParticipation != nil} {
		if set {
			n++
		}
	}
	return n
}

func (r Rule) validate(depth int) error {
	if depth > 4 {
		return errors.New("rule nested too deep")
	}
	if r.count() != 1 {
		return errors.New("a rule holds exactly one of all, any or a condition")
	}
	for _, sub := range append(append([]Rule{}, r.All...), r.Any...) {
		if err := sub.validate(depth + 1); err != nil {
			return err
		}
	}
	if len(r.All) > 10 || len(r.Any) > 10 {
		return errors.New("at most 10 sub-rules")
	}
	for _, l := range []*Name{r.label()} {
		if l != nil && !l.Complete() {
			return errors.New("label needs vi and en")
		}
	}
	switch {
	case r.QuestCompleted != nil && !uuidRe.MatchString(r.QuestCompleted.QuestID):
		return errors.New("quest_completed.quest_id must be a UUID")
	case r.ChaptersCompleted != nil && (r.ChaptersCompleted.Min < 1 || r.ChaptersCompleted.Min > 10):
		return errors.New("chapters_completed.min 1..10")
	case r.ObjectiveProgress != nil && (len(r.ObjectiveProgress.ObjectiveIDs) == 0 || r.ObjectiveProgress.MinCount < 1 ||
		r.ObjectiveProgress.MinCount > len(r.ObjectiveProgress.ObjectiveIDs) || masteryRank(r.ObjectiveProgress.MinLevel) < 1):
		return errors.New("objective_progress needs objective ids, 1 ≤ min_count ≤ their number and a mastery level")
	case r.MinLevel != nil && (r.MinLevel.Level < 1 || r.MinLevel.Level > 1000):
		return errors.New("min_level.level 1..1000")
	case r.AchievementUnlocked != nil && (r.AchievementUnlocked.AchievementID == "" || len(r.AchievementUnlocked.AchievementID) > 64):
		return errors.New("achievement_unlocked.achievement_id")
	case r.EventParticipation != nil && (r.EventParticipation.EventID == "" || len(r.EventParticipation.EventID) > 64):
		return errors.New("event_participation.event_id")
	}
	if r.ObjectiveProgress != nil {
		for _, id := range r.ObjectiveProgress.ObjectiveIDs {
			if !uuidRe.MatchString(id) {
				return errors.New("objective_progress.objective_ids must be UUIDs")
			}
		}
	}
	return nil
}

func (r Rule) label() *Name {
	switch {
	case r.QuestCompleted != nil:
		return r.QuestCompleted.Label
	case r.ChaptersCompleted != nil:
		return r.ChaptersCompleted.Label
	case r.ObjectiveProgress != nil:
		return r.ObjectiveProgress.Label
	case r.MinLevel != nil:
		return r.MinLevel.Label
	case r.AchievementUnlocked != nil:
		return r.AchievementUnlocked.Label
	case r.EventParticipation != nil:
		return r.EventParticipation.Label
	}
	return nil
}

// forbiddenForLearning returns the first condition a learning area may not use ("" when none).
func (r Rule) forbiddenForLearning() string {
	if r.MinLevel != nil {
		return "min_level"
	}
	if r.EventParticipation != nil {
		return "event_participation"
	}
	for _, sub := range append(append([]Rule{}, r.All...), r.Any...) {
		if k := sub.forbiddenForLearning(); k != "" {
			return k
		}
	}
	return ""
}

// Evaluate reports whether the facts meet the rule and lists every leaf condition with its state (the missing ones
// have Met = false) — F071-AC-14.
func (r Rule) Evaluate(f Facts) (bool, []Requirement) {
	switch {
	case len(r.All) > 0:
		ok, out := true, []Requirement{}
		for _, sub := range r.All {
			m, reqs := sub.Evaluate(f)
			ok = ok && m
			out = append(out, reqs...)
		}
		return ok, out
	case len(r.Any) > 0:
		ok, out := false, []Requirement{}
		for _, sub := range r.Any {
			m, reqs := sub.Evaluate(f)
			ok = ok || m
			out = append(out, reqs...)
		}
		return ok, out
	}
	req := r.leaf(f)
	return req.Met, []Requirement{req}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r Rule) leaf(f Facts) Requirement {
	switch {
	case r.QuestCompleted != nil:
		met := f.Quests[r.QuestCompleted.QuestID]
		return Requirement{Kind: ReqQuest, Ref: r.QuestCompleted.QuestID, Label: r.QuestCompleted.Label, Met: met, Current: b2i(met), Target: 1}
	case r.ChaptersCompleted != nil:
		n := f.Chapters
		if r.ChaptersCompleted.SubjectChapters {
			n = f.SubjectChapters
		}
		return Requirement{Kind: ReqChapters, Label: r.ChaptersCompleted.Label, Met: n >= r.ChaptersCompleted.Min, Current: min(n, r.ChaptersCompleted.Min),
			Target: r.ChaptersCompleted.Min}
	case r.ObjectiveProgress != nil:
		c := r.ObjectiveProgress
		n := 0
		for _, id := range c.ObjectiveIDs {
			if lvl, ok := f.Mastery[id]; ok && masteryRank(lvl) >= masteryRank(c.MinLevel) {
				n++
			}
		}
		return Requirement{Kind: ReqObjective, Ref: c.MinLevel, Label: c.Label, Met: n >= c.MinCount, Current: min(n, c.MinCount), Target: c.MinCount}
	case r.MinLevel != nil:
		return Requirement{Kind: ReqLevel, Label: r.MinLevel.Label, Met: f.Level >= r.MinLevel.Level, Current: min(f.Level, r.MinLevel.Level),
			Target: r.MinLevel.Level}
	case r.AchievementUnlocked != nil:
		met := f.Achievements[r.AchievementUnlocked.AchievementID]
		return Requirement{Kind: ReqAchieve, Ref: r.AchievementUnlocked.AchievementID, Label: r.AchievementUnlocked.Label, Met: met, Current: b2i(met), Target: 1}
	case r.EventParticipation != nil:
		met := f.Events[r.EventParticipation.EventID]
		return Requirement{Kind: ReqEvent, Ref: r.EventParticipation.EventID, Label: r.EventParticipation.Label, Met: met, Current: b2i(met), Target: 1}
	}
	return Requirement{}
}

// Missing keeps the requirements not met (at most 10, UnlockRequirement maxItems).
func Missing(reqs []Requirement) []Requirement {
	out := []Requirement{}
	for _, r := range reqs {
		if !r.Met && len(out) < 10 {
			out = append(out, r)
		}
	}
	return out
}

// UnlockedBy is the kind of the first met requirement (world.area_unlocked.unlocked_by).
func UnlockedBy(reqs []Requirement) string {
	for _, r := range reqs {
		if r.Met {
			return r.Kind
		}
	}
	return ReqQuest
}
