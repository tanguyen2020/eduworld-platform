package ent_test

import (
	"flag"
	"os"
	"testing"

	"entgo.io/ent/dialect/sql/schema"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/ent/migrate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// The generated block of each migration equals the DDL of the ent tables it introduces (AGENTS R27).
// go test ./modules/game/ent -run TestMigrationMatchesEnt -args -print=<migration file> prints the block.
var printDDL = flag.String("print", "", "print the entddl block of this migration file")

// migrations lists, per migration file, the ent tables created in its entddl block (in order). Hand-written:
// game.login_events (partitioned, composite key), game.content_manifests (composite key (tenant_id,
// manifest_version), written by the content release of F026/F078) and the ref.* projections (DATABASE_DESIGN
// §2.17 minimal columns, no entx mixin — same decision as F020).
var migrations = map[string][]string{
	"20261123000001_f014_game_session.sql": {"game_sessions"},
	"20261224000001_f070_profile.sql": {"game_profiles", "nickname_changes", "avatar_presets", "character_catalogs",
		"character_catalog_versions", "item_definitions", "item_definition_versions", "inventory_items"},
	"20261226000001_f071_world.sql": {"world_areas", "world_area_versions", "story_chapters", "story_chapter_versions",
		"npc_dialogues", "npc_dialogue_versions", "level_definitions", "level_versions", "content_releases", "level_sessions",
		"world_area_unlocks", "exploration_progress", "story_progress", "world_unlock_facts"},
	"20261227000001_f072_quest.sql": {"quest_definitions", "quest_versions", "quest_progress", "quest_step_attempts",
		"periodic_quest_grants", "tenant_resource_assignments"},
	"20261228000001_f074_progression.sql": {"xp_ledger", "xp_daily_caps", "reward_rules", "reward_rule_versions", "reward_grants",
		"progression_curves", "progression_curve_versions", "progression_levels", "level_history"},
	"20261229000001_f075_economy.sql": {"currencies", "wallets", "currency_ledger", "item_ledger", "shop_items", "shop_item_versions", "shop_purchases", "collections", "collection_versions", "collection_items", "collection_progress", "achievements", "achievement_versions", "student_achievements", "achievement_progress_events", "economy_adjustments"},
	"20261230000770_f077_liveops.sql": {"events", "event_versions", "event_templates", "event_template_versions", "event_runs",
		"event_participation", "event_point_ledger", "community_goals", "community_goal_contributions", "community_goal_progress", "season_tracks",
		"season_track_versions", "season_point_ledger", "season_track_progress"},
	"20261230000773_f073_minigame.sql":  {"minigame_configs", "minigame_config_versions", "minigame_plays"},
	"20261231000760_f076_challenge.sql": {"challenges", "challenge_versions", "challenge_attempts"},
	"20261231000780_f078_gameadmin.sql": {"config_items", "config_versions", "scheduled_publications", "preview_sessions", "render_jobs",
		"media_assets", "media_usages", "client_manifests"},
	"20270101000103_f103_boss_coop.sql": {"boss_definitions", "boss_versions", "boss_encounters", "boss_encounter_members",
		"boss_encounter_contributions", "boss_phase_plans", "boss_answer_results", "party_quest_sessions", "party_quest_contributions"},
}

// added lists, per migration file, the columns and indexes it adds to ent tables created by an earlier migration
// (ALTER TABLE … ADD COLUMN in its entddl block); earlier blocks are checked without them.
type addition struct {
	table            string
	columns, indexes []string
}

var added = map[string][]addition{
	"20261228000001_f074_progression.sql": {{"game_profiles", []string{"grade_transition_year"}, nil}},
	"20261229000001_f075_economy.sql": {{"item_definition_versions", []string{"stackable", "max_stack", "payload", "payload_sha256", "change_note",
		"snapshot_key", "snapshot_sha256", "published_at", "published_by"}, nil}},
}

// before returns tables as file created them: without the columns/indexes added by later migrations.
func before(file string, tbs []*schema.Table) []*schema.Table {
	out := make([]*schema.Table, 0, len(tbs))
	for _, tb := range tbs {
		for f, as := range added {
			for _, a := range as {
				if f != file && a.table == tb.Name {
					tb = entx.Without(tb, a.columns, a.indexes)
				}
			}
		}
		out = append(out, tb)
	}
	return out
}

func schemas() map[string]string {
	c := ent.DefaultSchemaConfig
	return map[string]string{"game_sessions": c.GameSession, "login_events": c.LoginEvent, "game_profiles": c.GameProfile,
		"nickname_changes": c.NicknameChange, "avatar_presets": c.AvatarPreset, "character_catalogs": c.CharacterCatalog,
		"character_catalog_versions": c.CharacterCatalogVersion, "item_definitions": c.ItemDefinition,
		"item_definition_versions": c.ItemDefinitionVersion, "inventory_items": c.InventoryItem,
		"world_areas": c.WorldArea, "world_area_versions": c.WorldAreaVersion, "story_chapters": c.StoryChapter,
		"story_chapter_versions": c.StoryChapterVersion, "npc_dialogues": c.NpcDialogue, "npc_dialogue_versions": c.NpcDialogueVersion,
		"level_definitions": c.LevelDefinition, "level_versions": c.LevelVersion, "content_releases": c.ContentRelease,
		"level_sessions": c.LevelSession, "world_area_unlocks": c.WorldAreaUnlock, "exploration_progress": c.ExplorationProgress,
		"story_progress": c.StoryProgress, "world_unlock_facts": c.WorldUnlockFact,
		"quest_definitions": c.QuestDefinition, "quest_versions": c.QuestVersion, "quest_progress": c.QuestProgress,
		"quest_step_attempts": c.QuestStepAttempt, "periodic_quest_grants": c.PeriodicQuestGrant,
		"minigame_configs": c.MiniGameConfig, "minigame_config_versions": c.MiniGameConfigVersion, "minigame_plays": c.MiniGamePlay,
		"tenant_resource_assignments": c.TenantResourceAssignment, "xp_ledger": c.XpLedgerEntry, "xp_daily_caps": c.XpDailyCap,
		"reward_rules": c.RewardRule, "reward_rule_versions": c.RewardRuleVersion, "reward_grants": c.RewardGrant,
		"progression_curves": c.ProgressionCurve, "progression_curve_versions": c.ProgressionCurveVersion,
		"progression_levels": c.ProgressionLevel, "level_history": c.LevelHistory,
		"currencies": c.Currency, "wallets": c.Wallet, "currency_ledger": c.CurrencyLedgerEntry, "item_ledger": c.ItemLedgerEntry, "shop_items": c.ShopItem, "shop_item_versions": c.ShopItemVersion, "shop_purchases": c.ShopPurchase, "collections": c.Collection, "collection_versions": c.CollectionVersion, "collection_items": c.CollectionItem, "collection_progress": c.CollectionProgress, "achievements": c.Achievement, "achievement_versions": c.AchievementVersion, "student_achievements": c.StudentAchievement, "achievement_progress_events": c.AchievementProgressEvent, "economy_adjustments": c.EconomyAdjustment,
		"events": c.Event, "event_versions": c.EventVersion, "event_templates": c.EventTemplate, "event_template_versions": c.EventTemplateVersion,
		"event_runs": c.EventRun, "event_participation": c.EventParticipation, "event_point_ledger": c.EventPointEntry,
		"community_goals": c.CommunityGoal, "community_goal_contributions": c.CommunityGoalContribution,
		"community_goal_progress": c.CommunityGoalProgress, "season_tracks": c.SeasonTrack, "season_track_versions": c.SeasonTrackVersion,
		"season_point_ledger": c.SeasonPointEntry, "season_track_progress": c.SeasonTrackProgress,
		"challenges": c.Challenge, "challenge_versions": c.ChallengeVersion, "challenge_attempts": c.ChallengeAttempt,
		"config_items": c.GameConfigItem, "config_versions": c.GameConfigVersion, "scheduled_publications": c.ScheduledPublication,
		"preview_sessions": c.PreviewSession, "render_jobs": c.RenderJob, "media_assets": c.MediaAsset, "media_usages": c.MediaUsage,
		"client_manifests": c.ClientManifest,
		"boss_definitions": c.BossDefinition, "boss_versions": c.BossVersion, "boss_encounters": c.BossEncounter,
		"boss_encounter_members": c.BossEncounterMember, "boss_encounter_contributions": c.BossEncounterContribution,
		"boss_phase_plans": c.BossPhasePlan, "boss_answer_results": c.BossAnswerResult, "party_quest_sessions": c.PartyQuestSession,
		"party_quest_contributions": c.PartyQuestContribution}
}

func tablesOf(t *testing.T, names []string) []*schema.Table {
	t.Helper()
	byName := map[string]*schema.Table{}
	for _, tb := range migrate.Tables {
		byName[tb.Name] = tb
	}
	out := make([]*schema.Table, 0, len(names))
	for _, n := range names {
		tb, ok := byName[n]
		if !ok {
			t.Fatalf("no ent table %s", n)
		}
		out = append(out, tb)
	}
	return out
}

func TestMigrationMatchesEnt(t *testing.T) {
	sc := schemas()
	listed := map[string]bool{"login_events": true} // hand-written, partitioned
	for _, tbs := range migrations {
		for _, n := range tbs {
			listed[n] = true
		}
	}
	for _, tb := range migrate.Tables {
		if sc[tb.Name] == "" {
			t.Fatalf("table %s has no schema in the test map", tb.Name)
		}
		if !listed[tb.Name] {
			t.Errorf("ent table %s is created by no migration", tb.Name)
		}
	}
	for file, names := range migrations {
		want, err := entx.DDL(before(file, tablesOf(t, names)), func(table string) string { return sc[table] })
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range added[file] {
			alter, err := entx.AddedDDL(tablesOf(t, []string{a.table})[0], a.columns, a.indexes, func(table string) string { return sc[table] })
			if err != nil {
				t.Fatal(err)
			}
			want += alter
		}
		if *printDDL == file {
			_, _ = os.Stdout.WriteString(want)
			continue
		}
		if *printDDL != "" {
			continue
		}
		b, err := os.ReadFile("../migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if ok, got := entx.CheckDDLBlock(string(b), want); !ok {
			t.Fatalf("entddl block of %s differs from the ent schema; regenerate with -print=%s.\n--- want\n%s\n--- got\n%s", file, file, want, got)
		}
	}
}
