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
	"20261230000001_f116_moderation.sql": {"sensitive_term_categories", "sensitive_terms", "sensitive_allowlist",
		"sensitive_catalog_versions", "tenant_category_settings", "moderation_switches", "moderation_switch_requests",
		"moderation_switch_changes", "sanctions", "quick_messages"},
	"20261231000001_f121_moderation_cases.sql": {"reports", "moderation_cases", "moderation_actions", "moderation_evidence",
		"school_safety_notifications"},
}

// added lists, per migration file, the columns and indexes it adds to ent tables created by an earlier migration
// (ALTER TABLE … ADD COLUMN in its entddl block); earlier blocks are checked without them.
type addition struct {
	table            string
	columns, indexes []string
}

var added = map[string][]addition{
	"20261231000001_f121_moderation_cases.sql": {
		{"sanctions", []string{"reason_code", "issued_by_kind", "revoked_by", "revoke_reason", "lifted_at"}, []string{"sanctions_expiry_idx"}},
	},
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
	return map[string]string{"friend_requests": c.FriendRequest, "friendships": c.Friendship, "friend_codes": c.FriendCode, "blocks": c.Block, "mutes": c.Mute,
		"channels": c.Channel, "channel_members": c.ChannelMember, "chat_policies": c.ChatPolicy,
		"sensitive_term_categories": c.SensitiveTermCategory, "sensitive_terms": c.SensitiveTerm,
		"sensitive_allowlist": c.SensitiveAllowlistEntry, "sensitive_catalog_versions": c.SensitiveCatalogVersion,
		"tenant_category_settings": c.TenantCategorySetting, "moderation_switches": c.ModerationSwitch,
		"moderation_switch_requests": c.ModerationSwitchRequest, "moderation_switch_changes": c.ModerationSwitchChange,
		"sanctions": c.Sanction, "quick_messages": c.QuickMessage, "reports": c.Report, "moderation_cases": c.ModerationCase,
		"moderation_actions": c.ModerationAction, "moderation_evidence": c.ModerationEvidence,
		"school_safety_notifications": c.SchoolSafetyNotification}
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
