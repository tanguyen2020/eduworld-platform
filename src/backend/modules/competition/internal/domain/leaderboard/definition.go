// Package leaderboard is the pure domain of the leaderboards (F106 US4, COMPETITION_SYSTEM §9): definitions
// (metric × scope × period × filter, no class — ADR-0095), the periods in Asia/Ho_Chi_Minh, the scope keys, the
// visibility per grade of the viewer (leaderboard_visibility), the collective goal of a grade (grade_goal) and the
// effort points. No SQL, no Valkey.
package leaderboard

import (
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
)

// Metrics (COMPETITION_SYSTEM §9.1). There is no Learning Score/Mastery metric and no class_goal (ADR-0095).
const (
	MetricCompetitionPoints = "competition_points"
	MetricEffortPoints      = "effort_points"
	MetricGradeGoal         = "grade_goal"
	MetricXP                = "xp"
	MetricQuestCompleted    = "quest_completed"
	MetricRating            = "rating"
	MetricTeamPoints        = "team_points"
)

// Scopes (§9.1): grade = one grade of one school (ADR-0095); there is no class scope.
const (
	ScopeGlobal     = "global"
	ScopeTenant     = "tenant"
	ScopeSchool     = "school"
	ScopeGrade      = "grade"
	ScopeFriends    = "friends"
	ScopeTeam       = "team"
	ScopeTournament = "tournament"
)

// Periods (§9.1).
const (
	PeriodDaily   = "daily"
	PeriodWeekly  = "weekly"
	PeriodMonthly = "monthly"
	PeriodSeason  = "season"
	PeriodAllTime = "all_time"
)

var (
	metrics = []string{MetricCompetitionPoints, MetricEffortPoints, MetricGradeGoal, MetricXP, MetricQuestCompleted, MetricRating,
		MetricTeamPoints}
	scopes  = []string{ScopeGlobal, ScopeTenant, ScopeSchool, ScopeGrade, ScopeFriends, ScopeTeam, ScopeTournament}
	periods = []string{PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodSeason, PeriodAllTime}
)

// ErrDefinition is a leaderboard definition the platform may not publish.
var ErrDefinition = errors.New("leaderboard: invalid definition")

// Filters narrow a board (§9.1): subject, topic, mode.
type Filters struct {
	SubjectID *uuid.UUID `json:"subject_id,omitempty"`
	TopicID   *uuid.UUID `json:"topic_id,omitempty"`
	Mode      *string    `json:"mode,omitempty"`
}

// Empty reports whether no filter is set.
func (f Filters) Empty() bool { return f.SubjectID == nil && f.TopicID == nil && f.Mode == nil }

// Definition is a published LeaderboardDefinition.
type Definition struct {
	ID        uuid.UUID
	Code      string
	Title     string
	Metric    string
	Scope     string
	Period    string
	Filters   Filters
	Grades    []int
	SortOrder int
	Status    string
	// GradeGoalTarget is display.grade_goal_target: the weekly goal of a grade_goal board (never hardcoded).
	GradeGoalTarget float64
}

// Active reports whether the platform serves the definition.
func (d Definition) Active() bool { return d.Status == "active" }

// ForGrade reports whether students of grade see the board.
func (d Definition) ForGrade(grade int) bool { return slices.Contains(d.Grades, grade) }

// Ranked reports whether the board ranks students (every metric but grade_goal).
func (d Definition) Ranked() bool { return d.Metric != MetricGradeGoal }

// SchoolScoped reports whether the board lives inside one school (school, grade): the boards a school may turn off
// (disabled_leaderboard_definitions) and whose snapshot keeps every member's rank.
func (d Definition) SchoolScoped() bool { return d.Scope == ScopeSchool || d.Scope == ScopeGrade }

// CrossSchool reports whether the board lists students of other schools (tenant, global): only students whose school
// allows it (allow_cross_school_listing) appear (F106-AC-18).
func (d Definition) CrossSchool() bool { return d.Scope == ScopeTenant || d.Scope == ScopeGlobal }

// Validate checks the invariants of a definition (F106-AC-12, AC-16): known metric/scope/period, never class, no XP in a
// school or grade scope (XP only with friends), grade_goal only for one grade of a school, weekly, with a target,
// all_time only for xp/quest_completed, filters only where the ledger carries them (competition_points, rating), rating only for grades 6–9.
func (d Definition) Validate() error {
	bad := func(what string) error { return fmt.Errorf("%w: %s", ErrDefinition, what) }
	switch {
	case d.Scope == "class" || d.Metric == "class_goal":
		return bad("no class leaderboard (ADR-0095)")
	case !slices.Contains(metrics, d.Metric):
		return bad("metric")
	case !slices.Contains(scopes, d.Scope):
		return bad("scope_type")
	case !slices.Contains(periods, d.Period):
		return bad("period")
	case d.Code == "" || len(d.Code) > 64 || d.Title == "" || len([]rune(d.Title)) > 128:
		return bad("code/title")
	case len(d.Grades) == 0:
		return bad("grade_levels")
	case d.Metric == MetricXP && d.Scope != ScopeFriends:
		return bad("xp only in a friends scope (no XP ranking inside a school or grade)")
	case d.Metric == MetricGradeGoal && (d.Scope != ScopeGrade || d.Period != PeriodWeekly || d.GradeGoalTarget <= 0):
		return bad("grade_goal: scope grade, weekly, display.grade_goal_target > 0")
	case d.Period == PeriodAllTime && d.Metric != MetricXP && d.Metric != MetricQuestCompleted:
		return bad("all_time only for xp and quest_completed")
	}
	seen := map[int]bool{}
	for _, g := range d.Grades {
		if g < 1 || g > 9 || seen[g] {
			return bad("grade_levels within 1–9, unique")
		}
		seen[g] = true
		if d.Metric == MetricRating && g < 6 {
			return bad("rating boards are for grades 6–9")
		}
	}
	if !d.Filters.Empty() {
		switch d.Metric {
		case MetricCompetitionPoints, MetricRating:
		default:
			return bad("filters only for competition_points and rating (the other ledgers carry no subject/mode)")
		}
		if d.Filters.TopicID != nil && d.Metric != MetricCompetitionPoints {
			return bad("topic filter only for competition_points")
		}
		if d.Filters.Mode != nil && (*d.Filters.Mode == "" || len(*d.Filters.Mode) > 64) {
			return bad("mode filter")
		}
	}
	if d.Metric == MetricRating && (d.Filters.Mode == nil || d.Filters.SubjectID == nil) {
		return bad("a rating board names its mode and subject")
	}
	return nil
}
