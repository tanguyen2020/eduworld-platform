// Package challengerules holds the application rules the challenge commands and queries share (F076): the server facts
// a challenge unlock rule reads, the period of an attempt and the GameChallengeAttempt view.
package challengerules

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/questrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// Student reads what eligibility needs: the grade of the projection and, when withFacts, the server facts of the unlock
// rules (level, completed quests, open areas — the same facts as the quest availability, F072).
func Student(ctx context.Context, w port.WorldContent, q port.QuestContent, tenant, student uuid.UUID, st *port.WorldStudent,
	withFacts bool) (challenge.Student, error) {
	out := challenge.Student{Grade: st.Grade}
	if !withFacts {
		return out, nil
	}
	hist, err := q.Histories(ctx, tenant, student)
	if err != nil {
		return out, err
	}
	f, err := questrules.Facts(ctx, w, q, tenant, student, st, hist)
	if err != nil {
		return out, err
	}
	out.Level, out.CompletedQuests, out.OpenAreas = f.Level, f.CompletedQuests, f.OpenAreas
	return out, nil
}

// PeriodKey is the period an attempt counts in: the day (quest_reset_time, ICT) — the ISO week for weekly, the season
// window (S + its start day, ICT) for seasonal. A run crossing the reset counts in the period it started in.
func PeriodKey(v challenge.Version, reset quest.Reset, at time.Time) string {
	switch v.Type {
	case challenge.TypeWeekly:
		return reset.PeriodKey(quest.RecurrenceWeekly, at)
	case challenge.TypeSeasonal:
		if from, _ := v.Window(); from != nil {
			loc := reset.Location
			if loc == nil {
				loc = time.UTC
			}
			return "S" + from.In(loc).Format("2006-01-02")
		}
	}
	return reset.PeriodKey(quest.RecurrenceDaily, at)
}

// PeriodEnd is when the attempts of the period come back (the next reset; the end of the season window).
func PeriodEnd(v challenge.Version, reset quest.Reset, at time.Time) *time.Time {
	switch v.Type {
	case challenge.TypeWeekly:
		return reset.PeriodEnd(quest.RecurrenceWeekly, at)
	case challenge.TypeSeasonal:
		_, to := v.Window()
		return to
	}
	return reset.PeriodEnd(quest.RecurrenceDaily, at)
}

// AttemptView is GameChallengeAttempt (score, lives and deadline from the checkpoint row: resume on any device,
// F076-AC-15). The answers, the scoring rule and the question bank never appear (R6).
type AttemptView struct {
	ChallengeAttemptID  uuid.UUID  `json:"challenge_attempt_id"`
	ChallengeID         uuid.UUID  `json:"challenge_id"`
	ChallengeVersion    int        `json:"challenge_version"`
	Difficulty          string     `json:"difficulty"`
	Status              string     `json:"status"`
	Score               int        `json:"score"`
	LivesLeft           *int       `json:"lives_left,omitempty"`
	ActivitiesCompleted int        `json:"activities_completed"`
	StartedAt           time.Time  `json:"started_at"`
	DeadlineAt          *time.Time `json:"deadline_at,omitempty"`
	EndedAt             *time.Time `json:"ended_at,omitempty"`
}

// ViewOf returns the view of a on its pinned version number.
func ViewOf(a *challenge.Attempt, versionNo int) AttemptView {
	return AttemptView{ChallengeAttemptID: a.ID, ChallengeID: a.ChallengeID, ChallengeVersion: max(versionNo, 1), Difficulty: a.Difficulty,
		Status: a.Status, Score: a.Score, LivesLeft: a.LivesLeft, ActivitiesCompleted: a.ActivitiesCompleted, StartedAt: a.StartedAt.UTC(),
		DeadlineAt: a.DeadlineAt, EndedAt: a.EndedAt}
}

// Push is the payload of challenge.attempt.updated (AsyncAPI ChallengeAttemptUpdated).
func Push(a *challenge.Attempt) map[string]any {
	m := map[string]any{"challenge_attempt_id": a.ID, "status": a.Status, "score": a.Score, "activities_completed": a.ActivitiesCompleted}
	if a.LivesLeft != nil {
		m["lives_left"] = *a.LivesLeft
	}
	if a.DeadlineAt != nil {
		m["deadline_at"] = a.DeadlineAt.UTC()
	}
	return m
}
