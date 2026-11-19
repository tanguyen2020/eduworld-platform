package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// GameAuthRevokeName is the durable consumer that revokes game login sessions (F013 T040, ADR-0086).
const GameAuthRevokeName = "identity.game_auth_revoke"

// GameAuthRevokeSubjects are the events it handles; producers: F011/F016 (user.*), F016 (student.cleaned_up).
var GameAuthRevokeSubjects = []string{"ew.evt.identity.user.access_revoked", "ew.evt.identity.user.status_changed",
	"ew.evt.identity.student.cleaned_up"}

// GameAuthRevoke maps the events to GameAuthSessionRevoker (F013-AC-29). Idempotency per event_id is
// handled by pkg/outbox.Consumer (processed_events); revoking twice has no effect anyway.
type GameAuthRevoke struct {
	Revoker command.GameAuthSessionRevoker
}

type accessRevoked struct {
	UserID   uuid.UUID  `json:"user_id"`
	Reason   string     `json:"reason"`
	TenantID *uuid.UUID `json:"tenant_id"`
}

type statusChanged struct {
	UserID uuid.UUID `json:"user_id"`
	To     string    `json:"to"`
}

type cleanedUp struct {
	StudentID uuid.UUID `json:"student_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
}

// Handle implements outbox.Handler.
func (c GameAuthRevoke) Handle(ctx context.Context, e outbox.Envelope) error {
	switch e.EventType {
	case "user.access_revoked":
		var p accessRevoked
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.UserID == uuid.Nil {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
		}
		reason := gamelogin.RevokeAccessRevoked
		switch p.Reason {
		case "locked":
			reason = gamelogin.RevokeAccountLocked
		case "password_changed":
			reason = gamelogin.RevokePasswordChanged
		}
		return c.revoke(ctx, write.SessionFilter{UserID: p.UserID, TenantID: p.TenantID}, reason)
	case "user.status_changed":
		var p statusChanged
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.UserID == uuid.Nil {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
		}
		switch p.To {
		case "locked":
			return c.revoke(ctx, write.SessionFilter{UserID: p.UserID}, gamelogin.RevokeAccountLocked)
		case "deactivated", "anonymized":
			return c.revoke(ctx, write.SessionFilter{UserID: p.UserID}, gamelogin.RevokeAccessRevoked)
		}
		return nil // pending/active: nothing to revoke
	case "student.cleaned_up":
		var p cleanedUp
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.StudentID == uuid.Nil || p.TenantID == uuid.Nil {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
		}
		tenant := p.TenantID
		return c.revoke(ctx, write.SessionFilter{UserID: p.StudentID, TenantID: &tenant}, gamelogin.RevokeCleanup)
	default:
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
}

func (c GameAuthRevoke) revoke(ctx context.Context, f write.SessionFilter, reason string) error {
	_, err := c.Revoker.RevokeSessions(ctx, f, reason)
	return err
}
