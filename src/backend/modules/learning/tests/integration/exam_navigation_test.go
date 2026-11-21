//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
)

// F064 T004 (F064-AC-06): an exam session carries the navigation of its pinned blueprint version — the blueprint default
// `free`, or `sequential_no_return` — in the staff view, the student view and the student list.
func TestExamSessionNavigation(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 1, nil)
	free := c.createSession(t, k, nil)
	if got := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+free, hsAn, nil, nil), http.StatusOK, "free").str("navigation"); got != "free" {
		t.Fatalf("default navigation: %q", got)
	}
	body := blueprintBody(k.s.qc, 4, []any{k.loRule(2, 2)}, nil)
	body["definition"].(map[string]any)["navigation"] = "sequential_no_return"
	k.blueprint = must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), body), http.StatusCreated, "blueprint").str("content_id")
	k.version = c.publishDoc(ctx, t, "assessment_blueprints", "assessment_blueprint_versions", "assessment_blueprint_id", k.blueprint, 1)
	seq := c.createSession(t, k, nil)
	staff := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+seq, qlcmA, nil, nil), http.StatusOK, "staff")
	student := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+seq, hsAn, nil, nil), http.StatusOK, "student")
	if staff.str("navigation") != "sequential_no_return" || student.str("navigation") != "sequential_no_return" {
		t.Fatalf("navigation: staff %s student %s", staff.raw, student.raw)
	}
	seen := map[string]string{}
	for _, s := range must(t, c.do(t, http.MethodGet, examSessionsPath, hsAn, nil, nil), http.StatusOK, "list").items("data") {
		seen[s["exam_session_id"].(string)], _ = s["navigation"].(string)
	}
	if seen[free] != "free" || seen[seq] != "sequential_no_return" {
		t.Fatalf("list: %v", seen)
	}
}
