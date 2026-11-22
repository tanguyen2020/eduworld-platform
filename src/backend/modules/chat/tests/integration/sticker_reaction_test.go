//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// libraryKey is an immutable media key media/<sha256>/<file> (R31) of a test asset.
func libraryKey(file string) string {
	return "media/" + strings.Repeat("ab", 32) + "/" + file
}

// seedPack writes a platform sticker pack with one sticker (status, optional unlock rule) and returns pack and sticker ids.
func (f *featureEnv) seedPack(status string, locked bool) (uuid.UUID, uuid.UUID) {
	f.t.Helper()
	pack, sticker := uuid.New(), uuid.New()
	var rule any
	if locked {
		rule = `{"item_id":"` + uuid.NewString() + `"}`
	}
	f.exec(`INSERT INTO chat.sticker_packs (pack_id, code, name_vi, name_en, unlock_rule, unlock_hint_vi, status, created_by)
		VALUES ($1, $2, 'Gói vui', 'Fun pack', $3::jsonb, 'Hoàn thành nhiệm vụ', $4, $5)`, pack, "p"+pack.String()[:8], rule, status, uuid.New())
	f.exec(`INSERT INTO chat.stickers (sticker_id, pack_id, asset_key, content_type, size_bytes, alt_text_vi, alt_text_en)
		VALUES ($1, $2, $3, 'image/webp', 1000, 'Mèo vẫy tay', 'Cat waving')`, sticker, pack, libraryKey("cat.webp"))
	return pack, sticker
}

func (f *featureEnv) assign(tenant, pack uuid.UUID) {
	f.t.Helper()
	f.exec(`INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, 'sticker_pack', $3, $4, now())`, uuid.New(), tenant, pack, uuid.New())
}

func (f *featureEnv) seedGif(status string, tags ...string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO chat.gif_assets (gif_id, asset_key, preview_key, size_bytes, width, height, status, created_by)
		VALUES ($1, $2, $3, 1000, 320, 240, $4, $5)`, id, libraryKey("g.gif"), libraryKey("g.png"), status, uuid.New())
	for _, tg := range tags {
		f.exec(`INSERT INTO chat.gif_tags (gif_id, tag, locale) VALUES ($1, $2, 'vi')`, id, tg)
	}
	return id
}

// T018 — F118-AC-12 … AC-15 (TC-CHAT-033): only a sticker of a published pack assigned to the tenant (and owned when the
// pack is locked) or a published GIF of the internal library is sent; listStickerPacks shows the assigned packs with the
// unlock state of the caller; listGifs only published GIFs by tag, served from the EduWorld CDN; sticker + GIF share a
// ceiling per minute; emoji only from the platform set.
func TestStickerGif(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	f.pace = 4 * time.Second // every asset of the test inside one minute, under the burst of the channel
	an, binh := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.party(an, binh)
	open, openSticker := f.seedPack("published", false)
	locked, lockedSticker := f.seedPack("published", true)
	_, draftSticker := f.seedPack("draft", false)
	_, unassignedSticker := f.seedPack("published", false)
	for _, p := range []uuid.UUID{open, locked} {
		f.assign(f.tenantA, p)
	}
	gif := f.seedGif("published", "co vu")
	draftGif := f.seedGif("in_review", "co vu")
	send := func(body map[string]any) resp {
		body["client_message_id"] = uuid.New()
		return f.sendREST(an, ch, body)
	}
	if r := send(map[string]any{"kind": "sticker", "sticker_id": openSticker}); r.status != http.StatusCreated {
		t.Fatalf("assigned published sticker: %d %s", r.status, r.raw)
	}
	for name, s := range map[string]uuid.UUID{"locked not owned": lockedSticker, "draft pack": draftSticker, "not assigned": unassignedSticker,
		"unknown": uuid.New()} {
		if r := send(map[string]any{"kind": "sticker", "sticker_id": s}); r.status != http.StatusUnprocessableEntity || r.code() != "CHAT_ASSET_NOT_AVAILABLE" {
			t.Errorf("%s sticker: %d %s", name, r.status, r.raw)
		}
	}
	// Owning the locked pack (projection of the unlock event of module game) makes it available.
	f.exec(`INSERT INTO ref.sticker_unlocks (student_id, tenant_id, pack_id, unlocked_at, source_version) VALUES ($1, $2, $3, now(), 1)`,
		an.id, f.tenantA, locked)
	if r := send(map[string]any{"kind": "sticker", "sticker_id": lockedSticker}); r.status != http.StatusCreated {
		t.Fatalf("owned locked sticker: %d %s", r.status, r.raw)
	}
	if r := send(map[string]any{"kind": "gif", "gif_id": gif}); r.status != http.StatusCreated {
		t.Fatalf("published gif: %d %s", r.status, r.raw)
	}
	for _, g := range []uuid.UUID{draftGif, uuid.New()} {
		if r := send(map[string]any{"kind": "gif", "gif_id": g}); r.code() != "CHAT_ASSET_NOT_AVAILABLE" {
			t.Errorf("gif %s: %d %s", g, r.status, r.raw)
		}
	}
	// F118-AC-15: sticker + GIF together ≤ 4 per minute per channel (3 sent so far: the 4th passes, the 5th waits).
	if r := send(map[string]any{"kind": "sticker", "sticker_id": openSticker}); r.status != http.StatusCreated {
		t.Fatalf("4th asset: %d %s", r.status, r.raw)
	}
	if r := send(map[string]any{"kind": "gif", "gif_id": gif}); r.status != http.StatusTooManyRequests || r.code() != "CHAT_RATE_LIMITED" {
		t.Fatalf("5th asset within a minute: %d %s", r.status, r.raw)
	}
	f.clock.Add(time.Minute)
	if r := send(map[string]any{"kind": "emoji", "emoji": "heart"}); r.status != http.StatusCreated {
		t.Fatalf("allowed emoji: %d %s", r.status, r.raw)
	}
	if r := send(map[string]any{"kind": "emoji", "emoji": "skull"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("emoji outside the set: %d %s", r.status, r.raw)
	}
	// F118-AC-14: listStickerPacks of binh (no unlock): the two assigned packs, the locked one with its hint.
	l := f.do(http.MethodGet, "/chat/stickers", binh.token, map[string]string{"X-Tenant-Id": f.tenantA.String(), "Accept-Language": "en"}, nil)
	if l.status != http.StatusOK || len(l.data()) != 2 {
		t.Fatalf("sticker packs: %d %s", l.status, l.raw)
	}
	for _, p := range l.data() {
		if p["name"] != "Fun pack" {
			t.Fatalf("English label: %v", p)
		}
		st := p["stickers"].([]any)[0].(map[string]any)
		if !strings.HasPrefix(st["url"].(string), "https://cdn.test.invalid/media/") || st["alt_text"] != "Cat waving" {
			t.Fatalf("sticker: %v", st)
		}
		if p["pack_id"] == locked.String() && (p["unlocked"] != false || p["unlock_hint"] == nil) {
			t.Fatalf("locked pack: %v", p)
		}
	}
	// F118-AC-13: listGifs: published only, by tag, CDN of EduWorld only.
	g := f.call(binh, http.MethodGet, "/chat/gifs?tag=co%20vu", "", nil)
	if g.status != http.StatusOK || len(g.data()) != 1 || g.data()[0]["gif_id"] != gif.String() {
		t.Fatalf("gifs: %d %s", g.status, g.raw)
	}
	if strings.Contains(g.raw, "giphy") || strings.Contains(g.raw, "tenor") || !strings.Contains(g.raw, "https://cdn.test.invalid/") {
		t.Fatalf("gif urls: %s", g.raw)
	}
}

// T019 — F118-AC-16 (TC-CHAT-034): a member toggles reactions of the allowed set, at most chat_reactions_per_user per
// message (the 4th → 422 CHAT_REACTION_LIMIT), counts pushed to every member and kept in chat.message_reactions (unset =
// soft delete).
func TestReactions(t *testing.T) {
	f := setupFeatures(t, chatOpts{})
	startFeatures(t, f)
	an, binh := f.student(f.tenantA, f.schoolA), f.student(f.tenantA, f.schoolA)
	ch := f.party(an, binh)
	inbox := channelInbox(t, f.vk, ch)
	id := f.sendText(an, ch, "ai di choi khong", nil)
	f.persisted(id)
	react := func(code string) resp {
		return f.call(binh, http.MethodPost, "/chat/messages/"+id.String()+"/reactions", newKey(), map[string]any{"reaction": code})
	}
	for _, code := range []string{"heart", "laugh", "clap"} {
		if r := react(code); r.status != http.StatusOK || r.body["active"] != true {
			t.Fatalf("react %s: %d %s", code, r.status, r.raw)
		}
	}
	if r := react("star"); r.status != http.StatusUnprocessableEntity || r.code() != "CHAT_REACTION_LIMIT" {
		t.Fatalf("4th reaction: %d %s", r.status, r.raw)
	}
	if r := react("skull"); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("reaction outside the set: %d %s", r.status, r.raw)
	}
	env := next(inbox, "chat.reaction.updated", 3*time.Second)
	if env == nil {
		t.Fatal("chat.reaction.updated missing")
	}
	// Toggle off (soft delete), via WS.
	rep := f.ws(binh, "cb", "chat.reaction.toggle", map[string]any{"message_id": id, "reaction": "heart"})
	if rep.Problem != nil {
		t.Fatalf("ws toggle: %+v", rep.Problem)
	}
	if n := f.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.message_reactions WHERE message_id = $1 AND emoji_code = 'heart'
		AND deleted_at IS NOT NULL`, id); n != 1 {
		t.Fatal("unset must be a soft delete")
	}
	if n := f.waitCount(2, 5*time.Second, `SELECT count(*) FROM chat.message_reactions WHERE message_id = $1 AND deleted_at IS NULL`, id); n != 2 {
		t.Fatalf("live reactions: %d", n)
	}
	h := f.call(an, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil)
	rx, _ := h.data()[0]["reactions"].(map[string]any)
	if counts, _ := rx["counts"].(map[string]any); counts["laugh"] != float64(1) || counts["clap"] != float64(1) || counts["heart"] != nil {
		t.Fatalf("history reactions: %s", h.raw)
	}
}
