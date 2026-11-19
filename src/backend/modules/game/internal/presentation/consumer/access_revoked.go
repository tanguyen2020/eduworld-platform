package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// SessionEnder ends the sessions of a student (command.Sessions.EndSessionsForUser).
type SessionEnder interface {
	EndSessionsForUser(ctx context.Context, r command.Revocation) (int, error)
}

// SessionEndOnRevoke is the consumer game.session_end_on_revoke (F014-AC-24, T039): user.access_revoked,
// user.status_changed (locked, deactivated, anonymized) and student.cleaned_up end the active game sessions of the
// student with the end reason of EVENT_DESIGN §4 (account_locked; cleanup; access_revoked for the other reasons).
// Idempotency per event_id is pkg/outbox.Consumer (processed_events); an ended session is never ended again.
type SessionEndOnRevoke struct {
	Sessions SessionEnder
}

// SessionEndOnRevokeOptions returns the outbox consumer options (user.* may be platform events: Platform true).
func SessionEndOnRevokeOptions(h SessionEndOnRevoke, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.SessionRevokeName, Stream: messaging.IdentityStream,
		Subjects: messaging.Subjects(messaging.SessionRevokeEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

type accessRevoked struct {
	UserID uuid.UUID `json:"user_id"`
	Reason string    `json:"reason"`
}

type statusChanged struct {
	UserID uuid.UUID `json:"user_id"`
	To     string    `json:"to"`
}

type cleanedUp struct {
	StudentID uuid.UUID `json:"student_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
}

func (c SessionEndOnRevoke) reject(e outbox.Envelope) error {
	observability.ConsumerRejected().WithLabelValues(messaging.SessionRevokeName, "invalid_payload").Inc()
	return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
}

// Handle implements outbox.Handler. The envelope tenant narrows the sessions to that tenant; a platform event
// (tenant null) ends the student's sessions in every tenant (platform scope of the consumer).
func (c SessionEndOnRevoke) Handle(ctx context.Context, e outbox.Envelope) error {
	r := command.Revocation{TenantID: e.TenantID}
	switch e.EventType {
	case "user.access_revoked":
		var p accessRevoked
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.UserID == uuid.Nil || p.Reason == "" {
			return c.reject(e)
		}
		r.StudentID, r.Reason, r.Realtime = p.UserID, gamesession.ReasonAccessRevoked, "access_revoked"
		switch p.Reason {
		case "locked":
			r.Reason, r.Realtime = gamesession.ReasonAccountLocked, "account_locked"
		case "password_changed":
			r.Realtime = "password_changed"
		}
	case "user.status_changed":
		var p statusChanged
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.UserID == uuid.Nil {
			return c.reject(e)
		}
		switch p.To {
		case "locked":
			r.StudentID, r.Reason, r.Realtime = p.UserID, gamesession.ReasonAccountLocked, "account_locked"
		case "deactivated", "anonymized":
			r.StudentID, r.Reason, r.Realtime = p.UserID, gamesession.ReasonAccessRevoked, "access_revoked"
		default:
			return nil // pending/active: nothing to end
		}
	case "student.cleaned_up":
		var p cleanedUp
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.StudentID == uuid.Nil || p.TenantID == uuid.Nil {
			return c.reject(e)
		}
		if e.TenantID != nil && *e.TenantID != p.TenantID {
			return c.reject(e)
		}
		tenant := p.TenantID
		r.StudentID, r.TenantID, r.Reason, r.Realtime = p.StudentID, &tenant, gamesession.ReasonCleanup, "cleanup"
	default:
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	_, err := c.Sessions.EndSessionsForUser(ctx, r)
	return err
}
