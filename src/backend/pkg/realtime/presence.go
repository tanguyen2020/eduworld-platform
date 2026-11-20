package realtime

import (
	"github.com/google/uuid"
)

// Presence statuses: the single presence enum of MULTIPLAYER_GAMEPLAY §5 (specs/schemas/common/enums.schema.json
// PresenceStatus). offline is set by the gateway only.
const (
	PresenceOffline    = "offline"
	PresenceOnline     = "online"
	PresenceInActivity = "in_activity"
	PresenceInMatch    = "in_match"
	PresenceAway       = "away"
)

// Presence change reasons (presence-changed.schema.json).
const (
	PresenceConnected    = "connected"
	PresenceDisconnected = "disconnected"
	PresenceTimeout      = "timeout"
	PresenceStatusSet    = "status_set"
	PresenceAreaChanged  = "area_changed"
)

// presencePrefix is the NATS core subject of presence signals (F091 research R3): ew.rt.presence.<tenant_id>.
const presencePrefix = "ew.rt.presence."

// PresenceSubject is ew.rt.presence.<tenant_id>.
func PresenceSubject(tenant uuid.UUID) string { return presencePrefix + tenant.String() }

// PresenceSubjectAll subscribes the presence signals of every tenant (consumers filter by tenant themselves).
const PresenceSubjectAll = presencePrefix + "*"

// PresenceChanged is the payload of ew.rt.presence.<tenant_id> (specs/schemas/realtime/presence-changed.schema.json):
// ids and status only, never sent to a client as such (F091-AC-25).
type PresenceChanged struct {
	V        int        `json:"v"`
	UserID   uuid.UUID  `json:"user_id"`
	TenantID uuid.UUID  `json:"tenant_id"`
	SchoolID *uuid.UUID `json:"school_id,omitempty"`
	Status   string     `json:"status"`
	Activity string     `json:"activity,omitempty"`
	AreaID   string     `json:"area_id,omitempty"`
	RoomID   string     `json:"room_id,omitempty"`
	Hidden   bool       `json:"hidden,omitempty"`
	Reason   string     `json:"reason,omitempty"`
	Ts       int64      `json:"ts"`
}

// Gone reports that the user lost the connection (offline, or away after a 4408 close): the room host holds the hub
// seat for disconnect_grace (F091-AC-09). away set by the client itself is not a disconnection.
func (p PresenceChanged) Gone() bool {
	return p.Status == PresenceOffline || (p.Status == PresenceAway && p.Reason == PresenceTimeout)
}
