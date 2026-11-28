package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PolicyCommands are the write use cases of the chat policy levels (command.PolicyAdmin).
type PolicyCommands interface {
	PutLevel(ctx context.Context, scopeType string, scopeID uuid.UUID, ifMatch int64, in command.PolicyInput) (command.PolicyResult, error)
	PutPlatform(ctx context.Context, ifMatch int64, in command.PolicyInput) (command.PolicyResult, error)
	PutCommonOverride(ctx context.Context, in command.CommonOverrideInput) (command.CommonOverrideResult, error)
}

// PolicyQueries are the read use cases of the chat policy levels (query.Policies).
type PolicyQueries interface {
	GetLevel(ctx context.Context, scopeType string, scopeID uuid.UUID) (read.PolicyRow, error)
	GetPlatform(ctx context.Context) (read.PolicyRow, error)
	GetEffective(ctx context.Context, school uuid.UUID, channelType string) (query.EffectiveView, error)
	GetCommonOverride(ctx context.Context) (read.CommonOverrideRow, error)
}

// PolicyHandlers serve the chat policy operations (F120 US2). /tenant/* routes: portal staff (httpx adds GuardPortalStaff);
// /admin/* routes: internal gateway, httpx adds GuardPortalStaff + GuardPlatformRole. Permissions in the use cases (R7).
type PolicyHandlers struct {
	Commands PolicyCommands
	Queries  PolicyQueries
}

// PolicyOperations are the F120 chat policy operations (checked against the spec by the contract test).
var PolicyOperations = []string{"getChatPolicy", "putChatPolicy", "getEffectiveChatPolicy", "getPlatformChatPolicy",
	"putPlatformChatPolicy", "getCommonTenantSocialOverride", "putCommonTenantSocialOverride"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "getChatPolicy", Method: http.MethodGet, Pattern: "/tenant/chat-policies/{scope_type}/{scope_id}", Tenant: httpx.TenantRequired,
			SuccessStatus: http.StatusOK},
		{ID: "putChatPolicy", Method: http.MethodPut, Pattern: "/tenant/chat-policies/{scope_type}/{scope_id}", Tenant: httpx.TenantRequired,
			SuccessStatus: http.StatusOK},
		{ID: "getEffectiveChatPolicy", Method: http.MethodGet, Pattern: "/tenant/chat-policies:effective", Tenant: httpx.TenantRequired,
			SuccessStatus: http.StatusOK},
		{ID: "getPlatformChatPolicy", Method: http.MethodGet, Pattern: "/admin/chat/policies/platform", Tenant: httpx.TenantNone,
			SuccessStatus: http.StatusOK},
		{ID: "putPlatformChatPolicy", Method: http.MethodPut, Pattern: "/admin/chat/policies/platform", Tenant: httpx.TenantNone,
			SuccessStatus: http.StatusOK},
		{ID: "getCommonTenantSocialOverride", Method: http.MethodGet, Pattern: "/admin/chat/common-tenant-social", Tenant: httpx.TenantRequired,
			SuccessStatus: http.StatusOK},
		{ID: "putCommonTenantSocialOverride", Method: http.MethodPut, Pattern: "/admin/chat/common-tenant-social", Tenant: httpx.TenantRequired,
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
	} {
		o.Module = "chat"
		Operations[o.ID] = o
	}
}

func registerPolicies(s *httpx.Server, h *PolicyHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the chat policy handlers are required")
	}
	routes := map[string]http.HandlerFunc{"getChatPolicy": h.getLevel, "putChatPolicy": h.putLevel, "getEffectiveChatPolicy": h.effective,
		"getPlatformChatPolicy": h.getPlatform, "putPlatformChatPolicy": h.putPlatform, "getCommonTenantSocialOverride": h.getCommon,
		"putCommonTenantSocialOverride": h.putCommon}
	for _, id := range PolicyOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

// versionMatch parses If-Match of a policy write (W/"n", "n" or n; "0" = no document yet): missing → 428, malformed → 412.
func versionMatch(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return 0, problem.New(problem.PreconditionRequired, "")
	}
	n, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(raw, "W/"), `"`), 10, 64)
	if err != nil || n < 0 {
		return 0, problem.New(problem.VersionMismatch, "")
	}
	return n, nil
}

// policyDocument renders a ChatPolicyDocument from a stored row.
func policyDocument(row read.PolicyRow) map[string]any {
	var d map[string]json.RawMessage
	_ = json.Unmarshal(row.Rules, &d)
	out := map[string]any{"policy_id": row.ID, "scope_type": row.ScopeType, "version": row.Version, "rules": d["rules"],
		"updated_at": row.UpdatedAt.UTC()}
	if row.ScopeID != nil {
		out["scope_id"] = *row.ScopeID
	}
	for _, k := range []string{"global_rate", "world_scope", "chat_policy_preset"} {
		if v, ok := d[k]; ok && string(v) != "null" {
			out[k] = v
		}
	}
	if row.Reason != nil {
		out["reason"] = *row.Reason
	}
	if row.UpdatedBy != nil {
		out["updated_by"] = *row.UpdatedBy
	}
	return out
}

func writePolicy(w http.ResponseWriter, row read.PolicyRow) {
	w.Header().Set("ETag", `"`+strconv.FormatInt(row.Version, 10)+`"`)
	writeJSON(w, http.StatusOK, policyDocument(row))
}

func policyInput(r *http.Request) (command.PolicyInput, error) {
	var b struct {
		Rules      json.RawMessage `json:"rules"`
		GlobalRate *policy.Rate    `json:"global_rate"`
		WorldScope string          `json:"world_scope"`
		Preset     string          `json:"chat_policy_preset"`
		Reason     string          `json:"reason"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return command.PolicyInput{}, problem.Invalid("rules", problem.SubInvalid, "a ChatPolicyPut body")
	}
	return command.PolicyInput{Rules: b.Rules, GlobalRate: b.GlobalRate, WorldScope: b.WorldScope, Preset: b.Preset, Reason: b.Reason}, nil
}

func (h PolicyHandlers) getLevel(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "scope_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	row, err := h.Queries.GetLevel(r.Context(), chi.URLParam(r, "scope_type"), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writePolicy(w, row)
}

func (h PolicyHandlers) putLevel(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "scope_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	version, err := versionMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in, err := policyInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Commands.PutLevel(r.Context(), chi.URLParam(r, "scope_type"), id, version, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writePolicy(w, res.Row)
}

func (h PolicyHandlers) getPlatform(w http.ResponseWriter, r *http.Request) {
	row, err := h.Queries.GetPlatform(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writePolicy(w, row)
}

func (h PolicyHandlers) putPlatform(w http.ResponseWriter, r *http.Request) {
	version, err := versionMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in, err := policyInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Commands.PutPlatform(r.Context(), version, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writePolicy(w, res.Row)
}

func (h PolicyHandlers) effective(w http.ResponseWriter, r *http.Request) {
	school, err := uuid.Parse(r.URL.Query().Get("school_id"))
	if err != nil {
		fail(w, r, problem.Invalid("school_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	v, err := h.Queries.GetEffective(r.Context(), school, r.URL.Query().Get("channel_type"))
	if err != nil {
		fail(w, r, err)
		return
	}
	e := v.Effective
	kinds := make([]string, 0, len(e.AllowedKinds))
	for _, k := range e.AllowedKinds {
		kinds = append(kinds, string(k))
	}
	out := map[string]any{"channel_type": v.ChannelType, "enabled": v.Enabled, "allowed_kinds": kinds}
	if v.Enabled {
		out["max_text_length"] = e.MaxTextLength
		if e.Rate.MessagesPerMinute > 0 {
			rate := map[string]any{"messages_per_minute": e.Rate.MessagesPerMinute}
			if e.Rate.Burst > 0 {
				rate["burst"] = e.Rate.Burst
			}
			out["rate"] = rate
		}
		out["slow_mode_seconds"] = e.SlowModeSeconds
		hours := []policy.TimeWindow{}
		for _, ws := range e.Hours {
			hours = append(hours, ws...)
		}
		out["allowed_hours"] = hours
		out["features"] = features(e.Features)
		preset := "standard"
		if e.Preset != "" {
			preset = e.Preset
		}
		out["preset"] = preset
	}
	sources := make([]map[string]any, len(v.Sources))
	for i, s := range v.Sources {
		sources[i] = map[string]any{"scope_type": s.ScopeType, "version": s.Version}
	}
	out["sources"] = sources
	writeJSON(w, http.StatusOK, out)
}

func commonOverride(row read.CommonOverrideRow) map[string]any {
	out := map[string]any{"tenant_id": row.TenantID, "direct_enabled": row.Override.DirectEnabled, "world_enabled": row.Override.WorldEnabled,
		"area_free_text_enabled": row.Override.AreaFreeTextEnabled, "friends_enabled": row.Override.FriendsEnabled, "version": row.Version}
	if row.Reason != "" {
		out["reason"] = row.Reason
	}
	if row.UpdatedBy != nil {
		out["updated_by"] = *row.UpdatedBy
		out["updated_at"] = row.UpdatedAt.UTC()
	}
	return out
}

func (h PolicyHandlers) getCommon(w http.ResponseWriter, r *http.Request) {
	row, err := h.Queries.GetCommonOverride(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, commonOverride(row))
}

func (h PolicyHandlers) putCommon(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Direct  *bool  `json:"direct_enabled"`
		World   *bool  `json:"world_enabled"`
		Area    *bool  `json:"area_free_text_enabled"`
		Friends *bool  `json:"friends_enabled"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Direct == nil || b.World == nil || b.Area == nil {
		fail(w, r, problem.Invalid("direct_enabled", problem.SubRequired, "direct_enabled, world_enabled, area_free_text_enabled"))
		return
	}
	res, err := h.Commands.PutCommonOverride(r.Context(), command.CommonOverrideInput{Override: policy.CommonOverride{DirectEnabled: *b.Direct,
		WorldEnabled: *b.World, AreaFreeTextEnabled: *b.Area}, Friends: b.Friends, Reason: b.Reason})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, commonOverride(res.Row))
}

// features renders ChatFeatures (the message features of the channel type, platform level).
func features(f policy.Features) map[string]any {
	out := map[string]any{"edit": f.Edit, "recall": f.Recall, "reply": f.Reply, "quote": f.Quote, "forward": f.Forward,
		"sticker": f.Sticker, "gif": f.Gif, "reaction": f.Reaction, "mention": f.Mention, "pin": f.Pin, "typing": f.Typing, "search": f.Search}
	if f.Receipts != "" {
		out["receipts"] = f.Receipts
	}
	if f.Edit {
		out["edit_window_minutes"] = int(f.EditWindow() / time.Minute)
	}
	if f.Recall {
		out["recall_window_hours"] = int(f.RecallWindow() / time.Hour)
	}
	return out
}
