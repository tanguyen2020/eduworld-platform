package quest

import (
	"slices"
	"time"
)

// Student is what the availability of a quest reads about a student (projection ref.students + school settings).
type Student struct {
	Grade           int
	EnabledSubjects []string // nil = every subject on
}

// SubjectEnabled reports a subject turned on by the school (nil list = every subject).
func (s Student) SubjectEnabled(code string) bool {
	return code == "" || s.EnabledSubjects == nil || slices.Contains(s.EnabledSubjects, code)
}

// Facts are the server facts the prerequisites read (never the client, ADR-0016).
type Facts struct {
	CompletedQuests map[string]bool   // quest ids completed by the student
	OpenAreas       map[string]bool   // areas the student may enter (learning areas of its grade, unlocked areas)
	Level           int               // highest level reached (F074; 0 before)
	Mastery         map[string]string // objective id → mastery level (ref.mastery_summary)
	Discovered      map[string]bool   // interactables discovered (exploration_progress)
}

// Requirement kinds of QUEST_PREREQUISITE_NOT_MET.
const (
	ReqCompletedQuest    = "completed_quest"
	ReqUnlockedArea      = "unlocked_area"
	ReqMinLevel          = "min_level"
	ReqLearningObjective = "learning_objective"
)

// Requirement is one start condition and whether the student meets it.
type Requirement struct {
	Kind            string `json:"kind"`
	Ref             string `json:"ref,omitempty"`
	Met             bool   `json:"met"`
	Current         int    `json:"current,omitempty"`
	Target          int    `json:"target,omitempty"`
	MinMasteryLevel string `json:"min_mastery_level,omitempty"`
}

// Availability is the decision for one quest and student.
type AvailabilityDecision struct {
	Visible      bool          // assigned quest the student may see (grade, subject, window, trigger)
	Requirements []Requirement // prerequisites and learning requirements
	Met          bool          // every requirement met
}

// Missing returns the requirements not met.
func (a AvailabilityDecision) Missing() []Requirement {
	var out []Requirement
	for _, r := range a.Requirements {
		if !r.Met {
			out = append(out, r)
		}
	}
	return out
}

var masteryOrder = []string{"not_started", "beginning", "developing", "proficient", "mastered"}

func masteryRank(level string) int { return slices.Index(masteryOrder, level) }

// Weak reports a mastery level the daily/weekly selection favours (beginning, developing).
func Weak(level string) bool { return level == "beginning" || level == "developing" }

// Decide is QuestAvailability.Check (F072-AC-01, AC-05; QUEST_SYSTEM §4): the tenant assignment is checked by the
// caller (an unassigned quest is never read). Visible = grade ∈ grade_levels ∧ subject on ∧ within availability ∧ the
// trigger can be accepted by the player (a discovery quest once one of its points is found); the requirements are the
// prerequisites (completed quests, open areas, min_level — never for a mandatory learning quest) and the learning
// requirements (mastery level of an objective).
func Decide(v Version, s Student, f Facts, now time.Time) AvailabilityDecision {
	subjectRequired := v.Prerequisites == nil || v.Prerequisites.RequiredSubjectEnabled == nil || *v.Prerequisites.RequiredSubjectEnabled
	if !v.AdmitsGrade(s.Grade) || (subjectRequired && !s.SubjectEnabled(v.SubjectCode)) || !v.InWindow(now) {
		return AvailabilityDecision{}
	}
	if v.StartTrigger.Kind == TriggerDiscovery {
		found := false
		for _, id := range v.StartTrigger.InteractableIDs {
			found = found || f.Discovered[id]
		}
		if !found {
			return AvailabilityDecision{}
		}
	}
	d := AvailabilityDecision{Visible: true, Met: true}
	add := func(r Requirement) {
		d.Requirements = append(d.Requirements, r)
		d.Met = d.Met && r.Met
	}
	if pr := v.Prerequisites; pr != nil {
		for _, q := range pr.CompletedQuestIDs {
			add(Requirement{Kind: ReqCompletedQuest, Ref: q, Met: f.CompletedQuests[q]})
		}
		for _, a := range pr.UnlockedAreaIDs {
			add(Requirement{Kind: ReqUnlockedArea, Ref: a, Met: f.OpenAreas[a]})
		}
		if pr.MinLevel > 0 && !v.MandatoryLearning() {
			add(Requirement{Kind: ReqMinLevel, Met: f.Level >= pr.MinLevel, Current: f.Level, Target: pr.MinLevel})
		}
	}
	for _, lr := range v.LearningRequirements {
		lvl, ok := f.Mastery[lr.ObjectiveID]
		add(Requirement{Kind: ReqLearningObjective, Ref: lr.ObjectiveID, MinMasteryLevel: lr.MinMasteryLevel,
			Met: ok && masteryRank(lvl) >= masteryRank(lr.MinMasteryLevel)})
	}
	return d
}

// History is the past progress of a student on a quest (every version).
type History struct {
	Completed         int        // progress completed
	LastCompletedAt   *time.Time // the latest completion
	CompletedInPeriod bool       // a progress of the current period is completed (per_period)
}

// Repeat decisions of acceptQuest (F072-AC-25).
const (
	RepeatOK               = ""
	RepeatAlreadyCompleted = "already_completed"
	RepeatCooldown         = "cooldown"
)

// CanRepeat applies repeatability to the history: once → one completion; per_period → one completion per period;
// repeatable → after cooldown_hours, at most max_completions. retryAt is the end of the cooldown.
func CanRepeat(v Version, h History, now time.Time) (string, *time.Time) {
	switch v.RepeatMode() {
	case RepeatPerPeriod:
		if h.CompletedInPeriod {
			return RepeatAlreadyCompleted, nil
		}
	case RepeatRepeatable:
		r := v.Repeatability
		if r.MaxCompletions > 0 && h.Completed >= r.MaxCompletions {
			return RepeatAlreadyCompleted, nil
		}
		if h.LastCompletedAt != nil && r.CooldownHours > 0 {
			at := h.LastCompletedAt.Add(time.Duration(r.CooldownHours) * time.Hour)
			if now.Before(at) {
				return RepeatCooldown, &at
			}
		}
	default:
		if h.Completed > 0 {
			return RepeatAlreadyCompleted, nil
		}
	}
	return RepeatOK, nil
}
