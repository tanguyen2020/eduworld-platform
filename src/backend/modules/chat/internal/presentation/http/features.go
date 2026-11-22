package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MessagingCommands are the write use cases of the message features (command.Messaging, F118).
type MessagingCommands interface {
	Edit(ctx context.Context, s command.Sender, id uuid.UUID, body string, mentions []uuid.UUID, expected int) (command.EditResult, error)
	Recall(ctx context.Context, s command.Sender, id uuid.UUID) error
	Forward(ctx context.Context, s command.Sender, id uuid.UUID, targets []uuid.UUID, key string) ([]command.SendResult, error)
	ToggleReaction(ctx context.Context, s command.Sender, id uuid.UUID, emoji string) (port.ReactionResult, error)
	Pin(ctx context.Context, s command.Sender, ch, id uuid.UUID) error
	Unpin(ctx context.Context, s command.Sender, ch, id uuid.UUID) error
}

// FeatureQueries are the read use cases of the message features (query.Features, F118).
type FeatureQueries interface {
	ListPins(ctx context.Context, v query.Viewer, ch uuid.UUID) ([]query.PinView, error)
	Receipts(ctx context.Context, v query.Viewer, ch, id uuid.UUID) (query.ReceiptsView, error)
	StickerPacks(ctx context.Context, v query.Viewer) ([]read.StickerPackRow, error)
	Gifs(ctx context.Context, v query.Viewer, tag string, after *read.GifCursor, limit int) (query.GifPage, error)
}

// FeatureHandlers serve the F118 player operations (game_player + X-Tenant-Id; F118-AC-26). AssetURL turns an immutable
// library key into its public CDN URL.
type FeatureHandlers struct {
	Commands MessagingCommands
	Queries  FeatureQueries
	AssetURL func(key string) string
	Recorder func(ctx context.Context, tenant uuid.UUID, body any) // REST idempotency record (Send.RecordResult)
}

// FeatureOperations are the F118 player operations.
var FeatureOperations = []string{"editMessage", "recallMessage", "forwardMessage", "toggleMessageReaction", "pinMessage",
	"unpinMessage", "listChannelPins", "getMessageReceipts", "listStickerPacks", "listGifs"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "editMessage", Method: http.MethodPatch, Pattern: "/chat/messages/{message_id}", SuccessStatus: http.StatusOK},
		{ID: "recallMessage", Method: http.MethodPost, Pattern: "/chat/messages/{message_id}:recall", SuccessStatus: http.StatusNoContent},
		{ID: "forwardMessage", Method: http.MethodPost, Pattern: "/chat/messages/{message_id}:forward", SuccessStatus: http.StatusCreated,
			Idempotency: httpx.IdempotencyRequired},
		{ID: "toggleMessageReaction", Method: http.MethodPost, Pattern: "/chat/messages/{message_id}/reactions", SuccessStatus: http.StatusOK,
			Idempotency: httpx.IdempotencyRequired},
		{ID: "pinMessage", Method: http.MethodPut, Pattern: "/chat/channels/{channel_id}/pins/{message_id}", SuccessStatus: http.StatusNoContent},
		{ID: "unpinMessage", Method: http.MethodDelete, Pattern: "/chat/channels/{channel_id}/pins/{message_id}", SuccessStatus: http.StatusNoContent},
		{ID: "listChannelPins", Method: http.MethodGet, Pattern: "/chat/channels/{channel_id}/pins", SuccessStatus: http.StatusOK},
		{ID: "getMessageReceipts", Method: http.MethodGet, Pattern: "/chat/channels/{channel_id}/messages/{message_id}/receipts",
			SuccessStatus: http.StatusOK},
		{ID: "listStickerPacks", Method: http.MethodGet, Pattern: "/chat/stickers", SuccessStatus: http.StatusOK},
		{ID: "listGifs", Method: http.MethodGet, Pattern: "/chat/gifs", SuccessStatus: http.StatusOK},
	} {
		o.Module, o.Tenant, o.Guard = "chat", httpx.TenantRequired, httpx.GuardGamePlayer
		Operations[o.ID] = o
	}
}

// registerFeatures mounts the F118 operations.
func registerFeatures(s *httpx.Server, h *FeatureHandlers) error {
	if h.Commands == nil || h.Queries == nil || h.AssetURL == nil {
		return errors.New("chat: the F118 handlers are required")
	}
	routes := map[string]http.HandlerFunc{"editMessage": h.editMessage, "recallMessage": h.recallMessage, "forwardMessage": h.forwardMessage,
		"toggleMessageReaction": h.toggleReaction, "pinMessage": h.pin(true), "unpinMessage": h.pin(false), "listChannelPins": h.listPins,
		"getMessageReceipts": h.receipts, "listStickerPacks": h.stickers, "listGifs": h.gifs}
	for _, id := range FeatureOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func sender(r *http.Request) (command.Sender, error) {
	tenant, user, err := caller(r)
	return command.Sender{TenantID: tenant, UserID: user}, err
}

// renderView renders a Message of the contract with its F118 parts.
func renderView(v query.MessageView) map[string]any {
	m := Message(v.Envelope, v.Sender, v.Recalled)
	MessageFeatures(m, v.Envelope, v.Recalled, v.Extras())
	return m
}

// MessageFeatures adds the F118 parts of an OpenAPI Message: quote (excerpt left out when the original is gone —
// F118-AC-05), forward origin, edit time, reply excerpt, mentions (PlayerCard: nickname only, R16), reactions, pin.
func MessageFeatures(m map[string]any, e message.Envelope, recalled bool, x port.MessageExtras) {
	if e.EditedAt != nil {
		m["edited_at"] = e.EditedAt.UTC()
	}
	if e.ForwardedFrom != nil {
		m["forwarded_from"] = map[string]any{"message_id": e.ForwardedFrom.MessageID, "channel_type": e.ForwardedFrom.ChannelType,
			"sender_nickname": e.ForwardedFrom.SenderNickname}
	}
	if recalled {
		return
	}
	if e.Quote != nil {
		q := map[string]any{"message_id": e.Quote.MessageID, "excerpt_start": e.Quote.Start, "excerpt_end": e.Quote.End}
		if e.Quote.Excerpt != "" {
			q["excerpt"] = e.Quote.Excerpt
		}
		m["quote"] = q
	}
	if x.ReplyExcerpt != nil {
		m["reply_excerpt"] = *x.ReplyExcerpt
	}
	if len(x.Mentions) > 0 {
		ms := make([]map[string]any, len(x.Mentions))
		for i, c := range x.Mentions {
			ms[i] = card(query.Card{UserID: c.UserID, Nickname: c.Nickname, Level: max(c.Level, 1), Avatar: c.Avatar})
		}
		m["mentions"] = ms
	}
	if len(x.Reactions) > 0 {
		mine := x.MyReactions
		if mine == nil {
			mine = []string{}
		}
		m["reactions"] = map[string]any{"counts": x.Reactions, "mine": mine}
	}
	if x.Pinned {
		m["pinned"] = true
	}
}

// revisionOf parses If-Match (a revision, optionally a quoted or weak ETag).
func revisionOf(r *http.Request) (int, error) {
	v := strings.TrimSpace(r.Header.Get("If-Match"))
	if v == "" {
		return 0, problem.New(problem.PreconditionRequired, "")
	}
	v = strings.Trim(strings.TrimPrefix(v, "W/"), `"`)
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, problem.New(problem.VersionMismatch, "")
	}
	return n, nil
}

func (h FeatureHandlers) editMessage(w http.ResponseWriter, r *http.Request) {
	s, err := sender(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "message_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	rev, err := revisionOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Body     string      `json:"body"`
		Mentions []uuid.UUID `json:"mentions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("body", problem.SubInvalid, "invalid JSON"))
		return
	}
	res, err := h.Commands.Edit(r.Context(), s, id, b.Body, b.Mentions, rev)
	if err != nil {
		retryHeader(w, err)
		fail(w, r, err)
		return
	}
	m := Message(res.Envelope, res.Sender, false)
	MessageFeatures(m, res.Envelope, false, port.MessageExtras{})
	w.Header().Set("ETag", `"`+strconv.Itoa(res.Envelope.Revision)+`"`)
	writeJSON(w, http.StatusOK, m)
}

func retryHeader(w http.ResponseWriter, err error) {
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.ChatTemporarilyUnavailable {
		w.Header().Set("Retry-After", "1")
	}
	var ra command.RetryAfter
	if errors.As(err, &ra) {
		w.Header().Set("Retry-After", strconv.Itoa(int(max(ra.After, time.Second)/time.Second)))
	}
}

func (h FeatureHandlers) recallMessage(w http.ResponseWriter, r *http.Request) {
	s, err := sender(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "message_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Commands.Recall(r.Context(), s, id); err != nil {
		retryHeader(w, err)
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h FeatureHandlers) forwardMessage(w http.ResponseWriter, r *http.Request) {
	s, err := sender(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "message_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Targets []uuid.UUID `json:"target_channel_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("target_channel_ids", problem.SubInvalid, "invalid JSON"))
		return
	}
	res, err := h.Commands.Forward(r.Context(), s, id, b.Targets, r.Header.Get("Idempotency-Key"))
	if err != nil {
		retryHeader(w, err)
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(res))
	for i, x := range res {
		m := Message(x.Envelope, x.Sender, false)
		MessageFeatures(m, x.Envelope, false, port.MessageExtras{})
		data[i] = m
	}
	body := map[string]any{"data": data}
	if h.Recorder != nil {
		if _, ok := idempotency.PendingFrom(r.Context()); ok {
			h.Recorder(r.Context(), s.TenantID, body)
		}
	}
	writeJSON(w, http.StatusCreated, body)
}

func (h FeatureHandlers) toggleReaction(w http.ResponseWriter, r *http.Request) {
	s, err := sender(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "message_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reaction string `json:"reaction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("reaction", problem.SubInvalid, "invalid JSON"))
		return
	}
	res, err := h.Commands.ToggleReaction(r.Context(), s, id, b.Reaction)
	if err != nil {
		retryHeader(w, err)
		fail(w, r, err)
		return
	}
	mine := res.Mine
	if mine == nil {
		mine = []string{}
	}
	body := map[string]any{"message_id": id, "active": res.Set, "reactions": map[string]any{"counts": res.Counts, "mine": mine}}
	if h.Recorder != nil {
		h.Recorder(r.Context(), s.TenantID, body)
	}
	writeJSON(w, http.StatusOK, body)
}

func (h FeatureHandlers) pin(add bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := sender(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		ch, err := pathUUID(r, "channel_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		id, err := pathUUID(r, "message_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		if add {
			err = h.Commands.Pin(r.Context(), s, ch, id)
		} else {
			err = h.Commands.Unpin(r.Context(), s, ch, id)
		}
		if err != nil {
			retryHeader(w, err)
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h FeatureHandlers) listPins(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	ch, err := pathUUID(r, "channel_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	pins, err := h.Queries.ListPins(r.Context(), v, ch)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(pins))
	for i, p := range pins {
		data[i] = map[string]any{"message": renderView(p.Message), "pinned_at": p.PinnedAt.UTC(),
			"pinned_by": card(query.Card{UserID: p.PinnedBy.UserID, Nickname: p.PinnedBy.Nickname, Level: max(p.PinnedBy.Level, 1),
				Avatar: p.PinnedBy.Avatar})}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h FeatureHandlers) receipts(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	ch, err := pathUUID(r, "channel_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	id, err := pathUUID(r, "message_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Queries.Receipts(r.Context(), v, ch, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	body := map[string]any{"message_id": res.MessageID, "mode": res.Mode, "delivered_count": res.DeliveredCount, "read_count": res.ReadCount}
	if res.Mode == "per_member" {
		ms := make([]map[string]any, len(res.Members))
		for i, m := range res.Members {
			ms[i] = map[string]any{"status": m.Status, "member": card(query.Card{UserID: m.Member.UserID, Nickname: m.Member.Nickname,
				Level: max(m.Member.Level, 1), Avatar: m.Member.Avatar})}
		}
		body["members"] = ms
	}
	writeJSON(w, http.StatusOK, body)
}

// localized picks the English label when the caller asks for English (ADR-0056), else Vietnamese.
func localized(r *http.Request, vi string, en *string) string {
	if en != nil && *en != "" && strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
		return *en
	}
	return vi
}

func (h FeatureHandlers) stickers(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	packs, err := h.Queries.StickerPacks(r.Context(), v)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(packs))
	for i, p := range packs {
		ss := make([]map[string]any, len(p.Stickers))
		for j, s := range p.Stickers {
			ss[j] = map[string]any{"sticker_id": s.StickerID, "url": h.AssetURL(s.AssetKey), "alt_text": localized(r, s.AltVI, s.AltEN)}
		}
		m := map[string]any{"pack_id": p.PackID, "code": p.Code, "name": localized(r, p.NameVI, p.NameEN), "unlocked": p.Unlocked,
			"stickers": ss}
		if !p.Unlocked && p.HintVI != nil {
			m["unlock_hint"] = localized(r, *p.HintVI, p.HintEN)
		}
		data[i] = m
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h FeatureHandlers) gifs(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var after *read.GifCursor
	if c := r.URL.Query().Get("cursor"); c != "" {
		raw, err := base64.RawURLEncoding.DecodeString(c)
		var cur read.GifCursor
		if err != nil || json.Unmarshal(raw, &cur) != nil || cur.GifID == uuid.Nil {
			fail(w, r, problem.Invalid("cursor", problem.SubInvalid, "invalid cursor"))
			return
		}
		after = &cur
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	tag := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tag")))
	p, err := h.Queries.Gifs(r.Context(), v, tag, after, limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, g := range p.Data {
		data[i] = map[string]any{"gif_id": g.GifID, "url": h.AssetURL(g.AssetKey), "preview_url": h.AssetURL(g.PreviewKey),
			"width": g.Width, "height": g.Height, "tags": g.Tags}
	}
	var next any
	if p.Next != nil {
		b, _ := json.Marshal(p.Next)
		next = base64.RawURLEncoding.EncodeToString(b)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": map[string]any{"has_more": p.HasMore, "limit": p.Limit, "next_cursor": next}})
}
