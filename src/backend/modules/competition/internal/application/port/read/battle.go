package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
)

// MatchPlayer is one participant of a match as the read side returns it (never its rating, R16).
type MatchPlayer struct {
	StudentID uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	TeamNo    int
	Outcome   *string
	Score     *float64
	Correct   *int
}

// MatchRow is a match of the session tenant with its result (getMatch, listMyMatches).
type MatchRow struct {
	ID            uuid.UUID
	Mode          string
	Type          string
	SubjectID     uuid.UUID
	Grade         int
	Status        string
	Ranked        bool
	IsBot         bool
	RoomID        *uuid.UUID
	StartedAt     *time.Time
	EndedAt       *time.Time
	EndReason     *string
	Rated         *bool
	PlanID        *uuid.UUID
	QuestionCount int
	Summary       json.RawMessage
	CreatedAt     time.Time
	Players       []MatchPlayer
}

// BattleReader reads matches under entx.ReadSession (F105): only through participant rows of the session tenant.
type BattleReader interface {
	// Match returns a match with a participant in the session tenant (nil otherwise).
	Match(ctx context.Context, id uuid.UUID) (*MatchRow, error)
	// MyMatches lists the matches of student (newest first) after the cursor (created_at, match_id), mode "" = all.
	MyMatches(ctx context.Context, student uuid.UUID, mode string, before *time.Time, beforeID uuid.UUID, limit int) ([]MatchRow, error)
	Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error)
}
