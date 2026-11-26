package unit

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
)

// F108 T008 — F108-AC-04: the business invariants of each competition config type (grade constraints, speed bonus, tiers,
// leaderboard scopes, template parameters, cosmetic-only rewards) on top of the seeds, which all pass.

func seedDoc(t *testing.T, file string) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../../seed/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func templateDoc(t *testing.T, code string) map[string]any {
	t.Helper()
	d := seedDoc(t, "tournament_templates.json")
	for _, x := range d["templates"].([]any) {
		m := x.(map[string]any)
		if m["code"] == code {
			return m["payload"].(map[string]any)
		}
	}
	t.Fatalf("no template %s", code)
	return nil
}

func check(t *testing.T, typ string, doc map[string]any) []gameconfig.Violation {
	t.Helper()
	ty, err := gameconfig.Lookup(typ)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(doc)
	return gameconfig.Check(ty, b)
}

func wantField(t *testing.T, vs []gameconfig.Violation, field string) {
	t.Helper()
	for _, v := range vs {
		if v.Field == field {
			return
		}
	}
	t.Fatalf("want a violation on %s, got %+v", field, vs)
}

func TestConfigSeedsPassInvariants(t *testing.T) {
	for typ, file := range map[string]string{gameconfig.TypeMatchmaking: "matchmaking_config.json", gameconfig.TypeRanking: "ranking_rules.json",
		gameconfig.TypeSeasons: "seasons.json", gameconfig.TypeLeaderboards: "leaderboard_definitions.json"} {
		if vs := check(t, typ, seedDoc(t, file)); len(vs) != 0 {
			t.Fatalf("%s seed: %+v", typ, vs)
		}
	}
	for code, typ := range map[string]string{"grade_math_single_elim": gameconfig.TypeChallengeTp, "grade_round_robin": gameconfig.TypeChallengeTp,
		"grade_quick_quiz": gameconfig.TypeChallengeTp, "school_team_groups": gameconfig.TypeTournamentTp, "school_double_elim": gameconfig.TypeTournamentTp} {
		if vs := check(t, typ, templateDoc(t, code)); len(vs) != 0 {
			t.Fatalf("%s: %+v", code, vs)
		}
	}
}

func TestConfigMatchmakingGradeConstraints(t *testing.T) {
	d := seedDoc(t, "matchmaking_config.json")
	d["grade_window_g6_9"] = map[string]any{"after_s": 10, "levels": 2}
	vs := check(t, gameconfig.TypeMatchmaking, d)
	wantField(t, vs, "payload/grade_window_g6_9/levels")
	wantField(t, vs, "payload/grade_window_g6_9/after_s")

	d = seedDoc(t, "matchmaking_config.json")
	d["modes"].([]any)[1].(map[string]any)["min_grade"] = 2
	wantField(t, check(t, gameconfig.TypeMatchmaking, d), "payload/modes/1/min_grade")
}

func TestConfigSpeedBonusFloor(t *testing.T) {
	d := seedDoc(t, "matchmaking_config.json")
	d["battle"] = map[string]any{"speed_bonus_min_grade": 3}
	wantField(t, check(t, gameconfig.TypeMatchmaking, d), "payload/battle/speed_bonus_min_grade")
	d["battle"] = map[string]any{"speed_bonus_min_grade": gameconfig.SpeedBonusMinGrade() + 1}
	if vs := check(t, gameconfig.TypeMatchmaking, d); len(vs) != 0 {
		t.Fatalf("a stricter floor is allowed: %+v", vs)
	}
	d["battle"] = map[string]any{"timed_quiz": map[string]any{"rank_min_grade": 2}}
	wantField(t, check(t, gameconfig.TypeMatchmaking, d), "payload/battle/timed_quiz/rank_min_grade")
}

func TestConfigRankingTiersAndVisibility(t *testing.T) {
	d := seedDoc(t, "ranking_rules.json")
	lv := d["tiers"].(map[string]any)["levels"].([]any)
	lv[2].(map[string]any)["min_rating"] = 1200.0 // below T2 1300: decreasing
	wantField(t, check(t, gameconfig.TypeRanking, d), "payload/tiers/levels/2/min_rating")

	d = seedDoc(t, "ranking_rules.json")
	vis := d["leaderboard_visibility"].(map[string]any)
	vis["g1_3"].(map[string]any)["personal_rank"] = true
	vis["g4_5"].(map[string]any)["top"] = 20
	vis["g4_5"].(map[string]any)["hide_last"] = false
	vis["g6_9"].(map[string]any)["around"] = 5
	d["analytics_min_cell_size"] = 2
	vs := check(t, gameconfig.TypeRanking, d)
	for _, f := range []string{"payload/leaderboard_visibility/g1_3", "payload/leaderboard_visibility/g4_5/top", "payload/leaderboard_visibility/g4_5/hide_last",
		"payload/leaderboard_visibility/g6_9/around", "payload/analytics_min_cell_size"} {
		wantField(t, vs, f)
	}
}

func TestConfigLeaderboardScopes(t *testing.T) {
	d := seedDoc(t, "leaderboard_definitions.json")
	defs := d["definitions"].([]any)
	defs[1].(map[string]any)["metric"] = "xp" // grade scope + xp
	wantField(t, check(t, gameconfig.TypeLeaderboards, d), "payload/definitions/1/metric")

	d = seedDoc(t, "leaderboard_definitions.json")
	defs = d["definitions"].([]any)
	defs[1].(map[string]any)["scope_type"] = "class"
	defs[2].(map[string]any)["metric"] = "class_goal"
	defs[3].(map[string]any)["grade_levels"] = []any{2, 3, 4}
	vs := check(t, gameconfig.TypeLeaderboards, d)
	wantField(t, vs, "payload/definitions/1/scope_type")
	wantField(t, vs, "payload/definitions/2/metric")
	wantField(t, vs, "payload/definitions/3/grade_levels")
}

func TestConfigTemplateParamsAndRewards(t *testing.T) {
	d := templateDoc(t, "grade_math_single_elim")
	d["kind"] = "class_challenge"
	d["allowed_params"] = []any{"subject_id", "reward", "chat_enabled"}
	d["tournament"].(map[string]any)["seeding_policy"] = "separate_class"
	d["reward_rule_refs"].([]any)[0].(map[string]any)["reward_ref"] = map[string]any{"coins": 100}
	vs := check(t, gameconfig.TypeChallengeTp, d)
	for _, f := range []string{"payload/kind", "payload/allowed_params/1", "payload/allowed_params/2", "payload/tournament/seeding_policy",
		"payload/reward_rule_refs/0/reward_ref/coins"} {
		wantField(t, vs, f)
	}
	// a school tournament template must be school_tournament
	wantField(t, check(t, gameconfig.TypeTournamentTp, templateDoc(t, "grade_round_robin")), "payload/kind")
}

func TestConfigCompetitionRewards(t *testing.T) {
	season := map[string]any{"schema_version": 1, "applies_to": "season", "name": "Mùa 1", "rules": []any{
		map[string]any{"rule_id": "t6", "tier_code": "T6", "reward_ref": map[string]any{"cosmetic_code": "cosmetic.season1_crown"}},
		map[string]any{"rule_id": "top1", "rank_min": 1, "rank_max": 1, "reward_ref": map[string]any{"title_code": "title.champion"}},
		map[string]any{"rule_id": "xu", "tier_code": "T1", "reward_ref": map[string]any{"currency": "xu"}},
	}}
	vs := check(t, gameconfig.TypeRewards, season)
	wantField(t, vs, "payload/rules/1/tier_code")
	wantField(t, vs, "payload/rules/2/reward_ref/currency")
	if len(check(t, gameconfig.TypeRewards, map[string]any{"schema_version": 1, "applies_to": "tournament", "name": "Giải", "rules": []any{
		map[string]any{"rule_id": "champion", "rank_min": 1, "rank_max": 1, "reward_ref": map[string]any{"badge_code": "badge.champion"}},
		map[string]any{"rule_id": "join", "min_matches": 1, "reward_ref": map[string]any{"badge_code": "badge.participant"}},
	}})) != 0 {
		t.Fatal("a valid tournament reward is refused")
	}
}

func TestConfigSeasonOverlapAndHistory(t *testing.T) {
	d := map[string]any{"schema_version": 1, "seasons": []any{
		map[string]any{"season_id": "01936a00-0000-7000-8000-000000106001", "code": "pre-season", "name": "Khởi động", "starts_at": "2026-01-01T00:00:00+07:00", "ends_at": "2099-12-31T00:00:00+07:00"},
		map[string]any{"season_id": "01936a00-0000-7000-8000-000000106002", "code": "2027-s1", "name": "Mùa 1", "starts_at": "2027-01-04T00:00:00+07:00", "ends_at": "2027-03-01T00:00:00+07:00"},
	}}
	wantField(t, check(t, gameconfig.TypeSeasons, d), "payload/seasons/1/starts_at")

	d["seasons"].([]any)[0].(map[string]any)["ends_at"] = "2027-01-04T00:00:00+07:00"
	if vs := check(t, gameconfig.TypeSeasons, d); len(vs) != 0 {
		t.Fatalf("back-to-back seasons: %+v", vs)
	}
	raw, _ := json.Marshal(d)
	existing := []gameconfig.SeasonState{{Code: "pre-season", Status: "active", StartsAt: mustTime(t, "2026-01-01T00:00:00+07:00"), EndsAt: mustTime(t, "2099-12-31T00:00:00+07:00")}}
	if vs := gameconfig.CheckSeasonChanges(raw, existing, mustTime(t, "2026-12-01T00:00:00Z")); len(vs) != 0 {
		t.Fatalf("shortening the active season to a future end: %+v", vs)
	}
	vs := gameconfig.CheckSeasonChanges(raw, existing, mustTime(t, "2027-02-01T00:00:00Z"))
	wantField(t, vs, "payload/seasons/1/starts_at") // a new season starting before the effective time
	wantField(t, vs, "payload/seasons/0/ends_at")   // the active season would end in the past
	ended := []gameconfig.SeasonState{{Code: "pre-season", Status: "ended", StartsAt: mustTime(t, "2026-01-01T00:00:00+07:00"), EndsAt: mustTime(t, "2026-06-01T00:00:00+07:00")}}
	if vs := gameconfig.CheckSeasonChanges(raw, ended, mustTime(t, "2026-12-01T00:00:00Z")); len(vs) == 0 || !strings.HasSuffix(vs[0].Field, "/ends_at") {
		t.Fatalf("an ended season changed its period: %+v", vs)
	}
}
