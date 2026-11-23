//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Owner rule 2026-11-23 (API_DESIGN §5): every list of a grid/table reads one keyset page in SQL — page 2 continues page
// 1 with no duplicate and no gap, the documented order holds, the tenant scope is kept, a tampered cursor is 400.

// walk follows page.next_cursor with limit n until has_more = false; it fails on a duplicate id, a page over n rows or
// a missing cursor, and returns the ids of the rows in the order read and the rows themselves.
func (c *curEnv) walk(t *testing.T, sub, path string, h hdr, key, idKey string, n int) ([]string, []map[string]any) {
	t.Helper()
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	var idList []string
	var rows []map[string]any
	seen := map[string]bool{}
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 2000 {
			t.Fatalf("%s: endless paging", path)
		}
		q := path + sep + "limit=" + strconv.Itoa(n)
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		r := must(t, c.do(t, http.MethodGet, q, sub, h, nil), http.StatusOK, "page of "+path)
		page := r.obj("page")
		data := r.items(key)
		if len(data) > n {
			t.Fatalf("%s: %d rows for limit %d", path, len(data), n)
		}
		for _, row := range data {
			id := fmt.Sprint(row[idKey])
			if seen[id] {
				t.Fatalf("%s: duplicate %s across pages", path, id)
			}
			seen[id] = true
			idList = append(idList, id)
			rows = append(rows, row)
		}
		if page["has_more"] != true {
			if page["next_cursor"] != nil {
				t.Fatalf("%s: next_cursor on the last page: %s", path, r.raw)
			}
			return idList, rows
		}
		if len(data) != n {
			t.Fatalf("%s: short page %d with has_more: %s", path, len(data), r.raw)
		}
		cursor, _ = page["next_cursor"].(string)
		if cursor == "" {
			t.Fatalf("%s: has_more without next_cursor: %s", path, r.raw)
		}
	}
}

// samePages checks that the pages walked with a small limit hold exactly the rows of one big page, in the same order.
func (c *curEnv) samePages(t *testing.T, sub, path string, h hdr, key, idKey string, n, want int) ([]string, []map[string]any) {
	t.Helper()
	paged, rows := c.walk(t, sub, path, h, key, idKey, n)
	full, _ := c.walk(t, sub, path, h, key, idKey, 100)
	if !slices.Equal(paged, full) {
		t.Fatalf("%s: pages of %d %v differ from one page %v (gap, duplicate or order)", path, n, paged, full)
	}
	if want >= 0 && len(paged) != want {
		t.Fatalf("%s: %d rows, want %d: %v", path, len(paged), want, paged)
	}
	return paged, rows
}

// firstCursor is the next_cursor of the first page of path with limit 1.
func (c *curEnv) firstCursor(t *testing.T, sub, path string, h hdr) string {
	t.Helper()
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	r := must(t, c.do(t, http.MethodGet, path+sep+"limit=1", sub, h, nil), http.StatusOK, "first page "+path)
	s, _ := r.obj("page")["next_cursor"].(string)
	if s == "" {
		t.Fatalf("%s: no next_cursor on page 1: %s", path, r.raw)
	}
	return s
}

// tamper changes one base64url character of a cursor (the HMAC no longer matches).
func tamper(s string) string {
	b := []byte(s)
	i := len(b) / 2
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}

// badCursor checks that path answers 400 to cursor.
func (c *curEnv) badCursor(t *testing.T, sub, path string, h hdr, cursor string) {
	t.Helper()
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	if r := c.do(t, http.MethodGet, path+sep+"limit=1&cursor="+url.QueryEscape(cursor), sub, h, nil); r.status != http.StatusBadRequest {
		t.Fatalf("%s with a bad cursor: %d %s", path, r.status, r.raw)
	}
}

func fieldsOf(rows []map[string]any, key string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = fmt.Sprint(r[key])
	}
	return out
}

func numsOf(rows []map[string]any, key string) []float64 {
	out := make([]float64, len(rows))
	for i, r := range rows {
		out[i], _ = r[key].(float64)
	}
	return out
}

// The curriculum catalog of the CMS (listCms*, textbooks) and the public listLearningObjectives.
func TestKeysetPagesCurriculum(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	// listCmsSubjects: (code, id); the grade filter is in SQL (jsonb containment).
	var subjects []string
	for _, code := range []string{"kp_c", "kp_a", "kp_b"} {
		subjects = append(subjects, c.subject(t, code))
	}
	g1 := must(t, c.do(t, http.MethodPost, "/cms/learning/subjects", cmNoidung, idem(), map[string]any{"code": "kp_g1",
		"name": map[string]string{"vi": "Môn lớp 1"}, "grade_levels": []int{1}}), http.StatusCreated, "subject grade 1").str("subject_id")
	_, rows := c.samePages(t, cmNoidung, "/cms/learning/subjects", nil, "data", "subject_id", 1, -1)
	if codes := fieldsOf(rows, "code"); !slices.IsSorted(codes) {
		t.Fatalf("subjects not in code order: %v", codes)
	}
	g3, _ := c.samePages(t, cmNoidung, "/cms/learning/subjects?grade_level=3", nil, "data", "subject_id", 1, -1)
	if slices.Contains(g3, g1) {
		t.Fatalf("grade 3 holds a grade-1 subject: %v", g3)
	}
	for _, s := range subjects {
		if !slices.Contains(g3, s) {
			t.Fatalf("grade 3 misses %s: %v", s, g3)
		}
	}
	math := subjects[1]

	// listCmsCurricula: (code, id) in the tenant scope.
	var curA []string
	for _, code := range []string{"KP_A3", "KP_A1", "KP_A2"} {
		id, _ := c.curriculum(t, qlcmA, code)
		curA = append(curA, id)
	}
	curB, _ := c.curriculum(t, qlcmB, "KP_B1")
	gotA, rows := c.samePages(t, qlcmA, "/cms/learning/curricula", nil, "data", "curriculum_id", 1, 3)
	if codes := fieldsOf(rows, "code"); !slices.Equal(codes, []string{"KP_A1", "KP_A2", "KP_A3"}) || slices.Contains(gotA, curB) {
		t.Fatalf("curricula of A: %v %v", codes, gotA)
	}
	for _, id := range curA {
		if !slices.Contains(gotA, id) {
			t.Fatalf("curricula of A miss %s", id)
		}
	}
	if gotB, _ := c.samePages(t, qlcmB, "/cms/learning/curricula", nil, "data", "curriculum_id", 1, 1); gotB[0] != curB {
		t.Fatalf("curricula of B: %v", gotB)
	}
	curCursor := c.firstCursor(t, qlcmA, "/cms/learning/curricula", nil)
	c.badCursor(t, qlcmA, "/cms/learning/curricula", nil, tamper(curCursor))
	c.badCursor(t, qlcmA, "/cms/learning/curricula", nil, "not-a-cursor")

	// A platform curriculum with three versions: topics, objectives, competencies of version 1.
	cur, v1 := c.curriculum(t, cmNoidung, "KP_PUB")
	t1 := c.topic(t, cmNoidung, v1, math, "KP.G3.T2", 3, "")
	t2 := c.topic(t, cmNoidung, v1, math, "KP.G3.T1", 3, "")
	t3 := c.topic(t, cmNoidung, v1, math, "KP.G2.T3", 2, "")
	topics, rows := c.samePages(t, cmNoidung, "/cms/learning/topics?curriculum_version_id="+v1, nil, "data", "topic_id", 1, 3)
	for _, id := range []string{t1, t2, t3} {
		if !slices.Contains(topics, id) {
			t.Fatalf("topics miss %s: %v", id, topics)
		}
	}
	if grades := numsOf(rows, "grade_level"); !slices.IsSorted(grades) {
		t.Fatalf("topics not in grade order: %v", grades)
	}
	topicCursor := c.firstCursor(t, cmNoidung, "/cms/learning/topics?curriculum_version_id="+v1, nil)
	// a cursor of another list (other sort columns) is refused
	c.badCursor(t, cmNoidung, "/cms/learning/curricula", nil, topicCursor)
	c.badCursor(t, cmNoidung, "/cms/learning/topics?curriculum_version_id="+v1, nil, tamper(topicCursor))
	var los []string
	for _, code := range []string{"KP.G3.NUM.C", "KP.G3.NUM.A", "KP.G3.NUM.B"} {
		los = append(los, must(t, c.objective(t, cmNoidung, t2, code, nil), http.StatusCreated, "LO "+code).str("objective_id"))
	}
	must(t, c.objective(t, cmNoidung, t3, "KP.G2.NUM.A", nil), http.StatusCreated, "LO grade 2")
	c.samePages(t, cmNoidung, "/cms/learning/objectives?curriculum_version_id="+v1, nil, "data", "objective_id", 1, 4)
	byTopic, _ := c.samePages(t, cmNoidung, "/cms/learning/objectives?topic_id="+t2, nil, "data", "objective_id", 2, 3)
	for _, id := range los {
		if !slices.Contains(byTopic, id) {
			t.Fatalf("LOs of the topic miss %s", id)
		}
	}
	c.badCursor(t, cmNoidung, "/cms/learning/objectives?topic_id="+t2, nil, tamper(c.firstCursor(t, cmNoidung, "/cms/learning/objectives?topic_id="+t2, nil)))
	// tenant B: the platform version is not assigned → no row
	c.samePages(t, qlcmB, "/cms/learning/objectives?curriculum_version_id="+v1, nil, "data", "objective_id", 1, 0)
	for _, code := range []string{"KP_NL3", "KP_NL1", "KP_NL2"} {
		must(t, c.do(t, http.MethodPost, "/cms/learning/competencies", cmNoidung, idem(), map[string]any{"curriculum_version_id": v1,
			"code": code, "name": map[string]string{"vi": code}, "kind": "quality"}), http.StatusCreated, "competency "+code)
	}
	_, rows = c.samePages(t, cmNoidung, "/cms/learning/competencies?curriculum_version_id="+v1, nil, "data", "competency_id", 1, 3)
	if codes := fieldsOf(rows, "code"); !slices.Equal(codes, []string{"KP_NL1", "KP_NL2", "KP_NL3"}) {
		t.Fatalf("competencies order: %v", codes)
	}
	c.samePages(t, qlcmB, "/cms/learning/competencies?curriculum_version_id="+v1, nil, "data", "competency_id", 1, 0)

	// listCurriculumVersions: newest first (version_no DESC).
	c.publishVersion(t, cur, 1, "2020-01-01", "2020-12-31")
	v2 := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions", cmNoidung, idem(), map[string]any{"from_version": 1,
		"version_label": "2.0"}), http.StatusCreated, "v2").str("curriculum_version_id")
	inV2 := must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?limit=100&curriculum_version_id="+v2, cmNoidung, nil, nil), http.StatusOK, "LOs v2")
	oldByCode := map[string]string{}
	for _, lo := range must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?limit=100&curriculum_version_id="+v1, cmNoidung, nil, nil),
		http.StatusOK, "LOs v1").items("data") {
		oldByCode[lo["code"].(string)] = lo["objective_id"].(string)
	}
	var mappings []map[string]any
	for _, lo := range inV2.items("data") {
		mappings = append(mappings, map[string]any{"from_lo_id": oldByCode[lo["code"].(string)], "to_lo_id": lo["objective_id"], "relation": "same"})
	}
	must(t, c.do(t, http.MethodPut, "/cms/learning/curricula/"+cur+"/versions/2/lo-mappings", cmNoidung, ifm(c.versionETag(t, cmNoidung, cur, 2)),
		map[string]any{"items": mappings}), http.StatusOK, "mappings")
	c.publishVersion(t, cur, 2, "2021-01-01", "")
	must(t, c.do(t, http.MethodPost, "/cms/learning/curricula/"+cur+"/versions", cmNoidung, idem(), map[string]any{"from_version": 2,
		"version_label": "3.0"}), http.StatusCreated, "v3 draft")
	_, rows = c.samePages(t, cmNoidung, "/cms/learning/curricula/"+cur+"/versions", nil, "data", "curriculum_version_id", 1, 3)
	if nos := numsOf(rows, "version"); !slices.Equal(nos, []float64{3, 2, 1}) {
		t.Fatalf("versions order: %v", nos)
	}
	if r := c.do(t, http.MethodGet, "/cms/learning/curricula/"+cur+"/versions", qlcmB, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("versions for tenant B: %d", r.status)
	}

	// Public listLearningObjectives of the applied version (v2, in effect today) for a grade-3 student of tenant A.
	c.assign(ctx, t, cur, c.tenantA)
	school := uuid.New()
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.schools (school_id, tenant_id, status, source_version) VALUES ($1, $2, 'active', 1)`,
		school, c.tenantA); err != nil {
		t.Fatal(err)
	}
	c.placeIn(ctx, t, hsAn, school, 3)
	pub, rows := c.samePages(t, hsAn, "/learning/objectives?grade_level=3", nil, "data", "objective_id", 1, 3)
	for _, row := range rows {
		if row["curriculum_version_id"] != v2 {
			t.Fatalf("public LO of another version: %v (%v)", row, pub)
		}
	}
	c.badCursor(t, hsAn, "/learning/objectives?grade_level=3", nil, tamper(c.firstCursor(t, hsAn, "/learning/objectives?grade_level=3", nil)))
	c.samePages(t, adB, "/learning/objectives?grade_level=3", nil, "data", "objective_id", 1, 0)

	// Textbook series (code, id), textbooks (grade, volume, id), sections (lesson_no, id).
	var series []string
	for _, code := range []string{"KP_S3", "KP_S1", "KP_S2"} {
		series = append(series, must(t, c.do(t, http.MethodPost, "/cms/learning/textbook-series", cmNoidung, idem(), map[string]any{"code": code,
			"name": "Bộ " + code}), http.StatusCreated, "series "+code).str("series_id"))
	}
	_, rows = c.samePages(t, cmNoidung, "/cms/learning/textbook-series", nil, "data", "series_id", 1, 3)
	if codes := fieldsOf(rows, "code"); !slices.Equal(codes, []string{"KP_S1", "KP_S2", "KP_S3"}) {
		t.Fatalf("series order: %v", codes)
	}
	var book string
	for _, b := range []struct{ grade, volume int }{{3, 2}, {2, 1}, {3, 1}} {
		id := must(t, c.do(t, http.MethodPost, "/cms/learning/textbooks", cmNoidung, idem(), map[string]any{"series_id": series[0],
			"subject_id": math, "grade_level": b.grade, "volume": b.volume}), http.StatusCreated, "textbook").str("textbook_id")
		if b.grade == 3 && b.volume == 1 {
			book = id
		}
	}
	_, rows = c.samePages(t, cmNoidung, "/cms/learning/textbooks?series_id="+series[0], nil, "data", "textbook_id", 1, 3)
	if g, v := numsOf(rows, "grade_level"), numsOf(rows, "volume"); !slices.Equal(g, []float64{2, 3, 3}) || !slices.Equal(v, []float64{1, 1, 2}) {
		t.Fatalf("textbooks order: grades %v volumes %v", g, v)
	}
	for _, n := range []int{3, 1, 2} {
		must(t, c.do(t, http.MethodPost, "/cms/learning/textbook-sections", cmNoidung, idem(), map[string]any{"textbook_id": book,
			"lesson_no": n, "title": "Bài " + strconv.Itoa(n)}), http.StatusCreated, "section")
	}
	_, rows = c.samePages(t, cmNoidung, "/cms/learning/textbook-sections?textbook_id="+book, nil, "data", "section_id", 1, 3)
	if nos := numsOf(rows, "lesson_no"); !slices.Equal(nos, []float64{1, 2, 3}) {
		t.Fatalf("sections order: %v", nos)
	}
	c.badCursor(t, cmNoidung, "/cms/learning/textbook-sections?textbook_id="+book, nil,
		tamper(c.firstCursor(t, cmNoidung, "/cms/learning/textbook-sections?textbook_id="+book, nil)))
}

// searchQuestions: the newest version of each question once, filters in SQL, keyset on question_id, tenant scope.
func TestKeysetPagesSearchQuestions(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	c.assign(ctx, t, qc.cur, c.tenantB)
	var drafts []string
	for range 3 {
		id, _ := c.question(t, qlcmA, qc, nil)
		drafts = append(drafts, id)
	}
	// two versions: v1 published, v2 draft → only v2 is listed, and never under status=published
	twoV := c.approved(ctx, t, qlcmA, qc, nil)
	must(t, c.publishV(ctx, t, qlcmA, "question", twoV, 1, nil, nil), http.StatusOK, "publish v1")
	must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+twoV+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}),
		http.StatusCreated, "v2")
	pubOnly := c.approved(ctx, t, qlcmA, qc, nil)
	must(t, c.publishV(ctx, t, qlcmA, "question", pubOnly, 1, nil, nil), http.StatusOK, "publish")
	idB, _ := c.question(t, qlcmB, qc, nil)

	all, rows := c.samePages(t, qlcmA, "/cms/learning/questions", nil, "data", "content_id", 1, 5)
	if !slices.IsSorted(all) || slices.Contains(all, idB) {
		t.Fatalf("questions of A not in question_id order or holding B: %v", all)
	}
	for _, row := range rows {
		if row["content_id"] == twoV && (row["version"] != float64(2) || row["status"] != "draft") {
			t.Fatalf("not the newest version: %v", row)
		}
	}
	draft, _ := c.samePages(t, qlcmA, "/cms/learning/questions?status=draft", nil, "data", "content_id", 1, 4)
	want := append(slices.Clone(drafts), twoV)
	slices.Sort(want)
	if !slices.Equal(draft, want) {
		t.Fatalf("status=draft %v, want %v", draft, want)
	}
	if pub, _ := c.samePages(t, qlcmA, "/cms/learning/questions?status=published", nil, "data", "content_id", 1, 1); pub[0] != pubOnly {
		t.Fatalf("status=published %v, want %s", pub, pubOnly)
	}
	if b, _ := c.samePages(t, qlcmB, "/cms/learning/questions", nil, "data", "content_id", 1, 1); b[0] != idB {
		t.Fatalf("questions of B: %v", b)
	}
	c.badCursor(t, qlcmA, "/cms/learning/questions", nil, tamper(c.firstCursor(t, qlcmA, "/cms/learning/questions", nil)))
}

// listContentVersions (version_no DESC, status in SQL) and listContentReleases (numbers of the page's versions only).
func TestKeysetPagesContentVersions(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id := c.approved(ctx, t, qlcmA, qc, nil)
	must(t, c.publishV(ctx, t, qlcmA, "question", id, 1, nil, nil), http.StatusOK, "v1")
	must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}), http.StatusCreated, "v2")
	must(t, c.do(t, http.MethodPatch, "/cms/learning/questions/"+id+"/versions/2", qlcmA, ifm(c.rowTag(ctx, t, "question", id, 2)),
		map[string]any{"difficulty": 4}), http.StatusOK, "edit v2")
	c.approve(ctx, t, qlcmA, "question", id, 2)
	must(t, c.publishV(ctx, t, qlcmA, "question", id, 2, nil, nil), http.StatusOK, "v2")
	must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 2}), http.StatusCreated, "v3")
	base := "/cms/learning/content/question/" + id
	// every page carries the current release number, read by id (never from the whole history)
	cursor := ""
	var nos []float64
	for {
		q := base + "/versions?limit=1"
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		r := must(t, c.do(t, http.MethodGet, q, qlcmA, nil, nil), http.StatusOK, "history page")
		if r.str("current_version") != "2" {
			t.Fatalf("current_version on a page: %s", r.raw)
		}
		for _, v := range r.items("data") {
			nos = append(nos, v["version"].(float64))
			if (v["version"] == float64(2)) != (v["is_current"] == true) {
				t.Fatalf("is_current: %v", v)
			}
		}
		if r.obj("page")["has_more"] != true {
			break
		}
		cursor = r.str("page", "next_cursor")
	}
	if !slices.Equal(nos, []float64{3, 2, 1}) {
		t.Fatalf("history order %v", nos)
	}
	c.samePages(t, qlcmA, base+"/versions", nil, "data", "content_version_id", 1, 3)
	_, rows := c.samePages(t, qlcmA, base+"/versions?status=published", nil, "data", "content_version_id", 1, 2)
	if got := numsOf(rows, "version"); !slices.Equal(got, []float64{2, 1}) {
		t.Fatalf("published versions %v", got)
	}
	c.badCursor(t, qlcmA, base+"/versions", nil, tamper(c.firstCursor(t, qlcmA, base+"/versions", nil)))
	if r := c.do(t, http.MethodGet, base+"/versions?limit=1", qlcmB, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("history for tenant B: %d", r.status)
	}
	// releases: page 1 names v1 → v2 although v1 is on no page of versions read here
	_, rows = c.samePages(t, qlcmA, base+"/releases", nil, "data", "release_id", 1, 2)
	if rows[0]["to_version"] != float64(2) || rows[0]["from_version"] != float64(1) || rows[1]["to_version"] != float64(1) {
		t.Fatalf("releases %v", rows)
	}
}

// The research reads of System Admin: agreements, studies, cohorts, waves, experiments (rows seeded in SQL so that
// equal timestamps test the id tie breaker).
func TestKeysetPagesResearch(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	at := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	exec := func(q string, args ...any) {
		t.Helper()
		// tenancyguard:ok TEST seed under learning_platform
		if _, err := c.platform.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// agreements: three of tenant A at the same instant, one of tenant B
	for i, tenant := range []uuid.UUID{c.tenantA, c.tenantA, c.tenantA, c.tenantB} {
		// tenancyguard:ok TEST seed under learning_platform (tenant_id given, live rows)
		exec(`INSERT INTO learning.research_agreements (agreement_id, tenant_id, agreement_ref, covers, agreed_on, recorded_by, created_at)
			VALUES ($1, $2, $3, ARRAY['efficacy_study']::varchar(64)[], '2026-09-01', $1, $4)`, uuid.New(), tenant, "KP-"+strconv.Itoa(i), at)
	}
	agA, rows := c.samePages(t, saAdmin, agreements+"?tenant_ids="+c.tenantA.String(), nil, "items", "agreement_id", 1, 3)
	if !slices.IsSorted(agA) {
		t.Fatalf("agreements with equal created_at not in id order: %v", agA)
	}
	for _, r := range rows {
		if r["tenant_id"] != c.tenantA.String() {
			t.Fatalf("agreement of another tenant: %v", r)
		}
	}
	_, rows = c.samePages(t, saAdmin, agreements+"?tenant_ids="+c.tenantA.String()+","+c.tenantB.String(), nil, "items", "agreement_id", 2, 4)
	if tenants := fieldsOf(rows, "tenant_id"); !slices.IsSorted(tenants) {
		t.Fatalf("agreements not in tenant order: %v", tenants)
	}
	agPath := agreements + "?tenant_ids=" + c.tenantA.String()
	c.badCursor(t, saAdmin, agPath, nil, tamper(c.firstCursor(t, saAdmin, agPath, nil)))

	// studies: created_at DESC, study_id DESC (two at the same instant)
	var studyIDs []uuid.UUID
	for i, created := range []time.Time{at, at.Add(time.Hour), at} {
		id := uuid.New()
		studyIDs = append(studyIDs, id)
		exec(`INSERT INTO learning.efficacy_studies (study_id, code, title, subject_id, grade_level, objective_ids, form_a_blueprint_version_id,
			form_b_blueprint_version_id, min_intervention_weeks, created_by, idempotency_key, created_at)
			VALUES ($1, $2, 'KP', $1, 3, ARRAY[$1]::uuid[], $1, $3, 8, $1, $2, $4)`, id, "kp-study-"+strconv.Itoa(i), uuid.New(), created)
	}
	st, _ := c.samePages(t, saAdmin, studies, nil, "items", "study_id", 1, 3)
	tie := []string{studyIDs[0].String(), studyIDs[2].String()}
	slices.Sort(tie)
	if want := []string{studyIDs[1].String(), tie[1], tie[0]}; !slices.Equal(st, want) {
		t.Fatalf("studies %v, want %v (created_at DESC, id DESC)", st, want)
	}
	c.badCursor(t, saAdmin, studies, nil, tamper(c.firstCursor(t, saAdmin, studies, nil)))

	// cohorts of study 0: (COALESCE(pair_no, 32767), school_id) — pair 1 twice, pair 2 of tenant B, a validation cohort last
	study := studyIDs[0]
	schools := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	pairs := []any{1, 1, nil, 2}
	tenants := []uuid.UUID{c.tenantA, c.tenantA, c.tenantA, c.tenantB}
	for i := range schools {
		// tenancyguard:ok TEST seed under learning_platform (tenant_id given, live rows)
		exec(`INSERT INTO learning.efficacy_study_cohorts (study_id, school_id, tenant_id, grade_level, pair_no, added_by)
			VALUES ($1, $2, $3, 3, $4, $1)`, study, schools[i], tenants[i], pairs[i])
	}
	cohortPath := studies + "/" + study.String() + "/cohorts"
	co, _ := c.samePages(t, saAdmin, cohortPath, nil, "items", "school_id", 1, 4)
	pair1 := []string{schools[0].String(), schools[1].String()}
	slices.Sort(pair1)
	if want := []string{pair1[0], pair1[1], schools[3].String(), schools[2].String()}; !slices.Equal(co, want) {
		t.Fatalf("cohorts %v, want %v", co, want)
	}
	coA, _ := c.samePages(t, saAdmin, cohortPath+"?tenant_ids="+c.tenantA.String(), nil, "items", "school_id", 1, 3)
	if slices.Contains(coA, schools[3].String()) {
		t.Fatalf("cohort of tenant B under tenant_ids=A: %v", coA)
	}
	c.badCursor(t, saAdmin, cohortPath, nil, tamper(c.firstCursor(t, saAdmin, cohortPath, nil)))
	if onlyB, _ := c.samePages(t, saAdmin, studies+"?tenant_ids="+c.tenantB.String(), nil, "items", "study_id", 1, 1); onlyB[0] != study.String() {
		t.Fatalf("studies of tenant B: %v", onlyB)
	}

	// waves: (window_start, wave_id), now paged
	var waves []string
	for i, kind := range []string{"post", "validation", "pre"} {
		id := uuid.New()
		waves = append(waves, id.String())
		var form any
		if kind == "validation" {
			form = "a"
		}
		start := at.Add(time.Duration(2-i) * 24 * time.Hour)
		exec(`INSERT INTO learning.efficacy_waves (wave_id, study_id, kind, form, window_start, window_end, duration_sec, created_by, idempotency_key)
			VALUES ($1, $2, $3, $4, $5, $6, 1800, $1, $7)`, id, study, kind, form, start, start.Add(time.Hour), id.String())
	}
	wavePath := studies + "/" + study.String() + "/waves"
	wv, _ := c.samePages(t, saAdmin, wavePath, nil, "items", "wave_id", 1, 3)
	if want := []string{waves[2], waves[1], waves[0]}; !slices.Equal(wv, want) {
		t.Fatalf("waves %v, want %v (window_start order)", wv, want)
	}
	c.badCursor(t, saAdmin, wavePath, nil, tamper(c.firstCursor(t, saAdmin, wavePath, nil)))

	// experiments: created_at DESC, experiment_id DESC
	var exps []uuid.UUID
	for i, created := range []time.Time{at, at, at.Add(time.Minute)} {
		id := uuid.New()
		exps = append(exps, id)
		exec(`INSERT INTO learning.experiments (experiment_id, code, kind, arms, tenant_ids, start_at, end_at, owner_id, idempotency_key, created_at)
			VALUES ($1, $2, 'mastery_model', '[]'::jsonb, ARRAY[$3]::uuid[], $4, $5, $1, $6, $4)`, id, "kp-exp-"+strconv.Itoa(i),
			c.tenantA, created, created.Add(30*24*time.Hour), id.String())
	}
	ex, _ := c.samePages(t, saAdmin, "/admin/learning/experiments", nil, "items", "experiment_id", 1, 3)
	tie = []string{exps[0].String(), exps[1].String()}
	slices.Sort(tie)
	if want := []string{exps[2].String(), tie[1], tie[0]}; !slices.Equal(ex, want) {
		t.Fatalf("experiments %v, want %v", ex, want)
	}
	c.badCursor(t, saAdmin, "/admin/learning/experiments", nil, tamper(c.firstCursor(t, saAdmin, "/admin/learning/experiments", nil)))
}

// listMyWrongQuestions: no cap any more (1,003 rows walked), keyset (next_due_at, entry_id), the student's own rows.
func TestKeysetPagesWrongQuestions(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	student := c.users[hsAn].UserID
	due := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	seed := func(tenant, who uuid.UUID, version uuid.UUID, next time.Time) {
		t.Helper()
		// tenancyguard:ok TEST seed under learning_platform (tenant_id given, live rows)
		if _, err := c.platform.ExecContext(ctx, `INSERT INTO learning.wrong_question_entries (entry_id, tenant_id, school_id, student_id, question_id,
			question_version_id, question_type, difficulty, first_wrong_attempt_id, last_wrong_at, next_due_at)
			VALUES ($1, $2, $1, $3, $1, $4, 'single_choice', 2, $1, $5, $5)`, uuid.New(), tenant, who, version, next); err != nil {
			t.Fatal(err)
		}
	}
	early, tieA, tieB := uuid.New(), uuid.New(), uuid.New()
	seed(c.tenantA, student, tieA, due.Add(time.Hour))
	seed(c.tenantA, student, early, due)
	seed(c.tenantA, student, tieB, due.Add(time.Hour))
	seed(c.tenantA, uuid.New(), uuid.New(), due) // another student of the tenant
	seed(c.tenantB, student, uuid.New(), due)    // the same user id in tenant B
	got, _ := c.samePages(t, hsAn, "/learning/wrong-questions", nil, "data", "question_version_id", 1, 3)
	if got[0] != early.String() || !slices.Contains(got, tieA.String()) || !slices.Contains(got, tieB.String()) {
		t.Fatalf("wrong questions %v (next_due_at order, own rows only)", got)
	}
	c.badCursor(t, hsAn, "/learning/wrong-questions", nil, tamper(c.firstCursor(t, hsAn, "/learning/wrong-questions", nil)))
	// past the former 1000-row cap
	// tenancyguard:ok TEST seed under learning_platform (tenant_id given, live rows)
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO learning.wrong_question_entries (entry_id, tenant_id, school_id, student_id, question_id,
		question_version_id, question_type, difficulty, first_wrong_attempt_id, last_wrong_at, next_due_at)
		SELECT e, $1, e, $2, e, e, 'single_choice', 2, e, $3, $3 FROM (SELECT gen_random_uuid() AS e FROM generate_series(1, 1000)) s`,
		c.tenantA, student, due.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	all, _ := c.walk(t, hsAn, "/learning/wrong-questions", nil, "data", "question_version_id", 100)
	if len(all) != 1003 {
		t.Fatalf("walked %d wrong questions, want 1003 (no cap)", len(all))
	}
}
