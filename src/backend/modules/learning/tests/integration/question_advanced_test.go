//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

func advancedBodies(img string) map[string]map[string]any {
	stem := map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": "Câu hỏi"}}}
	return map[string]map[string]any{
		"short_answer": {"type": "short_answer", "stem": stem, "render_payload": map[string]any{"max_length": 30},
			"answer_key": map[string]any{"accepted": []string{"Hà Nội"}}, "scoring_rule": map[string]any{"mode": "all_or_nothing"}},
		"image_based": {"type": "image_based", "stem": stem, "media_refs": []string{img},
			"render_payload": map[string]any{"interaction": "hotspot", "image_media_ref": img, "image_size": map[string]any{"w": 800, "h": 600}},
			"answer_key": map[string]any{"regions": []any{map[string]any{"id": "r_tim", "shape": "circle", "cx": 100, "cy": 100, "r": 40}},
				"correct_region_ids": []string{"r_tim"}}, "scoring_rule": map[string]any{"mode": "all_or_nothing"}},
		"interactive": {"type": "interactive", "stem": stem, "render_payload": map[string]any{"widget": "balance_scale", "widget_version": "1.0",
			"config": map[string]any{"left_weights": []int{5, 3}, "available_weights": []int{1, 2, 4, 8}, "max_moves": 6}},
			"answer_key":   map[string]any{"validator": "balance_scale.equal", "params": map[string]any{"target": 8}},
			"scoring_rule": map[string]any{"mode": "all_or_nothing", "require_action_log": true}},
		"parameterized": {"type": "numeric", "stem": map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": "{{a}} × {{b}} = ?"}}},
			"render_payload": map[string]any{"keyboard": "numeric", "params": map[string]any{"a": map[string]any{"type": "int", "min": 6, "max": 9},
				"b": map[string]any{"type": "int", "min": 2, "max": 9}, "constraints": []string{"a != b"}}},
			"answer_key": map[string]any{"value": "{{a}} * {{b}}"}, "scoring_rule": map[string]any{"mode": "all_or_nothing"}},
	}
}

// F023-AC-03, AC-13, AC-19, AC-20 (TC-TEN-001, TC-TEN-024): the new types go through the CMS API of F022 with its
// authorization and tenant isolation; fuzzy is refused for spelling subjects; limits answer 422 with the field; the
// widget list needs learning_content.read.
func TestQuestionAdvancedTypes(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	authztest.Cover(t, "listQuestionWidgets", authztest.CrossTenant, authztest.RoleDenied)
	img := uuid.NewString()
	ids := map[string]string{}
	for name, body := range advancedBodies(img) {
		r := must(t, c.createQ(t, qlcmA, qc.body(body, nil)), http.StatusCreated, name)
		ids[name] = r.str("content_id")
		if name == "image_based" && strings.Contains(r.raw, `"regions"`) == false {
			t.Fatalf("the CMS view lost the answer key: %s", r.raw)
		}
	}
	for name, id := range ids {
		must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+id, qlcmB, nil, nil), http.StatusNotFound, "tenant B "+name)
		must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+id, noRoleA, nil, nil), http.StatusForbidden, "no role "+name)
	}
	must(t, c.createQ(t, noRoleA, qc.body(advancedBodies(img)["short_answer"], nil)), http.StatusForbidden, "no write")
	// AC-03: Vietnamese/English subjects refuse fuzzy.
	vn := c.subject(t, "vietnamese")
	fuzzy := advancedBodies(img)["short_answer"]
	fuzzy["answer_key"] = map[string]any{"accepted": []string{"con mèo"}, "fuzzy": map[string]any{"enabled": true, "max_edit_distance": 1, "min_length": 3}}
	must(t, c.createQ(t, qlcmA, qc.body(fuzzy, nil)), http.StatusCreated, "fuzzy on math")
	vq := c.topic(t, cmNoidung, qc.version, vn, "VIETNAMESE.G3.T01", 3, "")
	lo := must(t, c.objective(t, cmNoidung, vq, "VIET.G3.SPELL.WORD", nil), http.StatusCreated, "LO").str("objective_id")
	r := c.createQ(t, qlcmA, qc.body(fuzzy, map[string]any{"subject_id": vn, "topic_id": vq, "objective_ids": []string{lo}}))
	if r.status != http.StatusUnprocessableEntity || !slices.Contains(r.errorCodes(), "/answer_key:FUZZY_NOT_ALLOWED_FOR_SUBJECT") {
		t.Fatalf("fuzzy on vietnamese: %d %v", r.status, r.errorCodes())
	}
	// AC-20: limits.
	long := advancedBodies(img)["short_answer"]
	long["answer_key"] = map[string]any{"accepted": []string{strings.Repeat("a", 256)}}
	must(t, c.createQ(t, qlcmA, qc.body(long, nil)), http.StatusUnprocessableEntity, "256 characters")
	poly := advancedBodies(img)["image_based"]
	pts := make([]any, 65)
	for i := range pts {
		pts[i] = []int{i, i % 7}
	}
	poly["answer_key"] = map[string]any{"regions": []any{map[string]any{"id": "r_p", "shape": "polygon", "points": pts}}, "correct_region_ids": []string{"r_p"}}
	must(t, c.createQ(t, qlcmA, qc.body(poly, nil)), http.StatusUnprocessableEntity, "65 points")
	vars := map[string]any{}
	for i := range 11 {
		vars["v"+string(rune('a'+i))] = map[string]any{"type": "int", "min": 1, "max": 2}
	}
	p11 := advancedBodies(img)["parameterized"]
	p11["render_payload"] = map[string]any{"params": vars}
	must(t, c.createQ(t, qlcmA, qc.body(p11, nil)), http.StatusUnprocessableEntity, "11 variables")
	pc := advancedBodies(img)["parameterized"]
	pc["render_payload"].(map[string]any)["params"].(map[string]any)["constraints"] = []string{strings.Repeat("a", 256)}
	must(t, c.createQ(t, qlcmA, qc.body(pc, nil)), http.StatusUnprocessableEntity, "256-character constraint")
	// AC-13: widgets.
	w := must(t, c.do(t, http.MethodGet, "/cms/learning/question-widgets", qlcmA, nil, nil), http.StatusOK, "widgets")
	if items := w.items("data"); len(items) != 1 || items[0]["widget"] != "balance_scale" || items[0]["widget_version"] != "1.0" {
		t.Fatalf("widgets %s", w.raw)
	}
	must(t, c.do(t, http.MethodGet, "/cms/learning/question-widgets", noRoleA, nil, nil), http.StatusForbidden, "widgets no role")
	must(t, c.do(t, http.MethodGet, "/cms/learning/question-widgets", hsAn, nil, nil), http.StatusForbidden, "widgets game token")
}

// F023-AC-16, AC-17, AC-18: a published parameterized question is rendered with its values by Delivery and graded on
// the values of the presentation.
func TestParameterizedDelivery(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	id := must(t, c.createQ(t, qlcmA, qc.body(advancedBodies(uuid.NewString())["parameterized"], nil)), http.StatusCreated, "create").str("content_id")
	vid := c.publishQuestion(ctx, t, id, 1)
	d := c.delivery(ctx, t)
	sctx := c.studentCtx(ctx)
	out, pres, err := d.Render(sctx, []uuid.UUID{uuid.MustParse(vid)}, 42)
	if err != nil || len(out) != 1 {
		t.Fatalf("render %v", err)
	}
	a, b := pres[0].Params["a"], pres[0].Params["b"]
	stem := mustJSON(t, out[0].Stem)
	if a == "" || a == b || !strings.Contains(string(stem), a+" × "+b) || strings.Contains(string(mustJSON(t, out[0])), "params") {
		t.Fatalf("render %s %v", mustJSON(t, out[0]), pres[0].Params)
	}
	want := atoiTest(a) * atoiTest(b)
	g, err := d.Grade(sctx, uuid.MustParse(vid), pres[0], []byte(`{"value":"`+itoaTest(want)+`"}`))
	if err != nil || !g.IsCorrect {
		t.Fatalf("grade %+v %v", g, err)
	}
	g, _ = d.Grade(sctx, uuid.MustParse(vid), pres[0], []byte(`{"value":"`+itoaTest(want+1)+`"}`))
	if g.IsCorrect {
		t.Fatal("wrong value graded correct")
	}
}

func (c *curEnv) delivery(_ context.Context, _ *testing.T) query.Delivery {
	return query.Delivery{Session: c.read, Reader: read.Delivery{}, Registry: question.Full(), Schema: c.schema}
}

func (c *curEnv) studentCtx(ctx context.Context) context.Context {
	return authztest.Ctx(ctx, authztest.Student(c.tenantA))
}

func atoiTest(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func itoaTest(n int) string { return strconv.Itoa(n) }
