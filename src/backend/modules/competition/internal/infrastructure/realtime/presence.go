// Package realtime adapts the realtime contract (src/backend/pkg/realtime) for module competition.
package realtime

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Presence implements port.Presence: control presence.update on ew:rt:user:<id> (F091-AC-23); the gateway records
// it and never forwards it to the client.
type Presence struct{ P rt.Publisher }

var _ port.Presence = Presence{}

// Update implements port.Presence.
func (p Presence) Update(ctx context.Context, user uuid.UUID, area, room string) error {
	e, err := rt.PresenceUpdate(area, room)
	if err != nil {
		return err
	}
	return p.P.ToUser(ctx, user, e)
}

// ToUser implements port.Presence: a message for the connection holding session (hub.instance.assigned).
func (p Presence) ToUser(ctx context.Context, user, session uuid.UUID, typ string, payload any) error {
	e, err := rt.NewMessage(typ, payload, time.Now())
	if err != nil {
		return err
	}
	if session != uuid.Nil {
		e = e.ForSession(session)
	}
	return p.P.ToUser(ctx, user, e)
}
