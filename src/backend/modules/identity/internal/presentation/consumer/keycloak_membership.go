// Package consumer holds the event consumers of module identity (presentation of the worker profile
// events): they parse the envelope and call one application handler; scope, idempotency (processed_events)
// and DLQ are handled by pkg/outbox.Consumer.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// KeycloakMembershipName is the durable consumer of the Organization/membership worker (F010).
const KeycloakMembershipName = "identity.keycloak_membership"

// KeycloakMembershipSubjects are the events it handles (user_role.changed is emitted by F012; user.status_changed
// by F016 is handled there).
var KeycloakMembershipSubjects = []string{"ew.evt.identity.tenant.created", "ew.evt.identity.tenant.updated",
	"ew.evt.identity.tenant_membership.changed", "ew.evt.identity.user_role.changed"}

// KeycloakMembership dispatches events to the sync use cases.
type KeycloakMembership struct {
	SyncOrganization command.SyncTenantOrganizationHandler
	SyncMembership   command.SyncUserMembershipHandler
}

type userEvent struct {
	UserID uuid.UUID `json:"user_id"`
}

type tenantEvent struct {
	TenantID uuid.UUID `json:"tenant_id"`
}

// Handle implements outbox.Handler.
func (c KeycloakMembership) Handle(ctx context.Context, e outbox.Envelope) error {
	switch e.EventType {
	case "tenant.created", "tenant.updated":
		var p tenantEvent
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.TenantID == uuid.Nil {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
		}
		return c.SyncOrganization.Handle(ctx, p.TenantID)
	case "tenant_membership.changed", "user_role.changed":
		var p userEvent
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.UserID == uuid.Nil {
			return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
		}
		return c.SyncMembership.Handle(ctx, p.UserID)
	default:
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
}
