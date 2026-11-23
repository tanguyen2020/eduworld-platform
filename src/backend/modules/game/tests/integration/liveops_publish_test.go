//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F077-AC-26, AC-27, AC-28 (T034, TC-EVT-009/010): the release core refuses a platform event without English text
// (TRANSLATION_INCOMPLETE), a festival shorter than 7 days, a check-in or streak source, a level gate on a learning event,
// a field locking content (outside the schema), an unknown quest or reward item, a decoration bundle over the budget or
// without its Low preset variant, and two event decorations of one area at the same time; nothing is stored.
func TestLiveOpsPublishChecks(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	deco := func(m map[string]any, size int) {
		m["components"].(map[string]any)["decoration"] = map[string]any{"bundle_id": "lantern_v1", "low_preset_bundle_id": "lantern_v1_low",
			"bundle_size_bytes": size, "areas": []string{"main_hub"}}
	}
	cases := map[string]struct {
		edit func(m map[string]any)
		want problem.Code
	}{
		"missing en":         {func(m map[string]any) { m["name"] = map[string]any{"vi": "Lễ hội"} }, problem.TranslationIncomplete},
		"missing tagline en": {func(m map[string]any) { m["short_tagline"] = map[string]any{"vi": "Vui"} }, problem.TranslationIncomplete},
		"festival 5 days": {func(m map[string]any) {
			s := m["schedule"].(map[string]any)
			start, _ := time.Parse(time.RFC3339, s["start_at"].(string))
			s["end_at"] = start.Add(5 * 24 * time.Hour).Format(time.RFC3339)
		}, problem.ValidationFailed},
		"check-in source": {func(m map[string]any) {
			m["event_points"].(map[string]any)["sources"] = []any{map[string]any{"source_code": "daily_login", "points": 10}}
		}, problem.ValidationFailed},
		"streak source": {func(m map[string]any) {
			m["event_points"].(map[string]any)["sources"] = []any{map[string]any{"source_code": "login_streak.day", "points": 10}}
		}, problem.ValidationFailed},
		"level gate on learning": {func(m map[string]any) {
			m["event_kind"] = "learning_campaign"
			m["eligibility"] = map[string]any{"min_level": 10}
		}, problem.ValidationFailed},
		"lock field": {func(m map[string]any) { m["locked_area_ids"] = []string{"learning_zone.math"} }, problem.ValidationFailed},
		"unknown quest": {func(m map[string]any) {
			m["components"].(map[string]any)["quest_ids"] = []string{uuid.NewString()}
		}, problem.ItemRefInvalid},
		"unknown reward item": {func(m map[string]any) {
			m["milestone_rewards"].([]any)[1].(map[string]any)["rewards"] = []any{map[string]any{"type": "badge", "badge_id": "badge_missing"}}
		}, problem.ItemRefInvalid},
		"xp reward": {func(m map[string]any) {
			m["participation_rewards"] = []any{map[string]any{"type": "xp", "amount": 50}}
		}, problem.ValidationFailed},
		"bundle over budget": {func(m map[string]any) { deco(m, 21<<20) }, problem.ValidationFailed},
		"no low preset": {func(m map[string]any) {
			deco(m, 1<<20)
			delete(m["components"].(map[string]any)["decoration"].(map[string]any), "low_preset_bundle_id")
		}, problem.ValidationFailed},
	}
	for name, c := range cases {
		_, err := e.publish(ctx, t, []command.ContentItem{e.festival(uuid.New(), c.edit)}, name)
		if codeOf(err) != c.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.events`); n != 0 {
		t.Fatalf("nothing stored: %d", n)
	}
	// One decoration per area at a time.
	e.publishEvent(ctx, t, e.festival(uuid.New(), func(m map[string]any) { deco(m, 15<<20) }))
	_, err := e.publish(ctx, t, []command.ContentItem{e.festival(uuid.New(), func(m map[string]any) { deco(m, 15<<20) })}, "overlap")
	if codeOf(err) != problem.ValidationFailed {
		t.Fatalf("overlapping decoration: %v", err)
	}
	// A school event comes from a template, never from the release core.
	_, err = e.publish(ctx, t, []command.ContentItem{e.festival(uuid.New(), func(m map[string]any) {
		m["owner_scope"], m["tenant_id"] = "tenant", e.tenantA.String()
	})}, "tenant")
	if codeOf(err) != problem.ValidationFailed {
		t.Fatalf("tenant event through the release core: %v", err)
	}
}

// T010 (data-model): the live-ops tenant tables are isolated by tenant (RLS) and the ledgers are append-only for the
// application roles; platform events are readable by every tenant, a school event by its tenant only.
func TestLiveOpsRLS(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	ev := uuid.New()
	e.publishEvent(ctx, t, e.festival(ev, nil))
	e.clock.Add(time.Hour)
	e.tick(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.liveDeliver(ctx, t, e.questEvent(an, "side"))
	e.liveDeliver(ctx, t, e.gradedEvent(an, uuid.New(), 3, 3, nil))
	seen := func(tenant uuid.UUID, q string) int {
		n := 0
		if err := e.inTenant(ctx, t, tenant, func(ctx context.Context) error {
			return txOf(ctx).QueryRowContext(ctx, q).Scan(&n)
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, table := range []string{"game.event_participation", "game.event_point_ledger", "game.community_goal_contributions",
		"game.community_goal_progress"} {
		if seen(e.tenantA, `SELECT count(*) FROM `+table) == 0 || seen(e.tenantB, `SELECT count(*) FROM `+table) != 0 {
			t.Fatalf("%s: tenant isolation", table)
		}
	}
	if seen(e.tenantB, `SELECT count(*) FROM game.event_runs`) != 1 || seen(e.tenantB, `SELECT count(*) FROM game.events`) != 1 {
		t.Fatal("platform rows are readable by every tenant")
	}
	for _, q := range []string{`UPDATE game.event_point_ledger SET delta = 1`, `DELETE FROM game.event_point_ledger`,
		`UPDATE game.community_goal_contributions SET amount = 1`, `DELETE FROM game.community_goal_contributions`,
		`DELETE FROM game.event_participation`, `INSERT INTO game.event_templates (template_id, created_at, updated_at) VALUES (gen_random_uuid(), now(), now())`} {
		if err := e.inTenant(ctx, t, e.tenantA, func(ctx context.Context) error {
			_, err := txOf(ctx).ExecContext(ctx, q)
			return err
		}); err == nil {
			t.Fatalf("game_app may not: %s", q)
		}
	}
}
