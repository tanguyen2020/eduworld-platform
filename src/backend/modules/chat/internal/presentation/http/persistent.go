package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DirectOpener is openDirectChannel (command.Persistent).
type DirectOpener interface {
	OpenDirect(ctx context.Context, other uuid.UUID) (command.DirectResult, error)
}

// Searcher is searchMessages (query.Search).
type Searcher interface {
	Search(ctx context.Context, v query.Viewer, in query.SearchInput) (query.SearchPage, error)
}

// PersistentHandlers serve the F120 player operations openDirectChannel and searchMessages (game_player + X-Tenant-Id).
type PersistentHandlers struct {
	Direct DirectOpener
	Search Searcher
	// Caps returns what the caller may do in a channel (query.Chat.Caps: effective policy and sanctions); nil: none listed.
	Caps func(ctx context.Context, v query.Viewer, ch channel.Channel) query.ChannelCaps
	// Peer returns the public card of the other student of a direct channel (query.Chat.Peer, delta F112-5); nil: none.
	Peer func(ctx context.Context, v query.Viewer, ch channel.Channel) *read.ChatCard
	// Settings is updateChannelMembership (command.ChannelSettings, delta F112-9).
	Settings ChannelSettingsSetter
}

// ChannelSettingsSetter is updateChannelMembership (command.ChannelSettings).
type ChannelSettingsSetter interface {
	SetNotificationLevel(ctx context.Context, ch uuid.UUID, level string) (command.ChannelMembership, error)
}

// PersistentOperations are the F120 player operations.
var PersistentOperations = []string{"openDirectChannel", "searchMessages", "updateChannelMembership"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "openDirectChannel", Method: http.MethodPost, Pattern: "/chat/direct-channels", SuccessStatus: http.StatusOK},
		{ID: "searchMessages", Method: http.MethodGet, Pattern: "/chat/search", SuccessStatus: http.StatusOK},
		{ID: "updateChannelMembership", Method: http.MethodPatch, Pattern: "/chat/channels/{channel_id}/membership",
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
	} {
		o.Module, o.Tenant, o.Guard = "chat", httpx.TenantRequired, httpx.GuardGamePlayer
		Operations[o.ID] = o
	}
}

func registerPersistent(s *httpx.Server, h *PersistentHandlers) error {
	if h.Direct == nil || h.Search == nil || h.Settings == nil {
		return errors.New("chat: the persistent channel handlers are required")
	}
	routes := map[string]http.HandlerFunc{"openDirectChannel": h.openDirect, "searchMessages": h.search,
		"updateChannelMembership": h.updateMembership}
	for _, id := range PersistentOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

// ChannelBody renders a Channel of the contract: no voice (capabilities.voice false, FR-CHAT-007, F120-AC-16); the kinds
// of the effective policy and, when the policy allows the channel, what the caller may use now — sendable_kinds,
// max_text_length and features (delta F119, CHAT_SYSTEM §7.1); purge_after shown to the student (F117-AC-14).
func ChannelBody(c channel.Channel, caps query.ChannelCaps) map[string]any {
	m := map[string]any{"channel_id": c.ID, "channel_type": string(c.Type), "allowed_kinds": kindNames(caps.AllowedKinds),
		"sendable_kinds": kindNames(caps.SendableKinds), "last_seq": c.LastSeq, "status": c.Status,
		"capabilities": map[string]any{"voice": false}}
	if caps.Allowed {
		if caps.MaxTextLength > 0 {
			m["max_text_length"] = caps.MaxTextLength
		}
		m["features"] = features(caps.Features)
	}
	if c.PurgeAfter != nil {
		m["purge_after"] = c.PurgeAfter.UTC()
	}
	return m
}

// SelfFields adds what a channel tells the caller about itself and the other side (listMyChannels): notification_level
// and muted where the caller has a membership (delta F112-9), the peer card of a direct channel (delta F112-5, R16).
func SelfFields(m map[string]any, level string, peer *read.ChatCard) {
	if level != "" {
		m["notification_level"], m["muted"] = level, level == command.NotificationNone
	}
	if peer != nil {
		m["peer"] = card(query.Card{UserID: peer.UserID, Nickname: peer.Nickname, Level: max(peer.Level, 1), Avatar: peer.Avatar})
	}
}

func kindNames(kinds []policy.Kind) []string {
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}

func (h PersistentHandlers) openDirect(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.UserID == uuid.Nil {
		fail(w, r, problem.Invalid("user_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	res, err := h.Direct.OpenDirect(r.Context(), b.UserID)
	if err != nil {
		fail(w, r, err)
		return
	}
	caps := query.ChannelCaps{}
	if h.Caps != nil {
		caps = h.Caps(r.Context(), v, res.Channel)
	}
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	body := ChannelBody(res.Channel, caps)
	if h.Peer != nil {
		SelfFields(body, "", h.Peer(r.Context(), v, res.Channel))
	}
	writeJSON(w, status, body)
}

func (h PersistentHandlers) updateMembership(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "channel_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		NotificationLevel string `json:"notification_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.NotificationLevel == "" {
		fail(w, r, problem.Invalid("notification_level", problem.SubRequired, "all, mentions or none"))
		return
	}
	out, err := h.Settings.SetNotificationLevel(r.Context(), id, b.NotificationLevel)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out.Body())
}

func optTime(r *http.Request, name string) (*time.Time, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, problem.Invalid(name, problem.SubInvalid, "must be a timestamp")
	}
	return &t, nil
}

func (h PersistentHandlers) search(w http.ResponseWriter, r *http.Request) {
	v, err := viewerOf(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := query.SearchInput{Q: r.URL.Query().Get("q"), Cursor: r.URL.Query().Get("cursor")}
	in.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if in.Channel, err = optUUID(r, "channel_id"); err != nil {
		fail(w, r, err)
		return
	}
	if in.Sender, err = optUUID(r, "sender_id"); err != nil {
		fail(w, r, err)
		return
	}
	if in.From, err = optTime(r, "from"); err != nil {
		fail(w, r, err)
		return
	}
	if in.To, err = optTime(r, "to"); err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Search.Search(r.Context(), v, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, hit := range p.Data {
		msg := Message(hit.Message.Envelope, hit.Sender, hit.Message.Recalled)
		item := map[string]any{"message": msg, "channel_id": hit.Message.Envelope.ChannelID, "channel_type": hit.ChannelType}
		if hit.Highlight != "" { // plain text + code point ranges, no markup (delta F112-8)
			item["highlight"] = hit.Highlight
			if len(hit.Ranges) > 0 {
				ranges := make([]map[string]int, len(hit.Ranges))
				for j, rg := range hit.Ranges {
					ranges[j] = map[string]int{"start": rg.Start, "length": rg.Length}
				}
				item["highlight_ranges"] = ranges
			}
		}
		data[i] = item
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}
