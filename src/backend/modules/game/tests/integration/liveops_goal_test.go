//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F077-AC-13, AC-14, AC-15, AC-16 (T025, T026, TC-EVT-005/006): community goal contributions come only from verified
// server events, once per source event, capped per student per day, never from a suspicious_speed attempt; PostgreSQL is
// the truth: a lost Valkey counter is rebuilt on read and goal-recompute corrects a drift; each milestone is reached once
// per scope and granted once to every contributor of the scope (a late first contributor too), never to a student
// without a contribution; getCommunityGoal shows the grades of the school (a grade with < 5 contributing students
// hidden), no ranking, no student id; the realtime update is throttled to one push per flush window.
func TestCommunityGoal(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	_, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	counter := cache.GoalCounter{C: vk}
	e.live().uc.Counter = counter
	ev := uuid.New()
	e.publishEvent(ctx, t, e.festival(ev, func(m map[string]any) {
		m["audience"].(map[string]any)["grade_levels"] = []int{3, 4, 5}
	}))
	e.clock.Add(time.Hour)
	e.tick(ctx, t)
	var goal uuid.UUID
	if err := e.platform.QueryRowContext(ctx, `SELECT g.goal_id FROM game.community_goals g JOIN game.event_runs r ON r.event_run_id = g.event_run_id
		WHERE r.event_id = $1`, ev).Scan(&goal); err != nil {
		t.Fatal(err)
	}
	newAt := func(grade int) student {
		s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
		e.setGrade(ctx, t, s, grade)
		return s
	}
	var g4, g5 []student
	for i := 0; i < 5; i++ {
		g4 = append(g4, newAt(4))
	}
	for i := 0; i < 4; i++ {
		g5 = append(g5, newAt(5))
	}
	online := g4[1]
	if r := e.start(t, online, e.device(online), newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("session: %d %s", r.status, r.raw)
	}
	cv := uuid.New()
	first := e.gradedEvent(g4[0], cv, 2, 4, nil)
	for i, s := range append(append([]student{}, g4...), g5...) {
		if i == 0 {
			e.liveDeliver(ctx, t, first)
			continue
		}
		e.liveDeliver(ctx, t, e.gradedEvent(s, cv, 2, 4, nil))
	}
	goalView := func(s student) resp {
		return e.do(t, http.MethodGet, "/game/community-goals/"+goal.String(), e.gtoken(s), th(s), nil)
	}
	r := goalView(g4[0])
	if r.status != http.StatusOK || r.body["progress"].(float64) != 18 || r.body["my_contributed"] != true || r.body["scope"] != "school" {
		t.Fatalf("goal: %d %s", r.status, r.raw)
	}
	grades := r.body["by_grade"].([]any)
	if len(grades) != 1 || grades[0].(map[string]any)["grade_level"].(float64) != 4 || grades[0].(map[string]any)["progress"].(float64) != 10 {
		t.Fatalf("grade 4 (5 contributors) shown, grade 5 (4) hidden: %v", grades)
	}
	for _, s := range append(append([]student{}, g4...), g5...) {
		if strings.Contains(r.raw, s.id.String()) {
			t.Fatal("no student id in the goal")
		}
	}
	// Replay, suspicious_speed: nothing added.
	e.liveDeliver(ctx, t, first)
	e.liveDeliver(ctx, t, e.gradedEvent(g5[0], cv, 10, 10, map[string]any{"rapid_guess_flagged": true}))
	if r := goalView(g4[0]); r.body["progress"].(float64) != 18 {
		t.Fatalf("replay and suspicious_speed add nothing: %s", r.raw)
	}
	// Daily cap 50: g4[0] has 2, then 20 + 20 + 8 (cut).
	for i := 0; i < 4; i++ {
		e.liveDeliver(ctx, t, e.gradedEvent(g4[0], cv, 20, 20, nil))
	}
	if n := count(ctx, t, e.platform, `SELECT coalesce(sum(amount), 0) FROM game.community_goal_contributions WHERE student_id = $1`, g4[0].id); n != 50 {
		t.Fatalf("cap 50/day: %d", n)
	}
	if r := goalView(g4[0]); r.body["progress"].(float64) != 66 {
		t.Fatalf("total 66: %s", r.raw)
	}
	reached := e.payloads(ctx, t, "community_goal.milestone_reached")
	if len(reached) != 2 || reached[0]["milestone_pct"].(float64) != 25 || reached[1]["milestone_pct"].(float64) != 50 {
		t.Fatalf("milestones 25, 50 once: %v", reached)
	}
	for _, s := range append(append([]student{}, g4...), g5...) {
		if n := e.grantsOf(ctx, t, s, "community_goal"); n != 2 {
			t.Fatalf("every contributor granted 25 and 50 once: %d", n)
		}
	}
	late := newAt(4)
	nobody := newAt(4)
	e.liveDeliver(ctx, t, e.gradedEvent(late, cv, 1, 1, nil))
	if e.grantsOf(ctx, t, late, "community_goal") != 2 || e.grantsOf(ctx, t, nobody, "community_goal") != 0 {
		t.Fatal("a late first contributor receives the reached milestones; a student without contribution nothing")
	}
	// Valkey is a cache: lost → rebuilt from PostgreSQL on read.
	key := cache.GoalKey(goal, e.schoolA)
	if n, ok, _ := counter.Get(ctx, goal, e.schoolA); !ok || n != 67 {
		t.Fatalf("counter 67: %d %v", n, ok)
	}
	if err := vk.Do(ctx, vk.B().Del().Key(key).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	if r := goalView(late); r.body["progress"].(float64) != 67 {
		t.Fatalf("rebuilt: %s", r.raw)
	}
	if n, ok, _ := counter.Get(ctx, goal, e.schoolA); !ok || n != 67 {
		t.Fatalf("counter rebuilt: %d %v", n, ok)
	}
	// goal-recompute corrects a drift of the progress row and the counter (the contributions are the truth).
	if _, err := e.migrator.ExecContext(ctx, `UPDATE game.community_goal_progress SET total = 5 WHERE goal_id = $1`, goal); err != nil {
		t.Fatal(err)
	}
	_ = counter.Set(ctx, goal, e.schoolA, 3)
	n, err := e.live().uc.RecomputeGoals(ctx, 100)
	if err != nil || n != 1 {
		t.Fatalf("recompute: %d %v", n, err)
	}
	if v, _, _ := counter.Get(ctx, goal, e.schoolA); v != 67 || count(ctx, t, e.platform, `SELECT total FROM game.community_goal_progress WHERE goal_id = $1`, goal) != 67 {
		t.Fatalf("recomputed: counter %d", v)
	}
	if e.audits(ctx, t, "community_goal.recomputed") != 1 {
		t.Fatal("recompute audit")
	}
	if p := e.live().push.of("event.community_goal.updated"); len(p) != 1 || p[0].student != online.id {
		t.Fatalf("one push per flush window: %d", len(p))
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.community_goal_contributions WHERE amount <= 0 OR grade_level IS NULL`); n != 0 {
		t.Fatalf("contributions with grade, positive: %d", n)
	}
	e.checkEventSchemas(ctx, t)
}
