// Package realtime publishes server messages of module game to the realtime gateway through Valkey (F014 T017,
// REALTIME_ARCHITECTURE §3.1): PUBLISH ew:rt:user:<student_id> with the RtPubSubEnvelope of pkg/realtime (F090
// research R1), targeted at the game session; the gateway assigns seq, delivers the message and closes the
// connection holding that session (4409 / 4410 / 4403). Game clients never talk to Valkey (R25).
package realtime

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Channel is the Valkey channel of a user's connections.
func Channel(user uuid.UUID) string { return rt.UserChannel(user) }

// ValkeyPublisher implements port.RealtimePublisher.
type ValkeyPublisher struct {
	C       valkey.Client
	Timeout time.Duration // per publish (default 2 s): a slow Valkey never blocks the caller for long
}

var _ port.RealtimePublisher = ValkeyPublisher{}

func (p ValkeyPublisher) publish(ctx context.Context, student, session uuid.UUID, typ string, payload map[string]any, at time.Time) error {
	e, err := rt.NewMessage(typ, payload, at)
	if err != nil {
		return err
	}
	return rt.Publisher{C: p.C, Timeout: p.Timeout}.ToUser(ctx, student, e.ForSession(session))
}

// SessionReplaced implements port.RealtimePublisher (session.session.replaced, close 4409).
func (p ValkeyPublisher) SessionReplaced(ctx context.Context, student, session uuid.UUID, newPlatform string, at time.Time) error {
	return p.publish(ctx, student, session, "session.session.replaced", map[string]any{
		"game_session_id": session.String(), "replaced_at": at.UnixMilli(), "new_device_platform": newPlatform}, at)
}

// SessionEnded implements port.RealtimePublisher (session.session.ended: curfew, daily_limit → 4410 with reopen_at;
// else 4403).
func (p ValkeyPublisher) SessionEnded(ctx context.Context, student, session uuid.UUID, reason string, at time.Time, reopenAt *time.Time) error {
	payload := map[string]any{"game_session_id": session.String(), "reason": reason, "ended_at": at.UnixMilli()}
	if reopenAt != nil && (reason == "curfew" || reason == "daily_limit") {
		payload["reopen_at"] = reopenAt.UnixMilli()
	}
	return p.publish(ctx, student, session, "session.session.ended", payload, at)
}

// SessionEndingSoon implements port.RealtimePublisher (session.session.ending_soon: warning before the daily play
// limit ends the session, OQ-17).
func (p ValkeyPublisher) SessionEndingSoon(ctx context.Context, student, session uuid.UUID, reason string, endsAt time.Time) error {
	return p.publish(ctx, student, session, "session.session.ending_soon", map[string]any{
		"game_session_id": session.String(), "reason": reason, "ends_at": endsAt.UnixMilli()}, time.Now())
}

// WorldPublisher implements port.WorldRealtime: world.area.unlocked to every connection of the student (AsyncAPI
// WorldAreaUnlocked) through Valkey ew:rt:user:<id>.
type WorldPublisher struct {
	C       valkey.Client
	Timeout time.Duration
}

var _ port.WorldRealtime = WorldPublisher{}

// AreaUnlocked implements port.WorldRealtime.
func (p WorldPublisher) AreaUnlocked(ctx context.Context, student uuid.UUID, areaID string, name world.Name, unlockedBy string, at time.Time) error {
	e, err := rt.NewMessage("world.area.unlocked", map[string]any{"area_id": areaID, "name": name, "unlocked_by": unlockedBy}, at)
	if err != nil {
		return err
	}
	return rt.Publisher{C: p.C, Timeout: p.Timeout}.ToUser(ctx, student, e)
}

// QuestPublisher implements port.QuestRealtime: quest.progress.updated to every connection of the student (AsyncAPI
// QuestProgressUpdated, F072) through Valkey ew:rt:user:<id>.
type QuestPublisher struct {
	C       valkey.Client
	Timeout time.Duration
}

var _ port.QuestRealtime = QuestPublisher{}

// QuestProgressUpdated implements port.QuestRealtime.
func (p QuestPublisher) QuestProgressUpdated(ctx context.Context, student uuid.UUID, payload map[string]any, at time.Time) error {
	e, err := rt.NewMessage("quest.progress.updated", payload, at)
	if err != nil {
		return err
	}
	return rt.Publisher{C: p.C, Timeout: p.Timeout}.ToUser(ctx, student, e)
}

// ProgressionPublisher implements port.ProgressionRealtime: progression.xp.updated and progression.level.reached to
// every connection of the student (AsyncAPI ProgressionXpUpdated, ProgressionLevelReached, F074) through Valkey
// ew:rt:user:<id>.
type ProgressionPublisher struct {
	C       valkey.Client
	Timeout time.Duration
}

var _ port.ProgressionRealtime = ProgressionPublisher{}

func (p ProgressionPublisher) send(ctx context.Context, typ string, student uuid.UUID, payload map[string]any, at time.Time) error {
	e, err := rt.NewMessage(typ, payload, at)
	if err != nil {
		return err
	}
	return rt.Publisher{C: p.C, Timeout: p.Timeout}.ToUser(ctx, student, e)
}

// XPUpdated implements port.ProgressionRealtime.
func (p ProgressionPublisher) XPUpdated(ctx context.Context, student uuid.UUID, payload map[string]any, at time.Time) error {
	return p.send(ctx, "progression.xp.updated", student, payload, at)
}

// LevelReached implements port.ProgressionRealtime.
func (p ProgressionPublisher) LevelReached(ctx context.Context, student uuid.UUID, payload map[string]any, at time.Time) error {
	return p.send(ctx, "progression.level.reached", student, payload, at)
}

// LiveOpsPublisher implements port.LiveOpsRealtime: event.status.changed, event.progress.updated,
// event.community_goal.updated and season.track.updated (AsyncAPI, F077) to every connection of each student through
// Valkey ew:rt:user:<id> — best effort after commit.
type LiveOpsPublisher struct {
	C       valkey.Client
	Timeout time.Duration
}

var _ port.LiveOpsRealtime = LiveOpsPublisher{}

// Push implements port.LiveOpsRealtime.
func (p LiveOpsPublisher) Push(ctx context.Context, typ string, students []uuid.UUID, payload map[string]any, at time.Time) error {
	e, err := rt.NewMessage(typ, payload, at)
	if err != nil {
		return err
	}
	pub := rt.Publisher{C: p.C, Timeout: p.Timeout}
	var first error
	for _, s := range students {
		if err := pub.ToUser(ctx, s, e); err != nil && first == nil {
			first = err
		}
	}
	return first
}
