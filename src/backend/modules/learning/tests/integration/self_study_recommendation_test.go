//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// recommendationPayloadValid checks the latest learning.recommendation_generated of a student against its payload schema.
func (c *curEnv) recommendationPayloadValid(ctx context.Context, t *testing.T, student string) {
	t.Helper()
	var raw []byte
	if err := c.relay.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'learning.recommendation_generated'
		AND envelope->'payload'->>'student_id' = $1 ORDER BY created_at DESC LIMIT 1`, student).Scan(&raw); err != nil {
		t.Fatalf("learning.recommendation_generated: %v", err)
	}
	path := filepath.Join(testkit.RepoRoot(t), "specs", "events", "payloads", "learning", "learning.recommendation_generated.v1.schema.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	comp := jsonschema.NewCompiler()
	if err := comp.AddResource(path, doc); err != nil {
		t.Fatal(err)
	}
	sch, err := comp.Compile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	_ = json.Unmarshal(raw, &v)
	if err := sch.Validate(v); err != nil {
		t.Fatalf("payload: %v (%s)", err, raw)
	}
	if strings.Contains(string(raw), "Mình") || strings.Contains(string(raw), "reason_text") {
		t.Fatalf("reason text in the event: %s", raw)
	}
}

func recs(r resp) []map[string]any { return r.items("items") }

// F032-AC-29…AC-33 (US6; quickstart 11, 12): a weak objective gets R2 with its reason in vi/en, due reviews add R9 the
// next day (job recommendation-nightly), at most three shown; a start from a recommendation marks it started then
// completed; an unknown id is ignored; a target skipped three times is not recommended again; the list changes publish
// learning.recommendation_generated.
func TestSelfStudyRecommendations(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s, qc2 := c.selfStudySetup(ctx, t)
	k := c.selfKit(ctx, t)
	right, wrong := correctOption()
	an := c.users[hsAn].UserID.String()
	start := func(body map[string]any, answers ...string) string {
		r := must(t, c.adhoc(t, hsAn, nil, body), http.StatusCreated, "start")
		id := r.str("attempt_id")
		for i, a := range answers {
			must(t, c.answerAfter(t, 10*time.Second, hsAn, id, i+1, a), http.StatusOK, "answer")
		}
		must(t, c.submit(t, hsAn, id), http.StatusOK, "submit")
		k.gradedOnly(ctx, t, id)
		return id
	}
	practice := func(src map[string]any, rec string, answers ...string) string {
		body := map[string]any{"mode": "practice", "source": src}
		if rec != "" {
			body["recommendation_id"] = rec
		}
		return start(body, answers...)
	}
	status := func(id string) (st string, skips int) {
		if err := c.platform.QueryRowContext(ctx, `SELECT status, skip_count FROM learning.rm_student_recommendations WHERE recommendation_id = $1`,
			id).Scan(&st, &skips); err != nil {
			t.Fatal(err)
		}
		return st, skips
	}
	// Three answers on MATH.G3.NUM.MUL: wrong, wrong, right → beginning with 3 evidence and no prerequisite → R2; two
	// questions enter the review list (due the next day).
	practice(map[string]any{"kind": "selection", "topic_id": s.qc.topic, "question_count": 3}, "", wrong, wrong, right)
	r := must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, nil, nil), http.StatusOK, "recommendations")
	items := recs(r)
	if len(items) != 1 || items[0]["rule_code"] != "R2" || items[0]["kind"] != "objective_review" || items[0]["target_id"] != s.qc.lo ||
		items[0]["target_type"] != "selection" || items[0]["priority"] != float64(2) || r.header.Get("Content-Language") != "vi" ||
		!strings.Contains(items[0]["reason_text"].(string), "Học sinh có thể MATH.G3.NUM.MUL") {
		t.Fatalf("R2: %s", r.raw)
	}
	r2 := items[0]["recommendation_id"].(string)
	en := must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, hdr{"Accept-Language": "en"}, nil), http.StatusOK, "en")
	if !strings.HasPrefix(recs(en)[0]["reason_text"].(string), "Let's practise") || en.header.Get("Content-Language") != "en" {
		t.Fatalf("en: %s", en.raw)
	}
	c.recommendationPayloadValid(ctx, t, an)
	// Another student sees nothing of it.
	if b := must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsBinh, nil, nil), http.StatusOK, "binh"); len(recs(b)) != 0 {
		t.Fatalf("binh: %s", b.raw)
	}
	// The next day the wrong questions are due: the nightly job adds R9 after R2.
	c.clk.Advance(25 * time.Hour)
	rep, err := c.recs.RunNightly(ctx, 100)
	if err != nil || rep.Students < 1 {
		t.Fatalf("nightly: %+v %v", rep, err)
	}
	r = must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, nil, nil), http.StatusOK, "after nightly")
	if items = recs(r); len(items) != 2 || items[0]["rule_code"] != "R2" || items[1]["rule_code"] != "R9" || items[1]["target_id"] != s.qc.subject ||
		items[1]["kind"] != "retry_wrong" || items[0]["recommendation_id"] != r2 || !strings.HasPrefix(items[1]["reason_text"].(string), "Có 2 câu") {
		t.Fatalf("R2 then R9: %s", r.raw)
	}
	r9 := items[1]["recommendation_id"].(string)
	// AC-32: a review following R9 marks it started (attempt and activity.started carry the id) then completed; it is a
	// first skip of R2.
	followed := start(map[string]any{"mode": "retry_wrong", "recommendation_id": r9, "source": map[string]any{"kind": "retry_wrong"}}, right, right)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND recommendation_id = $2`, followed, r9); n != 1 {
		t.Fatal("attempts.recommendation_id")
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'activity.started'
		AND envelope->'payload'->>'attempt_id' = $1 AND envelope->'payload'->>'recommendation_id' = $2`, followed, r9); n != 1 {
		t.Fatal("activity.started recommendation_id")
	}
	if st, _ := status(r9); st != "completed" {
		t.Fatalf("R9 %s", st)
	}
	if st, skips := status(r2); st != "active" || skips != 1 {
		t.Fatalf("R2 after one skip: %s %d", st, skips)
	}
	// An id of nobody is ignored: the attempt starts without it (and skips R2 again).
	unknown := practice(map[string]any{"kind": "selection", "objective_id": qc2.lo, "question_count": 1}, ids.New().String(), right)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND recommendation_id IS NULL`, unknown); n != 1 {
		t.Fatal("unknown recommendation kept")
	}
	// A third attempt not following R2: skipped three times → dismissed, never recommended again although still weak.
	practice(map[string]any{"kind": "selection", "objective_id": qc2.lo, "question_count": 1}, "", right)
	if st, skips := status(r2); st != "dismissed" || skips != 3 {
		t.Fatalf("R2 after three skips: %s %d", st, skips)
	}
	if _, err := c.recs.RunNightly(ctx, 100); err != nil {
		t.Fatal(err)
	}
	r = must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, nil, nil), http.StatusOK, "after skips")
	for _, it := range recs(r) {
		if it["rule_code"] == "R2" && it["target_id"] == s.qc.lo {
			t.Fatalf("R2 recommended again: %s", r.raw)
		}
	}
	if len(recs(r)) > 3 {
		t.Fatalf("more than three: %s", r.raw)
	}
	if m := c.masteryOf(ctx, t, c.users[hsAn].UserID, s.qc.lo); m.level != "beginning" && m.level != "developing" {
		t.Fatalf("MATH.G3.NUM.MUL is no longer weak (%s): the blocked R2 proves nothing", m.level)
	}
}
