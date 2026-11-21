//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func chaptersOf(t *testing.T, r resp) map[string]map[string]any {
	t.Helper()
	if r.status != http.StatusOK {
		t.Fatalf("getWorldExploration: %d %s", r.status, r.raw)
	}
	out := map[string]map[string]any{}
	list, _ := r.body["chapters"].([]any)
	for _, c := range list {
		m := c.(map[string]any)
		out[m["chapter_code"].(string)] = m
	}
	return out
}

func (e *env) exploration(t *testing.T, s student) resp {
	t.Helper()
	return e.do(t, http.MethodGet, "/game/world/exploration", e.gtoken(s), th(s), nil)
}

// F071-AC-14, AC-15 (T029, TC-WLD-002, TC-SVC-003, SC-003): the last main quest of chapter 0 unlocks the arena once —
// one unlock row, one world.area_unlocked, one world.area.unlocked push — and 1,000 replays of the event (new event ids)
// add nothing; level, achievement and event facts are recorded; malformed events are permanent failures.
func TestUnlockOnEvents(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	for i := 0; i < 1000; i++ {
		e.progress(ctx, t, "quest.completed", e.tenantA, map[string]any{"student_id": an.id, "quest_id": questCh0Last})
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.world_area_unlocks WHERE student_id = $1`, an.id); n != 1 {
		t.Fatalf("unlock rows: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.world_area_unlocks WHERE student_id = $1 AND area_id = 'arena'
		AND unlocked_by = 'quest' AND rule_version = 1`, an.id); n != 1 {
		t.Fatalf("arena unlock: %d", n)
	}
	if n := e.events(ctx, t, "world.area_unlocked"); n != 1 {
		t.Fatalf("world.area_unlocked: %d", n)
	}
	if p := e.worldPush.all(); len(p) != 1 || p[0] != an.id.String()+":arena:quest" {
		t.Fatalf("pushes: %v", p)
	}
	if a := e.listAreas(t, an)["arena"]; a["accessible"] != true {
		t.Fatalf("arena: %v", a)
	}
	// Other facts are kept for the rules that read them.
	e.progress(ctx, t, "level.reached", e.tenantA, map[string]any{"student_id": an.id, "from_level": 4, "to_level": 5})
	e.progress(ctx, t, "level.reached", e.tenantA, map[string]any{"student_id": an.id, "from_level": 2, "to_level": 3}) // older: kept at 5
	e.progress(ctx, t, "achievement.unlocked", e.tenantA, map[string]any{"student_id": an.id, "achievement_id": "ach.first_steps"})
	e.progress(ctx, t, "event.participation_started", e.tenantA, map[string]any{"student_id": an.id, "event_run_id": "evt.mid_autumn"})
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.world_unlock_facts WHERE student_id = $1 AND ((fact_kind = 'level' AND fact_value = 5)
		OR fact_kind IN ('achievement_unlocked','event_participation','quest_completed'))`, an.id); n != 4 {
		t.Fatalf("facts: %d", n)
	}
	for _, bad := range []map[string]any{{"quest_id": questCh0Last}, {"student_id": an.id}} {
		err := e.progressErr(ctx, t, "quest.completed", e.tenantA, bad)
		if err == nil || !outboxPermanent(err) {
			t.Fatalf("malformed quest.completed %v: %v", bad, err)
		}
	}
	if got := e.checkEventSchemas(ctx, t); got["world.area_unlocked"] != 1 {
		t.Fatalf("schemas: %v", got)
	}
}

func outboxPermanent(err error) bool { return err != nil && errors.Is(err, outbox.ErrPermanent) }

func (e *env) progressErr(ctx context.Context, t *testing.T, typ string, tenant uuid.UUID, payload map[string]any) error {
	t.Helper()
	return e.progressEnv(ctx, t, e.progressEnvelope(typ, tenant, payload))
}

// F071-AC-14 (T011 objective_progress, TC-SVC-003): learning_objective.progressed is projected into ref.mastery_summary by
// source_version (an older event never overwrites) and re-evaluates the rules — an area gated on objectives opens when
// enough of them reach the level.
func TestMasteryProjectionUnlocks(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	lo1, lo2 := uuid.New(), uuid.New()
	gated := seedItem(t, write.ItemWorldArea, "area_id", "social_zone", func(m map[string]any) {
		m["unlock_rule"] = map[string]any{"objective_progress": map[string]any{"objective_ids": []string{lo1.String(), lo2.String()},
			"min_count": 2, "min_level": "developing"}}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{gated}, "gate social"); err != nil {
		t.Fatal(err)
	}
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	h := consumer.MasteryProjection{World: e.worldUC(), Mastery: writerepo.MasteryRepo{}}
	deliver := func(lo uuid.UUID, level string, to float64, at int) {
		b, _ := json.Marshal(map[string]any{"student_id": an.id, "objective_id": lo, "from": 0, "to": to, "level": level, "model_version": "bkt-1",
			"source_event_id": uuid.New()})
		tt := e.tenantA
		env := outbox.Envelope{EventID: uuid.New(), EventType: "learning_objective.progressed", SchemaVersion: 1,
			OccurredAt: e.clock.Now().Add(time.Duration(at) * time.Second), Producer: "learning", TenantID: &tt, Payload: b}
		if err := e.uow.InTenantTx(ctx, scopeOf(t, e.tenantA), func(ctx context.Context) error { return h.Handle(ctx, env) }); err != nil {
			t.Fatal(err)
		}
	}
	deliver(lo1, "developing", 0.5, 10)
	deliver(lo1, "beginning", 0.2, 5) // older event: ignored
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM ref.mastery_summary WHERE student_id = $1 AND level = 'developing'`, an.id); n != 1 {
		t.Fatalf("projection: %d", n)
	}
	if a := e.listAreas(t, an)["social_zone"]; a["accessible"] != false {
		t.Fatalf("one objective is not enough: %v", a)
	}
	deliver(lo2, "proficient", 0.7, 20)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.world_area_unlocks WHERE student_id = $1 AND area_id = 'social_zone'
		AND unlocked_by = 'learning_objective'`, an.id); n != 1 {
		t.Fatalf("social zone unlock: %d", n)
	}
}

// F071-AC-16 (T030): publishing a learning area whose unlock_rule holds min_level or event_participation is refused (422)
// and nothing is published; an unknown (payment) condition is refused by the schema decoder.
func TestUnlockRulePublishValidation(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	for _, rule := range []any{
		map[string]any{"min_level": map[string]any{"level": 5}},
		map[string]any{"any": []any{map[string]any{"quest_completed": map[string]any{"quest_id": questCh0A}},
			map[string]any{"event_participation": map[string]any{"event_id": "evt.tet"}}}},
		map[string]any{"pay_coins": map[string]any{"amount": 100}},
	} {
		item := seedItem(t, write.ItemWorldArea, "area_id", "learning_zone.math", func(m map[string]any) { m["unlock_rule"] = rule })
		if _, err := e.publish(ctx, t, []command.ContentItem{item}, "bad rule"); codeOf(err) != problem.ValidationFailed {
			t.Fatalf("rule %v: %v", rule, err)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.world_area_versions WHERE area_id = 'learning_zone.math'`); n != 1 {
		t.Fatalf("versions: %d", n)
	}
	// A min_level rule stays allowed on a non-learning area (decoration zone).
	ok := seedItem(t, write.ItemWorldArea, "area_id", "social_zone", func(m map[string]any) {
		m["unlock_rule"] = map[string]any{"min_level": map[string]any{"level": 10}}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{ok}, "level gate"); err != nil {
		t.Fatalf("min_level on a social area: %v", err)
	}
}

// F071-AC-21, AC-22, AC-23 (T038, T039, TC-WLD-004, TC-WLD-005): chapter 0 is available at first entry; its first main
// quest → in_progress, its last → completed with story.chapter_completed once (replay does not repeat it); then the
// subject chapters open in parallel (Vietnamese is available while Math is not done); one subject chapter opens chapter
// 5 and the Discovery Ocean; the counts of main quests are reported.
func TestStoryProgress(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, an, 5)
	ch := chaptersOf(t, e.exploration(t, an))
	if len(ch) != 7 || ch["chapter.0.invitation"]["status"] != "available" || ch["chapter.1.number_kingdom"]["status"] != "locked" ||
		ch["chapter.0.invitation"]["order"] != float64(0) {
		t.Fatalf("initial chapters: %v", ch)
	}
	quest := func(q string) {
		e.progress(ctx, t, "quest.completed", e.tenantA, map[string]any{"student_id": an.id, "quest_id": q})
	}
	quest(questCh0A)
	ch = chaptersOf(t, e.exploration(t, an))
	if c := ch["chapter.0.invitation"]; c["status"] != "in_progress" || c["main_quests_completed"] != float64(1) || c["main_quests_total"] != float64(2) {
		t.Fatalf("chapter 0 in progress: %v", c)
	}
	quest(questCh0Last)
	quest(questCh0Last) // replay
	ch = chaptersOf(t, e.exploration(t, an))
	for _, code := range []string{"chapter.1.number_kingdom", "chapter.2.language_forest", "chapter.3.science_island", "chapter.4.history_kingdom"} {
		if ch[code]["status"] != "available" {
			t.Fatalf("%s after chapter 0: %v", code, ch[code])
		}
	}
	if ch["chapter.0.invitation"]["status"] != "completed" || ch["chapter.0.invitation"]["completed_at"] == nil {
		t.Fatalf("chapter 0: %v", ch["chapter.0.invitation"])
	}
	quest(questCh2A) // Vietnamese started while Math is untouched (parallel)
	quest(questCh1A)
	quest(questCh1B)
	quest(questCh1B)
	ch = chaptersOf(t, e.exploration(t, an))
	if ch["chapter.1.number_kingdom"]["status"] != "completed" || ch["chapter.2.language_forest"]["status"] != "in_progress" ||
		ch["chapter.5.discovery_ocean"]["status"] != "available" || ch["chapter.6.challenge_mountain"]["status"] != "locked" {
		t.Fatalf("after chapter 1: %v", ch)
	}
	if n := e.events(ctx, t, "story.chapter_completed"); n != 2 {
		t.Fatalf("story.chapter_completed: %d", n)
	}
	if a := e.listAreas(t, an)["quest_zone.discovery_ocean"]; a["accessible"] != true {
		t.Fatalf("discovery ocean after one subject chapter: %v", a)
	}
	if got := e.checkEventSchemas(ctx, t); got["story.chapter_completed"] != 2 {
		t.Fatalf("schemas: %v", got)
	}
}
