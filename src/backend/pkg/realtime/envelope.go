package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"
)

// Envelope kinds.
const (
	KindMessage = "message"
	KindControl = "control"
	KindBundle  = "bundle" // per-viewer messages packed per gateway node (F091 research R4)
)

// Control actions.
const (
	ActionClose          = "close"
	ActionPresenceUpdate = "presence.update" // area/room of the user for the gateway presence (F091-AC-23)
)

// Close codes of REALTIME_ARCHITECTURE §3.1 carried by control messages.
const (
	CloseRevoked     = 4403 // credential/session revoked (session.session.ended with the reason first)
	CloseReplaced    = 4409 // replaced by another connection or device
	CloseSessionDone = 4410 // session ended without revoking credentials (curfew, daily_limit)
)

// Envelope is RtPubSubEnvelope v1 (specs/schemas/realtime/rt-pubsub-envelope.schema.json).
type Envelope struct {
	V         int      `json:"v"`
	Kind      string   `json:"kind"`
	Target    *Target  `json:"target,omitempty"`
	Droppable bool     `json:"droppable,omitempty"`
	Message   *Message `json:"message,omitempty"`
	Control   *Control `json:"control,omitempty"`
	Bundle    *Bundle  `json:"bundle,omitempty"`
}

// Bundle carries one message per viewer (snapshot of a hub room for the users of one gateway node): each connection
// gets only the item of its user, as a message of type Type (F091-AC-14).
type Bundle struct {
	Type  string       `json:"type"`
	Ts    int64        `json:"ts,omitempty"`
	Items []BundleItem `json:"items"`
}

// BundleItem is the payload of one viewer.
type BundleItem struct {
	UserID  uuid.UUID       `json:"user_id"`
	Payload json.RawMessage `json:"payload"`
}

// Item returns the payload for user (nil when the bundle has none).
func (b *Bundle) Item(user uuid.UUID) json.RawMessage {
	if b == nil {
		return nil
	}
	for _, it := range b.Items {
		if it.UserID == user {
			return it.Payload
		}
	}
	return nil
}

// Target filters the local receivers of an envelope (nil: every local connection subscribed to the channel).
type Target struct {
	UserIDs       []uuid.UUID `json:"user_ids,omitempty"`
	GameSessionID *uuid.UUID  `json:"game_session_id,omitempty"`
	ConnID        string      `json:"conn_id,omitempty"`
	ExcludeConnID string      `json:"exclude_conn_id,omitempty"`
}

// Message is a server message without seq (the gateway assigns seq on delivery).
type Message struct {
	Type    string          `json:"type"`
	Ts      int64           `json:"ts,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// Control is a connection command: close with a code and a reason, or presence.update (area and room of the user,
// sent by the room host of module competition on ew:rt:user:<id>).
type Control struct {
	Action   string    `json:"action"` // close | presence.update
	Code     int       `json:"code,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	Presence *Presence `json:"presence,omitempty"`
}

// Presence is the area and room of a user (nil pointers: left the area).
type Presence struct {
	AreaID *string `json:"area_id"`
	RoomID *string `json:"room_id"`
}

// PresenceUpdate builds the control presence.update (area and room empty: the user left the hub).
func PresenceUpdate(area, room string) (Envelope, error) {
	p := &Presence{}
	if area != "" {
		p.AreaID = &area
	}
	if room != "" {
		p.RoomID = &room
	}
	e := Envelope{V: 1, Kind: KindControl, Control: &Control{Action: ActionPresenceUpdate, Presence: p}}
	return e, e.Validate()
}

var typeRe = regexp.MustCompile(`^[a-z_]+\.[a-z_]+\.[a-z_]+$`)

// ErrInvalidEnvelope is returned for an envelope that breaks the schema.
var ErrInvalidEnvelope = errors.New("realtime: invalid envelope")

// revocationReasons are the 4403 reasons (session.session.ended.reason, REALTIME_ARCHITECTURE §3.1).
var revocationReasons = map[string]bool{"account_locked": true, "access_revoked": true, "cleanup": true, "admin": true,
	"password_changed": true, "transferred": true}

// RevocationReason reports a reason that closes with 4403.
func RevocationReason(r string) bool { return revocationReasons[r] }

// Validate checks the envelope against the rules of the schema the Go types cannot express.
func (e Envelope) Validate() error {
	if e.V != 1 {
		return fmt.Errorf("%w: v", ErrInvalidEnvelope)
	}
	if t := e.Target; t != nil && (len(t.UserIDs) > 500 || len(t.ConnID) > 64 || len(t.ExcludeConnID) > 64) {
		return fmt.Errorf("%w: target", ErrInvalidEnvelope)
	}
	switch e.Kind {
	case KindMessage:
		m := e.Message
		if m == nil || !typeRe.MatchString(m.Type) || len(m.Type) > 64 || len(m.Payload) == 0 || m.Payload[0] != '{' {
			return fmt.Errorf("%w: message", ErrInvalidEnvelope)
		}
	case KindControl:
		if e.Control == nil {
			return fmt.Errorf("%w: control", ErrInvalidEnvelope)
		}
	case KindBundle:
		b := e.Bundle
		if b == nil || !typeRe.MatchString(b.Type) || len(b.Type) > 64 || len(b.Items) == 0 || len(b.Items) > 500 {
			return fmt.Errorf("%w: bundle", ErrInvalidEnvelope)
		}
		for _, it := range b.Items {
			if it.UserID == uuid.Nil || len(it.Payload) == 0 || it.Payload[0] != '{' {
				return fmt.Errorf("%w: bundle item", ErrInvalidEnvelope)
			}
		}
	default:
		return fmt.Errorf("%w: kind", ErrInvalidEnvelope)
	}
	if c := e.Control; c != nil {
		ok := c.Action == ActionClose && c.Presence == nil && ((c.Code == CloseRevoked && revocationReasons[c.Reason]) ||
			(c.Code == CloseReplaced && c.Reason == "replaced") ||
			(c.Code == CloseSessionDone && (c.Reason == "curfew" || c.Reason == "daily_limit")))
		if c.Action == ActionPresenceUpdate {
			p := c.Presence
			ok = p != nil && c.Code == 0 && c.Reason == "" && (p.AreaID == nil || len(*p.AreaID) <= 64) &&
				(p.RoomID == nil || len(*p.RoomID) <= 64)
		}
		if !ok {
			return fmt.Errorf("%w: control", ErrInvalidEnvelope)
		}
	}
	return nil
}

// NewMessage builds a kind=message envelope with ts = at (server time).
func NewMessage(typ string, payload any, at time.Time) (Envelope, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	e := Envelope{V: 1, Kind: KindMessage, Message: &Message{Type: typ, Ts: at.UnixMilli(), Payload: b}}
	return e, e.Validate()
}

// ForSession limits the envelope to the connection holding game session id (F090-AC-26).
func (e Envelope) ForSession(id uuid.UUID) Envelope {
	t := Target{}
	if e.Target != nil {
		t = *e.Target
	}
	t.GameSessionID = &id
	e.Target = &t
	return e
}

// Revoke builds the control that closes every connection of a user with 4403 after session.session.ended
// (reason ∈ account_locked, access_revoked, cleanup, admin, password_changed, transferred — F090-AC-24).
func Revoke(reason string) (Envelope, error) {
	e := Envelope{V: 1, Kind: KindControl, Control: &Control{Action: "close", Code: CloseRevoked, Reason: reason}}
	return e, e.Validate()
}

// Publisher publishes envelopes on Valkey Pub/Sub. Best effort (Pub/Sub has no delivery guarantee): callers
// publish after their state is durable (REALTIME_ARCHITECTURE §5).
type Publisher struct {
	C       valkey.Client
	Timeout time.Duration // per publish (default 2 s): a slow Valkey never blocks the caller for long
}

// Publish sends e on channel.
func (p Publisher) Publish(ctx context.Context, channel string, e Envelope) error {
	if err := e.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if err := p.C.Do(ctx, p.C.B().Publish().Channel(channel).Message(string(b)).Build()).Error(); err != nil {
		return fmt.Errorf("realtime: publish %s: %w", channel, err)
	}
	return nil
}

// ToUser publishes e on ew:rt:user:<user>.
func (p Publisher) ToUser(ctx context.Context, user uuid.UUID, e Envelope) error {
	return p.Publish(ctx, UserChannel(user), e)
}
