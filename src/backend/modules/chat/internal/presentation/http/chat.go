package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ChatSender is the single send use case (command.Send), shared with ew.cmd.chat.message.send (F115-AC-05).
type ChatSender interface {
	Send(ctx context.Context, s command.Sender, req message.Request) (command.SendResult, error)
	RecordResult(ctx context.Context, tenant uuid.UUID, body any)
}

// ChatQueries are the read use cases of the chat core (query.Chat).
type ChatQueries interface {
	ListMyChannels(ctx context.Context, v query.Viewer) ([]query.ChannelView, error)
	ListChannelMessages(ctx context.Context, v query.Viewer, id uuid.UUID, before, after *int64, limit int) (query.MessagePage, error)
}

// QuickLister is listQuickMessages (query.QuickMessages, F116 US4).
type QuickLister interface {
	List(ctx context.Context, v query.Viewer, channelType string) ([]read.QuickMessageRow, error)
}

// ChatHandlers are the F115 use cases served over HTTP (and listQuickMessages of F116 when Quick is set).
type ChatHandlers struct {
	Send    ChatSender
	Queries ChatQueries
	Quick   QuickLister
}

// ChatOperations are the player routes of F115 (game_player + X-Tenant-Id, F115-AC-12, AC-13).
var ChatOperations = []string{"listMyChannels", "listChannelMessages", "sendChannelMessage"}

func init() {
	Operations["listMyChannels"] = httpx.Operation{ID: "listMyChannels", Module: "chat", Method: http.MethodGet, Pattern: "/chat/channels",
		Tenant: httpx.TenantRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer}
	Operations["listChannelMessages"] = httpx.Operation{ID: "listChannelMessages", Module: "chat", Method: http.MethodGet,
		Pattern: "/chat/channels/{channel_id}/messages", Tenant: httpx.TenantRequired, SuccessStatus: http.StatusOK,
		Guard: httpx.GuardGamePlayer}
	Operations["listQuickMessages"] = httpx.Operation{ID: "listQuickMessages", Module: "chat", Method: http.MethodGet,
		Pattern: "/chat/quick-messages", Tenant: httpx.TenantRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer}
	Operations["sendChannelMessage"] = httpx.Operation{ID: "sendChannelMessage", Module: "chat", Method: http.MethodPost,
		Pattern: "/chat/channels/{channel_id}/messages", Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired,
		SuccessStatus: http.StatusCreated, Guard: httpx.GuardGamePlayer}
}

// caller is the calling student of a player route (the game_player guard already refused anyone else, R23).
func caller(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(r.Context())
	if !ok || !p.Valid() {
		return uuid.Nil, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return *p.TenantID, p.UserID, nil
}

func viewerOf(r *http.Request) (query.Viewer, error) {
	tenant, user, err := caller(r)
	return query.Viewer{TenantID: tenant, UserID: user}, err
}

func seqParam(r *http.Request, name string) (*int64, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return nil, problem.Invalid(name, problem.SubInvalid, "must be a non-negative integer")
	}
	return &n, nil
}

// Message renders a Message of the contract: display content only (never the original body), sender as PlayerCard
// (nickname, level, avatar — no real name, school or class: R16, F115-AC-14).
func Message(e message.Envelope, sender read.ChatCard, recalled bool) map[string]any {
	c := query.Card{UserID: sender.UserID, Nickname: sender.Nickname, Level: max(sender.Level, 1), Avatar: sender.Avatar}
	m := map[string]any{"message_id": e.MessageID, "channel_id": e.ChannelID, "seq": e.Seq, "sender": card(c),
		"kind": string(e.MsgKind), "revision": max(e.Revision, 1), "moderation_state": string(e.Moderation.State),
		"created_at": e.CreatedAt.UTC(), "recalled": recalled, "body": nil}
	if !recalled && e.BodyDisplay != nil && (e.MsgKind == message.KindText || e.MsgKind == message.KindEmoji) {
		m["body"] = *e.BodyDisplay
	}
	if e.QuickMessageID != nil {
		m["quick_message_id"] = *e.QuickMessageID
	}
	if len(e.Params) > 0 {
		m["params"] = e.Params
	}
	if e.StickerID != nil {
		m["sticker_id"] = *e.StickerID
	}
	if e.GifID != nil {
		m["gif_id"] = *e.GifID
	}
	if e.ReplyToMessageID != nil {
		m["reply_to_message_id"] = *e.ReplyToMessageID
	}
	return m
}

func (h ChatHandlers) listMyChannels(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rows, err := h.Queries.ListMyChannels(r.Context(), v)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(rows))
	for i, c := range rows {
		data[i] = ChannelBody(c.Channel, c.ChannelCaps)
		data[i]["unread_count"], data[i]["my_read_seq"] = c.UnreadCount, c.MyReadSeq
		SelfFields(data[i], c.NotificationLevel, c.Peer)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h ChatHandlers) listChannelMessages(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "channel_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	before, err := seqParam(r, "before_seq")
	if err != nil {
		fail(w, r, err)
		return
	}
	after, err := seqParam(r, "after_seq")
	if err != nil {
		fail(w, r, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	p, err := h.Queries.ListChannelMessages(r.Context(), v, id, before, after, limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, m := range p.Data {
		data[i] = renderView(m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": map[string]any{"has_more": p.HasMore, "limit": p.Limit,
		"next_cursor": nil}})
}

type messageCreate struct {
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

func (h ChatHandlers) sendChannelMessage(w http.ResponseWriter, r *http.Request) {
	tenant, user, err := caller(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "channel_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b messageCreate
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.ClientMessageID == uuid.Nil {
		fail(w, r, problem.Invalid("client_message_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	res, err := h.Send.Send(r.Context(), command.Sender{TenantID: tenant, UserID: user}, message.Request{ChannelID: id,
		ClientMessageID: b.ClientMessageID, Kind: message.Kind(b.Kind), Body: b.Body, Emoji: b.Emoji, QuickMessageID: b.QuickMessageID,
		Params: b.Params, StickerID: b.StickerID, GifID: b.GifID, ReplyToMessageID: b.ReplyToMessageID, Mentions: b.Mentions, Quote: clientQuote(b.Quote)})
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.ChatTemporarilyUnavailable {
			w.Header().Set("Retry-After", "1")
		}
		fail(w, r, err)
		return
	}
	body := Message(res.Envelope, res.Sender, false)
	MessageFeatures(body, res.Envelope, false, port.MessageExtras{})
	h.Send.RecordResult(r.Context(), tenant, body)
	writeJSON(w, http.StatusCreated, body)
}

// registerChat mounts the F115 operations.
func registerChat(s *httpx.Server, h *ChatHandlers) error {
	if h == nil || h.Send == nil || h.Queries == nil {
		return errors.New("chat: the F115 handlers are required")
	}
	routes := map[string]http.HandlerFunc{"listMyChannels": h.listMyChannels, "listChannelMessages": h.listChannelMessages,
		"sendChannelMessage": h.sendChannelMessage}
	for _, id := range ChatOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	if h.Quick != nil {
		return s.Handle(Operations["listQuickMessages"], http.HandlerFunc(h.listQuickMessages))
	}
	return nil
}

// listQuickMessages renders QuickMessage: code, category, text {vi, en}, emoji, sort_order (F116-AC-27).
func (h ChatHandlers) listQuickMessages(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rows, err := h.Quick.List(r.Context(), v, r.URL.Query().Get("channel_type"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(rows))
	for i, q := range rows {
		m := map[string]any{"quick_message_id": q.ID, "category": q.Category, "text": map[string]string{"vi": q.TextVI, "en": q.TextEN},
			"sort_order": q.SortOrder}
		if q.Emoji != nil {
			m["emoji"] = *q.Emoji
		}
		data[i] = m
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// clientQuote keeps only the reference and bounds of a quote: the excerpt is always cut by the server (F118-AC-08).
func clientQuote(q *message.Quote) *message.Quote {
	if q == nil {
		return nil
	}
	return &message.Quote{MessageID: q.MessageID, Start: q.Start, End: q.End}
}
