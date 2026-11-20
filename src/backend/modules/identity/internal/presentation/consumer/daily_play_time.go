package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// DailyPlayTimeName is the durable consumer that projects the time students play per ICT day (OQ-17, F013-AC-33).
const DailyPlayTimeName = "identity.daily_play_time"

// DailyPlayTimeSubjects: game_session.ended of module game (owner F014).
var DailyPlayTimeSubjects = []string{"ew.evt.game.game_session.ended"}

// PlayTimeRecorder is command.RecordPlayTime.
type PlayTimeRecorder interface {
	Handle(ctx context.Context, tenant, user uuid.UUID, endedAt time.Time, duration time.Duration) error
}

// DailyPlayTime adds the duration of every ended game session to identity.daily_play_time (asynchronous: game and
// identity have their own databases, ADR-0037). A replay of the same event is skipped by pkg/outbox.Consumer
// (processed events), so a session is counted once.
type DailyPlayTime struct {
	PlayTime PlayTimeRecorder
}

type gameSessionEnded struct {
	StudentID  uuid.UUID `json:"student_id"`
	DurationMS *int64    `json:"duration_ms"`
	EndedAt    time.Time `json:"ended_at"`
}

// Handle implements outbox.Handler.
func (c DailyPlayTime) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "game_session.ended" {
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	var p gameSessionEnded
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.StudentID == uuid.Nil || p.DurationMS == nil || *p.DurationMS < 0 ||
		p.EndedAt.IsZero() || e.TenantID == nil || *e.TenantID == uuid.Nil {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	return c.PlayTime.Handle(ctx, *e.TenantID, p.StudentID, p.EndedAt, time.Duration(*p.DurationMS)*time.Millisecond)
}
