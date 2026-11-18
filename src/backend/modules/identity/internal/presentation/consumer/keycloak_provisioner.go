package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// KeycloakProvisionerName is the durable consumer creating Keycloak accounts of SIS students (F011).
const KeycloakProvisionerName = "identity.keycloak_provisioner"

// KeycloakProvisionerSubjects are the events it handles.
var KeycloakProvisionerSubjects = []string{"ew.evt.identity.student.provisioning_requested"}

// KeycloakProvisioner dispatches student.provisioning_requested to ProvisionStudentAccount.
type KeycloakProvisioner struct {
	Provision command.ProvisionStudentAccountHandler
}

type provisioningRequested struct {
	StudentID uuid.UUID `json:"student_id"`
}

// Handle implements outbox.Handler.
func (c KeycloakProvisioner) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "student.provisioning_requested" {
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	var p provisioningRequested
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.StudentID == uuid.Nil {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	return c.Provision.Handle(ctx, p.StudentID)
}
