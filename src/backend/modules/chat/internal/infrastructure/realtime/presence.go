// Package realtime adapts the realtime contract (src/backend/pkg/realtime) for module chat.
package realtime

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// FriendPresenceType is the server message of the AsyncAPI (PresenceFriendUpdated).
const FriendPresenceType = "presence.friend.updated"

// Publisher implements port.RealtimePublisher on ew:rt:user:<id> (Valkey pub/sub, the gateway delivers it).
type Publisher struct {
	P   rt.Publisher
	Now func() time.Time
}

var _ port.RealtimePublisher = Publisher{}

// FriendPresence implements port.RealtimePublisher: payload {user_id, status, activity?, area_id?} only (no PII).
func (p Publisher) FriendPresence(ctx context.Context, to, user uuid.UUID, s social.Presence) error {
	payload := map[string]any{"user_id": user, "status": s.Status}
	if s.Activity != "" {
		payload["activity"] = s.Activity
	}
	if s.AreaID != "" {
		payload["area_id"] = s.AreaID
	}
	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	e, err := rt.NewMessage(FriendPresenceType, payload, now)
	if err != nil {
		return err
	}
	return p.P.ToUser(ctx, to, e)
}
