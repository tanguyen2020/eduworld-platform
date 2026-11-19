//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// OQ F053 follow-ups of F021 and F024: topics and objectives carry row_version so a list can send If-Match; a lesson
// keeps the rendered formulas of its blocks (LessonAuthoringInput.math_assets, stored like question formulas).
func TestF053Followups(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	l := must(t, c.do(t, http.MethodGet, "/cms/learning/topics?curriculum_version_id="+qc.version, cmNoidung, nil, nil), http.StatusOK, "topics")
	items := l.items("data")
	if len(items) == 0 || items[0]["row_version"] == nil {
		t.Fatalf("topic row_version: %s", l.raw)
	}
	tag := fmt.Sprintf(`W/"%v"`, items[0]["row_version"])
	must(t, c.do(t, http.MethodPatch, "/cms/learning/topics/"+qc.topic, cmNoidung, ifm(tag), map[string]any{"name": "Chủ đề đổi tên"}),
		http.StatusOK, "patch topic with the list row_version")
	o := must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?curriculum_version_id="+qc.version+"&topic_id="+qc.topic, cmNoidung, nil, nil),
		http.StatusOK, "objectives")
	if os := o.items("data"); len(os) == 0 || os[0]["row_version"] == nil {
		t.Fatalf("objective row_version: %s", o.raw)
	}
	body := lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("3/4")})
	body["owner_scope"] = "platform"
	body["math_assets"] = []any{map[string]any{"latex": `\frac{3}{4}`, "svg": mathSVG, "speech_text": "ba phần tư"}}
	r := must(t, c.do(t, http.MethodPost, lessonsPath, cmNoidung, idem(), body), http.StatusCreated, "lesson with formulas")
	assets, _ := r.body["math_assets"].([]any)
	if len(assets) != 1 || !strings.HasPrefix(fmt.Sprint(assets[0].(map[string]any)["svg_asset"]), "math/") || strings.Contains(r.raw, "<svg") {
		t.Fatalf("lesson math_assets: %s", r.raw)
	}
}
