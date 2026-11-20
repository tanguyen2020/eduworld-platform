//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

var leverParams = map[string]any{"arm_length_m": 1, "slots_per_side": 4, "mass_1_kg": 2, "mass_2_kg": 1}

// simConfig is a simulation config of physics.lever v1 (2 kg and 1 kg), goal "balanced", one conclusion question,
// fixed seed, three runs.
func simConfig(question string) map[string]any {
	return map[string]any{
		"content": map[string]any{"simulation_kind": "physics", "template_code": "physics.lever", "min_template_version": 1,
			"parameters": leverParams,
			"goals": []any{map[string]any{"goal_id": "balance", "description": map[string]string{"vi": "Cân bằng đòn bẩy"},
				"check": map[string]any{"metric": "balanced", "operator": "eq", "value": 1}}},
			"steps":       []any{map[string]any{"step_id": "place", "instruction": map[string]string{"vi": "Đặt hai vật lên đòn bẩy"}}},
			"questions":   []any{map[string]any{"question_version_id": question}},
			"seed_policy": "fixed"},
		"interaction":     map[string]any{"max_runs": 3, "show_measurements": true},
		"evaluation":      map[string]any{"strategies": []string{"deterministic_check", "auto"}, "max_score": 2},
		"feedback":        map[string]any{"timing": "immediate"},
		"completion_rule": map[string]any{"mode": "goals_met"},
	}
}

type simSetup struct {
	attSetup
	activityID string
}

func (c *curEnv) simSetup(ctx context.Context, t *testing.T) simSetup {
	t.Helper()
	s := simSetup{attSetup: c.attemptSetup(ctx, t, nil)}
	s.activityID = c.createActivity(t, qlcmA, activityBody(s.qc, "simulation", simConfig(s.questions[0])))
	c.publishActivity(ctx, t, s.activityID, 1)
	return s
}

func place(t int, mass int, side string, slot int) map[string]any {
	return map[string]any{"t_ms": t, "action": "place_mass", "args": map[string]any{"mass_index": mass, "side": side, "slot": slot}}
}

func release(t int) map[string]any {
	return map[string]any{"t_ms": t, "action": "release", "args": map[string]any{}}
}

func (c *curEnv) run(t *testing.T, sub, attemptID string, h hdr, seed any, log []any, client map[string]any) resp {
	t.Helper()
	body := map[string]any{"template_code": "physics.lever", "template_version": 1, "seed": seed, "parameters": leverParams, "action_log": log}
	if client != nil {
		body["client_outcome"] = client
	}
	if h == nil {
		h = idem()
	}
	return c.do(t, http.MethodPost, "/learning/activity-attempts/"+attemptID+"/simulation-runs", sub, h, body)
}

func (c *curEnv) outboxRuns(ctx context.Context, t *testing.T, typ, attemptID string) int {
	t.Helper()
	return count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1 AND envelope->'payload'->>'attempt_id' = $2`,
		typ, attemptID)
}

// F066-AC-10…AC-14, AC-16 (T025, T026, T027; TC-SIM-001, TC-SIM-002, TC-LRN-008): start serves the pinned template,
// parameters, a fixed seed and goals without their check; a balanced log is verified (goal met, simulation.goal_met,
// outbox simulation.run_verified); an unbalanced log claimed as met is not met and adds simulation_outcome_mismatch; the
// same key replays the same run; a run with another seed is rejected (422 SIMULATION_LOG_INVALID, stored, outbox
// simulation.run_rejected, simulation_log_invalid); at max_runs 422 SIMULATION_RUN_LIMIT_REACHED; submit scores the
// question plus the goals of the last verified run and writes activity.completed.
func TestSimulationRun(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.simSetup(ctx, t)
	authztest.Cover(t, "submitSimulationRun", authztest.CrossTenant, authztest.RoleDenied)

	r := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start")
	id := r.str("attempt_id")
	if strings.Contains(r.raw, `"check"`) || strings.Contains(r.raw, `"operator"`) || len(answerKeysIn(r.raw)) > 0 {
		t.Fatalf("render leaks the goal check or an answer: %s", r.raw)
	}
	seed := r.body["render"].(map[string]any)["render_payload"].(map[string]any)["seed"]
	if r.str("render", "render_payload", "template_code") != "physics.lever" || r.str("render", "render_payload", "template_version") != "1" ||
		r.str("render", "render_payload", "runs_used") != "0" || seed == nil || len(r.items("items")) != 1 {
		t.Fatalf("start: %s", r.raw)
	}
	again := must(t, c.start(t, hsBinh, s.activityID, nil, nil), http.StatusCreated, "start binh")
	if again.body["render"].(map[string]any)["render_payload"].(map[string]any)["seed"] != seed {
		t.Fatalf("seed_policy fixed: %v vs %v", seed, again.body["render"].(map[string]any)["render_payload"].(map[string]any)["seed"])
	}

	// AC-12: unbalanced, claimed as met.
	un := must(t, c.run(t, hsAn, id, nil, seed, []any{place(0, 0, "left", 2), place(500, 1, "right", 2), release(900)},
		map[string]any{"goal_met": true, "balanced": true}), http.StatusOK, "unbalanced")
	if un.body["verified"] != true || un.body["goal_met"] != false || un.str("verified_outcome", "balanced") != "0" || un.str("runs_used") != "1" {
		t.Fatalf("unbalanced: %s", un.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.integrity_signals WHERE attempt_id = $1
		AND signal_type = 'simulation_outcome_mismatch'`, id); n != 1 {
		t.Fatalf("mismatch signal: %d", n)
	}
	// AC-11, AC-14: balanced → verified; the same key replays the same run.
	key := idem()
	ok := must(t, c.run(t, hsAn, id, key, seed, []any{place(0, 0, "left", 2), place(500, 1, "right", 4), release(900)}, nil), http.StatusOK, "balanced")
	if ok.body["verification_status"] != "verified" || ok.body["goal_met"] != true || ok.str("runs_used") != "2" || ok.str("max_runs") != "3" {
		t.Fatalf("balanced: %s", ok.raw)
	}
	replay := c.run(t, hsAn, id, key, seed, []any{place(0, 0, "left", 2), place(500, 1, "right", 4), release(900)}, nil)
	if replay.status != 200 || replay.str("run_id") != ok.str("run_id") || replay.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", replay.status, replay.raw)
	}
	if n := c.learningEvents(ctx, t, "simulation.goal_met", id); n != 1 {
		t.Fatalf("simulation.goal_met: %d", n)
	}
	if n := c.outboxRuns(ctx, t, "simulation.run_verified", id); n != 2 {
		t.Fatalf("outbox simulation.run_verified: %d", n)
	}
	// AC-13: another seed → rejected and stored.
	bad := c.run(t, hsAn, id, nil, 12345, []any{release(0)}, nil)
	if bad.status != 422 || bad.str("code") != "SIMULATION_LOG_INVALID" {
		t.Fatalf("other seed: %d %s", bad.status, bad.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.simulation_runs WHERE attempt_id = $1 AND verification_status = 'rejected'
		AND reject_code = 'pinned_value_mismatch'`, id); n != 1 {
		t.Fatalf("rejected run: %d", n)
	}
	if n := c.outboxRuns(ctx, t, "simulation.run_rejected", id); n != 1 {
		t.Fatalf("outbox simulation.run_rejected: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.integrity_signals WHERE attempt_id = $1
		AND signal_type = 'simulation_log_invalid'`, id); n != 1 {
		t.Fatalf("log invalid signal: %d", n)
	}
	// AC-14: the fourth run is over max_runs.
	if r := c.run(t, hsAn, id, nil, seed, []any{release(0)}, nil); r.status != 422 || r.str("code") != "SIMULATION_RUN_LIMIT_REACHED" {
		t.Fatalf("limit: %d %s", r.status, r.raw)
	}
	g := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+id, hsAn, nil, nil), http.StatusOK, "get")
	if g.str("render", "render_payload", "runs_used") != "3" {
		t.Fatalf("runs_used: %s", g.raw)
	}

	// AC-16: the conclusion question, then submit: question + goals of the last verified run (balanced).
	right, _ := correctOption()
	q := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+"/responses", hsAn, idem(),
		map[string]any{"kind": "simulation_question", "position": 1, "response": map[string]string{"option_id": right}}), http.StatusOK, "question")
	if q.body["is_correct"] != true {
		t.Fatalf("question: %s", q.raw)
	}
	sub := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil), http.StatusOK, "submit")
	if sub.str("status") != "graded" || sub.str("evaluation", "score") != "2" || sub.str("evaluation", "max_score") != "2" {
		t.Fatalf("submit: %s", sub.raw)
	}
	if n := c.learningEvents(ctx, t, "activity.completed", id); n != 1 {
		t.Fatalf("activity.completed: %d", n)
	}
}

// F066-AC-13, AC-27, AC-28 (T026, T046, T047; TC-TEN-001, TC-AUTH-014, TC-TEN-024): t_ms going back and an action
// outside the template are rejected runs; an over-long action, 21 arguments, 51 parameters or 501 actions are 422 on
// the contract; another student or tenant gets 404, a staff token 403 GAME_ACCESS_STUDENT_ONLY.
func TestSimulationValidationAndIsolation(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.simSetup(ctx, t)
	r := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start")
	id := r.str("attempt_id")
	seed := r.body["render"].(map[string]any)["render_payload"].(map[string]any)["seed"]
	for name, log := range map[string][]any{
		"time_not_increasing": {release(500), release(100)},
		"unknown_action":      {map[string]any{"t_ms": 0, "action": "teleport", "args": map[string]any{}}},
	} {
		if r := c.run(t, hsAn, id, nil, seed, log, nil); r.status != 422 || r.str("code") != "SIMULATION_LOG_INVALID" || !strings.Contains(r.raw, name) {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	args := map[string]any{}
	for i := 0; i < 21; i++ {
		args[strings.Repeat("a", i+1)] = i
	}
	params := map[string]any{}
	for i := 0; i < 51; i++ {
		params[strings.Repeat("p", i+1)] = i
	}
	many := make([]any, 501)
	for i := range many {
		many[i] = release(i)
	}
	for name, body := range map[string]map[string]any{
		"action 65": {"template_code": "physics.lever", "template_version": 1, "seed": seed, "parameters": leverParams,
			"action_log": []any{map[string]any{"t_ms": 0, "action": strings.Repeat("x", 65)}}},
		"args 21": {"template_code": "physics.lever", "template_version": 1, "seed": seed, "parameters": leverParams,
			"action_log": []any{map[string]any{"t_ms": 0, "action": "release", "args": args}}},
		"parameters 51": {"template_code": "physics.lever", "template_version": 1, "seed": seed, "parameters": params, "action_log": []any{}},
		"template 65":   {"template_code": strings.Repeat("t", 65), "template_version": 1, "seed": seed, "parameters": leverParams, "action_log": []any{}},
		"501 actions":   {"template_code": "physics.lever", "template_version": 1, "seed": seed, "parameters": leverParams, "action_log": many},
	} {
		if r := c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+"/simulation-runs", hsAn, idem(), body); r.status != 422 ||
			r.str("code") != "VALIDATION_FAILED" {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	for _, sub := range []string{hsBinh, hsDung} {
		if r := c.run(t, sub, id, nil, seed, []any{release(0)}, nil); r.status != http.StatusNotFound {
			t.Fatalf("%s: %d %s", sub, r.status, r.raw)
		}
	}
	if r := c.run(t, qlcmA, id, nil, seed, []any{release(0)}, nil); r.status != http.StatusForbidden || r.str("code") != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.simulation_runs WHERE attempt_id = $1`, id); n != 2 {
		t.Fatalf("runs stored: %d", n)
	}
}

// F066-AC-09, AC-26 (T028; TC-SIM-004): a parameter out of range, an unknown parameter, a script field or an unknown
// metric is 422 ACTIVITY_CONFIG_INVALID; listSimulationTemplates returns the packaged template with its actions and
// metrics to content staff only.
func TestSimulationConfigAndTemplates(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.simSetup(ctx, t)
	authztest.Cover(t, "listSimulationTemplates", authztest.RoleDenied)
	for name, mutate := range map[string]func(map[string]any){
		"out of range": func(m map[string]any) {
			m["content"].(map[string]any)["parameters"] = map[string]any{"arm_length_m": 9, "slots_per_side": 4, "mass_1_kg": 2, "mass_2_kg": 1}
		},
		"unknown parameter": func(m map[string]any) {
			m["content"].(map[string]any)["parameters"] = map[string]any{"arm_length_m": 1, "slots_per_side": 4, "mass_1_kg": 2, "mass_2_kg": 1, "script": "x"}
		},
		"script field": func(m map[string]any) { m["content"].(map[string]any)["script"] = "print(1)" },
		"unknown metric": func(m map[string]any) {
			m["content"].(map[string]any)["goals"].([]any)[0].(map[string]any)["check"].(map[string]any)["metric"] = "temperature"
		},
		"unknown template": func(m map[string]any) { m["content"].(map[string]any)["template_code"] = "chemistry.boiling" },
	} {
		cfg := simConfig(s.questions[0])
		mutate(cfg)
		if r := c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(s.qc, "simulation", cfg)); r.status != 422 ||
			r.str("code") != "ACTIVITY_CONFIG_INVALID" {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	list := must(t, c.do(t, http.MethodGet, "/cms/learning/simulation-templates", qlcmA, nil, nil), http.StatusOK, "templates")
	data := list.items("data")
	if len(data) != 1 || data[0]["template_code"] != "physics.lever" || data[0]["simulation_kind"] != "physics" || data[0]["status"] != "active" ||
		data[0]["checker_available"] != true || len(data[0]["actions"].([]any)) != 3 || data[0]["title"] != "Cân bằng đòn bẩy" {
		t.Fatalf("templates: %s", list.raw)
	}
	if r := c.do(t, http.MethodGet, "/cms/learning/simulation-templates", hsAn, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("student reads the templates: %d", r.status)
	}
}
