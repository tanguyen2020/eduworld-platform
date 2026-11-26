package ent_test

import (
	"flag"
	"os"
	"testing"

	"entgo.io/ent/dialect/sql/schema"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/ent/migrate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// The generated block of each migration equals the DDL of the ent tables it introduces (AGENTS R27).
// go test ./modules/competition/ent -run TestMigrationMatchesEnt -args -print=<migration file> prints the block.
var printDDL = flag.String("print", "", "print the entddl block of this migration file")

// migrations lists, per migration file, the ent tables created in its entddl block (in order). Hand-written:
// the ref.* projections (DATABASE_DESIGN §2.17 rule 5), competition.integrity_signals and multiplayer.party_histories
// (append-only, partitioned).
var migrations = map[string][]string{
	"20261225000001_f091_hub.sql":           {"rooms", "session_memberships"},
	"20261229000001_f100_party.sql":         {"parties", "party_members", "party_invitations", "party_policies"},
	"20261230000001_f101_lobby_session.sql": {"party_activities"},
	"20261231000001_f104_matchmaking.sql": {"matches", "match_participants", "duels", "competition_restrictions", "game_configs",
		"game_config_versions", "content_releases"},
	"20261231000003_f105_battle.sql": {"match_results"},
	"20261231000005_f106_ranking.sql": {"ratings", "rating_changes", "competition_point_ledger", "leaderboard_contributions",
		"leaderboard_definitions", "leaderboard_snapshots", "seasons", "season_results"},
	"20261231000006_f107_tournament.sql": {"challenges", "tournaments", "tournament_entries", "tournament_groups", "bracket_nodes",
		"tournament_results", "tournament_rewards", "tenant_resource_assignments"},
	"20261231000007_f108_config_console.sql": {},
}

// added lists, per migration file, the columns and indexes it adds to ent tables created by an earlier migration
// (ALTER TABLE … ADD COLUMN in its entddl block); earlier blocks are checked without them.
type addition struct {
	table            string
	columns, indexes []string
}

var added = map[string][]addition{
	"20261231000007_f108_config_console.sql": {
		{"game_config_versions", []string{"review_note", "published_by"}, nil},
		{"content_releases", []string{"effective_from"}, nil},
	},
	"20261231000003_f105_battle.sql": {
		{"matches", []string{"question_plan_id", "question_plan_version", "room_id", "end_reason"}, nil},
		{"match_participants", []string{"correct_count", "disconnects"}, nil},
		{"rooms", []string{"match_id"}, nil},
	},
	"20261230000001_f101_lobby_session.sql": {
		{"parties", []string{"selected_activity_type", "selected_activity_ref", "selected_game_mode"}, nil},
		{"session_memberships", []string{"party_activity_id"}, []string{"session_memberships_activity_idx"}},
	},
}

// relaxed lists columns made nullable by a later migration (ALTER COLUMN … DROP NOT NULL, hand-written outside the
// entddl block): the block of the file that created them still declares NOT NULL.
var relaxed = map[string]map[string][]string{
	"20261225000001_f091_hub.sql": {"session_memberships": {"game_session_id"}},
}

// before returns tables as file created them: without the columns/indexes added later, with the relaxed columns NOT NULL.
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
		if cols := relaxed[file][tb.Name]; len(cols) > 0 {
			tb = entx.Without(tb, nil, nil)
			for i, c := range tb.Columns {
				for _, n := range cols {
					if c.Name == n {
						cc := *c
						cc.Nullable = false
						tb.Columns[i] = &cc
					}
				}
			}
		}
		out = append(out, tb)
	}
	return out
}

func schemas() map[string]string {
	c := ent.DefaultSchemaConfig
	return map[string]string{"rooms": c.Room, "session_memberships": c.SessionMembership, "parties": c.Party,
		"party_members": c.PartyMember, "party_invitations": c.PartyInvitation, "party_policies": c.PartyPolicy,
		"party_activities": c.PartyActivity, "matches": c.Match, "match_participants": c.MatchParticipant, "duels": c.Duel,
		"competition_restrictions": c.CompetitionRestriction, "game_configs": c.GameConfig, "game_config_versions": c.GameConfigVersion,
		"content_releases": c.ContentRelease, "match_results": c.MatchResult, "ratings": c.Rating, "rating_changes": c.RatingChange,
		"competition_point_ledger": c.CompetitionPointEntry, "leaderboard_contributions": c.LeaderboardContribution,
		"leaderboard_definitions": c.LeaderboardDefinition, "leaderboard_snapshots": c.LeaderboardSnapshot, "seasons": c.Season,
		"season_results": c.SeasonResult, "challenges": c.Challenge, "tournaments": c.Tournament, "tournament_entries": c.TournamentEntry,
		"tournament_groups": c.TournamentGroup, "bracket_nodes": c.BracketNode, "tournament_results": c.TournamentResult,
		"tournament_rewards": c.TournamentReward, "tenant_resource_assignments": c.TenantResourceAssignment}
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
	listed := map[string]bool{}
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
		want := "" // a file that only adds columns (F108) has no CREATE TABLE part
		if len(names) > 0 {
			ddl, err := entx.DDL(before(file, tablesOf(t, names)), func(table string) string { return sc[table] })
			if err != nil {
				t.Fatal(err)
			}
			want = ddl
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
