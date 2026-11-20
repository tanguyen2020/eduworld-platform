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
}

func schemas() map[string]string {
	c := ent.DefaultSchemaConfig
	return map[string]string{"game_sessions": c.GameSession, "login_events": c.LoginEvent, "game_profiles": c.GameProfile,
		"nickname_changes": c.NicknameChange, "avatar_presets": c.AvatarPreset, "character_catalogs": c.CharacterCatalog,
		"character_catalog_versions": c.CharacterCatalogVersion, "item_definitions": c.ItemDefinition,
		"item_definition_versions": c.ItemDefinitionVersion, "inventory_items": c.InventoryItem}
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
		want, err := entx.DDL(tablesOf(t, names), func(table string) string { return sc[table] })
		if err != nil {
			t.Fatal(err)
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
