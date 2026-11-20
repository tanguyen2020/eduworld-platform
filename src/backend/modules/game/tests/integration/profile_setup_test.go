//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

var defaultEmotes = []any{"emote_wave", "emote_clap", "emote_dance", "emote_celebrate", "emote_thank", "emote_apologize"}

// T009 (F070-AC-01, TC-GAM-011): createGameSession creates the game profile once (a second session does not create
// another); getGameProfile answers setup_required = true with avatar_config null; before any session the student
// still gets setup_required = true (nothing written by the query).
func TestEnsureProfileOnSessionStart(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.profile(t, s)
	if r.status != http.StatusOK || r.body["setup_required"] != true || r.body["avatar_config"] != nil || r.body["nickname"] != nil {
		t.Fatalf("before session: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1`, s.id); n != 0 {
		t.Fatalf("a query wrote %d profiles", n)
	}
	d := e.device(s)
	if r := e.start(t, s, d, newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	e.clock.Add(60 * 1e9)
	if r := e.start(t, s, d, newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("start again: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND tenant_id = $2
		AND school_id = $3 AND last_game_login_at IS NOT NULL AND level = 1`, s.id, s.tenant, s.school); n != 1 {
		t.Fatalf("profiles: %d", n)
	}
	r = e.profile(t, s)
	if r.status != http.StatusOK || r.body["setup_required"] != true || r.body["avatar_config"] != nil || r.body["student_id"] != s.id.String() ||
		r.body["level"] != float64(1) {
		t.Fatalf("profile: %s", r.raw)
	}
	for _, k := range []string{"full_name", "username", "school_id", "class_id"} {
		if _, ok := r.body[k]; ok {
			t.Fatalf("profile exposes %s", k)
		}
	}
}

// T010 (F070-AC-02): getCharacterCatalog answers the published catalog — 6 presets (avatar_creation_presets), body
// types, palettes, default items and the 6 default emotes, every name in vi and en — with its ETag; If-None-Match
// of the current version is 304; the nickname words never leave the server.
func TestCharacterCatalog(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.do(t, http.MethodGet, "/game/character/catalog", e.gtoken(s), th(s), nil)
	if r.status != http.StatusOK {
		t.Fatalf("catalog: %d %s", r.status, r.raw)
	}
	presets, _ := r.body["presets"].([]any)
	if len(presets) != 6 || r.body["version"] != float64(1) {
		t.Fatalf("presets: %s", r.raw)
	}
	for _, key := range []string{"body_types", "skin_tones", "hair_colors"} {
		list, _ := r.body[key].([]any)
		if len(list) == 0 {
			t.Fatalf("%s empty", key)
		}
		for _, it := range list {
			name := it.(map[string]any)["name"].(map[string]any)
			if name["vi"] == "" || name["en"] == "" {
				t.Fatalf("%s name %v", key, name)
			}
		}
	}
	for _, p := range presets {
		pm := p.(map[string]any)
		name := pm["name"].(map[string]any)
		if name["vi"] == nil || name["en"] == nil || pm["avatar_config"] == nil {
			t.Fatalf("preset %v", pm)
		}
	}
	if !slices.Equal(r.body["default_emote_ids"].([]any), defaultEmotes) || len(r.body["default_item_ids"].([]any)) != 16 {
		t.Fatalf("defaults: %s", r.raw)
	}
	if _, ok := r.body["nickname_parts"]; ok {
		t.Fatal("nickname words sent to the client")
	}
	etag := r.header.Get("ETag")
	h := th(s)
	h["If-None-Match"] = etag
	if r := e.do(t, http.MethodGet, "/game/character/catalog", e.gtoken(s), h, nil); r.status != http.StatusNotModified {
		t.Fatalf("304: %d", r.status)
	}
	// A new published version changes the ETag (republishing the same payload does not).
	e.publishCatalog(ctx, t)
	if r := e.do(t, http.MethodGet, "/game/character/catalog", e.gtoken(s), h, nil); r.status != http.StatusNotModified {
		t.Fatalf("same payload republished: %d", r.status)
	}
	e.publishCatalog(ctx, t, catalogPayload(t, map[string]any{"vi": map[string]any{"adjectives": []any{"Vui Vẻ"}, "nouns": []any{"Cú Mèo"}},
		"en": map[string]any{"adjectives": []any{"Happy"}, "nouns": []any{"Owl"}}}))
	if r := e.do(t, http.MethodGet, "/game/character/catalog", e.gtoken(s), h, nil); r.status != http.StatusOK || r.body["version"] != float64(2) {
		t.Fatalf("new version: %d %s", r.status, r.raw)
	}
}

// T011 (F070-AC-03, AC-04, AC-19): the first updateAvatar stores the avatar, sets avatar_created_at once, fills the
// 6 default emotes and publishes avatar.created exactly once — also under 10 concurrent identical requests and on
// repeats; a body type, skin tone or hair colour outside the catalog is 422 VALIDATION_FAILED and nothing is stored.
func TestAvatarCreatedOnce(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	for field, bad := range map[string]map[string]any{
		"skin_tone":  with(avatar(), "skin_tone", "tone_99"),
		"body_type":  with(avatar(), "body_type", "kid_z"),
		"hair.color": with(avatar(), "hair", map[string]any{"item_id": "hair_short_01", "color": "hair_col_99"}),
	} {
		r := e.putAvatar(t, s, bad)
		if r.status != http.StatusUnprocessableEntity || r.code() != string(problem.ValidationFailed) {
			t.Fatalf("%s: %d %s", field, r.status, r.raw)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND avatar_config IS NOT NULL`, s.id); n != 0 {
		t.Fatal("refused avatar stored")
	}
	var wg sync.WaitGroup
	codes := make([]int, 10)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = e.putAvatar(t, s, avatar()).status
		}(i)
	}
	wg.Wait()
	for i, c := range codes {
		if c != http.StatusOK {
			t.Fatalf("request %d: %d", i, c)
		}
	}
	r := e.putAvatar(t, s, avatar())
	if r.status != http.StatusOK || !slices.Equal(r.body["emote_slots"].([]any), defaultEmotes) {
		t.Fatalf("repeat: %d %s", r.status, r.raw)
	}
	if n := e.events(ctx, t, "avatar.created"); n != 1 {
		t.Fatalf("avatar.created: %d", n)
	}
	if n := e.events(ctx, t, "avatar.updated"); n != 0 {
		t.Fatalf("avatar.updated for the same avatar: %d", n)
	}
	p := e.payloads(ctx, t, "avatar.created")[0]
	if p["student_id"] != s.id.String() || p["preset_code"] != "preset_explorer" || len(p["avatar_config_hash"].(string)) != 64 {
		t.Fatalf("avatar.created payload %v", p)
	}
	prof := e.profile(t, s)
	if prof.body["avatar_created_at"] == nil || prof.body["avatar_config_hash"] != p["avatar_config_hash"] || prof.body["setup_required"] != true {
		t.Fatalf("profile: %s", prof.raw)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["avatar.created"] != 1 || seen["game_profile.updated"] != 1 {
		t.Fatalf("event schemas: %v", seen)
	}
}
