//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/storage"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// memStorage is a TEST DOUBLE (R17) of the S3 bucket of the library (no S3 container in testkit): PresignPut returns a
// fake URL, the test "uploads" by writing the bytes; Head/Open read them. The real inspector reads these bytes.
type memStorage struct {
	mu   sync.Mutex
	objs map[string][]byte
}

func (m *memStorage) PresignPut(_ context.Context, key string, _ int64, _ time.Duration) (string, error) {
	return "https://storage.test.invalid/" + key + "?X-Amz-Signature=test", nil
}

func (m *memStorage) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test.invalid/" + key + "?X-Amz-Signature=get", nil
}

func (m *memStorage) Head(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objs[key]
	if !ok {
		return 0, port.ErrObjectNotFound
	}
	return int64(len(b)), nil
}

func (m *memStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objs[key]
	if !ok {
		return nil, port.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memStorage) put(key string, b []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objs[key] = b
}

func pngOf(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func gifOf(w, h, frames int) []byte {
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		p := image.NewPaletted(image.Rect(0, 0, w, h), []color.Color{color.Black, color.White})
		p.SetColorIndex(i%w, 0, 1)
		g.Image, g.Delay = append(g.Image, p), append(g.Delay, 10)
	}
	var b bytes.Buffer
	_ = gif.EncodeAll(&b, g)
	return b.Bytes()
}

func shaOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// T020 — F118-AC-17 (TC-CHAT-033): only a platform role with game.chat_assets.write composes the library (a tenant role
// gets 403 PERMISSION_DENIED); files are checked from the bucket (format sniffed, size, ≤ 480 px, static preview), a GIF
// is approved by someone else than its author, a published version never changes (a new version is created), every
// action is audited and a publish writes content.published; the published GIF is then served to the students.
func TestChatAssetsAdmin(t *testing.T) {
	store := &memStorage{objs: map[string][]byte{}}
	f := setupFeatures(t, chatOpts{hook: func(c *chatEnv) {
		lim := chatasset.Limits{StickerMaxBytes: 512 * 1024, GifMaxBytes: 2 * 1024 * 1024, GifMaxPixels: 480}
		c.assetHandlers = &chathttp.AssetHandlers{
			Commands: command.ChatAssets{UoW: c.uow, Repo: persistence.Assets{}, Storage: store, Inspector: storage.Inspector{Storage: store},
				Events: persistence.Publisher{W: c.writer}, Audit: persistence.Auditor{R: c.rec}, Limits: lim, Now: c.clock.Now},
			Queries: query.ChatAssets{Read: c.read, Reader: persistence.Assets{}}}
	}})
	startFeatures(t, f)
	author, reviewer := f.platformStaff("system_admin"), f.platformStaff("system_admin")
	tenantAdmin := f.staff(f.tenantA, "tenant_admin")
	call := func(token, method, path string, ifMatch string, body any) resp {
		h := map[string]string{"Idempotency-Key": newKey()}
		if ifMatch != "" {
			h["If-Match"] = ifMatch
		}
		if method == http.MethodPatch {
			h["Content-Type"] = "application/merge-patch+json"
		}
		return f.do(method, path, token, h, body)
	}
	// A tenant role never composes the library (R20).
	if r := call(tenantAdmin, http.MethodPost, "/game-admin/gifs", "", map[string]any{"payload": map[string]any{}}); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("tenant role: %d %s", r.status, r.raw)
	}
	r := call(author, http.MethodPost, "/game-admin/gifs", "", map[string]any{"title": "Cổ vũ", "payload": map[string]any{}})
	if r.status != http.StatusCreated || r.body["status"] != "draft" || r.body["version"] != float64(1) {
		t.Fatalf("create gif: %d %s", r.status, r.raw)
	}
	id := r.body["config_id"].(string)
	base := "/game-admin/gifs/" + id
	// Presign: the size limit applies before signing; the key is media/<sha256>/<file>.
	big := gifOf(10, 10, 1)
	if r := call(author, http.MethodPost, base+"/assets:presign", "", map[string]any{"file_name": "a.gif", "content_type": "image/gif",
		"size_bytes": 3 * 1024 * 1024, "sha256": shaOf(big)}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("3 MB presign: %d %s", r.status, r.raw)
	}
	anim, still := gifOf(320, 240, 2), pngOf(320, 240)
	keys := map[string]string{}
	for name, b := range map[string][]byte{"cheer.gif": anim, "cheer.png": still} {
		ct := "image/gif"
		if strings.HasSuffix(name, ".png") {
			ct = "image/png"
		}
		p := call(author, http.MethodPost, base+"/assets:presign", "", map[string]any{"file_name": name, "content_type": ct,
			"size_bytes": len(b), "sha256": shaOf(b)})
		if p.status != http.StatusCreated || !strings.HasPrefix(p.body["asset_key"].(string), "media/"+shaOf(b)+"/") {
			t.Fatalf("presign %s: %d %s", name, p.status, p.raw)
		}
		keys[name] = p.body["asset_key"].(string)
		store.put(keys[name], b)
	}
	payload := map[string]any{"asset_key": keys["cheer.gif"], "preview_key": keys["cheer.png"], "size_bytes": len(anim), "width": 320,
		"height": 240, "tags_vi": []string{"co vu"}, "tags_en": []string{"cheer"}}
	if r := call(author, http.MethodPatch, base+"/versions/1", `"1"`, map[string]any{"payload": payload}); r.status != http.StatusOK {
		t.Fatalf("update draft: %d %s", r.status, r.raw)
	}
	if r := call(author, http.MethodPost, base+"/versions/1:transition", `"1"`, map[string]any{"action": "submit_review"}); r.status != http.StatusOK || r.body["status"] != "in_review" {
		t.Fatalf("submit: %d %s", r.status, r.raw)
	}
	// Four eyes: the author cannot approve its GIF.
	if r := call(author, http.MethodPost, base+"/versions/1:transition", `"1"`, map[string]any{"action": "approve"}); r.status != http.StatusForbidden {
		t.Fatalf("self review: %d %s", r.status, r.raw)
	}
	if r := call(reviewer, http.MethodPost, base+"/versions/1:transition", `"1"`, map[string]any{"action": "approve"}); r.status != http.StatusOK || r.body["status"] != "approved" {
		t.Fatalf("approve: %d %s", r.status, r.raw)
	}
	if r := call(reviewer, http.MethodPost, base+"/versions/1:publish", `"1"`, nil); r.status != http.StatusOK || r.body["to_version"] != float64(1) {
		t.Fatalf("publish: %d %s", r.status, r.raw)
	}
	if n := f.count(`SELECT count(*) FROM chat.gif_assets WHERE gif_id = $1 AND status = 'published' AND width = 320`, id); n != 1 {
		t.Fatal("GIF not materialized")
	}
	// Published versions are immutable (R31): the database refuses a change, an edit starts version 2.
	if _, err := f.su.Exec(`UPDATE chat.chat_asset_versions SET payload = '{}' WHERE config_id = $1 AND version = 1`, id); err == nil {
		t.Fatal("a published version was changed")
	}
	if r := call(author, http.MethodPatch, base+"/versions/1", `"1"`, map[string]any{"payload": payload}); r.status != http.StatusOK || r.body["version"] != float64(2) {
		t.Fatalf("edit after publish: %d %s", r.status, r.raw)
	}
	if len(f.events("content.published")) != 1 {
		t.Fatal("content.published missing")
	}
	f.checkEvents("content.published")
	if a := f.audits(); !containsAudit(a, command.AuditChatAssetCreated) || !containsAudit(a, command.AuditChatAssetPublished) ||
		!containsAudit(a, command.AuditChatAssetTransition) {
		t.Fatalf("audits: %v", a)
	}
	// The published GIF is served to students (internal library only).
	s := f.student(f.tenantA, f.schoolA)
	if g := f.call(s, http.MethodGet, "/chat/gifs?tag=cheer", "", nil); g.status != http.StatusOK || len(g.data()) != 1 {
		t.Fatalf("student gifs: %d %s", g.status, g.raw)
	}
	// File rules: a GIF over 480 px, an animated preview, a sticker over 512 KB or not WebP/PNG are refused at submit.
	tooWide := gifOf(500, 10, 1)
	g2 := call(author, http.MethodPost, "/game-admin/gifs", "", map[string]any{"payload": map[string]any{}}).body["config_id"].(string)
	wideKey, prevKey := "media/"+shaOf(tooWide)+"/wide.gif", "media/"+shaOf(anim)+"/anim.gif"
	store.put(wideKey, tooWide)
	store.put(prevKey, anim)
	for _, c := range []struct {
		asset, preview, want string
	}{{wideKey, keys["cheer.png"], "dimensions"}, {keys["cheer.gif"], prevKey, "preview"}} {
		if r := call(author, http.MethodPatch, "/game-admin/gifs/"+g2+"/versions/1", `"1"`, map[string]any{"payload": map[string]any{
			"asset_key": c.asset, "preview_key": c.preview}}); r.status != http.StatusOK {
			t.Fatalf("draft: %d %s", r.status, r.raw)
		}
		if r := call(author, http.MethodPost, "/game-admin/gifs/"+g2+"/versions/1:transition", `"1"`, map[string]any{"action": "submit_review"}); r.status != http.StatusUnprocessableEntity || !strings.Contains(r.raw, c.want) {
			t.Fatalf("file rule %s: %d %s", c.want, r.status, r.raw)
		}
	}
	huge := append(pngOf(2, 2), bytes.Repeat([]byte{0}, 600*1024)...)
	hugeKey := "media/" + shaOf(huge) + "/huge.png"
	store.put(hugeKey, huge)
	pr := call(author, http.MethodPost, "/game-admin/sticker-packs", "", map[string]any{"payload": map[string]any{"code": "vui-ve",
		"name_vi": "Vui vẻ", "name_en": "Happy", "stickers": []map[string]any{{"asset_key": hugeKey, "content_type": "image/png", "size_bytes": len(huge),
			"alt_text_vi": "Mèo", "alt_text_en": "Cat"}}}})
	if pr.status != http.StatusCreated {
		t.Fatalf("create pack: %d %s", pr.status, pr.raw)
	}
	if r := call(author, http.MethodPost, "/game-admin/sticker-packs/"+pr.body["config_id"].(string)+"/versions/1:transition", `"1"`,
		map[string]any{"action": "submit_review"}); r.status != http.StatusUnprocessableEntity || !strings.Contains(r.raw, "size") {
		t.Fatalf("sticker over 512 KB: %d %s", r.status, r.raw)
	}
	if l := call(reviewer, http.MethodGet, "/game-admin/gifs", "", nil); l.status != http.StatusOK || len(l.data()) < 2 {
		t.Fatalf("list: %d %s", l.status, l.raw)
	}
	if v := call(reviewer, http.MethodGet, base+"/versions", "", nil); v.status != http.StatusOK || v.body["current_version"] != float64(1) {
		t.Fatalf("versions: %d %s", v.status, v.raw)
	}
	if r := call(tenantAdmin, http.MethodGet, base+"/versions/1", "", nil); r.status != http.StatusForbidden {
		t.Fatalf("tenant role read: %d", r.status)
	}
	// Platform library: a tenant role and a student are refused whatever the tenant (no tenant data behind these routes).
	authztest.Cover(t, "listGameConfigs", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createGameConfig", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getGameConfigVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateGameConfigDraft", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "transitionGameConfigVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listGameConfigVersions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "publishGameConfigVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "presignChatAssetUpload", authztest.CrossTenant, authztest.RoleDenied)
}
