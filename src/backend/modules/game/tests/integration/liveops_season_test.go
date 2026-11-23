//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// trackItem is a free season track of season: 5 tiers × 100 points, 100 points/day, daily quests 20, learning activities 5.
func trackItem(t *testing.T, season uuid.UUID, edit func(m map[string]any)) command.ContentItem {
	t.Helper()
	var tiers []any
	for i := 1; i <= 5; i++ {
		tiers = append(tiers, map[string]any{"tier": i, "is_milestone": i == 5, "rewards": []any{xuGrant(10 * i)}})
	}
	m := map[string]any{"schema_version": 1, "season_track_id": uuid.NewSHA1(season, []byte("track")).String(), "code": "season.f077_test",
		"owner_scope": "platform", "tenant_id": nil, "season_id": season.String(), "name": map[string]any{"vi": "Mùa thử", "en": "Test season"},
		"paid_track": false, "points": map[string]any{"per_tier": 100, "daily_cap": 100, "sources": []any{
			map[string]any{"source_code": "daily_quest.completed", "points": 20}, map[string]any{"source_code": "learning_activity.completed", "points": 5}}},
		"tiers": tiers, "end_of_season": map[string]any{"unclaimed_rewards": "auto_grant"}}
	if edit != nil {
		edit(m)
	}
	b, _ := json.Marshal(m)
	return command.ContentItem{Type: write.ItemSeasonTrack, Payload: b}
}

// seasonEvent delivers season.started or season.ended (platform event of module competition — fixture before F106 runs).
func (e *env) seasonEvent(ctx context.Context, t *testing.T, typ string, season uuid.UUID, sv int64, payload map[string]any) {
	t.Helper()
	payload["season_id"], payload["code"], payload["source_version"] = season.String(), "s-"+season.String()[:8], sv
	b, _ := json.Marshal(payload)
	env := outbox.Envelope{EventID: uuid.Must(uuid.NewV7()), EventType: typ, SchemaVersion: 1, OccurredAt: e.clock.Now(), Producer: "competition",
		Payload: b}
	c := consumer.SeasonProjection{Seasons: e.live().uc}
	if err := e.systemScope(ctx, t, func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
}

// F077-AC-17, AC-18, AC-19, AC-20 (T028, TC-EVT-007): season.started (fixture) opens the published track; season points
// come from the catalog sources, once per source event, at most 100 per day, a tier every 100 points
// (season_track.tier_reached); a reached tier is claimed once (same key replays), a tier not reached is 409
// SEASON_TIER_NOT_REACHED; season.ended grants the reached tiers not claimed (auto_granted), keeps the history and the next
// season starts from 0; an older event never overwrites the projection; a paid track or an XP tier is refused.
func TestSeasonTrack(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	season := uuid.New()
	e.publishEvent(ctx, t, trackItem(t, season, nil))
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	daily := func() outbox.Envelope { return e.questEvent(an, "daily") }
	e.liveDeliver(ctx, t, daily())
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.season_point_ledger`); n != 0 {
		t.Fatal("no season points before season.started")
	}
	if r := e.do(t, http.MethodGet, "/game/season-track", e.gtoken(an), th(an), nil); r.status != http.StatusNotFound {
		t.Fatalf("no season yet: %d", r.status)
	}
	now := e.clock.Now()
	started := map[string]any{"name": "Mùa thử", "starts_at": now.Add(-time.Hour), "ends_at": now.Add(56 * 24 * time.Hour)}
	e.seasonEvent(ctx, t, "season.started", season, 1, started)
	e.seasonEvent(ctx, t, "season.started", season, 1, started) // replay: idempotent
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM ref.seasons WHERE season_id = $1 AND status = 'started'`, season); n != 1 {
		t.Fatalf("projection: %d", n)
	}
	q := daily()
	e.liveDeliver(ctx, t, q)
	e.liveDeliver(ctx, t, q) // replay
	for i := 0; i < 5; i++ {
		e.liveDeliver(ctx, t, daily())
	}
	r := e.do(t, http.MethodGet, "/game/season-track", e.gtoken(an), th(an), nil)
	if r.status != http.StatusOK || r.body["points"].(float64) != 100 || r.body["tier"].(float64) != 1 || r.body["daily_points_remaining"].(float64) != 0 ||
		r.body["max_tier"].(float64) != 5 {
		t.Fatalf("track: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.season_point_ledger WHERE student_id = $1 AND capped`, an.id); n != 0 {
		t.Fatalf("the cap cut nothing partially: %d", n)
	}
	if e.events(ctx, t, "season_track.tier_reached") != 1 || len(e.live().push.of("season.track.updated")) != 5 {
		t.Fatalf("tier_reached %d, pushes %d", e.events(ctx, t, "season_track.tier_reached"), len(e.live().push.of("season.track.updated")))
	}
	claim := func(tier string, h map[string]string) resp {
		return e.do(t, http.MethodPost, "/game/season-track/tiers/"+tier+":claim", e.gtoken(an), withTenant(h, an.tenant), nil)
	}
	k := idem()
	xu0 := e.balanceOf(ctx, t, an)
	if r := claim("1", k); r.status != http.StatusOK || r.body["claimed"] != true {
		t.Fatalf("claim tier 1: %d %s", r.status, r.raw)
	}
	if r := claim("1", map[string]string{"Idempotency-Key": k["Idempotency-Key"]}); r.status != http.StatusOK || r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", r.status, r.raw)
	}
	if e.grantsOf(ctx, t, an, "season_tier") != 1 || e.balanceOf(ctx, t, an) != xu0+10 || e.audits(ctx, t, "season_tier.claimed") != 1 {
		t.Fatal("one grant of tier 1")
	}
	if r := claim("2", idem()); r.status != http.StatusConflict || r.code() != string(problem.SeasonTierNotReached) {
		t.Fatalf("tier 2 not reached: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/game/season-track/tiers/2:claim", e.gtoken(an), th(an), nil); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no key: %d", r.status)
	}
	e.clock.Add(24 * time.Hour) // next ICT day: 100 more points → tier 2, not claimed
	for i := 0; i < 5; i++ {
		e.liveDeliver(ctx, t, daily())
	}
	e.seasonEvent(ctx, t, "season.ended", season, 2, map[string]any{"ended_at": e.clock.Now()})
	if e.grantsOf(ctx, t, an, "season_tier") != 2 {
		t.Fatalf("tier 2 auto-granted: %d", e.grantsOf(ctx, t, an, "season_tier"))
	}
	var auto int
	for _, p := range e.payloads(ctx, t, "season_track.reward_claimed") {
		if p["auto_granted"] == true && p["tier"].(float64) == 2 {
			auto++
		}
	}
	if auto != 1 {
		t.Fatal("season_track.reward_claimed auto_granted for tier 2")
	}
	e.seasonEvent(ctx, t, "season.ended", season, 2, map[string]any{"ended_at": e.clock.Now()}) // replay: nothing more
	if e.grantsOf(ctx, t, an, "season_tier") != 2 {
		t.Fatal("season.ended twice grants nothing more")
	}
	if r := e.do(t, http.MethodGet, "/game/season-track", e.gtoken(an), th(an), nil); r.status != http.StatusNotFound {
		t.Fatalf("no open season: %d", r.status)
	}
	h := e.do(t, http.MethodGet, "/game/season-track/history", e.gtoken(an), th(an), nil)
	items := dataOf(h)
	if h.status != http.StatusOK || len(items) != 1 || items[0]["final_tier"].(float64) != 2 || len(items[0]["auto_granted_tiers"].([]any)) != 1 {
		t.Fatalf("history: %d %s", h.status, h.raw)
	}
	// A new season starts from 0 (a new progress row); out of order: ended (newer) before started (older) stays ended.
	next := uuid.New()
	e.seasonEvent(ctx, t, "season.ended", next, 5, map[string]any{"ended_at": e.clock.Now()})
	e.seasonEvent(ctx, t, "season.started", next, 4, map[string]any{"starts_at": now, "ends_at": now.Add(30 * 24 * time.Hour)})
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM ref.seasons WHERE season_id = $1 AND status = 'ended'`, next); n != 1 {
		t.Fatal("an older season.started never reopens an ended season")
	}
	// F077-AC-20: never a paid track, never XP or a collection item in a tier.
	for name, edit := range map[string]func(m map[string]any){
		"paid": func(m map[string]any) { m["paid_track"] = true },
		"xp": func(m map[string]any) {
			m["tiers"].([]any)[0].(map[string]any)["rewards"] = []any{map[string]any{"type": "xp", "amount": 50}}
		},
		"collection item": func(m map[string]any) {
			m["tiers"].([]any)[0].(map[string]any)["rewards"] = []any{map[string]any{"type": "collection_item", "collection_item_id": "card_times_table_7"}}
		},
	} {
		if _, err := e.publish(ctx, t, []command.ContentItem{trackItem(t, uuid.New(), edit)}, name); codeOf(err) != problem.ValidationFailed {
			t.Fatalf("%s: %v", name, err)
		}
	}
	e.checkEventSchemas(ctx, t)
}
