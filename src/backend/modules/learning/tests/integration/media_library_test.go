//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media/mediatest"
)

// readyImage uploads and processes an image of tenant A (a different picture per size).
func (c *curEnv) readyImage(ctx context.Context, t *testing.T, sub string, size int, extra map[string]any) string {
	t.Helper()
	id, done := c.upload(t, sub, "image", "image/jpeg", mediatest.JPEGWithEXIF(t, size, size, 1), extra)
	must(t, done, http.StatusOK, "complete")
	if err := c.process(ctx, t, id); err != nil {
		t.Fatal(err)
	}
	return id
}

// questionWithImage creates a tenant A question whose stem shows the image.
func (c *curEnv) questionWithImage(t *testing.T, qc qbCurriculum, image string) resp {
	t.Helper()
	fx := qbFixture(t, "single_choice")
	fx["stem"] = map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": "Quan sát hình"},
		map[string]any{"type": "image", "media_ref": image, "alt": "hình"}}}
	return must(t, c.createQ(t, qlcmA, qc.body(fx, nil)), http.StatusCreated, "question with image")
}

// F027-AC-09, AC-10 (TC-CMS-TEN-01): the library filters by kind, status and name/alt text in the session tenant;
// System Admin reads the tenants of tenant_ids; an asset lists the versions using it.
func TestMediaLibrary(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	img := c.readyImage(ctx, t, edA, 40, map[string]any{"alt_text": "Con mèo tam thể", "filename": "meo.jpg"})
	bad, _ := c.upload(t, edA, "image", "image/png", []byte(`EICAR-like eicar.png bytes X5O!P%@AP EICAR`), map[string]any{"filename": "eicar.png"})
	if err := c.process(ctx, t, bad); err != nil {
		t.Fatal(err)
	}
	c.readyImage(ctx, t, edB, 41, map[string]any{"alt_text": "Con mèo của B"})
	ids := func(r resp) []string {
		var out []string
		for _, it := range r.items("data") {
			out = append(out, it["asset_id"].(string))
		}
		return out
	}
	all := must(t, c.do(t, http.MethodGet, "/cms/media", edA, nil, nil), http.StatusOK, "list")
	if got := ids(all); len(got) != 2 || !slices.Contains(got, img) || !slices.Contains(got, bad) {
		t.Fatalf("tenant A list %v", got)
	}
	rej := must(t, c.do(t, http.MethodGet, "/cms/media?kind=image&status=rejected&q=EICAR", edA, nil, nil), http.StatusOK, "rejected")
	if got := ids(rej); len(got) != 1 || got[0] != bad {
		t.Fatalf("rejected filter %v", got)
	}
	cat := must(t, c.do(t, http.MethodGet, "/cms/media?q=m%C3%A8o", edA, nil, nil), http.StatusOK, "q alt text")
	if got := ids(cat); len(got) != 1 || got[0] != img {
		t.Fatalf("q filter %v", got)
	}
	page1 := must(t, c.do(t, http.MethodGet, "/cms/media?limit=1", edA, nil, nil), http.StatusOK, "page 1")
	cur, _ := page1.body["page"].(map[string]any)["next_cursor"].(string)
	page2 := must(t, c.do(t, http.MethodGet, "/cms/media?limit=1&cursor="+cur, edA, nil, nil), http.StatusOK, "page 2")
	if len(ids(page1)) != 1 || len(ids(page2)) != 1 || ids(page1)[0] == ids(page2)[0] {
		t.Fatalf("pages %v %v", ids(page1), ids(page2))
	}
	sa := must(t, c.do(t, http.MethodGet, "/cms/media?tenant_ids="+c.tenantA.String()+","+c.tenantB.String(), saAdmin, nil, nil), http.StatusOK, "SA")
	if got := ids(sa); len(got) != 3 {
		t.Fatalf("System Admin tenant_ids: %v", got)
	}
	for _, it := range sa.items("data") {
		if it["tenant_id"] == nil {
			t.Fatalf("no tenant_id: %v", it)
		}
	}
	// AC-10: two questions use the image → used_in lists both versions.
	q1 := c.questionWithImage(t, qc, img)
	q2 := c.questionWithImage(t, qc, img)
	got := must(t, c.do(t, http.MethodGet, "/cms/media/"+img, edA, nil, nil), http.StatusOK, "get")
	used := got.items("used_in")
	if len(used) != 2 {
		t.Fatalf("used_in %s", got.raw)
	}
	for _, u := range used {
		if u["item_type"] != "question" || u["version"] != float64(1) || (u["item_id"] != q1.str("content_id") && u["item_id"] != q2.str("content_id")) {
			t.Fatalf("usage %v", u)
		}
	}
}

// F027-AC-11: saving a version records its media usages; a removed reference is dropped; an unknown asset or more
// than assets_per_version_max assets is 422.
func TestMediaUsagesOnSave(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	a := c.readyImage(ctx, t, edA, 30, nil)
	b := c.readyImage(ctx, t, edA, 31, nil)
	q := c.questionWithImage(t, qc, a)
	id := q.str("content_id")
	path := "/cms/learning/questions/" + id + "/versions/1"
	must(t, c.do(t, http.MethodPatch, path, qlcmA, ifm(q.header.Get("ETag")), map[string]any{"media_refs": []string{b}}), http.StatusOK, "add b")
	usages := func(asset string) int {
		return count(ctx, t, c.platform, `SELECT count(*) FROM content.media_usages WHERE asset_id = $1 AND deleted_at IS NULL`, asset)
	}
	if usages(a) != 1 || usages(b) != 1 {
		t.Fatalf("usages a=%d b=%d", usages(a), usages(b))
	}
	etag := must(t, c.do(t, http.MethodGet, path, qlcmA, nil, nil), http.StatusOK, "get").header.Get("ETag")
	fx := qbFixture(t, "single_choice")
	must(t, c.do(t, http.MethodPatch, path, qlcmA, ifm(etag), map[string]any{"stem": fx["stem"]}), http.StatusOK, "drop a")
	if usages(a) != 0 || usages(b) != 1 {
		t.Fatalf("after drop a=%d b=%d", usages(a), usages(b))
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.media_usages WHERE asset_id = $1 AND deleted_at IS NOT NULL`, a); n != 1 {
		t.Fatalf("soft-deleted usages %d", n)
	}
	r := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"media_refs": []string{uuid.NewString()}})),
		http.StatusUnprocessableEntity, "unknown asset")
	if !slices.Contains(r.errorCodes(), "/media_refs/0:INVALID") {
		t.Fatalf("errors %v", r.errorCodes())
	}
	// Tenant B's asset is not in tenant A's library.
	bAsset := c.readyImage(ctx, t, edB, 33, nil)
	must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"media_refs": []string{bAsset}})),
		http.StatusUnprocessableEntity, "other tenant asset")
	blocks := []any{}
	for range 51 {
		blocks = append(blocks, map[string]any{"type": "image", "media_ref": uuid.NewString(), "alt": "h"})
	}
	r = must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"stem": map[string]any{"blocks": blocks}})), http.StatusUnprocessableEntity, "51 in content")
	if !strings.Contains(r.raw, "assets_per_version_max") {
		t.Fatalf("content limit: %s", r.raw)
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = uuid.NewString()
	}
	r = must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"media_refs": many})), http.StatusUnprocessableEntity, "51")
	if !strings.Contains(r.raw, "media_refs") { // OpenAPI maxItems 50 = assets_per_version_max
		t.Fatalf("limit: %s", r.raw)
	}
}

// F027-AC-08 (ADR-0044 §4): replacing a picture is a new asset; the published version keeps the old one.
func TestMediaReplaceKeepsOldVersion(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	a := c.readyImage(ctx, t, edA, 50, nil)
	b := c.readyImage(ctx, t, edA, 51, nil)
	if a == b {
		t.Fatal("same asset")
	}
	q := c.questionWithImage(t, qc, a)
	id := q.str("content_id")
	c.publishQuestion(ctx, t, id, 1)
	v2 := must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}),
		http.StatusCreated, "v2")
	fx := map[string]any{"blocks": []any{map[string]any{"type": "image", "media_ref": b, "alt": "hình mới"}}}
	must(t, c.do(t, http.MethodPatch, "/cms/learning/questions/"+id+"/versions/2", qlcmA, ifm(v2.header.Get("ETag")), map[string]any{"stem": fx}),
		http.StatusOK, "replace in v2")
	v1 := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+id+"/versions/1", qlcmA, nil, nil), http.StatusOK, "v1")
	if !strings.Contains(v1.raw, a) || strings.Contains(v1.raw, b) {
		t.Fatalf("v1 changed: %s", v1.raw)
	}
	usedA := must(t, c.do(t, http.MethodGet, "/cms/media/"+a, edA, nil, nil), http.StatusOK, "a").items("used_in")
	usedB := must(t, c.do(t, http.MethodGet, "/cms/media/"+b, edA, nil, nil), http.StatusOK, "b").items("used_in")
	if len(usedA) != 1 || usedA[0]["version"] != float64(1) || len(usedB) != 1 || usedB[0]["version"] != float64(2) {
		t.Fatalf("used_in a=%v b=%v", usedA, usedB)
	}
	// The asset row has no field for its file: no endpoint can overwrite it (status/variants only move forward).
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.media_assets WHERE asset_id IN ($1, $2) AND status = 'ready'`, a, b); n != 2 {
		t.Fatalf("ready %d", n)
	}
}
