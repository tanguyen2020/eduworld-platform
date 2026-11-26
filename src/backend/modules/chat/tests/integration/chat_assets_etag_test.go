//go:build integration

package integration

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/storage"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
)

// versionWrite matches the path of a version write (draft edit, transition, publish); group 1 is the version to GET for its
// ETag.
var versionWrite = regexp.MustCompile(`^(/game-admin/[a-z-]+/[0-9a-f-]{36}/versions/[0-9]+)(:transition|:publish)?$`)

// TestChatAssetsOptimisticConcurrency — F082/F118 follow-up (Clarifications 2026-11-26): GET of a sticker-pack/GIF version
// answers ETag (the revision of the row), a draft edit, a transition and a publish require If-Match (428 when missing) and
// refuse a stale one with 412 VERSION_MISMATCH, so two Game Studio editors never overwrite each other silently. Every
// change of the row moves the ETag.
func TestChatAssetsOptimisticConcurrency(t *testing.T) {
	store := &memStorage{objs: map[string][]byte{}}
	f := setupFeatures(t, chatOpts{hook: func(c *chatEnv) {
		lim := chatasset.Limits{StickerMaxBytes: 512 * 1024, GifMaxBytes: 2 * 1024 * 1024, GifMaxPixels: 480}
		c.assetHandlers = &chathttp.AssetHandlers{
			Commands: command.ChatAssets{UoW: c.uow, Repo: persistence.Assets{}, Storage: store, Inspector: storage.Inspector{Storage: store},
				Events: persistence.Publisher{W: c.writer}, Audit: persistence.Auditor{R: c.rec}, Limits: lim, Now: c.clock.Now},
			Queries: query.ChatAssets{Read: c.read, Reader: persistence.Assets{}}}
	}})
	startFeatures(t, f)
	alice, bob := f.platformStaff("system_admin"), f.platformStaff("system_admin")
	call := func(token, method, path, ifMatch string, body any) resp {
		h := map[string]string{"Idempotency-Key": newKey()}
		if ifMatch != "" {
			h["If-Match"] = ifMatch
		}
		if method == http.MethodPatch {
			h["Content-Type"] = "application/merge-patch+json"
		}
		return f.do(method, path, token, h, body)
	}
	for _, typ := range []string{"sticker-packs", "gifs"} {
		t.Run(typ, func(t *testing.T) {
			first, second := map[string]any{}, map[string]any{}
			if typ == "sticker-packs" {
				first = map[string]any{"code": "dong-thoi-" + newKey()[:8], "name_vi": "Đồng thời", "stickers": []any{}}
				second = map[string]any{"code": first["code"], "name_vi": "Đồng thời 2", "stickers": []any{}}
			} else {
				first, second = map[string]any{"tags_vi": []string{"mot"}}, map[string]any{"tags_vi": []string{"hai"}}
			}
			c := call(alice, http.MethodPost, "/game-admin/"+typ, "", map[string]any{"title": "Một", "payload": first})
			if c.status != http.StatusCreated || c.header.Get("ETag") != `"1"` {
				t.Fatalf("create: %d %q %s", c.status, c.header.Get("ETag"), c.raw)
			}
			v1 := "/game-admin/" + typ + "/" + c.body["config_id"].(string) + "/versions/1"
			g := call(alice, http.MethodGet, v1, "", nil)
			if g.status != http.StatusOK || g.header.Get("ETag") != `"1"` {
				t.Fatalf("GET ETag: %d %q", g.status, g.header.Get("ETag"))
			}
			bobTag := call(bob, http.MethodGet, v1, "", nil).header.Get("ETag")
			// The reviewer and Live-ops read the version they approve/publish (action.game_ops.chat_assets.view) to get its ETag.
			for _, role := range []string{"content_reviewer", "liveops"} {
				if r := call(f.platformStaff(role), http.MethodGet, v1, "", nil); r.status != http.StatusOK || r.header.Get("ETag") != `"1"` {
					t.Fatalf("%s GET: %d %s", role, r.status, r.raw)
				}
			}
			// No If-Match → 428 PRECONDITION_REQUIRED (draft edit, transition, publish).
			if r := call(alice, http.MethodPatch, v1, "", map[string]any{"payload": first}); r.status != http.StatusPreconditionRequired || r.code() != "PRECONDITION_REQUIRED" {
				t.Fatalf("PATCH without If-Match: %d %s", r.status, r.raw)
			}
			if r := call(alice, http.MethodPost, v1+":transition", "", map[string]any{"action": "submit_review"}); r.status != http.StatusPreconditionRequired {
				t.Fatalf("transition without If-Match: %d %s", r.status, r.raw)
			}
			// Alice saves with the ETag she read: 200, a new ETag.
			a := call(alice, http.MethodPatch, v1, g.header.Get("ETag"), map[string]any{"title": "Một", "payload": first})
			if a.status != http.StatusOK || a.header.Get("ETag") != `"2"` {
				t.Fatalf("alice save: %d %q %s", a.status, a.header.Get("ETag"), a.raw)
			}
			// Bob still holds the old ETag: 412 VERSION_MISMATCH, nothing overwritten.
			b := call(bob, http.MethodPatch, v1, bobTag, map[string]any{"title": "Hai", "payload": second})
			if b.status != http.StatusPreconditionFailed || b.code() != "VERSION_MISMATCH" {
				t.Fatalf("bob stale save: %d %s", b.status, b.raw)
			}
			if r := call(bob, http.MethodGet, v1, "", nil); r.body["title"] != "Một" || r.header.Get("ETag") != `"2"` {
				t.Fatalf("bob overwrote alice: %s %q", r.raw, r.header.Get("ETag"))
			}
			// A weak tag of the current revision is accepted (W/"2"); the save moves the ETag again.
			if r := call(bob, http.MethodPatch, v1, `W/"2"`, map[string]any{"title": "Hai", "payload": second}); r.status != http.StatusOK || r.header.Get("ETag") != `"3"` {
				t.Fatalf("bob fresh save: %d %q %s", r.status, r.header.Get("ETag"), r.raw)
			}
			// A transition with a stale ETag is refused too; the current one passes and moves the ETag.
			if r := call(alice, http.MethodPost, v1+":transition", `"2"`, map[string]any{"action": "archive"}); r.status != http.StatusPreconditionFailed {
				t.Fatalf("stale transition: %d %s", r.status, r.raw)
			}
			if n := f.count(`SELECT count(*) FROM chat.chat_asset_versions WHERE config_type = $1 AND version = 1 AND revision = 3
				AND config_id = $2`, typ, c.body["config_id"]); n != 1 {
				t.Fatalf("revision column: %d", n)
			}
		})
	}
}
