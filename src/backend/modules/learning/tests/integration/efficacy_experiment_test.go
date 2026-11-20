//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/experiment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

const experimentsAPI = "/admin/learning/experiments"

// F035-AC-20…AC-27 (US5, SC-004): definition checks, the stable class assignment, the labels of the learning events,
// the arm mastery in shadow (nothing displayed changes) and live (only the student experience changes), the results
// with small cells hidden, stop and revocation back to the control.
func TestEfficacyExperiments(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.effSetup(ctx, t)
	authztest.Cover(t, "listLearningExperiments", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createLearningExperiment", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getLearningExperiment", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateLearningExperiment", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "startLearningExperiment", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "stopLearningExperiment", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getLearningExperimentResults", authztest.CrossTenant, authztest.RoleDenied)
	// A faster model version for the test arm.
	c.exec(ctx, t, `INSERT INTO learning.mastery_model_configs (config_id, model_version, algorithm, params, status)
		SELECT $1, 'we-1.1', algorithm, params || '{"alpha_base": 0.9, "alpha_max": 0.95, "n_min": 1, "d_min": 1}'::jsonb, 'shadow'
		FROM learning.mastery_model_configs WHERE model_version = 'we-1.0'`, uuid.New())
	agr := c.agreement(t, c.tenantA, "HD-EXP", "efficacy_study", "experiment")
	now := c.clk.Now()
	body := func(arms []map[string]any, tenants ...uuid.UUID) map[string]any {
		return map[string]any{"code": "mastery-we-1-1", "kind": "mastery_model", "arms": arms, "tenant_ids": tenants,
			"start_at": now.Format(time.RFC3339), "end_at": now.Add(30 * 24 * time.Hour).Format(time.RFC3339)}
	}
	arms := []map[string]any{{"code": "control", "allocation_pct": 50, "is_control": true, "model_version": "we-1.0"},
		{"code": "we11", "allocation_pct": 50, "is_control": false, "model_version": "we-1.1"}}
	// AC-21: invalid definitions, roles.
	bad := []map[string]any{{"code": "control", "allocation_pct": 50, "is_control": true, "model_version": "we-1.1"},
		{"code": "we11", "allocation_pct": 50, "is_control": false, "model_version": "we-1.1"}}
	for name, b := range map[string]map[string]any{"control on we-1.1": body(bad, c.tenantA), "tenant B without agreement": body(arms, c.tenantB)} {
		if x := c.do(t, http.MethodPost, experimentsAPI, saAdmin, idem(), b); x.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", name, x.status, x.raw)
		}
	}
	for _, sub := range []string{adA, cmNoidung} {
		if x := c.do(t, http.MethodPost, experimentsAPI, sub, idem(), body(arms, c.tenantA)); x.status != http.StatusForbidden {
			t.Fatalf("%s: %d %s", sub, x.status, x.raw)
		}
	}
	e := must(t, c.do(t, http.MethodPost, experimentsAPI, saAdmin, idem(), body(arms, c.tenantA)), http.StatusCreated, "experiment")
	id := uuid.MustParse(e.str("experiment_id"))
	if e.str("status") != "draft" || e.num("revision") != 1 {
		t.Fatalf("created %s", e.raw)
	}
	// Put the arm under test first when the bucket of 3A falls in the first half (deterministic test, F035-AC-22).
	b := experiment.Bucket(id, k.class)
	order := arms
	if b < 50 {
		order = []map[string]any{arms[1], arms[0]}
	}
	p := must(t, c.do(t, http.MethodPatch, experimentsAPI+"/"+id.String(), saAdmin, ifm(e.header.Get("ETag")), map[string]any{"arms": order}),
		http.StatusOK, "patch draft")
	if p.num("revision") != 2 {
		t.Fatalf("revision %s", p.raw)
	}
	must(t, c.do(t, http.MethodPost, experimentsAPI+"/"+id.String()+":start", saAdmin, idem(), map[string]any{"exposure": "shadow"}), http.StatusOK,
		"shadow")
	g := must(t, c.do(t, http.MethodGet, experimentsAPI+"/"+id.String(), saAdmin, nil, nil), http.StatusOK, "get")
	if x := c.do(t, http.MethodPatch, experimentsAPI+"/"+id.String(), saAdmin, ifm(g.header.Get("ETag")), map[string]any{"unit": "student"}); x.status !=
		http.StatusUnprocessableEntity || x.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("patch shadow: %d %s", x.status, x.raw)
	}
	// AC-22, AC-25: both students of 3A practise: same arm (class unit), learning events labelled, nothing to the student.
	an := c.quizRun(t, hsAn, k.s.activityID)
	binh := c.quizRun(t, hsBinh, k.s.activityID)
	if n := count(ctx, t, c.platform, `SELECT count(DISTINCT arm_code) FROM learning.experiment_assignments WHERE experiment_id = $1
		AND unit_id = $2`, id, k.class); n != 1 {
		t.Fatalf("arms in 3A: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.experiment_assignments WHERE experiment_id = $1 AND arm_code = 'we11'`,
		id); n != 2 {
		t.Fatalf("assignments of the test arm %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE payload->>'attempt_id' = $1
		AND NOT (COALESCE(experiment_arms, '{}') @> ARRAY['mastery-we-1-1:we11']::varchar(97)[])`, an); n != 0 {
		t.Fatalf("events of hs.an without the arm label: %d", n)
	}
	// AC-23: the mastery consumer computes the arm model in parallel; getMyMastery is unchanged in shadow.
	m := command.Mastery{Store: write.MasteryRepo{}, Evidence: write.MasteryRepo{}, Learning: write.LearningEventWriter{},
		Events: write.Publisher{W: c.writer}, Clock: c.clk.Now, Metrics: observability.MasteryCounter{}, Experiments: c.exps}
	for _, a := range []string{an, binh} {
		if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, c.tenantA)), func(ctx context.Context) error {
			return m.ApplyGraded(ctx, command.GradedAttempt{EventID: uuid.New(), TenantID: c.tenantA, AttemptID: uuid.MustParse(a),
				OccurredAt: c.clk.Now()})
		}); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.experiment_mastery_states WHERE experiment_id = $1 AND student_id = $2`,
		id, c.users[hsAn].UserID); n == 0 {
		t.Fatal("no arm mastery for hs.an")
	}
	var active, arm float64
	if err := c.platform.QueryRowContext(ctx, `SELECT value::float8 FROM learning.objective_masteries WHERE student_id = $1 AND deleted_at IS NULL
		LIMIT 1`, c.users[hsAn].UserID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := c.platform.QueryRowContext(ctx, `SELECT value::float8 FROM learning.experiment_mastery_states WHERE student_id = $1 LIMIT 1`,
		c.users[hsAn].UserID).Scan(&arm); err != nil {
		t.Fatal(err)
	}
	if arm <= active {
		t.Fatalf("arm model not faster: arm %v, active %v", arm, active)
	}
	shadow := must(t, c.do(t, http.MethodGet, "/learning/mastery/me", hsAn, nil, nil), http.StatusOK, "my mastery (shadow)")
	// AC-24: live — the student sees the arm model; the school keeps the active model.
	must(t, c.do(t, http.MethodPost, experimentsAPI+"/"+id.String()+":start", saAdmin, idem(), map[string]any{"exposure": "live"}), http.StatusOK,
		"live")
	live := must(t, c.do(t, http.MethodGet, "/learning/mastery/me", hsAn, nil, nil), http.StatusOK, "my mastery (live)")
	if live.raw == shadow.raw {
		t.Fatalf("live view equals the active model: %s", live.raw)
	}
	for _, r := range []resp{shadow, live} {
		if strings.Contains(r.raw, "we11") || strings.Contains(r.raw, "mastery-we-1-1") {
			t.Fatalf("arm in a student response: %s", r.raw)
		}
	}
	school := must(t, c.do(t, http.MethodGet, "/students/"+c.users[hsAn].UserID.String()+"/progress", adA, nil, nil), http.StatusOK, "school view")
	if strings.Contains(school.raw, "we11") {
		t.Fatal("arm in the school view")
	}
	// AC-27: results by arm, small cells hidden; tenant roles 403.
	if _, err := c.exps.RunGuardrail(ctx); err != nil {
		t.Fatal(err)
	}
	res := must(t, c.do(t, http.MethodGet, experimentsAPI+"/"+id.String()+"/results", saAdmin, nil, nil), http.StatusOK, "results")
	for _, a := range res.items("arms") {
		if a["suppressed"] != true || a["primary_metric"] != nil {
			t.Fatalf("small cell shown: %v", a)
		}
	}
	if x := c.do(t, http.MethodGet, experimentsAPI+"/"+id.String()+"/results", adA, nil, nil); x.status != http.StatusForbidden {
		t.Fatalf("results as school admin: %d", x.status)
	}
	// AC-20: revoking the agreement takes the students of the tenant out of the experiment.
	must(t, c.do(t, http.MethodPost, agreements+"/"+agr.str("agreement_id")+":revoke", saAdmin, merge(idem(), tenantHdr(c.tenantA)),
		map[string]any{"reason": "Trường rút khỏi pilot theo văn bản"}), http.StatusOK, "revoke")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.experiment_assignments WHERE experiment_id = $1 AND status = 'active'`,
		id); n != 0 {
		t.Fatalf("active assignments after revocation: %d", n)
	}
	back := must(t, c.do(t, http.MethodGet, "/learning/mastery/me", hsAn, nil, nil), http.StatusOK, "my mastery (control)")
	if back.raw != shadow.raw {
		t.Fatalf("not back on the active model: %s", back.raw)
	}
	// AC-26: stop by hand (reason 10–1000), then nothing runs.
	if x := c.do(t, http.MethodPost, experimentsAPI+"/"+id.String()+":stop", saAdmin, idem(), map[string]any{"reason": "ngắn"}); x.status !=
		http.StatusUnprocessableEntity {
		t.Fatalf("short reason: %d", x.status)
	}
	st := must(t, c.do(t, http.MethodPost, experimentsAPI+"/"+id.String()+":stop", saAdmin, idem(), map[string]any{"reason": "Dừng thử nghiệm theo kế hoạch"}),
		http.StatusOK, "stop")
	if st.str("status") != "stopped" || st.str("stop_reason") != "manual" || c.audits(ctx, t, "learning_experiment.stopped", nil) != 1 {
		t.Fatalf("stop %s", st.raw)
	}
	if x := c.do(t, http.MethodPost, experimentsAPI+"/"+id.String()+":start", saAdmin, idem(), map[string]any{"exposure": "live"}); x.status !=
		http.StatusUnprocessableEntity {
		t.Fatalf("restart a stopped experiment: %d", x.status)
	}
}
