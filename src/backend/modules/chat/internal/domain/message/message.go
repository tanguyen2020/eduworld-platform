// Package message holds the chat message of module chat (F115, CHAT_SYSTEM §5, EVENT_DESIGN §4.10): the moderation
// decision and its state, the request of one send and the envelope written to the durable stream before "Sent".
package message

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Kind is the kind of a message sent by a student (system messages are server-made only).
type Kind string

// Kinds of chat.message.send / MessageCreate.
const (
	KindText    Kind = "text"
	KindEmoji   Kind = "emoji"
	KindQuick   Kind = "quick"
	KindSticker Kind = "sticker"
	KindGif     Kind = "gif"
)

// Valid reports a sendable kind.
func (k Kind) Valid() bool {
	switch k {
	case KindText, KindEmoji, KindQuick, KindSticker, KindGif:
		return true
	}
	return false
}

// Decision is the result of the moderation pipeline (MODERATION §3; implemented by F116).
type Decision string

// Decisions.
const (
	Allow         Decision = "allow"
	Unchecked     Decision = "unchecked"
	Mask          Decision = "mask"
	Block         Decision = "block"
	HoldForReview Decision = "hold_for_review"
	BlockAndAlert Decision = "block_and_alert"
)

// State is moderation_state of a stored message.
type State string

// States.
const (
	StateAllowed   State = "allowed"
	StateMasked    State = "masked"
	StateUnchecked State = "unchecked"
	StateHeld      State = "held"
	StateRemoved   State = "removed"
)

// ErrDecision: the validator returned an unknown decision (the send fails closed).
var ErrDecision = errors.New("message: unknown moderation decision")

// Outcome maps a decision to the stored state and whether the message is delivered to the channel. A block is never
// stored nor published (stored false).
func (d Decision) Outcome() (state State, deliver, stored bool, err error) {
	switch d {
	case Allow:
		return StateAllowed, true, true, nil
	case Unchecked:
		return StateUnchecked, true, true, nil
	case Mask:
		return StateMasked, true, true, nil
	case HoldForReview, BlockAndAlert:
		return StateHeld, false, true, nil
	case Block:
		return "", false, false, nil
	}
	return "", false, false, ErrDecision
}

// Request is the content of one send (WS chat.message.send or REST sendChannelMessage).
type Request struct {
	ChannelID        uuid.UUID
	ClientMessageID  uuid.UUID
	Kind             Kind
	Body             string
	Emoji            string
	QuickMessageID   string
	Params           map[string]string
	StickerID        *uuid.UUID
	GifID            *uuid.UUID
	ReplyToMessageID *uuid.UUID
	Mentions         []uuid.UUID
}

// Errors of a request (VALIDATION_FAILED).
var (
	ErrContent  = errors.New("message: the content does not match the kind")
	ErrTooLong  = errors.New("message: body longer than the policy allows")
	ErrMentions = errors.New("message: at most 5 mentions")
)

// MaxMentions per message (DATABASE_DESIGN §3.8 CHECK).
const MaxMentions = 5

// Text returns what the pipeline checks and what is stored as body: the text, the emoji code, or nothing.
func (r Request) Text() string {
	switch r.Kind {
	case KindText:
		return r.Body
	case KindEmoji:
		return r.Emoji
	}
	return ""
}

// Validate checks the content of the request against its kind and maxText (max_text_length of the effective policy).
func (r Request) Validate(maxText int) error {
	if r.ChannelID == uuid.Nil || r.ClientMessageID == uuid.Nil || !r.Kind.Valid() {
		return ErrContent
	}
	if len(r.Mentions) > MaxMentions {
		return ErrMentions
	}
	switch r.Kind {
	case KindText:
		if strings.TrimSpace(r.Body) == "" {
			return ErrContent
		}
		if maxText > 0 && utf8.RuneCountInString(r.Body) > maxText {
			return ErrTooLong
		}
	case KindEmoji:
		if r.Emoji == "" || utf8.RuneCountInString(r.Emoji) > 64 {
			return ErrContent
		}
	case KindQuick:
		if r.QuickMessageID == "" || len(r.QuickMessageID) > 64 {
			return ErrContent
		}
		if !validParams(r.Params) {
			return ErrContent
		}
	default:
		if len(r.Params) > 0 {
			return ErrContent
		}
	}
	switch r.Kind {
	case KindSticker:
		if r.StickerID == nil || *r.StickerID == uuid.Nil {
			return ErrContent
		}
	case KindGif:
		if r.GifID == nil || *r.GifID == uuid.Nil {
			return ErrContent
		}
	}
	return nil
}

// Moderation is the decision part of an envelope (no matched term, position or catalog content: ADR-0030).
type Moderation struct {
	Decision             Decision   `json:"decision"`
	State                State      `json:"state"`
	CategoryCodes        []string   `json:"category_codes,omitempty"`
	StepsRun             []string   `json:"steps_run,omitempty"`
	StepsSkipped         []string   `json:"steps_skipped,omitempty"`
	CatalogVersion       *int64     `json:"catalog_version,omitempty"`
	CatalogVersionTenant *int64     `json:"catalog_version_tenant,omitempty"`
	SwitchSnapshotID     *uuid.UUID `json:"switch_snapshot_id,omitempty"`
	LatencyMS            int        `json:"latency_ms,omitempty"`
}

// Envelope is ChatEnvelope v1 (specs/events/chat-envelope.v1.schema.json).
type Envelope struct {
	V                int               `json:"v"`
	Kind             string            `json:"kind"`
	MessageID        uuid.UUID         `json:"message_id"`
	TenantID         uuid.UUID         `json:"tenant_id"`
	SchoolID         *uuid.UUID        `json:"school_id"`
	ChannelID        uuid.UUID         `json:"channel_id"`
	ChannelType      string            `json:"channel_type"`
	IsGameplay       bool              `json:"is_gameplay"`
	Seq              int64             `json:"seq"`
	Revision         int               `json:"revision"`
	SenderID         uuid.UUID         `json:"sender_id"`
	ClientMessageID  *uuid.UUID        `json:"client_message_id"`
	MsgKind          Kind              `json:"msg_kind"`
	BodyDisplay      *string           `json:"body_display"`
	BodyOriginal     *string           `json:"body_original"`
	QuickMessageID   *string           `json:"quick_message_id"`
	Params           map[string]string `json:"params,omitempty"`
	StickerID        *uuid.UUID        `json:"sticker_id"`
	GifID            *uuid.UUID        `json:"gif_id"`
	ReplyToMessageID *uuid.UUID        `json:"reply_to_message_id"`
	Mentions         []uuid.UUID       `json:"mentions"`
	Moderation       Moderation        `json:"moderation"`
	CreatedAt        time.Time         `json:"created_at"`
	TraceID          *string           `json:"trace_id"`
}

// EnvelopeKind is envelope.kind of a new message.
const EnvelopeKind = "msg"

// ErrEnvelope: an envelope read from the stream breaks the schema.
var ErrEnvelope = errors.New("message: invalid envelope")

// Check validates an envelope read back from the stream (persister, recent buffer).
func (e Envelope) Check() error {
	if e.V != 1 || e.Kind != EnvelopeKind || e.MessageID == uuid.Nil || e.TenantID == uuid.Nil || e.ChannelID == uuid.Nil ||
		e.SenderID == uuid.Nil || e.Seq < 1 || !e.MsgKind.Valid() || e.CreatedAt.IsZero() || len(e.Mentions) > MaxMentions {
		return ErrEnvelope
	}
	if _, _, stored, err := e.Moderation.Decision.Outcome(); err != nil || !stored {
		return ErrEnvelope
	}
	for _, s := range []*string{e.BodyDisplay, e.BodyOriginal} {
		if s != nil && utf8.RuneCountInString(*s) > 1000 {
			return ErrEnvelope
		}
	}
	return nil
}

// Body returns the stored original body (evidence): body_original when present, else body_display.
func (e Envelope) Body() *string {
	if e.BodyOriginal != nil {
		return e.BodyOriginal
	}
	return e.BodyDisplay
}

// Visible reports whether a reader may see the message: delivered states to every member, held only to its sender.
func (e Envelope) Visible(reader uuid.UUID) bool {
	switch e.Moderation.State {
	case StateAllowed, StateMasked, StateUnchecked:
		return true
	case StateHeld:
		return reader == e.SenderID
	}
	return false
}

// MaxParams is the largest number of quick message parameters (MessageCreate.params maxProperties).
const MaxParams = 5

// validParams checks quick message parameters: ≤ 5, key [a-z][a-z0-9_]{0,31}, value 1–64 characters.
func validParams(p map[string]string) bool {
	if len(p) > MaxParams {
		return false
	}
	for k, v := range p {
		if k == "" || len(k) > 32 || k[0] < 'a' || k[0] > 'z' {
			return false
		}
		for _, c := range k {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
				return false
			}
		}
		if n := utf8.RuneCountInString(v); n == 0 || n > 64 {
			return false
		}
	}
	return true
}
