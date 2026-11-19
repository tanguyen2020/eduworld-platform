//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// F021-AC-26, AC-27 (US6): a version that supersedes another is published only when every LO of the old version
// has a mapping; the mapping table travels in curriculum.version_published; the old version is untouched.
func TestCurriculumLoMappings(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	math := c.subject(t, "math")
	cur, v1 := c.curriculum(t, cmNoidung, "MAP_MATH")
	topic := c.topic(t, cmNoidung, v1, math, "MATH.G3.T01", 3, "")
	a := must(t, c.objective(t, cmNoidung, topic, "MATH.G3.NUM.MUL", nil), http.StatusCreated, "A").str("objective_id")
	b := must(t, c.objective(t, cmNoidung, topic, "MATH.G3.NUM.DIV", nil), http.StatusCreated, "B").str("objective_id")
	must(t, c.publishFlow(t, cmNoidung, cur, 1), http.StatusOK, "publish v1")
	v2 := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions", cmNoidung, idem(), map[string]any{"from_version": 1,
		"version_label": "2.0"}), http.StatusCreated, "v2").str("curriculum_version_id")
	los := must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?curriculum_version_id="+v2, cmNoidung, nil, nil), http.StatusOK, "LOs v2")
	newID := map[string]string{}
	for _, lo := range los.items("data") {
		newID[lo["code"].(string)] = lo["objective_id"].(string)
	}
	t2 := must(t, c.do(t, http.MethodGet, "/cms/learning/topics?curriculum_version_id="+v2, cmNoidung, nil, nil), http.StatusOK, "topics v2").
		items("data")[0]["topic_id"].(string)
	split := must(t, c.objective(t, cmNoidung, t2, "MATH.G3.NUM.MUL.2", nil), http.StatusCreated, "split LO").str("objective_id")

	// Without mappings: GET lists both old LOs as unmapped; publish is refused listing them (AC-26).
	r := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+cur+"/versions/2/lo-mappings", cmNoidung, nil, nil), http.StatusOK, "mappings")
	if len(r.body["unmapped_lo_ids"].([]any)) != 2 || r.str("supersedes_version_id") != v1 {
		t.Fatalf("unmapped %s", r.raw)
	}
	tag := c.versionETag(t, cmNoidung, cur, 2)
	// Mapping only A (split into two LOs): B is still unmapped.
	partial := []map[string]any{{"from_lo_id": a, "to_lo_id": newID["MATH.G3.NUM.MUL"], "relation": "split"},
		{"from_lo_id": a, "to_lo_id": split, "relation": "split"}}
	r = must(t, c.do(t, http.MethodPut, "/cms/learning/curricula/"+cur+"/versions/2/lo-mappings", cmNoidung, ifm(tag), map[string]any{"items": partial}),
		http.StatusOK, "partial mappings")
	if un := r.body["unmapped_lo_ids"].([]any); len(un) != 1 || un[0] != b {
		t.Fatalf("partial %s", r.raw)
	}
	r = c.publishFlow(t, cmNoidung, cur, 2)
	if must(t, r, http.StatusUnprocessableEntity, "publish with an unmapped LO"); !strings.Contains(r.raw, "MATH.G3.NUM.DIV") {
		t.Fatalf("unmapped error %s", r.raw)
	}
	// Invalid rows: removed with a target, a target of another version.
	must(t, c.transition(t, cmNoidung, cur, 2, "revise", ""), http.StatusOK, "revise")
	tag = c.versionETag(t, cmNoidung, cur, 2)
	must(t, c.do(t, http.MethodPut, "/cms/learning/curricula/"+cur+"/versions/2/lo-mappings", cmNoidung, ifm(tag), map[string]any{"items": []map[string]any{
		{"from_lo_id": b, "to_lo_id": a, "relation": "same"}}}), http.StatusUnprocessableEntity, "target of the old version")
	must(t, c.do(t, http.MethodPut, "/cms/learning/curricula/"+cur+"/versions/2/lo-mappings", cmNoidung, ifm(tag), map[string]any{"items": []map[string]any{
		{"from_lo_id": b, "to_lo_id": newID["MATH.G3.NUM.DIV"], "relation": "removed"}}}), http.StatusUnprocessableEntity, "removed with a target")
	full := append(partial, map[string]any{"from_lo_id": b, "to_lo_id": nil, "relation": "removed"})
	must(t, c.do(t, http.MethodPut, "/cms/learning/curricula/"+cur+"/versions/2/lo-mappings", cmNoidung, ifm(tag), map[string]any{"items": full}),
		http.StatusOK, "full mappings")
	before := count(ctx, t, c.platform, `SELECT version FROM curriculum.curriculum_versions WHERE curriculum_version_id = $1`, v1)
	must(t, c.publishFlow(t, cmNoidung, cur, 2), http.StatusOK, "publish v2")

	// AC-27: the payload carries the mappings; version 1 and its LOs are untouched (still published, same row version).
	var payload string
	if err := c.relay.QueryRowContext(ctx, `SELECT envelope->>'payload' FROM infra.outbox_events WHERE event_type = 'curriculum.version_published'
		AND envelope->'payload'->>'curriculum_version_id' = $1`, v2).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var p struct {
		Supersedes string `json:"supersedes_version_id"`
		Mappings   []struct {
			From     string  `json:"from_lo_id"`
			To       *string `json:"to_lo_id"`
			Relation string  `json:"relation"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatal(err)
	}
	if p.Supersedes != v1 || len(p.Mappings) != 3 {
		t.Fatalf("payload %s", payload)
	}
	removed := 0
	for _, m := range p.Mappings {
		if m.Relation == "removed" && m.To == nil && m.From == b {
			removed++
		}
	}
	if removed != 1 {
		t.Fatalf("removed mapping missing: %s", payload)
	}
	if after := count(ctx, t, c.platform, `SELECT version FROM curriculum.curriculum_versions WHERE curriculum_version_id = $1 AND status = 'published'`, v1); after != before {
		t.Fatalf("version 1 changed (%d → %d)", before, after)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.learning_objectives WHERE curriculum_version_id = $1 AND version = 1
		AND deleted_at IS NULL`, v1); n != 2 {
		t.Fatalf("LOs of version 1 changed (%d untouched)", n)
	}
}

// F021-AC-28, AC-29 (US7): textbook series/volumes/sections are metadata only (platform staff), a section maps to LOs,
// and listing LOs by section is limited to what the tenant may see.
func TestCurriculumTextbooks(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	math := c.subject(t, "math")
	cur, ver := c.curriculum(t, cmNoidung, "TB_MATH")
	topic := c.topic(t, cmNoidung, ver, math, "MATH.G3.T01", 3, "")
	lo1 := must(t, c.objective(t, cmNoidung, topic, "MATH.G3.NUM.ONE", nil), http.StatusCreated, "LO1").str("objective_id")
	lo2 := must(t, c.objective(t, cmNoidung, topic, "MATH.G3.NUM.TWO", nil), http.StatusCreated, "LO2").str("objective_id")
	must(t, c.objective(t, cmNoidung, topic, "MATH.G3.NUM.THREE", nil), http.StatusCreated, "LO3")

	series := must(t, c.do(t, http.MethodPost, "/cms/learning/textbook-series", cmNoidung, idem(), map[string]any{"code": "CANH_DIEU",
		"name": "Cánh Diều", "publisher": "NXB Đại học Sư phạm"}), http.StatusCreated, "series").str("series_id")
	book := must(t, c.do(t, http.MethodPost, "/cms/learning/textbooks", cmNoidung, idem(), map[string]any{"series_id": series, "subject_id": math,
		"grade_level": 3, "volume": 1}), http.StatusCreated, "textbook").str("textbook_id")
	// AC-28: metadata only — a field holding the text of the book is refused by the contract.
	must(t, c.do(t, http.MethodPost, "/cms/learning/textbook-sections", cmNoidung, idem(), map[string]any{"textbook_id": book, "lesson_no": 12,
		"title": "Bảng nhân 6", "content": "nguyên văn"}), http.StatusUnprocessableEntity, "section with the text of the book")
	sec := must(t, c.do(t, http.MethodPost, "/cms/learning/textbook-sections", cmNoidung, idem(), map[string]any{"textbook_id": book, "chapter": "Chương 2",
		"lesson_no": 12, "title": "Bảng nhân 6", "page_range": "34-35"}), http.StatusCreated, "section")
	secID := sec.str("section_id")
	r := must(t, c.do(t, http.MethodPut, "/cms/learning/textbook-sections/"+secID+"/objectives", cmNoidung, idem(),
		map[string]any{"objective_ids": []string{lo1, lo2}}), http.StatusOK, "map section")
	if len(r.body["objective_ids"].([]any)) != 2 {
		t.Fatalf("section objectives %s", r.raw)
	}
	must(t, c.do(t, http.MethodPatch, "/cms/learning/textbook-sections/"+secID, cmNoidung, ifm(sec.header.Get("ETag")),
		map[string]any{"page_range": "34-36"}), http.StatusOK, "update section")
	// Tenant users cannot write the platform catalog (AC-06).
	must(t, c.do(t, http.MethodPost, "/cms/learning/textbook-series", qlcmA, idem(), map[string]any{"code": "KNTT", "name": "Kết nối"}),
		http.StatusForbidden, "tenant writes a series")

	// AC-29: filter LOs by section — platform staff, tenant A (assigned) and tenant B (not assigned).
	l := must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?textbook_section_id="+secID, cmNoidung, nil, nil), http.StatusOK, "LOs by section")
	if len(l.items("data")) != 2 {
		t.Fatalf("platform LOs of the section %s", l.raw)
	}
	c.assign(ctx, t, cur, c.tenantA)
	if l = must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?textbook_section_id="+secID, qlcmA, nil, nil), http.StatusOK, "A"); len(l.items("data")) != 2 {
		t.Fatalf("tenant A LOs of the section %s", l.raw)
	}
	if l = must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?textbook_section_id="+secID, qlcmB, nil, nil), http.StatusOK, "B"); len(l.items("data")) != 0 {
		t.Fatalf("tenant B sees LOs of the section %s", l.raw)
	}
	secs := must(t, c.do(t, http.MethodGet, "/cms/learning/textbook-sections?textbook_id="+book, qlcmB, nil, nil), http.StatusOK, "B sections")
	if items := secs.items("data"); len(items) != 1 || len(items[0]["objective_ids"].([]any)) != 0 {
		t.Fatalf("tenant B section LOs %s", secs.raw)
	}
	books := must(t, c.do(t, http.MethodGet, "/cms/learning/textbooks?series_id="+series, qlcmA, nil, nil), http.StatusOK, "textbooks")
	ss := must(t, c.do(t, http.MethodGet, "/cms/learning/textbook-series", qlcmA, nil, nil), http.StatusOK, "series list")
	if len(books.items("data")) != 1 || len(ss.items("data")) != 1 {
		t.Fatalf("catalog lists %s %s", books.raw, ss.raw)
	}
}
