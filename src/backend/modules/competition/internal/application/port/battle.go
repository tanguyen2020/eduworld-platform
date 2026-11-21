package port

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
)

// Errors of the question plans of module learning (F105): a refusal (4xx: pool too small, invalid request) or the
// module did not answer (5xx, deadline). Both abort a match before its first question; a scoring failure replays it.
var (
	ErrPlanRefused     = errors.New("battle: module learning refused the question plan")
	ErrPlanUnavailable = errors.New("battle: module learning unavailable")
)

// PlanRequest asks module learning for the questions of a match (createQuestionPlan).
type PlanRequest struct {
	MatchID      uuid.UUID
	Mode         string
	Grade        int
	SubjectID    uuid.UUID
	TopicID      *uuid.UUID
	Count        int
	TimeS        int
	Participants []uuid.UUID
}

// PlanItem is one question of a plan: render payload only (ADR-0016).
type PlanItem struct {
	Index             int
	QuestionVersionID uuid.UUID
	Type              string
	TimeS             int
	Render            json.RawMessage
}

// Plan is the pinned plan of a match (ADR-0043).
type Plan struct {
	ID      uuid.UUID
	Version string
	Items   []PlanItem
}

// QuestionPlans is the facade of module learning for the battle engine (x-allowed-callers: competition): plan, grade one
// answer, reveal the answer of a closed question. Called outside any transaction of module competition.
type QuestionPlans interface {
	Create(ctx context.Context, req PlanRequest) (Plan, error)
	Score(ctx context.Context, plan uuid.UUID, index int, student uuid.UUID, response json.RawMessage, at time.Time) (bool, error)
	Reveal(ctx context.Context, plan uuid.UUID, index int, closedAt time.Time) (battle.Reveal, error)
}

// RoomOutput publishes the messages of a match room (ew:rt:room:<id>:out for the room, ew:rt:user:<id> for one player).
type RoomOutput interface {
	Room(ctx context.Context, room uuid.UUID, typ string, payload any, to []uuid.UUID) error
	User(ctx context.Context, user uuid.UUID, typ string, payload any) error
}

// BattleMetrics observes the battle engine (plan.md §Quan sát); labels never carry ids.
type BattleMetrics interface {
	Rooms(roomType string, delta float64)
	AnswerRejected(code string)
	ScoreCall(d time.Duration, result string)
	Abandon(mode string)
	Aborted(reason string)
	Reconnect(result string)
	Signal(kind string)
}
