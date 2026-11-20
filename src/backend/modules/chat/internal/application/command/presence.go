package command

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ReadSession runs read-only work under a scope (entx.ReadSession).
type ReadSession interface {
	InTenantRead(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error
}

// PresenceSignal is one presence change of the realtime gateway (ew.rt.presence.<tenant_id>), ids and status only.
type PresenceSignal struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
	Presence social.Presence
}

// PresenceFanout sends presence.friend.updated to the online friends of a student when what they may see changes
// (F095-AC-20, AC-21, MULTIPLAYER_GAMEPLAY §5): only friends without a block in either direction, never anyone else;
// hidden → offline; exam → generic activity. The last presence is kept in Valkey for listFriends (AC-22); presence
// is never written to PostgreSQL (ADR-0004).
type PresenceFanout struct {
	Read      ReadSession
	Relations read.SocialReader
	Store     port.PresenceStore
	Cache     port.FriendCache // nil: no cache
	Realtime  port.RealtimePublisher
	Metrics   port.Metrics // nil: no metrics
}

var errSignal = errors.New("presence: signal without tenant or user")

// Handle applies one signal and returns the number of friends notified.
func (h PresenceFanout) Handle(ctx context.Context, s PresenceSignal) (int, error) {
	if s.TenantID == uuid.Nil || s.UserID == uuid.Nil {
		return 0, errSignal
	}
	prev, stored, err := h.Store.Put(ctx, s.UserID, s.Presence)
	if err != nil || !stored {
		return 0, err // an older signal than the stored one is ignored
	}
	before := social.Presence{Status: social.PresenceOffline}
	if prev != nil {
		before = prev.Visible()
	}
	now := s.Presence.Visible()
	if before.SameView(now) {
		return 0, nil // TTL refresh or a change friends cannot see (hidden student changing activity)
	}
	friends, err := h.friends(ctx, s.TenantID, s.UserID)
	if err != nil || len(friends) == 0 {
		return 0, err
	}
	states, err := h.Store.Get(ctx, friends)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range friends {
		st, ok := states[f]
		if !ok || st.Status == "" || st.Status == social.PresenceOffline {
			continue // only connected friends receive realtime messages
		}
		if err := h.Realtime.FriendPresence(ctx, f, s.UserID, now); err != nil {
			slog.WarnContext(ctx, "chat: presence.friend.updated not published", "error", err) // ephemeral (ADR-0004)
			continue
		}
		n++
	}
	if h.Metrics != nil {
		h.Metrics.PresenceFanout(n)
	}
	return n, nil
}

// friends returns the friend ids of user (cache, else the read model under the tenant of the signal).
func (h PresenceFanout) friends(ctx context.Context, tenant, user uuid.UUID) ([]uuid.UUID, error) {
	if h.Cache != nil {
		if ids, ok, err := h.Cache.Get(ctx, user); err == nil && ok {
			return ids, nil
		}
	}
	scope, err := authz.EventScope(tenant)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	if err := h.Read.InTenantRead(ctx, scope, func(ctx context.Context) error {
		var err error
		ids, err = h.Relations.FriendIDs(ctx, tenant, user)
		return err
	}); err != nil {
		return nil, err
	}
	if h.Cache != nil {
		_ = h.Cache.Set(ctx, user, ids)
	}
	return ids, nil
}
