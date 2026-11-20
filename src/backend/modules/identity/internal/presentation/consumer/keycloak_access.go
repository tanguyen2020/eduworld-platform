package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// KeycloakAccessName is the durable consumer that mirrors account access on Keycloak (F016 T012).
const KeycloakAccessName = "identity.keycloak_access"

// KeycloakAccessSubjects are the events it handles.
var KeycloakAccessSubjects = []string{"ew.evt.identity.user.access_revoked", "ew.evt.identity.user.access_restored",
	"ew.evt.identity.student.cleaned_up", "ew.evt.identity.user.anonymized", "ew.evt.identity.user.email_changed"}

// KeycloakAccess dispatches the events to command.KeycloakAccess.
type KeycloakAccess struct {
	Access command.KeycloakAccess
}

// Handle implements outbox.Handler.
func (c KeycloakAccess) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		UserID    uuid.UUID `json:"user_id"`
		StudentID uuid.UUID `json:"student_id"`
		Reason    string    `json:"reason"`
	}
	err := json.Unmarshal(e.Payload, &p)
	if p.UserID == uuid.Nil {
		p.UserID = p.StudentID // student.cleaned_up carries student_id
	}
	if err != nil || p.UserID == uuid.Nil {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	switch e.EventType {
	case "user.access_revoked":
		return c.Access.Revoked(ctx, p.UserID, p.Reason)
	case "user.access_restored":
		return c.Access.Restored(ctx, p.UserID)
	case "student.cleaned_up":
		return c.Access.CleanedUp(ctx, p.UserID)
	case "user.anonymized":
		return c.Access.Anonymized(ctx, p.UserID)
	case "user.email_changed": // OQ F012-1; an email Keycloak gives to another user needs an operator (DLQ)
		if err := c.Access.EmailChanged(ctx, p.UserID); errors.Is(err, port.ErrKCConflict) {
			return outbox.Permanent(fmt.Errorf("%s: keycloak conflict: %w", e.EventType, err))
		} else if err != nil {
			return err
		}
		return nil
	}
	return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
}
