package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// KeycloakAccessName is the durable consumer that mirrors account access on Keycloak (F016 T012).
const KeycloakAccessName = "identity.keycloak_access"

// KeycloakAccessSubjects are the events it handles.
var KeycloakAccessSubjects = []string{"ew.evt.identity.user.access_revoked", "ew.evt.identity.user.access_restored"}

// KeycloakAccess dispatches the events to command.KeycloakAccess.
type KeycloakAccess struct {
	Access command.KeycloakAccess
}

// Handle implements outbox.Handler.
func (c KeycloakAccess) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		UserID uuid.UUID `json:"user_id"`
		Reason string    `json:"reason"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.UserID == uuid.Nil {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	switch e.EventType {
	case "user.access_revoked":
		return c.Access.Revoked(ctx, p.UserID, p.Reason)
	case "user.access_restored":
		return c.Access.Restored(ctx, p.UserID)
	}
	return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
}
