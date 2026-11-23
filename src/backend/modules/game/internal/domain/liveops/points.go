package liveops

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// Source codes of event and season points (EVENT_SEASON_SYSTEM §7.2): verified server events of completion only — never a
// login, a session, online time, a check-in or a streak (R19, NFR-SAFE-003).
const (
	SrcQuestCompleted   = "quest.completed"
	SrcDailyQuest       = "daily_quest.completed"
	SrcWeeklyQuest      = "weekly_quest.completed"
	SrcLearningActivity = "learning_activity.completed"
	SrcEventActivity    = "event_activity.completed"
	SrcEventQuest       = "event_quest.completed"
	SrcEventBossPhase   = "event_boss.phase_cleared" // boss phases of an event (producer F103; never earned before it)
	SrcDiscovery        = "discovery.found"
	SrcCoop             = "coop.completed"
	SrcMatch            = "match.completed"
)

// SourceCodes is the catalog of point sources.
var SourceCodes = []string{SrcQuestCompleted, SrcDailyQuest, SrcWeeklyQuest, SrcLearningActivity, SrcEventActivity, SrcEventQuest,
	SrcEventBossPhase, SrcDiscovery, SrcCoop, SrcMatch}

// Fact is a verified server event as live-ops sees it for one student (never a client claim, R5).
type Fact struct {
	EventType    string
	EventID      uuid.UUID
	At           time.Time
	StudentID    uuid.UUID
	QuestID      *uuid.UUID
	QuestType    string
	Recurrence   string // daily, weekly of a periodic quest
	Correct      int    // correct answers of a graded activity (first try when known)
	RapidGuess   bool   // the attempt is flagged suspicious_speed (rapid_guess_flagged): no contribution
	SubjectID    *uuid.UUID
	ObjectiveIDs []uuid.UUID
}

// SourceCodes returns the point sources the fact earns; eventQuest reports a quest that is a component of the run being
// credited (event_activity.completed).
func (f Fact) SourceCodes(eventQuest func(uuid.UUID) bool) []string {
	var out []string
	switch f.EventType {
	case "quest.completed":
		out = append(out, SrcQuestCompleted)
		if f.QuestType == "daily" || f.Recurrence == "daily" {
			out = append(out, SrcDailyQuest)
		}
		if f.QuestType == "weekly" || f.Recurrence == "weekly" {
			out = append(out, SrcWeeklyQuest)
		}
		if f.QuestID != nil && eventQuest != nil && eventQuest(*f.QuestID) {
			out = append(out, SrcEventQuest, SrcEventActivity)
		}
	case "activity.graded":
		if !f.RapidGuess {
			out = append(out, SrcLearningActivity)
		}
	case "world.discovery_found":
		out = append(out, SrcDiscovery)
	case "party.activity_completed":
		out = append(out, SrcCoop)
	case "match.completed":
		out = append(out, SrcMatch)
	}
	return out
}

// PointsFor returns the best source of codes among sources (one credit per source event, the highest points).
func PointsFor(sources []Source, codes []string) (string, int) {
	best, pts := "", 0
	for _, s := range sources {
		if slices.Contains(codes, s.SourceCode) && s.Points > pts {
			best, pts = s.SourceCode, s.Points
		}
	}
	return best, pts
}

// Earn returns what a credit of points adds today under the daily cap: the credit and whether the cap cut it.
func Earn(points, earnedToday, dailyCap int) (int, bool) {
	if dailyCap <= 0 {
		return points, false
	}
	left := dailyCap - earnedToday
	if left <= 0 {
		return 0, points > 0
	}
	if points > left {
		return left, true
	}
	return points, false
}

// Reached returns the milestones of ms whose threshold points reaches, merged into before (never lowered: a milestone
// reached under an older version stays reached, F077-AC-11).
func Reached(ms []Milestone, points int, before []string) []string {
	out := append([]string{}, before...)
	for _, m := range ms {
		if points >= m.EventPoints && !slices.Contains(out, m.MilestoneID) {
			out = append(out, m.MilestoneID)
		}
	}
	return out
}

// Unclaimed returns the reached milestones neither claimed nor granted.
func Unclaimed(reached, claimed, granted []string) []string {
	var out []string
	for _, id := range reached {
		if !slices.Contains(claimed, id) && !slices.Contains(granted, id) {
			out = append(out, id)
		}
	}
	return out
}

// MetricAmount returns what a fact contributes to a goal metric (0 = nothing): one per completed quest, activity,
// discovery or co-op session, the correct answers of a graded activity. A graded activity flagged suspicious_speed never
// contributes (F077-AC-13); a filter on subjects or objectives must match.
func (f Fact) MetricAmount(metric string, filter *MetricFilter) int {
	if filter != nil {
		if len(filter.SubjectIDs) > 0 && (f.SubjectID == nil || !slices.Contains(filter.SubjectIDs, *f.SubjectID)) {
			return 0
		}
		if len(filter.ObjectiveIDs) > 0 && !anyIn(f.ObjectiveIDs, filter.ObjectiveIDs) {
			return 0
		}
	}
	switch metric {
	case "correct_answers":
		if f.EventType == "activity.graded" && !f.RapidGuess {
			return f.Correct
		}
	case "activities_completed":
		if f.EventType == "activity.graded" && !f.RapidGuess {
			return 1
		}
	case "quests_completed":
		if f.EventType == "quest.completed" {
			return 1
		}
	case "discoveries_found":
		if f.EventType == "world.discovery_found" {
			return 1
		}
	case "coop_sessions":
		if f.EventType == "party.activity_completed" {
			return 1
		}
	}
	return 0
}

func anyIn(a, b []uuid.UUID) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

// Threshold is the total of a goal milestone (ceil(target × percent / 100)).
func Threshold(target int64, percent int) int64 {
	return (target*int64(percent) + 99) / 100
}

// Crossed returns the milestone percents a total reaches that reached does not hold yet (each milestone once).
func Crossed(target, total int64, percents, reached []int) []int {
	var out []int
	for _, p := range percents {
		if total >= Threshold(target, p) && !slices.Contains(reached, p) {
			out = append(out, p)
		}
	}
	return out
}

// GradeRow is the contribution of one grade of a school.
type GradeRow struct {
	Grade        int
	Progress     int64
	Contributors int
}

// VisibleGrades keeps the grades shown to students and school staff (EVENT_SEASON_SYSTEM §6, ADR-0095): grades 4–9 only,
// a grade with fewer than minCell contributing students is hidden (no row) — no name, no ranking.
func VisibleGrades(rows []GradeRow, minCell int) []GradeRow {
	var out []GradeRow
	for _, r := range rows {
		if r.Grade >= 4 && r.Grade <= 9 && r.Contributors >= minCell {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b GradeRow) int { return a.Grade - b.Grade })
	return out
}
