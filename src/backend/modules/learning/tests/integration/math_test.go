//go:build integration

package integration

import (
	"bytes"
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
)

// A MathJax 4 style SVG with a script and an event handler (TEST FIXTURE).
const mathSVG = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 -1000 928 1458" onload="alert(1)">` +
	`<defs><path id="MJX-1-TEX-N-33" d="M127 463Q100 463 85 480T69 524"></path></defs><script>alert(2)</script>` +
	`<g stroke="currentColor" fill="currentColor"><use data-c="33" xlink:href="#MJX-1-TEX-N-33"></use></g></svg>`

func mathQuestion(qc qbCurriculum, fx map[string]any, latex string, asset map[string]any) map[string]any {
	fx["stem"] = map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": "Tính"}, map[string]any{"type": "math", "latex": latex}}}
	return qc.body(fx, map[string]any{"math_assets": []any{asset}})
}

// F027-AC-12, AC-13, AC-15 (TC-MATH-001, TC-MATH-002), SC-004: the server checks the LaTeX allowlist, sanitizes and
// stores the SVG once at math/<sha256>.svg; a published version keeps its SVG when a later version re-renders.
func TestMathFormulas(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	frac := `\frac{3}{4}`
	r := must(t, c.createQ(t, qlcmA, mathQuestion(qc, qbFixture(t, "single_choice"), frac,
		map[string]any{"latex": frac, "svg": mathSVG, "speech_text": "ba phần tư"})), http.StatusCreated, "formula")
	assets, _ := r.body["math_assets"].([]any)
	if len(assets) != 1 {
		t.Fatalf("math_assets %s", r.raw)
	}
	key, _ := assets[0].(map[string]any)["svg_asset"].(string)
	if !strings.HasPrefix(key, "math/") || strings.Contains(r.raw, "<svg") {
		t.Fatalf("stored form %s", r.raw)
	}
	svg := c.object(ctx, t, pkgmedia.BucketPublic, key)
	if bytes.Contains(svg, []byte("<script")) || bytes.Contains(svg, []byte("onload")) || !bytes.Contains(svg, []byte(`xlink:href="#MJX-1-TEX-N-33"`)) {
		t.Fatalf("stored SVG not sanitized: %s", svg)
	}
	h, err := c.media.s3.Stat(ctx, pkgmedia.BucketPublic, key)
	if err != nil || h.Get("Cache-Control") != pkgmedia.ImmutableCacheControl {
		t.Fatalf("headers %v %v", h, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.math_svgs WHERE storage_key = $1 AND latex = $2 AND speech_text = 'ba phần tư'`,
		key, frac); n != 1 {
		t.Fatalf("math_svgs row %d", n)
	}
	// AC-12 / AC-13: refused formulas carry MATH_RENDER_FAILED and the JSON pointer.
	bad := []struct {
		name, latex, field string
		asset              map[string]any
	}{
		{"href", `\href{http://x}{y}`, "/math_assets/0/latex", map[string]any{"svg": mathSVG}},
		{"no svg", `x^2`, "/math_assets/0/svg", map[string]any{}},
		{"unknown svg_asset", `x^3`, "/math_assets/0/svg_asset", map[string]any{"svg_asset": "math/" + strings.Repeat("0", 64) + ".svg"}},
		{"not svg", `x^4`, "/math_assets/0/svg", map[string]any{"svg": "<html/>"}},
		{"too big", `x^5`, "/math_assets/0/svg", map[string]any{"svg": `<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat(`<path d="M0 0"/>`, 17000) + `</svg>`}},
	}
	for _, b := range bad {
		a := map[string]any{"latex": b.latex, "speech_text": "x"}
		for k, v := range b.asset {
			a[k] = v
		}
		r := c.createQ(t, qlcmA, mathQuestion(qc, qbFixture(t, "single_choice"), b.latex, a))
		if b.name == "too big" { // over math_svg_max = the OpenAPI maxLength of svg: refused before the use case
			if r.status != http.StatusUnprocessableEntity {
				t.Errorf("%s: %d %s", b.name, r.status, r.raw)
			}
			continue
		}
		if r.status != http.StatusUnprocessableEntity || r.str("code") != "MATH_RENDER_FAILED" || !slices.Contains(r.errorCodes(), b.field+":INVALID") {
			t.Errorf("%s: %d %s", b.name, r.status, r.raw)
		}
	}
	// AC-15: publish v1, copy it to v2 (same SVG), then re-render the formula in v2 → new key; v1 and its object stay.
	id := r.str("content_id")
	c.publishQuestion(ctx, t, id, 1)
	v2 := must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}), http.StatusCreated, "v2")
	if !strings.Contains(v2.raw, key) {
		t.Fatalf("v2 lost the SVG: %s", v2.raw)
	}
	newer := strings.Replace(mathSVG, `M127 463Q100 463 85 480T69 524`, `M128 463Q100 463 85 480T69 524`, 1)
	p := must(t, c.do(t, http.MethodPatch, "/cms/learning/questions/"+id+"/versions/2", qlcmA, ifm(v2.header.Get("ETag")),
		map[string]any{"math_assets": []any{map[string]any{"latex": frac, "svg": newer, "speech_text": "ba phần tư"}}}), http.StatusOK, "re-render v2")
	if strings.Contains(p.raw, key) {
		t.Fatalf("v2 kept the old SVG: %s", p.raw)
	}
	v1 := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+id+"/versions/1", qlcmA, nil, nil), http.StatusOK, "v1")
	if !strings.Contains(v1.raw, key) || !bytes.Equal(c.object(ctx, t, pkgmedia.BucketPublic, key), svg) {
		t.Fatalf("v1 SVG changed: %s", v1.raw)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.math_svgs SET latex = 'x' WHERE storage_key = $1`, key); err == nil {
		t.Fatal("a stored SVG row was updated")
	}
}
