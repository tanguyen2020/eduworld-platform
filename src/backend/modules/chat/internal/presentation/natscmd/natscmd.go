// Package natscmd holds the NATS command handlers of module chat called by the realtime gateway (ew.cmd.chat.<type>,
// REALTIME_ARCHITECTURE §4.1, F090): thin adapters from the Ew-* headers and the payload to one use case. The tenant
// and user are those of the ticket (Ew-Tenant-Id, Ew-User-Id): only the NATS user of binary realtime may publish
// ew.cmd.> (SECURITY_ARCHITECTURE §8.1), never a client (R25).
package natscmd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Module and command types of the AsyncAPI served here (x-routing module chat).
const (
	Module          = "chat"
	TypeMessageSend = "chat.message.send"
	TypeChannelJoin = "chat.channel.join"
	TypeChannelLeft = "chat.channel.leave"
	TypeHistory     = "chat.history.fetch"
	TypeChannelSync = "chat.channel.sync"
)

// Sender is the send use case (command.Send).
type Sender interface {
	Send(ctx context.Context, s command.Sender, req message.Request) (command.SendResult, error)
}

// Reader is the read use cases of the WS commands (query.Chat).
type Reader interface {
	Join(ctx context.Context, v query.Viewer, id uuid.UUID, since *int64) (int, bool, error)
	Leave(ctx context.Context, v query.Viewer, id uuid.UUID) error
	Fetch(ctx context.Context, v query.Viewer, id uuid.UUID, before *int64, limit int) (int, bool, error)
	Sync(ctx context.Context, v query.Viewer, channels map[uuid.UUID]int64) ([]query.SyncResult, error)
}

func viewer(c rt.Caller) query.Viewer {
	return query.Viewer{TenantID: c.TenantID, UserID: c.UserID, ConnID: c.ConnID}
}

func invalid() rt.Reply { return rt.Fail(string(problem.ValidationFailed), false) }

// failure maps a use case error to the reply problem (AsyncAPI ErrorCode): Retry-After becomes retry_after_ms; unknown
// errors are INTERNAL, retryable (logged without content, R14).
func failure(ctx context.Context, err error) rt.Reply {
	var pe *problem.Error
	if !errors.As(err, &pe) {
		slog.ErrorContext(ctx, "chat command failed", "error", err)
		return rt.Fail("INTERNAL", true)
	}
	r := rt.Fail(string(pe.Code), pe.Retryable)
	var ra command.RetryAfter
	if errors.As(err, &ra) {
		r.Problem.RetryAfterMs = max(ra.After, time.Second).Milliseconds()
	}
	if c, ok := pe.Ext["category_code"].(string); ok && c != "" { // F116-AC-05: the category only, never a matched term
		r.Problem.Details = map[string]string{"category_code": c}
	}
	return r
}

// ackedResult is the chat.message.acked payload carried by system.command.acked.
func ackedResult(e message.Envelope) map[string]any {
	p := map[string]any{"message_id": e.MessageID, "channel_id": e.ChannelID, "seq": e.Seq, "moderation_state": string(e.Moderation.State)}
	if e.ClientMessageID != nil {
		p["client_message_id"] = *e.ClientMessageID
	}
	return p
}

type sendPayload struct {
	ChannelID        uuid.UUID         `json:"channel_id"`
	ClientMessageID  uuid.UUID         `json:"client_message_id"`
	Kind             string            `json:"kind"`
	Body             string            `json:"body"`
	QuickMessageID   string            `json:"quick_message_id"`
	Params           map[string]string `json:"params"`
	Emoji            string            `json:"emoji"`
	StickerID        *uuid.UUID        `json:"sticker_id"`
	GifID            *uuid.UUID        `json:"gif_id"`
	ReplyToMessageID *uuid.UUID        `json:"reply_to_message_id"`
	Mentions         []uuid.UUID       `json:"mentions"`
	Quote            *message.Quote    `json:"quote"`
}

// MessageSend handles ew.cmd.chat.message.send: the same use case as REST sendChannelMessage (F115-AC-05); the result
// of system.command.acked is the chat.message.acked payload.
func MessageSend(h Sender) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p sendPayload
		if err := json.Unmarshal(payload, &p); err != nil || p.ChannelID == uuid.Nil || p.ClientMessageID == uuid.Nil {
			return invalid()
		}
		res, err := h.Send(ctx, command.Sender{TenantID: c.TenantID, UserID: c.UserID, ConnID: c.ConnID}, message.Request{
			ChannelID: p.ChannelID, ClientMessageID: p.ClientMessageID, Kind: message.Kind(p.Kind), Body: p.Body, Emoji: p.Emoji,
			QuickMessageID: p.QuickMessageID, Params: p.Params, StickerID: p.StickerID, GifID: p.GifID, ReplyToMessageID: p.ReplyToMessageID,
			Mentions: p.Mentions, Quote: clientQuote(p.Quote)})
		if err != nil {
			return failure(ctx, err)
		}
		return rt.OK(ackedResult(res.Envelope))
	}
}

// ChannelJoin handles ew.cmd.chat.channel.join: membership checked (NOT_FOUND otherwise, F115-AC-10), the gateway
// subscribes the connection to ew:chat:fanout:<channel_id>; since_seq pushes the missing messages (F115-AC-19).
func ChannelJoin(h Reader) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			ChannelID uuid.UUID `json:"channel_id"`
			SinceSeq  *int64    `json:"since_seq"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.ChannelID == uuid.Nil || (p.SinceSeq != nil && *p.SinceSeq < 0) {
			return invalid()
		}
		n, more, err := h.Join(ctx, viewer(c), p.ChannelID, p.SinceSeq)
		if err != nil {
			return failure(ctx, err)
		}
		r := rt.OK(map[string]any{"channel_id": p.ChannelID, "count": n, "has_more": more})
		r.Subscribe = []string{rt.ChatFanoutChannel(p.ChannelID.String())}
		return r
	}
}

// ChannelLeave handles ew.cmd.chat.channel.leave: the gateway unsubscribes the connection (membership unchanged).
func ChannelLeave(h Reader) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			ChannelID uuid.UUID `json:"channel_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.ChannelID == uuid.Nil {
			return invalid()
		}
		if err := h.Leave(ctx, viewer(c), p.ChannelID); err != nil {
			return failure(ctx, err)
		}
		r := rt.OK(map[string]any{"channel_id": p.ChannelID})
		r.Unsubscribe = []string{rt.ChatFanoutChannel(p.ChannelID.String())}
		return r
	}
}

// HistoryFetch handles ew.cmd.chat.history.fetch: chat.history.loaded is pushed to the connection (F115-AC-18).
func HistoryFetch(h Reader) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			ChannelID uuid.UUID `json:"channel_id"`
			BeforeSeq *int64    `json:"before_seq"`
			Limit     int       `json:"limit"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.ChannelID == uuid.Nil || p.Limit < 0 || p.Limit > query.MaxFetchLimit {
			return invalid()
		}
		n, more, err := h.Fetch(ctx, viewer(c), p.ChannelID, p.BeforeSeq, p.Limit)
		if err != nil {
			return failure(ctx, err)
		}
		return rt.OK(map[string]any{"channel_id": p.ChannelID, "count": n, "has_more": more})
	}
}

// ChannelSync handles ew.cmd.chat.channel.sync (renamed from chat.sync, OQ F090-1): per channel the messages after
// last_seen_seq, at most chat_recent_buffer, pushed as chat.history.loaded (F115-AC-19).
func ChannelSync(h Reader) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var p struct {
			Channels map[uuid.UUID]int64 `json:"channels"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || len(p.Channels) == 0 || len(p.Channels) > query.MaxSyncChannels {
			return invalid()
		}
		for id, seq := range p.Channels {
			if id == uuid.Nil || seq < 0 {
				return invalid()
			}
		}
		res, err := h.Sync(ctx, viewer(c), p.Channels)
		if err != nil {
			return failure(ctx, err)
		}
		out := make([]map[string]any, len(res))
		for i, r := range res {
			m := map[string]any{"channel_id": r.ChannelID, "count": r.Count, "has_more": r.HasMore}
			if r.Code != "" {
				m["code"] = r.Code
			}
			out[i] = m
		}
		return rt.OK(map[string]any{"channels": out})
	}
}

// SendTimeout is the handling budget of ew.cmd.chat.message.send: the gateway waits nats_request_timeout (2 s); the
// PubAck wait is bounded inside it so CHAT_TEMPORARILY_UNAVAILABLE still reaches the client (F115-AC-03).
const SendTimeout = 2 * time.Second

// Serve answers every chat command of F115 until ctx ends (queue group chat: one core-api replica per request).
func Serve(ctx context.Context, nc *nats.Conn, send Sender, read Reader) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 5)
	serve := func(typ string, h rt.CommandHandler) {
		go func() { errs <- rt.ServeCommand(ctx, nc, Module, typ, h, SendTimeout) }()
	}
	serve(TypeMessageSend, MessageSend(send))
	serve(TypeChannelJoin, ChannelJoin(read))
	serve(TypeChannelLeft, ChannelLeave(read))
	serve(TypeHistory, HistoryFetch(read))
	serve(TypeChannelSync, ChannelSync(read))
	var first error
	for range 5 {
		if err := <-errs; err != nil && first == nil {
			first = err
			cancel()
		}
	}
	return first
}

// clientQuote keeps only the reference and bounds of a quote: the excerpt is always cut by the server (F118-AC-08).
func clientQuote(q *message.Quote) *message.Quote {
	if q == nil {
		return nil
	}
	return &message.Quote{MessageID: q.MessageID, Start: q.Start, End: q.End}
}
