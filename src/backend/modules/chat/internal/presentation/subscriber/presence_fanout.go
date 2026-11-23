// Package subscriber holds the NATS core subscribers of module chat: the presence signals of the realtime gateway
// (ew.rt.presence.<tenant_id>, presence-changed.schema.json), at-most-once by design (ADR-0004).
package subscriber

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// PresenceQueue is the queue group of the presence fan-out: one core-api replica handles each signal.
const PresenceQueue = "chat.presence_fanout"

// handleTimeout bounds the handling of one signal (fan-out budget ≤ 2 s, SC-004).
const handleTimeout = 2 * time.Second

var statuses = map[string]bool{rt.PresenceOffline: true, rt.PresenceOnline: true, rt.PresenceInActivity: true,
	rt.PresenceInMatch: true, rt.PresenceAway: true}

// Handler is the presence fan-out use case (command.PresenceFanout).
type Handler interface {
	Handle(ctx context.Context, s command.PresenceSignal) (int, error)
}

// Signal decodes one presence signal; ok is false for a payload outside the schema (dropped).
func Signal(data []byte) (command.PresenceSignal, bool) {
	var p rt.PresenceChanged
	if err := json.Unmarshal(data, &p); err != nil || p.V != 1 || !statuses[p.Status] || len(p.Activity) > 64 || len(p.AreaID) > 64 ||
		len(p.RoomID) > 64 {
		return command.PresenceSignal{}, false
	}
	return command.PresenceSignal{TenantID: p.TenantID, UserID: p.UserID, Presence: social.Presence{Status: p.Status,
		Activity: p.Activity, AreaID: p.AreaID, Hidden: p.Hidden, Ts: p.Ts}, RoomID: p.RoomID}, true
}

// PresenceFanout subscribes ew.rt.presence.* in queue group chat.presence_fanout until ctx ends (F095 T024).
func PresenceFanout(ctx context.Context, nc *nats.Conn, h Handler) error {
	sub, err := nc.QueueSubscribe(rt.PresenceSubjectAll, PresenceQueue, func(m *nats.Msg) {
		s, ok := Signal(m.Data)
		if !ok {
			slog.Debug("chat: presence signal dropped (schema)")
			return
		}
		hctx, cancel := context.WithTimeout(ctx, handleTimeout)
		defer cancel()
		if _, err := h.Handle(hctx, s); err != nil {
			slog.WarnContext(hctx, "chat: presence fan-out failed", "error", err) // ephemeral: next signal repairs it
		}
	})
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	<-ctx.Done()
	return nil
}
