//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// F025 T046 (OQ F054-1, owner 2026-11-20): transitionContentVersion takes field_comments[] with a reject only (≤ 50,
// path JSON pointer ≤ 256, comment ≤ 1000); they are kept on the review task and the transition and read back in
// ContentReviewTask and ContentVersionSummary.
func TestWorkflowFieldComments(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id, vid := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "grade_levels": []int{4}})
	must(t, c.act(ctx, t, cmNoidung, "question", id, 1, "submit_review", ""), http.StatusOK, "submit")
	comments := []map[string]any{{"path": "/stem/vi", "comment": "Sai chính tả ở câu hỏi"}, {"path": "/options/1", "comment": "Phương án trùng"}}
	wf := func(body map[string]any) resp { return c.wfDo(ctx, t, cmDuyet, "question", id, 1, body, nil) }
	// Only a reject carries field comments.
	if r := wf(map[string]any{"action": "approve", "field_comments": comments}); r.status != http.StatusUnprocessableEntity ||
		!slices.Contains(r.errorCodes(), "/field_comments:INVALID") {
		t.Fatalf("approve with comments: %d %s", r.status, r.raw)
	}
	// Invalid entries: not a JSON pointer, blank comment, too long, too many.
	if r := wf(map[string]any{"action": "reject", "note": "x", "field_comments": []map[string]any{{"path": "stem", "comment": "x"},
		{"path": "/ok", "comment": "  "}}}); r.status != http.StatusUnprocessableEntity ||
		!slices.Contains(r.errorCodes(), "/field_comments/0/path:INVALID") || !slices.Contains(r.errorCodes(), "/field_comments/1/comment:REQUIRED") {
		t.Fatalf("invalid entries: %d %s", r.status, r.raw)
	}
	if r := wf(map[string]any{"action": "reject", "note": "x", "field_comments": []map[string]any{{"path": "/a",
		"comment": strings.Repeat("a", 1001)}}}); r.status != http.StatusUnprocessableEntity || r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("comment > 1000: %d %s", r.status, r.raw)
	}
	many := make([]map[string]any, 51)
	for i := range many {
		many[i] = map[string]any{"path": "/a", "comment": "b"}
	}
	if r := wf(map[string]any{"action": "reject", "note": "x", "field_comments": many}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("51 comments: %d %s", r.status, r.raw)
	}
	// A valid reject keeps them on the task and the transition.
	must(t, wf(map[string]any{"action": "reject", "note": "Cần sửa hai chỗ", "field_comments": comments}), http.StatusOK, "reject")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.content_transitions WHERE content_version_id = $1 AND action = 'reject'
		AND jsonb_array_length(field_comments) = 2`, vid); n != 1 {
		t.Fatalf("transition field comments = %d", n)
	}
	tasks := must(t, c.do(t, http.MethodGet, "/cms/learning/review-tasks?lane=platform_learning&status=rejected", cmDuyet, nil, nil), http.StatusOK,
		"rejected tasks")
	var task map[string]any
	for _, x := range tasks.items("data") {
		if x["content_version_id"] == vid {
			task = x
		}
	}
	if fc, _ := task["field_comments"].([]any); len(fc) != 2 || fc[0].(map[string]any)["path"] != "/stem/vi" {
		t.Fatalf("task field comments: %s", tasks.raw)
	}
	hist := must(t, c.do(t, http.MethodGet, "/cms/learning/content/question/"+id+"/versions", cmNoidung, nil, nil), http.StatusOK, "history")
	data := hist.items("data")
	if len(data) != 1 {
		t.Fatalf("history: %s", hist.raw)
	}
	if fc, _ := data[0]["field_comments"].([]any); len(fc) != 2 || fc[1].(map[string]any)["comment"] != "Phương án trùng" {
		t.Fatalf("history field comments: %s", hist.raw)
	}
}
