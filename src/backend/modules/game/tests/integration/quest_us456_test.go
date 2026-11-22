//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// completeLearn completes the test learning quest: meet, solve (passing attempt), back; the optional step stays open.
func (e *env) completeLearn(ctx context.Context, t *testing.T, s student) {
	t.Helper()
	tok := e.gtoken(s)
	if r := e.accept(t, s, tok, learnQuest, ""); r.status != http.StatusOK {
		t.Fatalf("accept: %d %s", r.status, r.raw)
	}
	a := e.startActivityAfterMeet(ctx, t, s, tok)
	if err := e.graded(ctx, t, s, a.body["attempt_id"].(string), 3, 3, uuid.New()); err != nil {
		t.Fatal(err)
	}
	sess, _ := e.sessionOf(ctx, t, s)
	if r := e.report(t, s, tok, learnQuest, "back", "", map[string]any{"game_session_id": sess, "npc_id": "npc.mr_numbers"}); r.body["status"] != "completed" {
		t.Fatalf("back: %d %s", r.status, r.raw)
	}
}

// F072-AC-19, AC-20, T049 (T032, TC-GAM-004): the completion rule (all required steps; the optional collect never
// blocks) completes the quest once: quest.completed with quest_id, quest_version_id, quest_type, objective_ids,
// quest_progress_id and the reward intents, an audit record, completed_at set by the server; the quest engine grants
// no XP, coin or item; a replay of the graded event does not complete it twice; every quest.* payload matches its
// schema; F071 game.unlock_on_events accepts the event (story chapter / unlock facts).
func TestQuestComplete(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.completeLearn(ctx, t, an)
	done := e.payloads(ctx, t, "quest.completed")
	if len(done) != 1 {
		t.Fatalf("quest.completed: %v", done)
	}
	c := done[0]
	obj, _ := c["objective_ids"].([]any)
	rw, _ := c["rewards"].([]any)
	if c["quest_id"] != learnQuest || c["quest_type"] != "learning" || len(obj) != 1 || c["quest_progress_id"] == nil || c["quest_version_id"] == nil ||
		len(rw) != 1 || c["completed_at"] == nil {
		t.Fatalf("payload: %v", c)
	}
	if n := e.audits(ctx, t, "quest.completed"); n != 1 {
		t.Fatalf("audit quest.completed: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND xp_total > 0`, an.id); n != 0 {
		t.Fatal("the quest engine granted XP")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.inventory_items WHERE student_id = $1 AND reward_grant_id IS NOT NULL`, an.id); n != 0 {
		t.Fatal("the quest engine granted an item")
	}
	var aid string
	_ = e.platform.QueryRowContext(ctx, `SELECT attempt_id::text FROM game.quest_step_attempts WHERE student_id = $1 AND status = 'passed'`, an.id).Scan(&aid)
	if err := e.graded(ctx, t, an, aid, 3, 3, uuid.New()); err != nil || e.events(ctx, t, "quest.completed") != 1 {
		t.Fatalf("replay completed again: %v", err)
	}
	// The F071 consumer takes quest.completed (quest_id present, T049): the fact is recorded once.
	ev := e.progressEnvelope("quest.completed", an.tenant, c)
	if err := e.progressEnv(ctx, t, ev); err != nil {
		t.Fatalf("game.unlock_on_events: %v", err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.world_unlock_facts WHERE student_id = $1 AND fact_kind = 'quest_completed' AND fact_ref = $2`,
		an.id, learnQuest); n != 1 {
		t.Fatalf("unlock fact: %d", n)
	}
	seen := e.checkEventSchemas(ctx, t)
	for _, typ := range []string{"quest.accepted", "quest.step_completed", "quest.completed"} {
		if seen[typ] == 0 {
			t.Fatalf("%s not checked", typ)
		}
	}
}

// dailyQuest is a daily learning quest of grade 3 granted by auto_daily (objective obj, subject math).
func dailyQuest(id, code, obj string, trigger string) command.ContentItem {
	return learningQuest(id, code, func(m map[string]any) {
		m["quest_type"], m["objective_ids"] = "daily", []string{obj}
		m["start_trigger"] = map[string]any{"kind": trigger}
		m["repeatability"] = map[string]any{"mode": "per_period"}
		if trigger == "auto_weekly" {
			m["quest_type"] = "weekly"
		}
	})
}

// F072-AC-22, AC-23 (T036, T037): the first game session of a period grants daily_quests_per_day auto_daily quests of
// the grade (weak objectives first) and the weekly quests of the week, with their period_key; a second session grants
// nothing; after quest_reset_time the job expires the unfinished ones (quest.abandoned reason expired), nothing is
// taken from the student, and the next period gets new quests.
func TestQuestPeriodicGrantAndExpiry(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	weak := "0193a5f4-6c1e-7b2a-9d1f-0000000a0099"
	items := []command.ContentItem{
		dailyQuest("0193a5f4-6c1e-7b2a-9d1f-0000000d0001", "daily.g3.one", "0193a5f4-6c1e-7b2a-9d1f-0000000a0011", "auto_daily"),
		dailyQuest("0193a5f4-6c1e-7b2a-9d1f-0000000d0002", "daily.g3.two", "0193a5f4-6c1e-7b2a-9d1f-0000000a0012", "auto_daily"),
		dailyQuest("0193a5f4-6c1e-7b2a-9d1f-0000000d0003", "daily.g3.three", "0193a5f4-6c1e-7b2a-9d1f-0000000a0013", "auto_daily"),
		dailyQuest("0193a5f4-6c1e-7b2a-9d1f-0000000d0004", "daily.g3.weak", weak, "auto_daily"),
		dailyQuest("0193a5f4-6c1e-7b2a-9d1f-0000000d0005", "weekly.g3.one", "0193a5f4-6c1e-7b2a-9d1f-0000000a0021", "auto_weekly"),
	}
	e.publishQuests(ctx, t, items, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if _, err := e.migrator.ExecContext(ctx, `INSERT INTO ref.mastery_summary (tenant_id, student_id, objective_id, level, value, source_version)
		VALUES ($1, $2, $3, 'beginning', 0.2, 1)`, an.tenant, an.id, weak); err != nil {
		t.Fatal(err)
	}
	now := e.clock.Now() // 2026-11-19 10:00 ICT (Thursday)
	e.sessionStarted(ctx, t, an, now)
	daily := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress WHERE student_id = $1 AND start_trigger = 'auto_daily' AND period_key = '2026-11-19'`, an.id)
	weekly := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress WHERE student_id = $1 AND start_trigger = 'auto_weekly' AND period_key = '2026-W47'`, an.id)
	if daily != 3 || weekly != 1 {
		t.Fatalf("granted daily %d weekly %d", daily, weekly)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress WHERE student_id = $1 AND quest_id = '0193a5f4-6c1e-7b2a-9d1f-0000000d0004'`, an.id); n != 1 {
		t.Fatal("the quest of the weak objective is not picked first")
	}
	e.sessionStarted(ctx, t, an, now.Add(time.Hour))
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress WHERE student_id = $1`, an.id); n != 4 {
		t.Fatalf("second session granted more: %d", n)
	}
	if got := listCodes(e.do(t, http.MethodGet, "/game/quests?status=active", e.gtoken(an), th(an), nil)); len(got) != 4 {
		t.Fatalf("granted quests listed: %v", got)
	}
	// Grade 4: none of the grade 3 dailies; the seed daily of every grade is granted.
	em := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, em, 4)
	e.sessionStarted(ctx, t, em, now)
	var codes []string
	rows, _ := e.platform.QueryContext(ctx, `SELECT v.code FROM game.quest_progress p JOIN game.quest_versions v ON v.quest_version_id = p.quest_version_id
		WHERE p.student_id = $1`, em.id)
	for rows.Next() {
		var c string
		_ = rows.Scan(&c)
		codes = append(codes, c)
	}
	_ = rows.Close()
	if len(codes) != 1 || codes[0] != "daily.hub.lanterns" {
		t.Fatalf("grade 4 grants: %v", codes)
	}
	// Reset: the next day 04:00 ICT expires the daily quests, the weekly one stays.
	e.clock.Set(time.Date(2026, 11, 19, 21, 30, 0, 0, time.UTC)) // 2026-11-20 04:30 ICT
	n, err := e.questsUC().ExpirePeriodQuests(ctx, 100)
	if err != nil || n != 4 { // 3 dailies of an, 1 of em
		t.Fatalf("expired %d %v", n, err)
	}
	exp := e.payloads(ctx, t, "quest.abandoned")
	if len(exp) != 4 || exp[0]["reason"] != "expired" {
		t.Fatalf("quest.abandoned: %v", exp)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress WHERE student_id = $1 AND status = 'active'`, an.id); n != 1 {
		t.Fatalf("weekly quest kept: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND xp_total < 0`, an.id); n != 0 {
		t.Fatal("XP taken")
	}
	e.sessionStarted(ctx, t, an, e.clock.Now())
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress WHERE student_id = $1 AND period_key = '2026-11-20'`, an.id); n != 3 {
		t.Fatalf("next period: %d", n)
	}
}

// F072-AC-24 (T039, TC-CNT-001): a new version published while a student works on v1 keeps v1 pinned until the end; a
// student who accepts afterwards gets v2.
func TestQuestVersionPinning(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	a := e.accept(t, an, e.gtoken(an), questMath3, "")
	e.publishQuests(ctx, t, []command.ContentItem{questItem(t, questMath3, func(m map[string]any) {
		m["name"] = map[string]any{"vi": "Phiên chợ mới", "en": "New market"}
	})}, e.tenantA)
	b := e.accept(t, binh, e.gtoken(binh), questMath3, "")
	if a.body["quest_version_id"] == b.body["quest_version_id"] {
		t.Fatal("B got the version pinned by A")
	}
	ga := e.do(t, http.MethodGet, "/game/quests/"+questMath3, e.gtoken(an), th(an), nil)
	gb := e.do(t, http.MethodGet, "/game/quests/"+questMath3, e.gtoken(binh), th(binh), nil)
	if ga.body["quest_version_id"] != a.body["quest_version_id"] || ga.body["name"].(map[string]any)["vi"] != "Phụ giúp phiên chợ" ||
		gb.body["name"].(map[string]any)["vi"] != "Phiên chợ mới" {
		t.Fatalf("pinned delivery: %s / %s", ga.raw, gb.raw)
	}
	p := e.play(t, an, "learning_zone.math")
	e.completeMath3(t, p)
	var v string
	_ = e.platform.QueryRowContext(ctx, `SELECT quest_version_id::text FROM game.quest_progress WHERE student_id = $1 AND status = 'completed'`, an.id).Scan(&v)
	if v != a.body["quest_version_id"] {
		t.Fatalf("completed on %s", v)
	}
	// Unassigned from the tenant: no new accept (404), the history stays the student's own.
	e.unassign(ctx, t, uuid.MustParse(questMath3), e.tenantA)
	if r := e.accept(t, binh, e.gtoken(binh), questMath3, ""); r.status != http.StatusOK || r.body["quest_progress_id"] != b.body["quest_progress_id"] {
		t.Fatalf("active progress of B after unassign: %d %s", r.status, r.raw)
	}
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.accept(t, other, e.gtoken(other), questMath3, ""); r.status != http.StatusNotFound {
		t.Fatalf("accept an unassigned quest: %d %s", r.status, r.raw)
	}
}

// completeMath3 completes math.g3.market_helper (presence in learning_zone.math).
func (e *env) completeMath3(t *testing.T, p played) {
	t.Helper()
	s := p.s
	if r := e.accept(t, s, p.d.token, questMath3, ""); r.status != http.StatusOK {
		t.Fatalf("accept: %d %s", r.status, r.raw)
	}
	for _, rep := range []struct{ step, k, v string }{{"meet_mr_numbers", "npc_id", "npc.mr_numbers"}, {"go_fruit_stall", "interactable_id", "quest.math.fruit_stall"},
		{"collect_baskets", "interactable_id", "quest.math.apple_basket_1"}, {"collect_baskets", "interactable_id", "quest.math.apple_basket_2"},
		{"collect_baskets", "interactable_id", "quest.math.apple_basket_3"}, {"report_back", "npc_id", "npc.mr_numbers"}} {
		if r := e.report(t, s, p.d.token, questMath3, rep.step, "", body(p, rep.k, rep.v)); r.status != http.StatusOK {
			t.Fatalf("%s: %d %s", rep.step, r.status, r.raw)
		}
	}
}

// F072-AC-25 (T040): once → QUEST_ALREADY_COMPLETED; repeatable within cooldown_hours → QUEST_COOLDOWN with retry_after;
// after the cooldown it starts again; beyond max_completions → QUEST_ALREADY_COMPLETED.
func TestQuestRepeatability(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	e.publishQuests(ctx, t, []command.ContentItem{questItem(t, questMath3, func(m map[string]any) {
		m["repeatability"] = map[string]any{"mode": "repeatable", "cooldown_hours": 24, "max_completions": 2}
	})}, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	p := e.play(t, an, "main_hub")
	e.completeMainA(t, p)
	if r := e.accept(t, an, p.d.token, questMainA, ""); r.status != http.StatusConflict || r.code() != string(problem.QuestAlreadyCompleted) {
		t.Fatalf("once: %d %s", r.status, r.raw)
	}
	if r := e.enter(t, an, p.d, p.session, "learning_zone.math", ""); r.status != http.StatusOK {
		t.Fatal(r.raw)
	}
	e.completeMath3(t, p)
	cd := e.accept(t, an, p.d.token, questMath3, "")
	if cd.status != http.StatusConflict || cd.code() != string(problem.QuestCooldown) || !strings.Contains(cd.raw, "retry_after") {
		t.Fatalf("cooldown: %d %s", cd.status, cd.raw)
	}
	if got := listCodes(e.do(t, http.MethodGet, "/game/quests", p.d.token, th(an), nil))["math.g3.market_helper"]; got["locked_reason"] != "cooldown" || got["retry_after"] == nil {
		t.Fatalf("list in cooldown: %v", got)
	}
	e.clock.Add(25 * time.Hour)
	p = e.play(t, an, "learning_zone.math") // the old session timed out with the clock
	e.completeMath3(t, p)
	e.clock.Add(25 * time.Hour)
	if r := e.accept(t, an, e.gtoken(an), questMath3, ""); r.status != http.StatusConflict || r.code() != string(problem.QuestAlreadyCompleted) {
		t.Fatalf("max_completions: %d %s", r.status, r.raw)
	}
	var nos []int
	rows, _ := e.platform.QueryContext(ctx, `SELECT completion_no FROM game.quest_progress WHERE student_id = $1 AND quest_id = $2 ORDER BY started_at`, an.id, questMath3)
	for rows.Next() {
		var n int
		_ = rows.Scan(&n)
		nos = append(nos, n)
	}
	_ = rows.Close()
	if len(nos) != 2 || nos[0] != 1 || nos[1] != 2 {
		t.Fatalf("completion_no: %v", nos)
	}
}

// F072-AC-26 (T041): time_limit_exceeded with reset_step resets only that step and restarts the timer; fail_quest ends the
// progress as failed (quest.abandoned failed + audit quest.failed) and the quest can be accepted again at once; the end
// of availability expires an active progress; nothing is taken from the student.
func TestQuestFailureConditions(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	e.publishQuests(ctx, t, []command.ContentItem{questItem(t, questMath3, func(m map[string]any) {
		m["availability"] = map[string]any{"time_limit_minutes": 10}
		m["failure_conditions"] = []any{map[string]any{"kind": "time_limit_exceeded", "step_id": "collect_baskets"}}
	})}, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	p := e.play(t, an, "learning_zone.math")
	e.accept(t, an, p.d.token, questMath3, "")
	for _, rep := range []struct{ step, k, v string }{{"meet_mr_numbers", "npc_id", "npc.mr_numbers"}, {"go_fruit_stall", "interactable_id", "quest.math.fruit_stall"},
		{"collect_baskets", "interactable_id", "quest.math.apple_basket_1"}} {
		if r := e.report(t, an, p.d.token, questMath3, rep.step, "", body(p, rep.k, rep.v)); r.status != http.StatusOK {
			t.Fatalf("%s: %d %s", rep.step, r.status, r.raw)
		}
	}
	e.clock.Add(11 * time.Minute)
	if n, err := e.questsUC().ApplyQuestFailures(ctx, 100); err != nil || n != 1 {
		t.Fatalf("failure timer: %d %v", n, err)
	}
	st, steps := e.progressRow(ctx, t, an, questMath3)
	if st != "active" || steps["collect_baskets"].Count != 0 || steps["collect_baskets"].Status != "active" || steps["go_fruit_stall"].Status != "completed" {
		t.Fatalf("reset_step: %s %v", st, steps)
	}
	if n, _ := e.questsUC().ApplyQuestFailures(ctx, 100); n != 0 {
		t.Fatal("the timer restarted with the reset")
	}
	// fail_quest
	e.publishQuests(ctx, t, []command.ContentItem{questItem(t, questMath3, func(m map[string]any) {
		m["availability"] = map[string]any{"time_limit_minutes": 10}
		m["failure_conditions"] = []any{map[string]any{"kind": "time_limit_exceeded", "on_fail": "fail_quest"}}
	})}, e.tenantA)
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.accept(t, binh, e.gtoken(binh), questMath3, "")
	e.clock.Add(11 * time.Minute)
	if _, err := e.questsUC().ApplyQuestFailures(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.progressRow(ctx, t, binh, questMath3); st != "failed" {
		t.Fatalf("fail_quest: %s", st)
	}
	var reasons []string
	for _, a := range e.payloads(ctx, t, "quest.abandoned") {
		reasons = append(reasons, a["reason"].(string))
	}
	if strings.Join(reasons, ",") != "failed" || e.audits(ctx, t, "quest.failed") != 1 {
		t.Fatalf("failed events: %v", reasons)
	}
	if r := e.accept(t, binh, e.gtoken(binh), questMath3, ""); r.status != http.StatusOK || r.body["status"] != "active" {
		t.Fatalf("accept again after failure: %d %s", r.status, r.raw)
	}
	// availability ended → expired.
	end := e.clock.Now().Add(time.Hour)
	e.publishQuests(ctx, t, []command.ContentItem{learningQuest(learnQuest, "math.g3.test_learning", func(m map[string]any) {
		m["availability"] = map[string]any{"end_at": end.Format(time.RFC3339)}
	})}, e.tenantA)
	cuong := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.accept(t, cuong, e.gtoken(cuong), learnQuest, "")
	e.clock.Set(end.Add(time.Minute))
	if _, err := e.questsUC().ApplyQuestFailures(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.progressRow(ctx, t, cuong, learnQuest); st != "expired" {
		t.Fatalf("availability ended: %s", st)
	}
}

// F072-AC-27 (T042): publishing a quest is refused (422) for a payload outside quest.schema.json v2, a text without en
// (TRANSLATION_INCOMPLETE), an unknown NPC, area or interactable (ITEM_REF_INVALID), a learning quest without objectives,
// questions in a row beyond learning_block_max_items of the grade band, a step type without handler or a content kind
// createAttempt does not open (VALIDATION_FAILED); the seed quests publish and a second run publishes nothing.
func TestQuestPublishValidator(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	cases := map[string]struct {
		item command.ContentItem
		code problem.Code
	}{
		"v1 quest type": {questItem(t, questMath3, func(m map[string]any) { m["quest_type"] = "educational" }), problem.ValidationFailed},
		"missing en": {questItem(t, questMath3, func(m map[string]any) {
			m["steps"].([]any)[0].(map[string]any)["title"] = map[string]any{"vi": "Gặp"}
		}), problem.TranslationIncomplete},
		"unknown npc": {questItem(t, questMath3, func(m map[string]any) {
			m["steps"].([]any)[0].(map[string]any)["npc_id"] = "npc.ghost"
		}), problem.ItemRefInvalid},
		"unknown interactable": {questItem(t, questMath3, func(m map[string]any) {
			m["steps"].([]any)[1].(map[string]any)["interactable_id"] = "quest.math.none"
		}), problem.ItemRefInvalid},
		"unknown area":  {questItem(t, questMath3, func(m map[string]any) { m["area_id"] = "nowhere" }), problem.ItemRefInvalid},
		"no objectives": {learningQuest(learnQuest, "math.g3.test_learning", func(m map[string]any) { m["objective_ids"] = []string{} }), problem.ValidationFailed},
		"learning block": {learningQuest(learnQuest, "math.g3.test_learning", func(m map[string]any) {
			m["steps"].([]any)[1].(map[string]any)["pass_rule"].(map[string]any)["min_questions"] = 4
		}), problem.ValidationFailed},
		"boss step": {questItem(t, questMath3, func(m map[string]any) {
			m["steps"].([]any)[1] = map[string]any{"step_id": "boss", "order": 2, "type": "boss", "title": map[string]any{"vi": "Boss", "en": "Boss"},
				"boss_id": "0193a5f4-6c1e-7b2a-9d1f-0000000e0001"}
		}), problem.ValidationFailed},
		"lesson content": {learningQuest(learnQuest, "math.g3.test_learning", func(m map[string]any) {
			m["steps"].([]any)[1].(map[string]any)["content_ref"] = map[string]any{"kind": "lesson", "id": testContent}
		}), problem.ValidationFailed},
	}
	for name, c := range cases {
		_, err := e.publish(ctx, t, []command.ContentItem{c.item}, "invalid")
		if !codeIs(err, c.code) {
			t.Errorf("%s: %v, want %s", name, err, c.code)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_versions`); n != 0 {
		t.Fatalf("refused quests stored: %d", n)
	}
	res, err := e.publish(ctx, t, questSeed(t), "seed")
	if err != nil || res.Published != 4 {
		t.Fatalf("seed: %+v %v", res, err)
	}
	again, err := e.publish(ctx, t, questSeed(t), "seed")
	if err != nil || again.Published != 0 || again.Unchanged != 4 {
		t.Fatalf("seed again: %+v %v", again, err)
	}
	var payload []byte
	if err := e.platform.QueryRowContext(ctx, `SELECT payload FROM game.quest_versions WHERE quest_id = $1`, questMath3).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(payload, &m)
	if m["status"] != "published" || m["version"] != float64(1) || m["quest_version_id"] == nil {
		t.Fatalf("server fields: %v", m)
	}
	// Seed assignment by tenant code (T002): unknown codes are skipped.
	created, skipped, err := command.AssignSeedQuests{UoW: e.uow, Assignments: writerepoQuest(), Audit: writerepoAuditor(e)}.Assign(seedCtx(t),
		[]command.SeedAssignment{{QuestID: uuid.MustParse(questMath3), Tenants: []string{"t-a", "t-b", "pgd-unknown"}}})
	if err != nil || created != 2 || len(skipped) != 1 {
		t.Fatalf("seed assignment: %d %v %v", created, skipped, err)
	}
	if created, _, _ := (command.AssignSeedQuests{UoW: e.uow, Assignments: writerepoQuest(), Audit: writerepoAuditor(e)}).Assign(seedCtx(t),
		[]command.SeedAssignment{{QuestID: uuid.MustParse(questMath3), Tenants: []string{"t-a"}}}); created != 0 {
		t.Fatal("assigned twice")
	}
	// T010: the tenant manifests hold the quests assigned to them only.
	gm := e.gameManifest()
	gm.Quests = questContent()
	if err := e.uow.InTenantTx(ctx, platformScope(t), func(ctx context.Context) error {
		return gm.Rebuild(ctx, uuid.New(), write.ItemQuest, time.Now())
	}); err != nil {
		t.Fatal(err)
	}
	for tenant, want := range map[uuid.UUID]bool{e.tenantA: true, e.tenantB: true} {
		var entries string
		_ = e.platform.QueryRowContext(ctx, `SELECT entries::text FROM game.content_manifests WHERE tenant_id = $1 ORDER BY manifest_version DESC LIMIT 1`, tenant).Scan(&entries)
		if strings.Contains(entries, questMath3) != want || strings.Contains(entries, questMainA) {
			t.Fatalf("manifest of %s: %s", tenant, entries)
		}
	}
}
