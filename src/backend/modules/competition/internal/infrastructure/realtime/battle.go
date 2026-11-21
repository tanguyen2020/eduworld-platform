package realtime

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// BattleOutput implements port.RoomOutput (F105): room messages on ew:rt:room:<id>:out (Target when not for everyone),
// a message of one player on ew:rt:user:<id> (always subscribed, also before the room output).
type BattleOutput struct {
	P   rt.Publisher
	Now func() time.Time
}

var _ port.RoomOutput = BattleOutput{}

func (o BattleOutput) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Room implements port.RoomOutput.
func (o BattleOutput) Room(ctx context.Context, room uuid.UUID, typ string, payload any, to []uuid.UUID) error {
	e, err := rt.NewMessage(typ, payload, o.now())
	if err != nil {
		return err
	}
	if to != nil {
		if len(to) == 0 {
			return nil
		}
		e.Target = &rt.Target{UserIDs: to}
	}
	return o.P.Publish(ctx, rt.RoomOutChannel(room.String()), e)
}

// User implements port.RoomOutput.
func (o BattleOutput) User(ctx context.Context, user uuid.UUID, typ string, payload any) error {
	e, err := rt.NewMessage(typ, payload, o.now())
	if err != nil {
		return err
	}
	return o.P.ToUser(ctx, user, e)
}
