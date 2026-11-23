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
	// Kinds returns the allowed kinds of a channel for the caller (query.Chat effective policy); nil: none listed.
	Kinds func(ctx context.Context, v query.Viewer, ch channel.Channel) []policy.Kind
}

// PersistentOperations are the F120 player operations.
var PersistentOperations = []string{"openDirectChannel", "searchMessages"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "openDirectChannel", Method: http.MethodPost, Pattern: "/chat/direct-channels", SuccessStatus: http.StatusOK},
		{ID: "searchMessages", Method: http.MethodGet, Pattern: "/chat/search", SuccessStatus: http.StatusOK},
	} {
		o.Module, o.Tenant, o.Guard = "chat", httpx.TenantRequired, httpx.GuardGamePlayer
		Operations[o.ID] = o
	}
}

func registerPersistent(s *httpx.Server, h *PersistentHandlers) error {
	if h.Direct == nil || h.Search == nil {
		return errors.New("chat: the persistent channel handlers are required")
	}
	routes := map[string]http.HandlerFunc{"openDirectChannel": h.openDirect, "searchMessages": h.search}
	for _, id := range PersistentOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

// ChannelBody renders a Channel of the contract: no voice (capabilities.voice false, FR-CHAT-007, F120-AC-16).
func ChannelBody(c channel.Channel, kinds []policy.Kind) map[string]any {
	ks := make([]string, len(kinds))
	for i, k := range kinds {
		ks[i] = string(k)
	}
	m := map[string]any{"channel_id": c.ID, "channel_type": string(c.Type), "allowed_kinds": ks, "last_seq": c.LastSeq,
		"status": c.Status, "capabilities": map[string]any{"voice": false}}
	if c.PurgeAfter != nil {
		m["purge_after"] = c.PurgeAfter.UTC()
	}
	return m
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
	var kinds []policy.Kind
	if h.Kinds != nil {
		kinds = h.Kinds(r.Context(), v, res.Channel)
	}
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, ChannelBody(res.Channel, kinds))
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
		if body, ok := msg["body"].(string); ok && body != "" {
			item["highlight"] = body
		}
		data[i] = item
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}
