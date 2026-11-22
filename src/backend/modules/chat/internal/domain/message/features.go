package message

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Quote is MessageCreate.quote / chat.messages.quote (CHAT_SYSTEM §7.4): the server cuts Excerpt (≤ 200 characters) from
// the display (moderated) content of the quoted message at send time; it never takes quoted text from the client and the
// excerpt never follows a later edit (immutable).
type Quote struct {
	MessageID uuid.UUID `json:"message_id"`
	Start     int       `json:"excerpt_start"`
	End       int       `json:"excerpt_end"`
	Excerpt   string    `json:"excerpt"`
}

// ForwardedFrom is chat.messages.forwarded_from of a forwarded message (CHAT_SYSTEM §7.4): nickname of the game profile,
// never a login name (R16).
type ForwardedFrom struct {
	MessageID      uuid.UUID `json:"message_id"`
	ChannelType    string    `json:"channel_type"`
	SenderNickname string    `json:"sender_nickname"`
}

// Excerpt limits (CHAT_SYSTEM §7.4, §7.7).
const (
	QuoteExcerptMax  = 200
	ReplyExcerptMax  = 80
	MentionPreviewMx = 80
)

// Excerpt cuts the runes [start, end) of display, at most limit runes (end ≤ start or past the text: to the end).
func Excerpt(display string, start, end, limit int) string {
	r := []rune(display)
	start = min(max(start, 0), len(r))
	if end <= start || end > len(r) {
		end = len(r)
	}
	if end-start > limit {
		end = start + limit
	}
	return strings.TrimSpace(string(r[start:end]))
}

// Preview is the first limit runes of display.
func Preview(display string, limit int) string { return Excerpt(display, 0, 0, limit) }

// Errors of the message features (mapped to the contract by the use cases).
var (
	ErrNotSender     = errors.New("message: only the sender changes a message")
	ErrNotEditable   = errors.New("message: only a delivered text message is edited")
	ErrEditWindow    = errors.New("message: edit window expired")
	ErrRecallWindow  = errors.New("message: recall window expired")
	ErrRecalled      = errors.New("message: already recalled")
	ErrNotAvailable  = errors.New("message: the original is not available (recalled, removed, held or masked)")
	ErrRevision      = errors.New("message: revision_expected does not match")
	ErrMentionsLimit = errors.New("message: too many mentions")
)

// Recalled reports a recalled message.
func (e Envelope) Recalled() bool { return e.RecalledAt != nil }

// Delivered reports a message every member sees (allowed, masked, unchecked) and not recalled.
func (e Envelope) Delivered() bool {
	switch e.Moderation.State {
	case StateAllowed, StateMasked, StateUnchecked:
		return !e.Recalled()
	}
	return false
}

// CanEdit checks an edit of e by user at now (CHAT_SYSTEM §7.2): the sender, a delivered text message, within window
// from created_at (server clock).
func (e Envelope) CanEdit(user uuid.UUID, now time.Time, window time.Duration) error {
	switch {
	case e.SenderID != user:
		return ErrNotSender
	case e.MsgKind != KindText || !e.Delivered():
		return ErrNotEditable
	case now.Sub(e.CreatedAt) > window:
		return ErrEditWindow
	}
	return nil
}

// CanRecall checks a recall of e by user at now (CHAT_SYSTEM §7.3): the sender, within window, not recalled yet.
func (e Envelope) CanRecall(user uuid.UUID, now time.Time, window time.Duration) error {
	switch {
	case e.SenderID != user:
		return ErrNotSender
	case e.Recalled():
		return ErrRecalled
	case e.Moderation.State == StateRemoved:
		return ErrNotAvailable
	case now.Sub(e.CreatedAt) > window:
		return ErrRecallWindow
	}
	return nil
}

// Forwardable reports a message that may leave its channel (CHAT_SYSTEM §7.4): delivered as is — never recalled,
// masked, removed or held (F118-AC-10).
func (e Envelope) Forwardable() error {
	if e.Recalled() || e.Moderation.State != StateAllowed && e.Moderation.State != StateUnchecked {
		return ErrNotAvailable
	}
	return nil
}

// Quotable reports a message whose display content may be quoted or previewed (reply): delivered, not recalled.
func (e Envelope) Quotable() bool { return e.Delivered() && e.BodyDisplay != nil }

// Edited returns e after an applied edit: the next revision with the new display/original body and mentions.
func (e Envelope) Edited(display string, original *string, mentions []uuid.UUID, mod Moderation, now time.Time) Envelope {
	out := e
	out.Revision = max(e.Revision, 1) + 1
	out.EditCount = e.EditCount + 1
	d := display
	out.BodyDisplay, out.BodyOriginal = &d, original
	out.Mentions = append([]uuid.UUID{}, mentions...)
	out.Moderation = mod
	t := now.UTC()
	out.EditedAt = &t
	return out
}

// WithRecall returns e after a recall: the next revision, recalled_at/by set and no content in the copy (the original
// stays in PostgreSQL as evidence, written by chat-persister — ADR-0028, R25).
func (e Envelope) WithRecall(by uuid.UUID, now time.Time) Envelope {
	out := e
	out.Revision = max(e.Revision, 1) + 1
	t := now.UTC()
	b := by
	out.RecalledAt, out.RecalledBy = &t, &b
	out.BodyDisplay, out.BodyOriginal, out.Quote = nil, nil, nil
	return out
}

// FilterMentions keeps the mentions of a message that may be notified (CHAT_SYSTEM §7.7, F118-AC-18): active members
// other than the sender with no block either way, each once, in order. Invalid ids are dropped silently (the message is
// still sent). More than limit ids is ErrMentionsLimit (CHAT_MENTION_LIMIT).
func FilterMentions(ids []uuid.UUID, sender uuid.UUID, limit int, member func(uuid.UUID) bool, blocked func(a, b uuid.UUID) bool) ([]uuid.UUID, error) {
	if len(ids) > limit {
		return nil, ErrMentionsLimit
	}
	out := make([]uuid.UUID, 0, len(ids))
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if id == uuid.Nil || id == sender || seen[id] || !member(id) || blocked(id, sender) {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

// Change ops of the durable stream (subject ew.chat.<tenant_id>.evt.<channel_id>, EVENT_DESIGN §4.10).
const (
	OpEdit          = "edit"
	OpEditHeld      = "edit_held"
	OpRecall        = "recall"
	OpReactionSet   = "reaction_set"
	OpReactionUnset = "reaction_unset"
	OpPin           = "pin"
	OpUnpin         = "unpin"
)

// ChangeKind is Change.kind.
const ChangeKind = "evt"

// Previous is the version an edit replaces (written into chat.message_revisions by chat-persister as evidence,
// whatever the order the changes reach it).
type Previous struct {
	Revision     int         `json:"revision"`
	BodyDisplay  *string     `json:"body_display"`
	BodyOriginal *string     `json:"body_original"`
	Mentions     []uuid.UUID `json:"mentions"`
	Decision     Decision    `json:"decision"`
}

// Change is ChatChange v1 (specs/events/payloads/chat/chat-evt.v1.schema.json): one change of a message published to
// the durable stream before anyone is told about it (R25, ADR-0035); chat-persister applies it by revision.
type Change struct {
	V            int         `json:"v"`
	Kind         string      `json:"kind"`
	Op           string      `json:"op"`
	ChangeID     uuid.UUID   `json:"change_id"`
	TenantID     uuid.UUID   `json:"tenant_id"`
	ChannelID    uuid.UUID   `json:"channel_id"`
	ChannelType  string      `json:"channel_type"`
	IsGameplay   bool        `json:"is_gameplay"`
	MessageID    uuid.UUID   `json:"message_id"`
	ActorID      uuid.UUID   `json:"actor_id"`
	Revision     int         `json:"revision,omitempty"`
	BodyDisplay  *string     `json:"body_display,omitempty"`
	BodyOriginal *string     `json:"body_original,omitempty"`
	Mentions     []uuid.UUID `json:"mentions,omitempty"`
	Moderation   *Moderation `json:"moderation,omitempty"`
	Previous     *Previous   `json:"previous,omitempty"`
	Emoji        string      `json:"emoji_code,omitempty"`
	At           time.Time   `json:"at"`
	TraceID      *string     `json:"trace_id,omitempty"`
}

// ErrChange: a change read from the stream breaks the schema.
var ErrChange = errors.New("message: invalid change")

// Check validates a change read back from the stream (chat-persister).
func (c Change) Check() error {
	if c.V != 1 || c.Kind != ChangeKind || c.ChangeID == uuid.Nil || c.TenantID == uuid.Nil || c.ChannelID == uuid.Nil ||
		c.MessageID == uuid.Nil || c.ActorID == uuid.Nil || c.At.IsZero() || len(c.Mentions) > MaxMentions {
		return ErrChange
	}
	for _, s := range []*string{c.BodyDisplay, c.BodyOriginal} {
		if s != nil && utf8.RuneCountInString(*s) > 1000 {
			return ErrChange
		}
	}
	switch c.Op {
	case OpEdit, OpEditHeld:
		if c.Revision < 2 || c.BodyDisplay == nil || c.Moderation == nil || c.Previous == nil {
			return ErrChange
		}
	case OpRecall:
		if c.Revision < 2 {
			return ErrChange
		}
	case OpReactionSet, OpReactionUnset:
		if !ValidEmojiCode(c.Emoji) {
			return ErrChange
		}
	case OpPin, OpUnpin:
	default:
		return ErrChange
	}
	return nil
}

// ValidEmojiCode reports a reaction/emoji code of the allowed-set format (^[a-z0-9_]{1,64}$).
func ValidEmojiCode(code string) bool {
	if code == "" || len(code) > 64 {
		return false
	}
	for _, c := range code {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}
