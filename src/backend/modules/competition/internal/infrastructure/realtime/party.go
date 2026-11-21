package realtime

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// PartyPublisher implements port.PartyRealtime: one PUBLISH ew:rt:user:<id> per recipient (research R1: no party topic,
// a kicked student stops receiving at once; a party has at most 8 members).
type PartyPublisher struct{ P rt.Publisher }

var _ port.PartyRealtime = PartyPublisher{}

// Send implements port.PartyRealtime.
func (p PartyPublisher) Send(ctx context.Context, to []uuid.UUID, typ string, payload any) error {
	e, err := rt.NewMessage(typ, payload, time.Now())
	if err != nil {
		return err
	}
	seen := make(map[uuid.UUID]bool, len(to))
	var errs []error
	for _, id := range to {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		if err := p.P.ToUser(ctx, id, e); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
