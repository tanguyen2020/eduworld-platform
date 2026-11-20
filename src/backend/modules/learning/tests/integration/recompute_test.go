//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
)

// F031-AC-23, AC-24 (quickstart 20, 21): a new model version recomputed in shadow leaves the mastery in use unchanged and
// reports the level changes; activating it retires the old version, applies the new one and emits
// learning_objective.mastered only for an objective never mastered before.
func TestMasteryRecomputeAndActivate(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	k := c.masteryKit(ctx, t)
	right, _ := correctOption()
	run := func(sub string, options ...string) {
		k.deliver(ctx, t, k.graded(ctx, t, c.quizAnswers(t, sub, s.activityID, options...)))
	}
	run(hsAn, right, right, right)
	run(hsBinh, right, right, right)
	c.clk.Advance(25 * time.Hour)
	run(hsAn, right, right, right)
	run(hsBinh, right)
	an, binh := c.users[hsAn].UserID, c.users[hsBinh].UserID
	if m := c.masteryOf(ctx, t, binh, s.qc.lo); m.level != "proficient" || m.evidence != 4 {
		t.Fatalf("binh under we-1.0: %+v", m)
	}
	h := command.Recompute{UoW: c.uow, Store: write.RecomputeRepo{}, Mastery: k.mastery, Audit: write.Auditor{R: c.rec}, Clock: c.clk.Now}
	p := mastery.DefaultParams()
	p.M0, p.NMin = 0.5, 3
	params, _ := json.Marshal(p)
	if err := h.AddModel(ctx, "we-1.1", params); err != nil {
		t.Fatal(err)
	}
	if err := h.AddModel(ctx, "we-1.1", params); err == nil {
		t.Fatal("duplicate model version accepted")
	}
	bad := mastery.DefaultParams()
	bad.Thresholds.Mastered = 0.2
	if raw, _ := json.Marshal(bad); h.AddModel(ctx, "we-bad", raw) == nil {
		t.Fatal("invalid parameters accepted")
	}

	// F031-AC-23: shadow run — report, shadow rows, mastery in use unchanged.
	rep, err := h.Shadow(ctx, "we-1.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pairs != 2 || rep.ChangedLevel != 1 || rep.LevelsShadow["mastered"] != 2 || rep.LevelsCurrent["mastered"] != 1 ||
		rep.CurrentVersion != "we-1.0" {
		t.Fatalf("report: %+v", rep)
	}
	for _, id := range []string{an.String(), binh.String()} {
		if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.objective_masteries WHERE student_id = $1 AND model_version = 'we-1.0'`, id); n != 1 {
			t.Fatalf("mastery in use changed for %s", id)
		}
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.objective_mastery_shadows WHERE run_id = $1 AND model_version = 'we-1.1'`,
		rep.RunID); n != 2 {
		t.Fatalf("shadow rows = %d", n)
	}
	var report []byte
	if err := c.platform.QueryRowContext(ctx, `SELECT report FROM learning.mastery_recompute_runs WHERE run_id = $1 AND status = 'completed'`,
		rep.RunID).Scan(&report); err != nil || len(report) == 0 {
		t.Fatalf("stored report: %v", err)
	}
	mastered := `SELECT count(*) FROM learning.learning_events WHERE event_type = 'learning_objective.mastered' AND student_id = $1`
	if count(ctx, t, c.platform, mastered, an) != 1 || count(ctx, t, c.platform, mastered, binh) != 0 {
		t.Fatal("mastered events before activation")
	}

	// F031-AC-24: activation by the System Admin.
	sa := c.users[saAdmin].UserID
	act, err := h.Activate(ctx, rep.RunID, sa)
	if err != nil || act.Pairs != 2 || act.ModelVersion != "we-1.1" {
		t.Fatalf("activate: %+v %v", act, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.mastery_model_configs WHERE status = 'active' AND deleted_at IS NULL`); n != 1 {
		t.Fatalf("active versions = %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.mastery_model_configs WHERE model_version = 'we-1.0' AND status = 'retired'`); n != 1 {
		t.Fatal("we-1.0 not retired")
	}
	for _, id := range []string{an.String(), binh.String()} {
		if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.objective_masteries WHERE student_id = $1 AND model_version = 'we-1.1'
			AND level = 'mastered'`, id); n != 1 {
			t.Fatalf("mastery of %s not applied", id)
		}
	}
	if count(ctx, t, c.platform, mastered, an) != 1 {
		t.Fatal("mastered re-emitted for an objective already mastered")
	}
	if count(ctx, t, c.platform, mastered, binh) != 1 {
		t.Fatal("first mastery under the new model not emitted")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.mastery_history WHERE source_kind = 'recompute'`); n != 2 {
		t.Fatalf("recompute history rows = %d", n)
	}
	if n := count(ctx, t, c.migrator, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'mastery_model.activated'`); n != 1 {
		t.Fatalf("activation audits = %d", n)
	}
	if _, err := h.Activate(ctx, rep.RunID, sa); err == nil {
		t.Fatal("run activated twice")
	}
	if _, err := h.Shadow(ctx, "we-1.1", nil); err == nil {
		t.Fatal("shadow run of the active version")
	}
}
