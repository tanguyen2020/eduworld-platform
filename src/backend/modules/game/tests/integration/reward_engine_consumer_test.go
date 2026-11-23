//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
)

// F074-AC-01 (TC-GAM-004): every trigger of the catalog posts the XP of its published rule with source_code, rule_id and
// rule_version; quest types map to their source codes.
func TestRewardEngineTriggers(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	type want struct {
		code, rule, sourceType string
		amount                 int
	}
	events := []struct {
		name string
		do   func()
		want want
	}{
		{"activity.graded", func() { e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 4, 5, nil)) }, want{"activity.completed", ruleActivity, "activity", 12}},
		{"lesson.completed", func() { e.mustDeliver(ctx, t, uc, e.lessonEvent(s, uuid.New(), []int{3}, "practice")) }, want{"lesson.completed", ruleLesson, "lesson", 20}},
		{"quest.completed main", func() { e.mustDeliver(ctx, t, uc, e.questEvent(s, "main")) }, want{"quest.main_completed", ruleQuestMain, "quest", 150}},
		{"quest.completed side", func() { e.mustDeliver(ctx, t, uc, e.questEvent(s, "learning")) }, want{"quest.side_completed", ruleQuestSide, "quest", 80}},
		{"quest.completed daily", func() { e.mustDeliver(ctx, t, uc, e.questEvent(s, "daily")) }, want{"quest.daily_completed", ruleQuestDaily, "quest", 40}},
		{"learning_objective.mastered", func() {
			e.mustDeliver(ctx, t, uc, e.envelope("learning_objective.mastered", "learning", s.tenant, map[string]any{"student_id": s.id,
				"objective_id": uuid.New(), "value": 0.9, "evidence_count": 6, "distinct_days": 3, "model_version": "m1", "new_level": "mastered",
				"mastered_at": e.clock.Now()}))
		}, want{"mastery.objective_mastered", ruleObjective, "objective", 50}},
		{"world.discovery_found", func() {
			e.mustDeliver(ctx, t, uc, e.envelope("world.discovery_found", "game", s.tenant, map[string]any{"student_id": s.id, "area_id": "main_hub",
				"interactable_id": "discovery.hub.spark_tree", "kind": "discovery", "completion_pct": 10, "found_at": e.clock.Now()}))
		}, want{"exploration.discovery", ruleDiscovery, "world", 10}},
		{"world.area_completed", func() {
			e.mustDeliver(ctx, t, uc, e.envelope("world.area_completed", "game", s.tenant, map[string]any{"student_id": s.id, "area_id": "main_hub",
				"completion_pct": 100, "completed_at": e.clock.Now()}))
		}, want{"exploration.area_completed", ruleAreaComplete, "world", 100}},
	}
	for i, ev := range events {
		ev.do()
		rows := e.ledger(ctx, t, s)
		if len(rows) != i+1 {
			t.Fatalf("%s: %d ledger rows", ev.name, len(rows))
		}
		r := rows[i]
		if r.sourceCode != ev.want.code || r.ruleID.UUID.String() != ev.want.rule || r.version == nil || *r.version != 1 ||
			r.sourceType != ev.want.sourceType || r.amount != ev.want.amount || r.capped {
			t.Fatalf("%s: %+v", ev.name, r)
		}
	}
	// A failed activity (pass rule not met) earns nothing.
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 2, 5, map[string]any{"passed": false}))
	if n := len(e.ledger(ctx, t, s)); n != len(events) {
		t.Fatalf("failed attempt posted: %d rows", n)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["xp.awarded"] != len(events) {
		t.Fatalf("xp.awarded events: %v", seen)
	}
}

// F074-AC-02 (TC-GAM-004, SC-001): the same envelope delivered again, or another event of the same source, adds nothing.
func TestRewardEngineReplay(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	env := e.questEvent(s, "main")
	for i := 0; i < 3; i++ {
		e.mustDeliver(ctx, t, uc, env)
	}
	again := env
	again.EventID = uuid.Must(uuid.NewV7()) // a redelivery under a new event id: same (student, source_type, source_id, rule_id)
	e.mustDeliver(ctx, t, uc, again)
	rows := e.ledger(ctx, t, s)
	if len(rows) != 1 || rows[0].amount != 150 {
		t.Fatalf("ledger after replays: %+v", rows)
	}
	if _, xp := e.xpOf(ctx, t, s); xp != 150 {
		t.Fatalf("xp_total %d", xp)
	}
	if n := e.events(ctx, t, "xp.awarded"); n != 1 {
		t.Fatalf("xp.awarded %d", n)
	}
}

// F074-AC-03 (TC-ECO-002): ledger, xp_total/level cache and outbox xp.awarded are one transaction — a rollback leaves
// nothing; the cache equals Σ ledger.
func TestRewardEngineOneTransaction(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	env := e.questEvent(s, "main")
	boom := errors.New("boom after the award")
	err := e.uow.InTenantTx(ctx, scopeOf(t, s.tenant), func(ctx context.Context) error {
		if err := (consumer.RewardEngine{Awards: uc}).Handle(ctx, env); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if n := len(e.ledger(ctx, t, s)) + e.events(ctx, t, "xp.awarded") + count(ctx, t, e.platform,
		`SELECT count(*) FROM game.game_profiles WHERE student_id = $1`, s.id); n != 0 {
		t.Fatalf("rolled back award left %d rows", n)
	}
	e.mustDeliver(ctx, t, uc, env)
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, uuid.New(), []int{3}, "practice"))
	level, xp := e.xpOf(ctx, t, s)
	if xp != sumAmounts(e.ledger(ctx, t, s)) || xp != 170 || level != 2 {
		t.Fatalf("cache %d level %d, ledger %d", xp, level, sumAmounts(e.ledger(ctx, t, s)))
	}
	if n := e.events(ctx, t, "xp.awarded"); n != 2 {
		t.Fatalf("xp.awarded %d", n)
	}
}

// F074-AC-04 (TC-GAM-002): the same content version 4 times in the window → 100% / 50% / 25% / 0%.
func TestRewardEngineRepeatMultipliers(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	cv := uuid.New()
	for i := 0; i < 4; i++ {
		e.mustDeliver(ctx, t, uc, e.gradedEvent(s, cv, 5, 5, nil)) // 5 first-try × 3 XP = 15
	}
	rows := e.ledger(ctx, t, s)
	want := []int{15, 7, 3, 0}
	if len(rows) != 4 {
		t.Fatalf("rows %+v", rows)
	}
	for i, w := range want {
		if rows[i].amount != w || rows[i].capped {
			t.Fatalf("completion %d: %+v, want %d", i+1, rows[i], w)
		}
	}
	// Another content version starts again at 100%; after the 7-day window the count restarts.
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 5, 5, nil))
	e.clock.Add(8 * 24 * 3600 * 1e9)
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, cv, 5, 5, nil))
	rows = e.ledger(ctx, t, s)
	if rows[4].amount != 15 || rows[5].amount != 15 {
		t.Fatalf("new version / after window: %+v", rows[4:])
	}
}

// F074-AC-05 (TC-GAM-001): at xp_daily_cap only the remainder is posted, capped = true also on a 0 XP posting, and
// progression.xp.updated carries the flag.
func TestRewardEngineDailyCap(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	p := defaultPolicy()
	p.DailyCap = 200 // xp_daily_cap of this test configuration
	uc := e.progressionWith(p)
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main")) // 150
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main")) // 50 of 150
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main")) // 0
	rows := e.ledger(ctx, t, s)
	if len(rows) != 3 || rows[0].amount != 150 || rows[0].capped || rows[1].amount != 50 || !rows[1].capped || rows[2].amount != 0 || !rows[2].capped {
		t.Fatalf("ledger %+v", rows)
	}
	pushes := e.progPush().of("progression.xp.updated")
	if len(pushes) != 3 || pushes[2]["capped"] != true || pushes[2]["daily_xp_remaining"] != 0 || pushes[0]["capped"] != false {
		t.Fatalf("pushes %+v", pushes)
	}
	// The next calendar day (ICT) opens the cap again.
	e.clock.Add(24 * 3600 * 1e9)
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main"))
	if rows = e.ledger(ctx, t, s); rows[3].amount != 150 || rows[3].capped {
		t.Fatalf("next day %+v", rows[3])
	}
}

// F074-AC-06 (TC-GAM-001): XP counts first-try correct answers only; 0 first-try answers → 0 XP.
func TestRewardEngineFirstTryCorrect(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 0, 5, map[string]any{"correct_count": 3}))
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 2, 5, map[string]any{"correct_count": 4}))
	rapid := e.gradedEvent(s, uuid.New(), 4, 5, map[string]any{"rapid_guess_flagged": true})
	rapid.Payload = withoutKey(t, rapid.Payload, "first_try_correct_count")
	e.mustDeliver(ctx, t, uc, rapid)
	rows := e.ledger(ctx, t, s)
	if len(rows) != 3 || rows[0].amount != 0 || rows[1].amount != 6 || rows[2].amount != 0 {
		t.Fatalf("ledger %+v", rows)
	}
}

// F074-AC-07: content ≥ 2 grades below the student × xp_below_grade_multiplier; a recommendation or an assignment gets
// full XP.
func TestRewardEngineBelowGrade(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, s, 5)
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 4, 5, map[string]any{"content_grade_levels": []int{3}}))
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 4, 5, map[string]any{"content_grade_levels": []int{3}, "recommendation_id": uuid.New()}))
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 4, 5, map[string]any{"content_grade_levels": []int{3}, "mode": "assignment"}))
	e.mustDeliver(ctx, t, uc, e.gradedEvent(s, uuid.New(), 4, 5, map[string]any{"content_grade_levels": []int{4}}))
	rows := e.ledger(ctx, t, s)
	if len(rows) != 4 || rows[0].amount != 3 || rows[1].amount != 12 || rows[2].amount != 12 || rows[3].amount != 12 {
		t.Fatalf("ledger %+v", rows)
	}
}
