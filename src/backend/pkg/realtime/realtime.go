// Package realtime is the shared contract between the realtime gateway (binary realtime, module realtime) and the
// modules that push events to it or answer its commands (F090, REALTIME_ARCHITECTURE §2–§5, research R1):
//   - Valkey Pub/Sub channels ew:rt:* / ew:chat:fanout:* and the RtPubSubEnvelope
//     (specs/schemas/realtime/rt-pubsub-envelope.schema.json) with its Publisher;
//   - the realtime ticket key written by module game and consumed (GETDEL) by the gateway;
//   - NATS request/reply ew.cmd.<module>.<type>: the Ew-* headers set by the gateway, the reply shape and
//     ServeCommand for the handlers in core-api.
//
// Game clients never talk to Valkey or NATS (R25). Nothing here carries PII, tokens or payload logs (R14).
package realtime

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"

	"github.com/google/uuid"
)

// Valkey Pub/Sub channels (VALKEY_DESIGN §3.2: shared channel namespace, ACL by channel).
const (
	// BroadcastChannel carries system notices to every gateway node (module identity, admin).
	BroadcastChannel = "ew:rt:broadcast"
	userPrefix       = "ew:rt:user:"
	partyPrefix      = "ew:rt:party:"
	roomPrefix       = "ew:rt:room:"
	roomOutSuffix    = ":out"
	chatPrefix       = "ew:chat:fanout:"
	nodePrefix       = "ew:rt:node:"
	nodeInSuffix     = ":in"
)

// idRe bounds ids used inside channel names and NATS subjects (no '.', '*', '>', ':' or whitespace).
var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidID reports an id usable in a channel name or a NATS subject token.
func ValidID(id string) bool { return idRe.MatchString(id) }

// UserChannel is ew:rt:user:<user_id>: every message for one user (notification, takeover, revocation…).
func UserChannel(user uuid.UUID) string { return userPrefix + user.String() }

// PartyChannel is ew:rt:party:<party_id>.
func PartyChannel(party string) string { return partyPrefix + party }

// RoomOutChannel is ew:rt:room:<room_id>:out (room host output).
func RoomOutChannel(room string) string { return roomPrefix + room + roomOutSuffix }

// ChatFanoutChannel is ew:chat:fanout:<channel_id>.
func ChatFanoutChannel(channel string) string { return chatPrefix + channel }

// NodeChannel is ew:rt:node:<node_id>:in: connection signals between gateway nodes (takeover across nodes).
func NodeChannel(node string) string { return nodePrefix + node + nodeInSuffix }

// TopicKind classifies a channel a module may ask a connection to subscribe (reply.subscribe).
type TopicKind int

// Topic kinds.
const (
	TopicInvalid TopicKind = iota
	TopicUser
	TopicParty
	TopicRoom
	TopicChat
	TopicBroadcast
)

// ParseTopic returns the kind and id of a subscribable channel; node channels and malformed names are invalid.
func ParseTopic(topic string) (TopicKind, string) {
	cut := func(prefix, suffix string) (string, bool) {
		if len(topic) <= len(prefix)+len(suffix) || topic[:len(prefix)] != prefix || topic[len(topic)-len(suffix):] != suffix {
			return "", false
		}
		id := topic[len(prefix) : len(topic)-len(suffix)]
		return id, ValidID(id)
	}
	switch {
	case topic == BroadcastChannel:
		return TopicBroadcast, ""
	case len(topic) > len(userPrefix) && topic[:len(userPrefix)] == userPrefix:
		if id, err := uuid.Parse(topic[len(userPrefix):]); err == nil && id.String() == topic[len(userPrefix):] {
			return TopicUser, id.String()
		}
	case len(topic) > len(partyPrefix) && topic[:len(partyPrefix)] == partyPrefix:
		if id, ok := cut(partyPrefix, ""); ok {
			return TopicParty, id
		}
	case len(topic) > len(roomPrefix) && topic[:len(roomPrefix)] == roomPrefix:
		if id, ok := cut(roomPrefix, roomOutSuffix); ok {
			return TopicRoom, id
		}
	case len(topic) > len(chatPrefix) && topic[:len(chatPrefix)] == chatPrefix:
		if id, ok := cut(chatPrefix, ""); ok {
			return TopicChat, id
		}
	}
	return TopicInvalid, ""
}

// TicketKey is the Valkey key of a realtime ticket: ew:game:rt:ticket:<sha256 hex> (VALKEY_DESIGN §1: written by
// module game with SET NX EX, read once by the gateway with GETDEL). The plain ticket is never stored.
func TicketKey(ticketHash string) string { return "ew:game:rt:ticket:" + ticketHash }

// HashTicket is the lowercase hex SHA-256 of a ticket.
func HashTicket(ticket string) string {
	h := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(h[:])
}

// TicketClaims is the value stored under TicketKey: ids only, no PII (F090-AC-01). Platform (client platform of the
// game session) and GradeBand (g1_3…g8_9 of the student) are the bounded labels of eduworld_players_online
// (MONITORING §3); empty when unknown (tickets of an older core-api).
type TicketClaims struct {
	UserID        uuid.UUID `json:"user_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	SchoolID      uuid.UUID `json:"school_id"`
	GameSessionID uuid.UUID `json:"game_session_id"`
	DeviceID      uuid.UUID `json:"device_id"`
	IssuedAt      int64     `json:"iat"` // epoch ms
	Platform      string    `json:"platform,omitempty"`
	GradeBand     string    `json:"grade_band,omitempty"`
}

// Valid reports claims with every id set.
func (c TicketClaims) Valid() bool {
	return c.UserID != uuid.Nil && c.TenantID != uuid.Nil && c.GameSessionID != uuid.Nil && c.DeviceID != uuid.Nil
}
