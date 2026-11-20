package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ProfileCommands are the write use cases of the game profile (command.Profiles, F070).
type ProfileCommands interface {
	UpdateAvatar(ctx context.Context, cfg profile.AvatarConfig) (profile.AvatarConfig, error)
	UpdateProfile(ctx context.Context, in command.ProfileUpdate) (*profile.Profile, error)
	SavePreset(ctx context.Context, slot int, name *string, cfg profile.AvatarConfig) (command.PresetResult, error)
}

// ProfileQueries are the read use cases of the game profile (query.Profile, F070).
type ProfileQueries interface {
	GetGameProfile(ctx context.Context) (query.GameProfile, error)
	GetCharacterCatalog(ctx context.Context) (*catalog.Catalog, error)
	SuggestNicknames(ctx context.Context, count int, acceptLanguage string) ([]query.Suggestion, error)
	ListAvatarPresets(ctx context.Context) ([]read.Preset, error)
}

// ProfileOperations are the player routes of F070 (F070-AC-21: staff tokens get 403 GAME_ACCESS_STUDENT_ONLY).
var ProfileOperations = []string{"getGameProfile", "updateGameProfile", "suggestNicknames", "updateAvatar", "listAvatarPresets",
	"saveAvatarPreset", "getCharacterCatalog"}

func (h Handlers) profileRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"getGameProfile":      h.getGameProfile,
		"updateGameProfile":   h.updateGameProfile,
		"suggestNicknames":    h.suggestNicknames,
		"updateAvatar":        h.updateAvatar,
		"listAvatarPresets":   h.listAvatarPresets,
		"saveAvatarPreset":    h.saveAvatarPreset,
		"getCharacterCatalog": h.getCharacterCatalog,
	}
}

func optTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

// gameProfile is the GameProfile of the contract: student_id, nickname, level, avatar, settings — never a real name,
// username, school or class (F070-AC-12, R16).
func gameProfile(student uuid.UUID, nickname *string, level int, xp int64, setup bool, created *time.Time, hash *string,
	avatar json.RawMessage, area *string, settings json.RawMessage, lang *string) map[string]any {
	m := map[string]any{"student_id": student, "nickname": nickname, "level": level, "xp_total": xp, "setup_required": setup,
		"avatar_created_at": optTime(created), "avatar_config_hash": hash, "avatar_config": nil, "settings": map[string]any{}}
	if len(avatar) > 0 {
		m["avatar_config"] = avatar
	}
	if len(settings) > 0 && !bytes.Equal(settings, []byte("null")) {
		m["settings"] = settings
	}
	if area != nil {
		m["last_area_id"] = *area
	}
	if lang != nil {
		m["preferred_language"] = *lang
	}
	return m
}

func (h Handlers) getGameProfile(w http.ResponseWriter, r *http.Request) {
	p, err := h.ProfileQueries.GetGameProfile(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, gameProfile(p.StudentID, p.Nickname, p.Level, p.XPTotal, p.SetupRequired, p.AvatarCreatedAt,
		p.AvatarConfigHash, p.AvatarConfig, p.LastAreaID, p.Settings, p.PreferredLanguage))
}

func (h Handlers) updateGameProfile(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	var in command.ProfileUpdate
	if raw, ok := body["nickname_option_id"]; ok {
		var id uuid.UUID
		if err := json.Unmarshal(raw, &id); err != nil {
			fail(w, r, problem.Invalid("nickname_option_id", problem.SubInvalid, "must be a UUID"))
			return
		}
		in.NicknameOptionID = &id
	}
	if raw, ok := body["settings"]; ok {
		if err := json.Unmarshal(raw, &in.Settings); err != nil {
			fail(w, r, problem.Invalid("settings", problem.SubInvalid, "must be an object"))
			return
		}
		if in.Settings == nil {
			in.Settings = map[string]any{}
		}
	}
	if raw, ok := body["preferred_language"]; ok {
		in.SetLanguage = true
		if err := json.Unmarshal(raw, &in.PreferredLanguage); err != nil {
			fail(w, r, problem.Invalid("preferred_language", problem.SubInvalid, "must be a locale"))
			return
		}
	}
	p, err := h.Profiles.UpdateProfile(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	var avatar json.RawMessage
	if p.Avatar != nil {
		avatar, _ = p.Avatar.Canonical()
	}
	settings, _ := json.Marshal(p.Settings)
	writeJSON(w, http.StatusOK, gameProfile(p.StudentID, p.Nickname, p.Level, p.XPTotal, p.NeedsSetup(), p.AvatarCreatedAt,
		p.AvatarHash, avatar, p.LastAreaID, settings, p.PreferredLanguage))
}

func decodeAvatar(raw []byte) (profile.AvatarConfig, error) {
	var c profile.AvatarConfig
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, problem.New(problem.BadRequest, "invalid avatar_config")
	}
	return c, nil
}

func (h Handlers) updateAvatar(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	if len(raw) > profile.MaxAvatarBytes {
		fail(w, r, problem.Invalid("avatar_config", problem.SubMaxLength, "at most 16 KB"))
		return
	}
	cfg, err := decodeAvatar(raw)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Profiles.UpdateAvatar(r.Context(), cfg)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func presetJSON(slot int, name *string, avatar any, updated time.Time) map[string]any {
	m := map[string]any{"slot": slot, "avatar_config": avatar, "updated_at": updated.UTC()}
	if name != nil {
		m["name"] = *name
	}
	return m
}

func (h Handlers) listAvatarPresets(w http.ResponseWriter, r *http.Request) {
	list, err := h.ProfileQueries.ListAvatarPresets(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(list))
	for i, p := range list {
		data[i] = presetJSON(p.Slot, p.Name, p.AvatarConfig, p.UpdatedAt)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h Handlers) saveAvatarPreset(w http.ResponseWriter, r *http.Request) {
	slot, err := strconv.Atoi(chi.URLParam(r, "slot"))
	if err != nil {
		fail(w, r, problem.Invalid("slot", problem.SubInvalid, "must be an integer"))
		return
	}
	var body struct {
		Name         *string         `json:"name"`
		AvatarConfig json.RawMessage `json:"avatar_config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	if len(body.AvatarConfig) > profile.MaxAvatarBytes {
		fail(w, r, problem.Invalid("avatar_config", problem.SubMaxLength, "at most 16 KB"))
		return
	}
	cfg, err := decodeAvatar(body.AvatarConfig)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Profiles.SavePreset(r.Context(), slot, body.Name, cfg)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, presetJSON(out.Slot, out.Name, out.AvatarConfig, out.UpdatedAt))
}

func (h Handlers) suggestNicknames(w http.ResponseWriter, r *http.Request) {
	count := query.DefaultSuggestions
	if v := r.URL.Query().Get("count"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			fail(w, r, problem.Invalid("count", problem.SubInvalid, "must be an integer"))
			return
		}
		count = n
	}
	list, err := h.ProfileQueries.SuggestNicknames(r.Context(), count, r.Header.Get("Accept-Language"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(list))
	for i, s := range list {
		data[i] = map[string]any{"nickname_option_id": s.OptionID, "nickname": s.Nickname}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// getCharacterCatalog answers the published catalog with its ETag; If-None-Match of the current version is 304
// (F070-AC-02). The nickname words are not sent (suggestions are generated on the server).
func (h Handlers) getCharacterCatalog(w http.ResponseWriter, r *http.Request) {
	c, err := h.ProfileQueries.GetCharacterCatalog(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	etag := c.ETag()
	w.Header().Set("ETag", `"`+etag+`"`)
	w.Header().Set("Cache-Control", "private, no-cache")
	if inm := r.Header.Get("If-None-Match"); inm != "" && etagMatches(inm, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	presets := make([]map[string]any, len(c.Presets))
	for i, p := range c.Presets {
		m := map[string]any{"preset_code": p.PresetCode, "name": p.Name, "avatar_config": p.AvatarConfig.Normalized(c.DefaultEmoteIDs)}
		if p.ThumbnailAssetID != nil {
			m["thumbnail_asset_id"] = *p.ThumbnailAssetID
		}
		presets[i] = m
	}
	writeJSON(w, http.StatusOK, map[string]any{"catalog_version_id": c.VersionID, "version": c.Version, "presets": presets,
		"body_types": c.BodyTypes, "skin_tones": c.SkinTones, "hair_colors": c.HairColors, "default_item_ids": c.DefaultItemIDs,
		"default_emote_ids": c.DefaultEmoteIDs})
}
