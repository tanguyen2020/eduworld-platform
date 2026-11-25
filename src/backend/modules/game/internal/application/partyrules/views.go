package partyrules

import (
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
)

// EncounterMember is one member of BossEncounter: nickname only (R16) and contribution_met only (F103-AC-09).
type EncounterMember struct {
	StudentID       uuid.UUID `json:"student_id"`
	Nickname        string    `json:"nickname"`
	Ready           bool      `json:"ready"`
	Present         bool      `json:"present"`
	Connected       bool      `json:"connected"`
	ContributionMet bool      `json:"contribution_met"`
}

// Question is BossQuestion: render payload only, answered flag of the caller (ADR-0016).
type Question struct {
	QuestionInstanceID string         `json:"question_instance_id"`
	RequirementID      string         `json:"requirement_id"`
	QuestionType       string         `json:"question_type"`
	TimeLimitSeconds   int            `json:"time_limit_seconds,omitempty"`
	Answered           bool           `json:"answered"`
	Render             map[string]any `json:"render"`
}

// EncounterView is BossEncounter of game.v1.yaml (getBossEncounter, createBossEncounter, reply of boss.encounter.ready).
type EncounterView struct {
	EncounterID      uuid.UUID         `json:"encounter_id"`
	BossID           uuid.UUID         `json:"boss_id"`
	BossVersion      int               `json:"boss_version"`
	Mode             string            `json:"mode"`
	PartyID          *uuid.UUID        `json:"party_id,omitempty"`
	Status           string            `json:"status"`
	CurrentPhase     int               `json:"current_phase"`
	PhaseID          string            `json:"phase_id,omitempty"`
	PhaseCount       int               `json:"phase_count"`
	EnergyPct        float64           `json:"energy_remaining_pct"`
	PhaseProgress    float64           `json:"phase_progress"`
	PhaseDeadlineAt  *time.Time        `json:"phase_deadline_at,omitempty"`
	DeadlineAt       *time.Time        `json:"deadline_at,omitempty"`
	PhaseRetriesLeft int               `json:"phase_retries_left"`
	PhasesCleared    int               `json:"phases_cleared"`
	StateVersion     int64             `json:"state_version"`
	Outcome          string            `json:"outcome,omitempty"`
	ServerTime       time.Time         `json:"server_time"`
	Members          []EncounterMember `json:"members"`
	MyQuestions      []Question        `json:"my_questions,omitempty"`
	StartedAt        *time.Time        `json:"started_at,omitempty"`
	EndedAt          *time.Time        `json:"ended_at,omitempty"`
}

// ViewInput is what a view of an encounter is built from.
type ViewInput struct {
	E         *boss.Encounter
	Row       port.BossRow
	Round     boss.Round
	Names     map[uuid.UUID]string
	Me        uuid.UUID          // the caller (my_questions); uuid.Nil = none
	Plans     []port.BossPlanRow // plans of the current round
	Answered  map[uuid.UUID]map[int]bool
	Now       time.Time
	Questions bool
}

func pct(f float64) float64 { return math.Round(f*10000) / 100 }

// View builds BossEncounter (no correct/wrong, no correct count of anyone).
func View(in ViewInput) EncounterView {
	e, v := in.E, in.Row.Version
	out := EncounterView{EncounterID: e.ID, BossID: e.BossID, BossVersion: in.Row.VersionNo, Mode: e.Mode, PartyID: e.PartyID,
		Status: e.Status, CurrentPhase: e.PhaseIndex, PhaseID: e.PhaseID, PhaseCount: len(v.Phases), EnergyPct: pct(e.Energy),
		PhaseProgress: math.Round(e.Progress(v, in.Round)*1000) / 1000, PhaseDeadlineAt: e.PhaseDeadline, DeadlineAt: e.Deadline,
		PhaseRetriesLeft: max(0, v.MaxRetries()-e.Retries), PhasesCleared: e.PhasesCleared, StateVersion: e.StateVersion,
		Outcome: e.Outcome, ServerTime: in.Now, StartedAt: e.StartedAt, EndedAt: e.EndedAt, Members: []EncounterMember{}}
	for _, m := range e.Members {
		out.Members = append(out.Members, EncounterMember{StudentID: m.StudentID, Nickname: in.Names[m.StudentID], Ready: m.Ready,
			Present: m.Present(), Connected: m.Present() && m.DisconnectedAt == nil,
			ContributionMet: e.Status == boss.StatusPhaseActive && e.ContributionMet(v, in.Round, m.StudentID)})
	}
	if in.Questions && in.Me != uuid.Nil {
		out.MyQuestions = Questions(in.Plans, in.Me, in.Answered)
	}
	return out
}

// Questions are the questions of the plans of a student (render only), answered flags from answered.
func Questions(plans []port.BossPlanRow, student uuid.UUID, answered map[uuid.UUID]map[int]bool) []Question {
	out := []Question{}
	for _, p := range plans {
		if p.StudentID != student {
			continue
		}
		for _, it := range p.Items {
			out = append(out, Question{QuestionInstanceID: QuestionInstance(p.PlanID, it.Index), RequirementID: p.RequirementID,
				QuestionType: it.QuestionType, TimeLimitSeconds: it.TimeLimitSeconds, Answered: answered[p.PlanID][it.Index], Render: it.Render})
		}
	}
	return out
}

// Updated is the payload of boss.encounter.updated (shared state: energy, phase progress, members with
// contribution_met only).
func Updated(view EncounterView) map[string]any {
	ms := make([]map[string]any, 0, len(view.Members))
	for _, m := range view.Members {
		ms = append(ms, map[string]any{"student_id": m.StudentID, "nickname": m.Nickname, "ready": m.Ready, "connected": m.Connected,
			"contribution_met": m.ContributionMet})
	}
	out := map[string]any{"encounter_id": view.EncounterID, "state_version": view.StateVersion, "status": view.Status,
		"current_phase": view.CurrentPhase, "energy_remaining_pct": view.EnergyPct, "phase_progress": view.PhaseProgress, "members": ms}
	if view.PhaseID != "" {
		out["phase_id"] = view.PhaseID
	}
	if view.PhaseDeadlineAt != nil {
		out["phase_deadline_at"] = view.PhaseDeadlineAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}

// QuestionRender is the AsyncAPI QuestionRender of a plan item.
func QuestionRender(plan uuid.UUID, it port.BossPlanItem) map[string]any {
	out := map[string]any{"question_instance_id": QuestionInstance(plan, it.Index), "question_type": it.QuestionType,
		"stem": map[string]any{}}
	for k, val := range it.Render {
		out[k] = val
	}
	if it.TimeLimitSeconds > 0 {
		out["time_limit_ms"] = it.TimeLimitSeconds * 1000
	}
	return out
}
