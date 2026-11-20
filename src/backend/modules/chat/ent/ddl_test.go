package ent_test

import (
	"flag"
	"os"
	"testing"

	"entgo.io/ent/dialect/sql/schema"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/ent/migrate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// The generated block of each migration equals the DDL of the ent tables it introduces (AGENTS R27).
// go test ./modules/chat/ent -run TestMigrationMatchesEnt -args -print=<migration file> prints the block.
var printDDL = flag.String("print", "", "print the entddl block of this migration file")

// migrations lists, per migration file, the ent tables created in its entddl block (in order). Hand-written:
// the ref.* projections (DATABASE_DESIGN §2.17 rule 5) and the partitioned message tables (F115: composite keys).
var migrations = map[string][]string{
	"20261228000001_f095_social.sql":    {"friend_requests", "friendships", "friend_codes", "blocks", "mutes"},
	"20261229000001_f115_chat_core.sql": {"channels", "channel_members", "chat_policies"},
}

func schemas() map[string]string {
	c := ent.DefaultSchemaConfig
	return map[string]string{"friend_requests": c.FriendRequest, "friendships": c.Friendship, "friend_codes": c.FriendCode, "blocks": c.Block, "mutes": c.Mute,
		"channels": c.Channel, "channel_members": c.ChannelMember, "chat_policies": c.ChatPolicy}
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
