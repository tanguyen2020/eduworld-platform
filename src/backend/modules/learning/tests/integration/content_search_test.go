//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// F024-AC-17, AC-18, AC-19 (TC-CMS-TEN-01, TC-TEN-024): "nhan vat" finds "nhân vật" in the five lists, combined with
// filters, inside the tenant scope only, without deleted rows; System Admin reads tenant_ids; q over 200 is 422.
func TestContentSearch(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	c.assign(ctx, t, qc.cur, c.tenantB)
	stem := map[string]any{"stem": rich("Nhân vật Mèo có 7 con cá")}
	_, qv := c.question(t, qlcmA, qc, stem)
	c.question(t, qlcmA, qc, nil)
	c.question(t, qlcmB, qc, stem)
	set := setBody(qc, staticDef(qv))
	set["title"] = map[string]any{"vi": "Bộ câu Nhân Vật"}
	must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), set), http.StatusCreated, "set")
	l := must(t, c.do(t, http.MethodPost, lessonsPath, qlcmA, idem(), lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("Bài về loài mèo")})),
		http.StatusCreated, "lesson")
	gone := must(t, c.do(t, http.MethodPost, lessonsPath, qlcmA, idem(), lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("nhân vật đã xóa")})),
		http.StatusCreated, "lesson to delete")
	must(t, c.do(t, http.MethodDelete, lessonsPath+"/"+gone.str("content_id"), qlcmA, ifm(gone.header.Get("ETag")), nil), http.StatusNoContent, "delete")
	mb := lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("Học liệu nhân vật")})
	delete(mb, "completion_rule")
	mb["material_kind"] = "rich_text"
	must(t, c.do(t, http.MethodPost, materialsPath, qlcmA, idem(), mb), http.StatusCreated, "material")
	bp := blueprintBody(qc, 1, []any{map[string]any{"objective_ids": []string{qc.lo}, "count": 1, "points_each": 1}}, nil)
	bp["title"] = map[string]any{"vi": "Đề kiểm tra nhân vật"}
	must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), bp), http.StatusCreated, "blueprint")
	q := "?q=" + url.QueryEscape("nhan vat")
	n := func(r resp) int { return len(r.items("data")) }
	for path, want := range map[string]int{"/cms/learning/questions": 1, setsPath: 1, lessonsPath: 1, materialsPath: 1, blueprintsPath: 1} {
		r := must(t, c.do(t, http.MethodGet, path+q, qlcmA, nil, nil), http.StatusOK, "search "+path)
		if n(r) != want {
			t.Errorf("%s: %d hits, want %d: %s", path, n(r), want, r.raw)
		}
	}
	lessons := must(t, c.do(t, http.MethodGet, lessonsPath+"?q=meo&grade_level=3&topic_id="+qc.topic+"&objective_id="+qc.lo, qlcmA, nil, nil),
		http.StatusOK, "lesson filters")
	if n(lessons) != 1 || lessons.items("data")[0]["content_id"] != l.str("content_id") {
		t.Fatalf("lesson filters %s", lessons.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, lessonsPath+"?q=meo&grade_level=4", qlcmA, nil, nil), http.StatusOK, "grade 4"); n(r) != 0 {
		t.Fatalf("grade filter %s", r.raw)
	}
	// AC-18: tenant B sees only its own question; System Admin with tenant_ids sees both.
	if r := must(t, c.do(t, http.MethodGet, "/cms/learning/questions"+q, qlcmB, nil, nil), http.StatusOK, "B"); n(r) != 1 {
		t.Fatalf("tenant B %s", r.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, setsPath+q, qlcmB, nil, nil), http.StatusOK, "B sets"); n(r) != 0 {
		t.Fatalf("tenant B sets %s", r.raw)
	}
	sa := must(t, c.do(t, http.MethodGet, "/cms/learning/questions"+q+"&tenant_ids="+c.tenantA.String()+","+c.tenantB.String(), saAdmin, nil, nil),
		http.StatusOK, "SA")
	if n(sa) != 2 {
		t.Fatalf("System Admin %s", sa.raw)
	}
	// AC-19.
	long := "?q=" + strings.Repeat("a", 201)
	for _, path := range []string{"/cms/learning/questions", setsPath, lessonsPath, materialsPath, blueprintsPath} {
		must(t, c.do(t, http.MethodGet, path+long, qlcmA, nil, nil), http.StatusUnprocessableEntity, "long q "+path)
	}
}
