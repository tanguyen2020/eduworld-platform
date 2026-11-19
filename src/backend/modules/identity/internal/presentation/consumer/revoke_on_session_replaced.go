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

// RevokeOnSessionReplacedName is the durable consumer that revokes the login of the device that lost a takeover
// (F014-AC-04, T019, ADR-0086).
const RevokeOnSessionReplacedName = "identity.revoke_on_session_replaced"

// RevokeOnSessionReplacedSubjects: game_session.replaced of module game (owner F014).
var RevokeOnSessionReplacedSubjects = []string{"ew.evt.game.game_session.replaced"}

// RevokeOnSessionReplaced maps game_session.replaced to GameAuthSessionRevoker (F013) with reason takeover: the game
// login sessions stored on the old device (old_device_id) and the session of the old Keycloak sid are revoked, and
// their Keycloak sessions end, so a refresh or resume of the old device gets 401 REFRESH_TOKEN_INVALID. Idempotency
// per event_id is pkg/outbox.Consumer; revoking twice has no effect anyway.
type RevokeOnSessionReplaced struct {
	Revoker command.GameAuthSessionRevoker
}

type sessionReplaced struct {
	StudentID      uuid.UUID `json:"student_id"`
	OldKeycloakSID string    `json:"old_keycloak_sid"`
	OldDeviceID    uuid.UUID `json:"old_device_id"`
}

// Handle implements outbox.Handler.
func (c RevokeOnSessionReplaced) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "game_session.replaced" {
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	var p sessionReplaced
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.StudentID == uuid.Nil || p.OldDeviceID == uuid.Nil ||
		len(p.OldKeycloakSID) > 64 {
		return outbox.Permanent(fmt.Errorf("%s: invalid payload", e.EventType))
	}
	device := p.OldDeviceID
	if _, err := c.Revoker.RevokeSessions(ctx, write.SessionFilter{UserID: p.StudentID, TenantID: e.TenantID, DeviceID: &device},
		gamelogin.RevokeTakeover); err != nil {
		return err
	}
	if p.OldKeycloakSID == "" || p.OldKeycloakSID == "-" { // "-": the old game token carried no sid
		return nil
	}
	_, err := c.Revoker.RevokeSessions(ctx, write.SessionFilter{UserID: p.StudentID, TenantID: e.TenantID,
		KeycloakSID: p.OldKeycloakSID}, gamelogin.RevokeTakeover)
	return err
}
