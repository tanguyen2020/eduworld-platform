package quest

import (
	"fmt"
	"hash/fnv"
	"slices"
	"time"
)

// DefaultPassRule is quest_pass_rule of CONFIG_DEFAULTS: what a learning step without its own value uses.
type DefaultPassRule struct {
	MinCorrectRatio   float64 // 0.7
	MinQuestions      int     // 8
	MaxAttemptsPerDay int     // 5
}

// EffectivePassRule is a pass rule with its defaults filled in.
type EffectivePassRule struct {
	MinCorrectRatio   float64
	MinQuestions      int
	MaxAttemptsPerDay int
}

// Effective fills the missing values of r with d.
func (d DefaultPassRule) Effective(r *PassRule) EffectivePassRule {
	out := EffectivePassRule(d)
	if r == nil {
		return out
	}
	if r.MinCorrectRatio != nil {
		out.MinCorrectRatio = *r.MinCorrectRatio
	}
	if r.MinQuestions != nil {
		out.MinQuestions = *r.MinQuestions
	}
	if r.MaxAttemptsPerDay != nil {
		out.MaxAttemptsPerDay = *r.MaxAttemptsPerDay
	}
	return out
}

// Graded is the server result of an attempt (activity.graded of module learning).
type Graded struct {
	CorrectCount *int
	ItemCount    *int
	Passed       *bool
}

// Passes is PassRule.Evaluate (F072-AC-15): correct_count / item_count ≥ min_correct_ratio ∧ item_count ≥
// min_questions. An activity without items (video, simulation) passes only when the server graded it passed.
func (r EffectivePassRule) Passes(g Graded) bool {
	if g.ItemCount == nil || *g.ItemCount == 0 || g.CorrectCount == nil {
		return g.Passed != nil && *g.Passed
	}
	items, correct := *g.ItemCount, *g.CorrectCount
	return items >= r.MinQuestions && float64(correct)/float64(items) >= r.MinCorrectRatio
}

// Reset is quest_reset_time: the daily reset (04:00 ICT) and the weekly reset (Monday at the same time).
type Reset struct {
	Hour, Minute int
	Location     *time.Location
}

// local is t in the reset time zone shifted by the reset time: the calendar day of the shifted time is the period day.
func (r Reset) shifted(t time.Time) time.Time {
	loc := r.Location
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Add(-time.Duration(r.Hour)*time.Hour - time.Duration(r.Minute)*time.Minute)
}

// PeriodKey is the key of the period of recurrence at t: YYYY-MM-DD (daily) or YYYY-Www (ISO week, weekly); "" for none.
func (r Reset) PeriodKey(recurrence string, t time.Time) string {
	s := r.shifted(t)
	switch recurrence {
	case RecurrenceDaily:
		return s.Format("2006-01-02")
	case RecurrenceWeekly:
		y, w := s.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	}
	return ""
}

// PeriodEnd is the instant the period of recurrence containing t ends (the next reset); nil for none.
func (r Reset) PeriodEnd(recurrence string, t time.Time) *time.Time {
	s := r.shifted(t)
	day := time.Date(s.Year(), s.Month(), s.Day(), 0, 0, 0, 0, s.Location())
	var end time.Time
	switch recurrence {
	case RecurrenceDaily:
		end = day.AddDate(0, 0, 1)
	case RecurrenceWeekly:
		wd := (int(day.Weekday()) + 6) % 7 // Monday = 0
		end = day.AddDate(0, 0, 7-wd)
	default:
		return nil
	}
	end = end.Add(time.Duration(r.Hour)*time.Hour + time.Duration(r.Minute)*time.Minute).UTC()
	return &end
}

// GradeBand returns the band of a grade (g1_3, g4_5, g6_9).
func GradeBand(grade int) string {
	switch {
	case grade >= 1 && grade <= 3:
		return "g1_3"
	case grade >= 4 && grade <= 5:
		return "g4_5"
	case grade >= 6 && grade <= 9:
		return "g6_9"
	}
	return ""
}

// Candidate is a quest the periodic grant may pick.
type Candidate struct {
	QuestID      string
	ObjectiveIDs []string
}

// SelectPeriodic picks n candidates for a student and a period (F072-AC-22, QUEST_SYSTEM §2.4): the quests on the weak
// objectives of the student first (mastery beginning/developing), then a stable per-student, per-period order (no
// randomness that a replay would change).
func SelectPeriodic(cands []Candidate, mastery map[string]string, n int, seed string) []string {
	type scored struct {
		id   string
		weak int
		h    uint64
	}
	list := make([]scored, 0, len(cands))
	for _, c := range cands {
		w := 0
		for _, o := range c.ObjectiveIDs {
			if Weak(mastery[o]) {
				w++
			}
		}
		f := fnv.New64a()
		_, _ = f.Write([]byte(seed + ":" + c.QuestID))
		list = append(list, scored{id: c.QuestID, weak: w, h: f.Sum64()})
	}
	slices.SortFunc(list, func(a, b scored) int {
		switch {
		case a.weak != b.weak:
			return b.weak - a.weak
		case a.h < b.h:
			return -1
		case a.h > b.h:
			return 1
		}
		return 0
	})
	out := make([]string, 0, n)
	for _, s := range list {
		if len(out) == n {
			break
		}
		out = append(out, s.id)
	}
	return out
}
