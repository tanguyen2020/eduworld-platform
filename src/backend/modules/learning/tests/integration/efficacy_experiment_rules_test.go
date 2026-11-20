//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/experiment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F035-AC-24 (T051): a live recommendation_rule arm applies its rule_params to the rule engine of F032 for its students
// (getMyRecommendations bounded by max_recommendations, practice difficulty shifted by practice_difficulty); stopping the
// experiment brings the default engine back from the next computation. School views never read the arm.
func TestEfficacyExperimentRuleParams(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s, _ := c.selfStudySetup(ctx, t)
	k := c.selfKit(ctx, t)
	right, wrong := correctOption()
	selection := map[string]any{"mode": "practice", "source": map[string]any{"kind": "selection", "topic_id": s.qc.topic, "question_count": 3}}
	r := must(t, c.adhoc(t, hsAn, nil, selection), http.StatusCreated, "practice")
	id := r.str("attempt_id")
	for i, a := range []string{wrong, wrong, right} {
		must(t, c.answerAfter(t, 10*time.Second, hsAn, id, i+1, a), http.StatusOK, "answer")
	}
	must(t, c.submit(t, hsAn, id), http.StatusOK, "submit")
	k.gradedOnly(ctx, t, id)
	// The next day: R2 (weak objective) and R9 (due reviews) — two recommendations with the default engine.
	c.clk.Advance(25 * time.Hour)
	if _, err := c.recs.RunNightly(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if items := recs(must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, nil, nil), http.StatusOK, "default")); len(items) != 2 {
		t.Fatalf("default engine: %v", items)
	}
	// A live recommendation_rule experiment, unit student, hs.an in the arm "fewer" (deterministic order of the arms).
	c.agreement(t, c.tenantA, "HD-REC", "experiment")
	now := c.clk.Now()
	control := map[string]any{"code": "control", "allocation_pct": 50, "is_control": true}
	fewer := map[string]any{"code": "fewer", "allocation_pct": 50, "is_control": false,
		"rule_params": map[string]any{"max_recommendations": 1, "practice_difficulty": -1}}
	body := map[string]any{"code": "rec-fewer", "kind": "recommendation_rule", "unit": "student", "arms": []any{control, fewer},
		"tenant_ids": []uuid.UUID{c.tenantA}, "start_at": now.Format(time.RFC3339), "end_at": now.Add(30 * 24 * time.Hour).Format(time.RFC3339)}
	// AC-21: values outside the bounds of the engine and parameters on the control are refused.
	for name, arm := range map[string]map[string]any{
		"max_recommendations 5": {"code": "fewer", "allocation_pct": 50, "is_control": false, "rule_params": map[string]any{"max_recommendations": 5}},
		"unknown parameter":     {"code": "fewer", "allocation_pct": 50, "is_control": false, "rule_params": map[string]any{"xp_bonus": 2}},
	} {
		b := map[string]any{}
		for key, v := range body {
			b[key] = v
		}
		b["arms"] = []any{control, arm}
		if x := c.do(t, http.MethodPost, experimentsAPI, saAdmin, idem(), b); x.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", name, x.status, x.raw)
		}
	}
	e := must(t, c.do(t, http.MethodPost, experimentsAPI, saAdmin, idem(), body), http.StatusCreated, "experiment")
	eid := uuid.MustParse(e.str("experiment_id"))
	if experiment.Bucket(eid, c.users[hsAn].UserID) < 50 {
		must(t, c.do(t, http.MethodPatch, experimentsAPI+"/"+eid.String(), saAdmin, ifm(e.header.Get("ETag")),
			map[string]any{"arms": []any{fewer, control}}), http.StatusOK, "order the arms")
	}
	must(t, c.do(t, http.MethodPost, experimentsAPI+"/"+eid.String()+":start", saAdmin, idem(), map[string]any{"exposure": "live"}),
		http.StatusOK, "live")
	recompute := func() {
		t.Helper()
		if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, c.tenantA)), func(ctx context.Context) error {
			return c.recs.Recompute(ctx, c.users[hsAn].UserID, nil)
		}); err != nil {
			t.Fatal(err)
		}
	}
	recompute()
	got := must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, nil, nil), http.StatusOK, "arm")
	if items := recs(got); len(items) != 1 || items[0]["rule_code"] != "R2" {
		t.Fatalf("max_recommendations 1: %s", got.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.experiment_assignments WHERE experiment_id = $1 AND student_id = $2
		AND arm_code = 'fewer' AND status = 'active'`, eid, c.users[hsAn].UserID); n != 1 {
		t.Fatalf("assignment of hs.an: %d", n)
	}
	// practice_difficulty shifts the target range (unit test TestPracticeDifficultyByArm); the selection still widens it
	// by one step when the pool is short (F032), so the practice starts.
	must(t, c.adhoc(t, hsAn, nil, selection), http.StatusCreated, "practice with the arm")
	// Stop: the students are back on the default engine from the next computation.
	must(t, c.do(t, http.MethodPost, experimentsAPI+"/"+eid.String()+":stop", saAdmin, idem(), map[string]any{"reason": "Kết thúc thử nghiệm luật gợi ý"}),
		http.StatusOK, "stop")
	recompute()
	if items := recs(must(t, c.do(t, http.MethodGet, "/learning/recommendations/me", hsAn, nil, nil), http.StatusOK, "after stop")); len(items) != 2 {
		t.Fatalf("default engine after stop: %v", items)
	}
}
