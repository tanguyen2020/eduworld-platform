// Package questrules holds the application rules the quest commands and queries share (F072): the server facts of
// the availability of a quest and the repeatability history of a student.
package questrules

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
)

// Facts reads the server facts the availability of the quests reads (completed quests, open areas, level,
// mastery, discoveries) for a student; shared by the use cases and the read model. completed holds the quests the
// student completed at least once (CompletedOf of the histories, or the read model).
func Facts(ctx context.Context, w port.WorldContent, c port.QuestContent, tenant, student uuid.UUID, st *port.WorldStudent,
	completed map[uuid.UUID]bool) (quest.Facts, error) {
	f := quest.Facts{CompletedQuests: map[string]bool{}, OpenAreas: map[string]bool{}, Mastery: map[string]string{}}
	chapters, err := w.CurrentChapters(ctx)
	if err != nil {
		return f, err
	}
	pr, err := w.Progress(ctx, tenant, student, chapters)
	if err != nil {
		return f, err
	}
	for q := range pr.Facts.Quests {
		f.CompletedQuests[q] = true
	}
	for id, ok := range completed {
		if ok {
			f.CompletedQuests[id.String()] = true
		}
	}
	areas, err := w.CurrentAreas(ctx)
	if err != nil {
		return f, err
	}
	for _, a := range areas {
		_, unlocked := pr.Unlocks[a.Area.AreaID]
		if world.Decide(a.Area, st.Domain(), unlocked, pr.Facts).Accessible {
			f.OpenAreas[a.Area.AreaID] = true
		}
	}
	f.Level, f.Mastery = pr.Facts.Level, pr.Facts.Mastery
	if f.Mastery == nil {
		f.Mastery = map[string]string{}
	}
	if f.Discovered, err = c.Discovered(ctx, tenant, student); err != nil {
		return f, err
	}
	return f, nil
}

// CompletedOf returns the quests of the histories completed at least once.
func CompletedOf(hist map[uuid.UUID]*port.QuestHistory) map[uuid.UUID]bool {
	out := make(map[uuid.UUID]bool, len(hist))
	for id, hs := range hist {
		if hs.Completed > 0 {
			out[id] = true
		}
	}
	return out
}

// Student maps the projected student to the domain.
func Student(st *port.WorldStudent) quest.Student {
	return quest.Student{Grade: st.Grade, EnabledSubjects: st.EnabledSubjects}
}

// History is the repeatability history of a quest in the current period of v.
func History(hs *port.QuestHistory, v quest.Version, reset quest.Reset, now time.Time) quest.History {
	if hs == nil {
		return quest.History{}
	}
	period := reset.PeriodKey(v.Recurrence(), now)
	return quest.History{Completed: hs.Completed, LastCompletedAt: hs.LastCompletedAt, CompletedInPeriod: period != "" && hs.CompletedPeriods[period]}
}

// StepStateView is QuestStepState.
type StepStateView struct {
	Status       string `json:"status"`
	Count        *int   `json:"count,omitempty"`
	Target       *int   `json:"target,omitempty"`
	AttemptCount *int   `json:"attempt_count,omitempty"`
}

// ProgressView is QuestProgress.
type ProgressView struct {
	QuestProgressID uuid.UUID                `json:"quest_progress_id"`
	QuestID         uuid.UUID                `json:"quest_id"`
	QuestVersionID  uuid.UUID                `json:"quest_version_id"`
	Status          string                   `json:"status"`
	StepStates      map[string]StepStateView `json:"step_states"`
	StartTrigger    string                   `json:"start_trigger"`
	PeriodKey       string                   `json:"period_key,omitempty"`
	CompletionNo    int                      `json:"completion_no"`
	PartyID         *uuid.UUID               `json:"party_id,omitempty"`
	StartedAt       time.Time                `json:"started_at"`
	CompletedAt     *time.Time               `json:"completed_at,omitempty"`
	ExpiresAt       *time.Time               `json:"expires_at,omitempty"`
	DeadlineAt      *time.Time               `json:"deadline_at,omitempty"`
	FailedAt        *time.Time               `json:"failed_at,omitempty"`
}

// ViewOf returns the QuestProgress of p on its pinned version v.
func ViewOf(p *quest.Progress, v quest.Version) ProgressView {
	out := ProgressView{QuestProgressID: p.ID, QuestID: p.QuestID, QuestVersionID: p.VersionID, Status: p.Status, StepStates: map[string]StepStateView{},
		StartTrigger: p.StartTrigger, PeriodKey: p.PeriodKey, CompletionNo: p.CompletionNo, PartyID: p.PartyID, StartedAt: p.StartedAt,
		CompletedAt: p.CompletedAt, ExpiresAt: p.ExpiresAt, DeadlineAt: p.DeadlineAt, FailedAt: p.FailedAt}
	for _, s := range v.Steps {
		st := p.Steps[s.ID]
		sv := StepStateView{Status: st.Status}
		if sv.Status == "" {
			sv.Status = quest.StepLocked
		}
		switch s.Type {
		case quest.StepCollect, quest.StepDiscover:
			c, t := st.Count, s.Target()
			sv.Count, sv.Target = &c, &t
		case quest.StepActivity, quest.StepMinigame:
			n := len(st.AttemptIDs)
			sv.AttemptCount = &n
		}
		out.StepStates[s.ID] = sv
	}
	return out
}
