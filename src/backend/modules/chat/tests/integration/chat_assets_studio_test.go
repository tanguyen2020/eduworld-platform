//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/storage"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// F082 — Game Studio sticker/GIF administration on module chat: the library list (one row per item, keyset pages in the
// query, status/author/tag filters), preview URLs signed from the private bucket, the en texts of a pack required at
// submit (TRANSLATION_INCOMPLETE), file facts read from the bucket, the item code of an unlock rule, the GIF reviewer
// other than its uploader, and diff/rollback/unpublish/release history (F082-AC-01…AC-07, AC-19).
func TestChatAssetsStudio(t *testing.T) {
	store := &memStorage{objs: map[string][]byte{}}
	f := setupFeatures(t, chatOpts{hook: func(c *chatEnv) {
		codec, err := page.NewCodec([]byte(strings.Repeat("k", 32)))
		if err != nil {
			t.Fatal(err)
		}
		lim := chatasset.Limits{StickerMaxBytes: 512 * 1024, GifMaxBytes: 2 * 1024 * 1024, GifMaxPixels: 480}
		c.assetHandlers = &chathttp.AssetHandlers{
			Commands: command.ChatAssets{UoW: c.uow, Repo: persistence.Assets{}, Storage: store, Inspector: storage.Inspector{Storage: store},
				Events: persistence.Publisher{W: c.writer}, Audit: persistence.Auditor{R: c.rec}, Limits: lim, Now: c.clock.Now},
			Queries: query.ChatAssets{Read: c.read, Reader: persistence.Assets{}, Cursors: codec, Storage: store}}
	}})
	startFeatures(t, f)
	designerData := authztest.Platform("game_designer").PrincipalData
	designerSub := "kc-platform-" + uuid.NewString()
	f.res.Set(designerSub, designerData)
	designer, reviewer, liveops := f.idp.Token(designerSub, httpxtest.PortalClient, time.Now()), f.platformStaff("content_reviewer"),
		f.platformStaff("liveops")
	other := f.platformStaff("game_designer")
	call := func(token, method, path string, body any) resp {
		h := map[string]string{"Idempotency-Key": newKey()}
		// A version write sends If-Match = the current ETag of that version (412 when stale — F082 Clarifications 2026-11-26).
		if m := versionWrite.FindStringSubmatch(path); m != nil && method != http.MethodGet {
			h["If-Match"] = f.do(http.MethodGet, m[1], token, nil, nil).header.Get("ETag")
		}
		if method == http.MethodPatch {
			h["Content-Type"] = "application/merge-patch+json"
		}
		return f.do(method, path, token, h, body)
	}
	upload := func(tok, base, name, ct string, b []byte) string {
		p := call(tok, http.MethodPost, base+"/assets:presign", map[string]any{"file_name": name, "content_type": ct, "size_bytes": len(b), "sha256": shaOf(b)})
		if p.status != http.StatusCreated {
			t.Fatalf("presign %s: %d %s", name, p.status, p.raw)
		}
		k := p.body["asset_key"].(string)
		store.put(k, b)
		return k
	}

	// ---- sticker pack: unlock item code, en required at submit, stamped file facts, preview URL ----
	if r := call(designer, http.MethodPost, "/game-admin/sticker-packs", map[string]any{"payload": map[string]any{"code": "ban-dong-hanh",
		"name_vi": "Bạn đồng hành", "unlock_rule": map[string]any{"item_id": uuid.NewString()}, "stickers": []any{}}}); r.status != 422 {
		t.Fatalf("uuid item_id accepted: %d %s", r.status, r.raw)
	}
	pr := call(designer, http.MethodPost, "/game-admin/sticker-packs", map[string]any{"payload": map[string]any{"code": "ban-dong-hanh",
		"name_vi": "Bạn đồng hành", "unlock_rule": map[string]any{"item_id": "star_hat"}, "stickers": []any{}}})
	if pr.status != http.StatusCreated {
		t.Fatalf("create pack: %d %s", pr.status, pr.raw)
	}
	pack := pr.body["config_id"].(string)
	pbase := "/game-admin/sticker-packs/" + pack
	img := pngOf(64, 64)
	key := upload(designer, pbase, "cat.png", "image/png", img)
	payload := map[string]any{"code": "ban-dong-hanh", "name_vi": "Bạn đồng hành", "unlock_rule": map[string]any{"item_id": "star_hat"},
		"stickers": []any{map[string]any{"asset_key": key, "content_type": "image/webp", "size_bytes": 1, "alt_text_vi": "Mèo vẫy tay", "sort_order": 0}}}
	if r := call(designer, http.MethodPatch, pbase+"/versions/1", map[string]any{"payload": payload}); r.status != http.StatusOK {
		t.Fatalf("draft: %d %s", r.status, r.raw)
	}
	g := call(designer, http.MethodGet, pbase+"/versions/1", nil)
	st := g.body["payload"].(map[string]any)["stickers"].([]any)[0].(map[string]any)
	if u, _ := st["preview_url"].(string); !strings.Contains(u, key) || !strings.Contains(u, "Signature=get") {
		t.Fatalf("preview url: %s", g.raw)
	}
	// F082-AC-02 server side: name_en and the alt text en are required at submit.
	sub := call(designer, http.MethodPost, pbase+"/versions/1:transition", map[string]any{"action": "submit_review"})
	if sub.status != 422 || sub.code() != "TRANSLATION_INCOMPLETE" || !strings.Contains(sub.raw, "payload/name_en") ||
		!strings.Contains(sub.raw, "payload/stickers/0/alt_text_en") {
		t.Fatalf("missing en: %d %s", sub.status, sub.raw)
	}
	payload["name_en"] = "Companions"
	payload["stickers"].([]any)[0].(map[string]any)["alt_text_en"] = "Cat waving"
	if r := call(designer, http.MethodPatch, pbase+"/versions/1", map[string]any{"payload": payload}); r.status != http.StatusOK {
		t.Fatalf("draft en: %d %s", r.status, r.raw)
	}
	if r := call(designer, http.MethodPost, pbase+"/versions/1:transition", map[string]any{"action": "submit_review"}); r.status != http.StatusOK {
		t.Fatalf("submit: %d %s", r.status, r.raw)
	}
	g = call(designer, http.MethodGet, pbase+"/versions/1", nil)
	st = g.body["payload"].(map[string]any)["stickers"].([]any)[0].(map[string]any)
	if st["content_type"] != "image/png" || st["size_bytes"] != float64(len(img)) {
		t.Fatalf("file facts not read from the bucket: %v", st)
	}
	if r := call(reviewer, http.MethodPost, pbase+"/versions/1:transition", map[string]any{"action": "approve"}); r.status != http.StatusOK {
		t.Fatalf("approve: %d %s", r.status, r.raw)
	}
	if r := call(liveops, http.MethodPost, pbase+"/versions/1:publish", map[string]any{}); r.status != http.StatusOK {
		t.Fatalf("publish v1: %d %s", r.status, r.raw)
	}
	// version 2: another alt text, published; then diff, rollback to 1, release history, unpublish.
	payload["stickers"].([]any)[0].(map[string]any)["alt_text_vi"] = "Mèo chào"
	if r := call(designer, http.MethodPatch, pbase+"/versions/1", map[string]any{"payload": payload}); r.status != http.StatusOK || r.body["version"] != float64(2) {
		t.Fatalf("v2: %d %s", r.status, r.raw)
	}
	for _, step := range []struct{ tok, path, action string }{{designer, "/versions/2:transition", "submit_review"}, {reviewer, "/versions/2:transition", "approve"}} {
		if r := call(step.tok, http.MethodPost, pbase+step.path, map[string]any{"action": step.action}); r.status != http.StatusOK {
			t.Fatalf("%s v2: %d %s", step.action, r.status, r.raw)
		}
	}
	if r := call(liveops, http.MethodPost, pbase+"/versions/2:publish", map[string]any{}); r.status != http.StatusOK {
		t.Fatalf("publish v2: %d %s", r.status, r.raw)
	}
	d := call(designer, http.MethodGet, pbase+"/versions/2:diff?with=1", nil)
	if d.status != http.StatusOK || !strings.Contains(d.raw, "/stickers/0/alt_text_vi") || strings.Contains(d.raw, "preview_url") {
		t.Fatalf("diff: %d %s", d.status, d.raw)
	}
	if r := call(liveops, http.MethodPost, pbase+":rollback", map[string]any{"to_version": 1}); r.status != 422 {
		t.Fatalf("rollback without reason: %d %s", r.status, r.raw)
	}
	if r := call(designer, http.MethodPost, pbase+":rollback", map[string]any{"to_version": 1, "reason": "Sai chữ"}); r.status != http.StatusForbidden {
		t.Fatalf("rollback by the designer (no game.content.publish): %d", r.status)
	}
	rb := call(liveops, http.MethodPost, pbase+":rollback", map[string]any{"to_version": 1, "reason": "Sai chữ"})
	if rb.status != http.StatusOK || rb.body["action"] != "rollback" || rb.body["to_version"] != float64(1) || rb.body["from_version"] != float64(2) {
		t.Fatalf("rollback: %d %s", rb.status, rb.raw)
	}
	if n := f.count(`SELECT count(*) FROM chat.stickers WHERE pack_id = $1 AND alt_text_vi = 'Mèo vẫy tay' AND deleted_at IS NULL`, pack); n != 1 {
		t.Fatal("rollback did not restore the library state")
	}
	v := call(designer, http.MethodGet, pbase+"/versions", nil)
	if v.body["current_version"] != float64(1) {
		t.Fatalf("current after rollback: %s", v.raw)
	}
	rel := call(designer, http.MethodGet, pbase+"/releases?limit=2", nil)
	if rel.status != http.StatusOK || len(rel.data()) != 2 || rel.data()[0]["action"] != "rollback" || rel.body["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("releases: %d %s", rel.status, rel.raw)
	}
	rel2 := call(designer, http.MethodGet, pbase+"/releases?limit=2&cursor="+rel.body["page"].(map[string]any)["next_cursor"].(string), nil)
	if len(rel2.data()) != 1 || rel2.data()[0]["action"] != "publish" {
		t.Fatalf("releases page 2: %s", rel2.raw)
	}
	up := call(liveops, http.MethodPost, pbase+":unpublish", map[string]any{"reason": "Hết mùa"})
	if up.status != http.StatusOK || up.body["action"] != "unpublish" {
		t.Fatalf("unpublish: %d %s", up.status, up.raw)
	}
	if n := f.count(`SELECT count(*) FROM chat.sticker_packs WHERE pack_id = $1 AND status = 'archived' AND current_version_id IS NULL`, pack); n != 1 {
		t.Fatal("unpublish did not withdraw the pack")
	}
	if r := call(liveops, http.MethodPost, pbase+":unpublish", map[string]any{"reason": "again"}); r.status != 422 {
		t.Fatalf("second unpublish: %d", r.status)
	}
	if len(f.events(command.EventContentRolledBack)) != 1 || len(f.events(command.EventContentUnpublished)) != 1 {
		t.Fatal("rollback/unpublish events missing")
	}
	f.checkEvents(command.EventContentRolledBack)
	f.checkEvents(command.EventContentUnpublished)

	// ---- GIF library: uploader ≠ reviewer, list one row per item with keyset pages and filters ----
	var gifs []string
	for i, tags := range [][]string{{"cheer", "hello"}, {"cheer"}, {"math"}} {
		tok := designer
		if i == 2 {
			tok = other
		}
		cr := call(tok, http.MethodPost, "/game-admin/gifs", map[string]any{"payload": map[string]any{}})
		id := cr.body["config_id"].(string)
		gb := "/game-admin/gifs/" + id
		anim, still := gifOf(40+i, 30, 2), pngOf(40+i, 30)
		gp := map[string]any{"asset_key": upload(tok, gb, "a.gif", "image/gif", anim), "preview_key": upload(tok, gb, "a.png", "image/png", still),
			"tags_en": tags}
		if r := call(tok, http.MethodPatch, gb+"/versions/1", map[string]any{"payload": gp}); r.status != http.StatusOK {
			t.Fatalf("gif draft: %d %s", r.status, r.raw)
		}
		gifs = append(gifs, id)
	}
	// F082-AC-06: the uploader cannot approve (server 403 PERMISSION_DENIED).
	gb := "/game-admin/gifs/" + gifs[0]
	if r := call(designer, http.MethodPost, gb+"/versions/1:transition", map[string]any{"action": "submit_review"}); r.status != http.StatusOK {
		t.Fatalf("gif submit: %d %s", r.status, r.raw)
	}
	gv := call(designer, http.MethodGet, gb+"/versions/1", nil)
	gpl := gv.body["payload"].(map[string]any)
	if gpl["width"] != float64(40) || gpl["preview_url"] == nil || gpl["url"] == nil {
		t.Fatalf("gif stamped/preview: %s", gv.raw)
	}
	if r := call(designer, http.MethodPost, gb+"/versions/1:transition", map[string]any{"action": "approve"}); r.status != http.StatusForbidden {
		t.Fatalf("uploader approve: %d %s", r.status, r.raw)
	}
	l := call(designer, http.MethodGet, "/game-admin/gifs?limit=2", nil)
	if l.status != http.StatusOK || len(l.data()) != 2 || l.body["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("gif page 1: %d %s", l.status, l.raw)
	}
	l2 := call(designer, http.MethodGet, "/game-admin/gifs?limit=2&cursor="+l.body["page"].(map[string]any)["next_cursor"].(string), nil)
	if len(l2.data()) != 1 || l2.body["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("gif page 2: %s", l2.raw)
	}
	seen := map[any]bool{}
	for _, x := range append(l.data(), l2.data()...) {
		if seen[x["config_id"]] {
			t.Fatal("an item twice across pages")
		}
		seen[x["config_id"]] = true
	}
	if r := call(designer, http.MethodGet, "/game-admin/gifs?tag=cheer", nil); len(r.data()) != 2 {
		t.Fatalf("tag filter: %s", r.raw)
	}
	if r := call(designer, http.MethodGet, "/game-admin/gifs?author_id="+designerData.UserID.String(), nil); len(r.data()) != 2 {
		t.Fatalf("author filter: %s", r.raw)
	}
	if r := call(designer, http.MethodGet, "/game-admin/gifs?status=in_review", nil); len(r.data()) != 1 || r.data()[0]["config_id"] != gifs[0] {
		t.Fatalf("status filter: %s", r.raw)
	}
	// F082-AC-19: an invalid tag, a tag filter on packs, translation_status on chat types are 422.
	for _, q := range []string{"/game-admin/gifs?tag=" + strings.Repeat("a", 65), "/game-admin/sticker-packs?tag=x", "/game-admin/gifs?translation_status=partial"} {
		if r := call(designer, http.MethodGet, q, nil); r.status != 422 {
			t.Errorf("%s: %d %s", q, r.status, r.raw)
		}
	}
	authztest.Cover(t, "diffGameConfigVersions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "rollbackGameConfig", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "unpublishGameConfig", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listGameConfigReleases", authztest.CrossTenant, authztest.RoleDenied)
	if r := call(f.staff(f.tenantA, "tenant_admin"), http.MethodPost, pbase+":rollback", map[string]any{"to_version": 1, "reason": "x"}); r.status != http.StatusForbidden {
		t.Fatalf("tenant role rollback: %d", r.status)
	}
}
