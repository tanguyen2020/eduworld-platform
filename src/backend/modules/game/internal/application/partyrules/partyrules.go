// Package partyrules holds the application rules the boss and co-op quest commands and queries share (F103): the
// eligibility of a party member for a boss or a party quest (reason codes of the facade, never the grade or school of
// the student — R16), the boss encounter views and realtime payloads (no correct/wrong of another member, F103-AC-09),
// and the idempotency key of a question plan.
package partyrules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/questrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// Reason codes of a member not eligible (game.internal.v1.yaml PartyActivityMemberCheck.reason_code).
const (
	ReasonGrade        = "grade_not_allowed"
	ReasonPrerequisite = "prerequisite_missing"
	ReasonArea         = "area_locked"
	ReasonAttempts     = "attempt_limit"
)

// Check is the eligibility of one member: the requirement is the requirement of the activity, never data of the student.
type Check struct {
	StudentID   uuid.UUID
	Eligible    bool
	Reason      string
	Requirement string
}

// Activity is a party activity candidate with the checks of its members.
type Activity struct {
	Type          string // quest, boss
	Ref           string // code
	Title         map[string]string
	AreaID        string
	MinMembers    int
	MaxMembers    int
	AllowContinue bool
	Checks        []Check
}

// AllEligible reports every member eligible.
func (a Activity) AllEligible() bool {
	for _, c := range a.Checks {
		if !c.Eligible {
			return false
		}
	}
	return len(a.Checks) > 0
}

func grades(gs []int) string {
	parts := make([]string, len(gs))
	for i, g := range gs {
		parts[i] = fmt.Sprint(g)
	}
	return "grade_levels:" + strings.Join(parts, ",")
}

// Member is what the checks read of a member: its projection and the server facts of the unlock rules.
type Member struct {
	ID      uuid.UUID
	Student *port.WorldStudent
	Facts   quest.Facts
	Hist    map[uuid.UUID]*port.QuestHistory
}

// Members reads the projections and facts of the members in tenant; nil projection = not a student of tenant.
func Members(ctx context.Context, w port.WorldContent, q port.QuestContent, tenant uuid.UUID, ids []uuid.UUID) ([]Member, error) {
	out := make([]Member, 0, len(ids))
	for _, id := range ids {
		st, err := w.Student(ctx, tenant, id)
		if err != nil {
			return nil, err
		}
		m := Member{ID: id, Student: st}
		if st != nil && !st.Deleted && st.Status == "active" {
			if m.Hist, err = q.Histories(ctx, tenant, id); err != nil {
				return nil, err
			}
			if m.Facts, err = questrules.Facts(ctx, w, q, tenant, id, st, questrules.CompletedOf(m.Hist)); err != nil {
				return nil, err
			}
		}
		out = append(out, m)
	}
	return out, nil
}

// Valid reports a projected active student of the tenant.
func (m Member) Valid() bool {
	return m.Student != nil && !m.Student.Deleted && m.Student.Status == "active"
}

// BossCheck is the eligibility of a member for a boss (F103-AC-01, AC-02): grade in grade_levels, area of the boss
// open, encounters of the day below challenge_attempts_per_day.
func BossCheck(v boss.Version, m Member, usage port.BossUsage, perDay int) Check {
	c := Check{StudentID: m.ID, Eligible: true}
	switch {
	case !m.Valid() || !v.AdmitsGrade(m.Student.Grade):
		c.Eligible, c.Reason, c.Requirement = false, ReasonGrade, grades(v.GradeLevels)
	case v.AreaID != "" && !m.Facts.OpenAreas[v.AreaID]:
		c.Eligible, c.Reason, c.Requirement = false, ReasonArea, "area:"+v.AreaID
	case perDay > 0 && usage.Encounters >= perDay:
		c.Eligible, c.Reason, c.Requirement = false, ReasonAttempts, fmt.Sprintf("attempts_per_day:%d", perDay)
	}
	return c
}

// QuestCheck is the eligibility of a member for a party quest (F103-AC-02, AC-13): grade, prerequisites and learning
// requirements of the quest (quest.Decide on the server facts), repeatability.
func QuestCheck(v quest.Version, m Member, reset quest.Reset, questID uuid.UUID, at time.Time) Check {
	c := Check{StudentID: m.ID, Eligible: true}
	if !m.Valid() || !v.AdmitsGrade(m.Student.Grade) {
		c.Eligible, c.Reason, c.Requirement = false, ReasonGrade, grades(v.GradeLevels)
		return c
	}
	d := quest.Decide(v, questrules.Student(m.Student), m.Facts, at)
	if !d.Visible || !d.Met {
		c.Eligible, c.Reason = false, ReasonPrerequisite
		if miss := d.Missing(); len(miss) > 0 {
			c.Requirement = truncate(miss[0].Kind+":"+miss[0].Ref, 255)
		}
		return c
	}
	if r, _ := quest.CanRepeat(v, questrules.History(m.Hist[questID], v, reset, at), at); r != "" {
		c.Eligible, c.Reason, c.Requirement = false, ReasonAttempts, "repeatability:"+v.RepeatMode()
	}
	return c
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// BossPeriod is the period of the boss encounters of a day (quest_reset_time, like the solo challenges).
func BossPeriod(reset quest.Reset, at time.Time) string {
	return reset.PeriodKey(quest.RecurrenceDaily, at)
}

// planNS is the UUIDv5 namespace of the question plan keys of the boss rounds.
var planNS = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-00000000f103")

// PlanKey is the idempotency key of the plan of one member, requirement and round (match_id of createQuestionPlan).
func PlanKey(encounter uuid.UUID, phaseID string, retry int, student uuid.UUID, requirement string) uuid.UUID {
	return uuid.NewSHA1(planNS, []byte(fmt.Sprintf("%s|%s|%d|%s|%s", encounter, phaseID, retry, student, requirement)))
}

// QuestionInstance is the question_instance_id of a plan item: "<plan_id>:<index>".
func QuestionInstance(plan uuid.UUID, index int) string { return fmt.Sprintf("%s:%d", plan, index) }

// ParseInstance reads a question_instance_id.
func ParseInstance(s string) (uuid.UUID, int, bool) {
	id, idx, ok := strings.Cut(s, ":")
	if !ok {
		return uuid.Nil, 0, false
	}
	plan, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, 0, false
	}
	var n int
	if _, err := fmt.Sscanf(idx, "%d", &n); err != nil || n < 0 || n > 29 || fmt.Sprint(n) != idx {
		return uuid.Nil, 0, false
	}
	return plan, n, true
}
