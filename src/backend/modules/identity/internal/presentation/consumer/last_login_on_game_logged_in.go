package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// LastLoginName is the durable consumer that records the last game login of a student (F014-AC-10, T025).
const LastLoginName = "identity.last_login_on_game_logged_in"

// LastLoginSubjects: student.game_logged_in of module game (owner F014).
var LastLoginSubjects = []string{"ew.evt.game.student.game_logged_in"}

// GameLoginRecorder is command.RecordGameLogin.
type GameLoginRecorder interface {
	Handle(ctx context.Context, user uuid.UUID, at time.Time) (bool, error)
}

// LastLoginOnGameLoggedIn sets identity.users.last_login_at from student.game_logged_in (asynchronous: game and
// identity have their own databases, ADR-0037); a replayed or older event never moves the time back.
type LastLoginOnGameLoggedIn struct {
	Users GameLoginRecorder
}

type gameLoggedIn struct {
	StudentID uuid.UUID `json:"student_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	LoginAt   time.Time `json:"login_at"`
}

// Handle implements outbox.Handler.
func (c LastLoginOnGameLoggedIn) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "student.game_logged_in" {
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	var p gameLoggedIn
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.StudentID == uuid.Nil || p.LoginAt.IsZero() ||
		(e.TenantID != nil && p.TenantID != *e.TenantID) {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	_, err := c.Users.Handle(ctx, p.StudentID, p.LoginAt)
	return err
}
