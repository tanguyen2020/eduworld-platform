//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// F035-AC-26 (T047, T052): the job experiment-guardrail stops an experiment whose arm is at least
// experiment_guardrail_drop_pp points under the control (stop_reason guardrail_breach, students back on the control,
// audit, results stored with breached = true, no learning_objective.mastered replayed); the job experiment-lifecycle
// completes the experiments past end_at (stop_reason ended).
func TestEfficacyExperimentJobs(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	c.agreement(t, c.tenantA, "HD-JOBS", "experiment")
	now := c.clk.Now()
	create := func(code string, end time.Time) uuid.UUID {
		t.Helper()
		arms := []any{map[string]any{"code": "control", "allocation_pct": 50, "is_control": true},
			map[string]any{"code": "fewer", "allocation_pct": 50, "is_control": false, "rule_params": map[string]any{"max_recommendations": 1}}}
		e := must(t, c.do(t, http.MethodPost, experimentsAPI, saAdmin, idem(), map[string]any{"code": code, "kind": "recommendation_rule",
			"unit": "student", "arms": arms, "tenant_ids": []uuid.UUID{c.tenantA}, "start_at": now.Format(time.RFC3339),
			"end_at": end.Format(time.RFC3339)}), http.StatusCreated, code)
		id := uuid.MustParse(e.str("experiment_id"))
		must(t, c.do(t, http.MethodPost, experimentsAPI+"/"+id.String()+":start", saAdmin, idem(), map[string]any{"exposure": "live"}),
			http.StatusOK, "start "+code)
		return id
	}
	guarded := create("rec-guarded", now.Add(30*24*time.Hour))
	// TEST data: 5 students per arm assigned 8 days ago (past experiment_eval_delay_days) answering two new questions
	// each; the control answers 10/10, the arm 5/10 (50 points under the control ≥ 5).
	school := uuid.New()
	for arm, correct := range map[string]int{"control": 2, "fewer": 1} {
		for i := range 5 {
			student := uuid.New()
			c.exec(ctx, t, `INSERT INTO learning.experiment_assignments (experiment_id, student_id, tenant_id, unit_id, bucket, arm_code, assigned_at)
				VALUES ($1, $2, $3, $2, 0, $4, $5)`, guarded, student, c.tenantA, arm, now.Add(-8*24*time.Hour))
			for q := range 2 {
				c.exec(ctx, t, `INSERT INTO learning.learning_events (event_id, occurred_at, event_type, student_id, tenant_id, school_id, context,
					payload, idempotency_key, experiment_arms) VALUES ($1, $2, 'question.answered', $3, $4, $5, 'practice', $6, $7, $8)`,
					uuid.New(), now.Add(-time.Hour), student, c.tenantA, school, fmt.Sprintf(`{"is_correct": %t, "attempt_no": 1}`, q < correct),
					fmt.Sprintf("test-%s-%d-%d", arm, i, q), []string{"rec-guarded:" + arm})
			}
		}
	}
	mastered := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'learning_objective.mastered'`)
	if n, err := c.exps.RunGuardrail(ctx); err != nil || n != 1 {
		t.Fatalf("guardrail: %d %v", n, err)
	}
	g := must(t, c.do(t, http.MethodGet, experimentsAPI+"/"+guarded.String(), saAdmin, nil, nil), http.StatusOK, "guarded")
	if g.str("status") != "stopped" || g.str("stop_reason") != "guardrail_breach" {
		t.Fatalf("not stopped by the guardrail: %s", g.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.experiment_assignments WHERE experiment_id = $1 AND status = 'active'`,
		guarded); n != 0 {
		t.Fatalf("students still on the arm: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.experiment_results WHERE experiment_id = $1 AND guardrail_breached`,
		guarded); n != 1 {
		t.Fatalf("breached results: %d", n)
	}
	res := must(t, c.do(t, http.MethodGet, experimentsAPI+"/"+guarded.String()+"/results", saAdmin, nil, nil), http.StatusOK, "results")
	metrics := map[any]any{}
	for _, a := range res.items("arms") {
		metrics[a["arm_code"]] = a["primary_metric"]
	}
	if metrics["control"] != 1.0 || metrics["fewer"] != 0.5 {
		t.Fatalf("primary metric by arm: %s", res.raw)
	}
	if c.audits(ctx, t, "learning_experiment.stopped", nil) != 1 {
		t.Fatal("audit learning_experiment.stopped")
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'learning_objective.mastered'`); n != mastered {
		t.Fatalf("learning_objective.mastered replayed: %d → %d", mastered, n)
	}
	// Lifecycle: an experiment past end_at completes (ended); a second run finds nothing.
	ending := create("rec-ending", now.Add(time.Hour))
	c.clk.Advance(2 * time.Hour)
	if n, err := c.exps.RunLifecycle(ctx); err != nil || n != 1 {
		t.Fatalf("lifecycle: %d %v", n, err)
	}
	e := must(t, c.do(t, http.MethodGet, experimentsAPI+"/"+ending.String(), saAdmin, nil, nil), http.StatusOK, "ending")
	if e.str("status") != "completed" || e.str("stop_reason") != "ended" {
		t.Fatalf("not completed at end_at: %s", e.raw)
	}
	if n, err := c.exps.RunLifecycle(ctx); err != nil || n != 0 {
		t.Fatalf("lifecycle again: %d %v", n, err)
	}
}
