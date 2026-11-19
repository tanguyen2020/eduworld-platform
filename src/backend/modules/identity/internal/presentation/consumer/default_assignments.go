package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// DefaultAssignmentsName is the durable consumer assigning the default platform resources to a new tenant
// (F045-AC-11).
const DefaultAssignmentsName = "identity.tenant_default_assignments"

// DefaultAssignmentsSubjects are the events it handles.
var DefaultAssignmentsSubjects = []string{"ew.evt.identity.tenant.created"}

// DefaultAssignments dispatches tenant.created to ApplyDefaultAssignments.
type DefaultAssignments struct {
	Apply command.ApplyDefaultAssignments
}

type tenantCreated struct {
	TenantID uuid.UUID `json:"tenant_id"`
}

// Handle implements outbox.Handler.
func (c DefaultAssignments) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "tenant.created" {
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	var p tenantCreated
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.TenantID == uuid.Nil {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	_, err := c.Apply.Handle(ctx, p.TenantID, e.EventID, e.OccurredAt)
	return err
}
