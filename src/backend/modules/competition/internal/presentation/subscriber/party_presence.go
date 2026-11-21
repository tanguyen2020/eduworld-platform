// Package subscriber holds the NATS core subscribers of module competition: the presence signals of the realtime
// gateway (ew.rt.presence.<tenant_id>, presence-changed.schema.json), at-most-once by design (ADR-0004).
package subscriber

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// PartyPresenceQueue is the queue group of the party presence store: one core-api replica stores each signal.
const PartyPresenceQueue = "competition.party_presence"

const handleTimeout = 2 * time.Second

// PresenceStore stores the last presence of a student for the party rules (cache.PartyPresence).
type PresenceStore interface {
	Put(ctx context.Context, user uuid.UUID, status string, ts int64) error
}

// Signal decodes one presence signal; ok false for a payload outside the schema (dropped). A lost connection (offline,
// away after a timeout) is stored as offline.
func Signal(data []byte) (user uuid.UUID, status string, ts int64, ok bool) {
	var p rt.PresenceChanged
	if err := json.Unmarshal(data, &p); err != nil || p.V != 1 || p.UserID == uuid.Nil || p.Status == "" || len(p.Status) > 64 {
		return uuid.Nil, "", 0, false
	}
	status = p.Status
	if p.Gone() {
		status = rt.PresenceOffline
	}
	return p.UserID, status, p.Ts, true
}

// PartyPresence subscribes ew.rt.presence.* in queue group competition.party_presence until ctx ends (F100: transfer
// to an online member, next leader).
func PartyPresence(ctx context.Context, nc *nats.Conn, store PresenceStore) error {
	sub, err := nc.QueueSubscribe(rt.PresenceSubjectAll, PartyPresenceQueue, func(m *nats.Msg) {
		user, status, ts, ok := Signal(m.Data)
		if !ok {
			slog.Debug("competition: presence signal dropped (schema)")
			return
		}
		hctx, cancel := context.WithTimeout(ctx, handleTimeout)
		defer cancel()
		if err := store.Put(hctx, user, status, ts); err != nil {
			slog.WarnContext(hctx, "competition: party presence not stored", "error", err) // ephemeral: next signal repairs it
		}
	})
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	<-ctx.Done()
	return nil
}
