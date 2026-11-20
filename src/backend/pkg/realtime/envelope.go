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

// Control is a connection command (close with a code and a reason).
type Control struct {
	Action string `json:"action"` // close
	Code   int    `json:"code"`
	Reason string `json:"reason"`
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
	default:
		return fmt.Errorf("%w: kind", ErrInvalidEnvelope)
	}
	if c := e.Control; c != nil {
		ok := c.Action == "close" && ((c.Code == CloseRevoked && revocationReasons[c.Reason]) ||
			(c.Code == CloseReplaced && c.Reason == "replaced") ||
			(c.Code == CloseSessionDone && (c.Reason == "curfew" || c.Reason == "daily_limit")))
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
