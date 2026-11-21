package ent_test

import (
	"flag"
	"os"
	"testing"

	"entgo.io/ent/dialect/sql/schema"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/migrate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// The generated block of each migration equals the DDL of the ent tables it introduces (AGENTS R27).
// go test ./modules/identity/ent -run TestMigrationMatchesEnt -print=<migration file> prints the block.
var printDDL = flag.String("print", "", "print the entddl block of this migration file")

// migrations lists, per migration file, the ent tables created in its entddl block (in order). Tables
// created by hand (partitioned sis.staging_records, sis.sync_record_errors) are not listed.
var migrations = map[string][]string{
	"20261119000001_f010_org.sql": {"academic_years", "countries", "external_identities", "provinces",
		"province_successions", "schools", "students", "tenants", "users"},
	"20261120000001_f011_sis.sql": {"reconciliation_issues", "sis_connection_schools", "sis_connections", "sis_identity_links",
		"sis_imports", "sis_links", "student_login_keys", "sync_runs", "synced_students"},
	"20261121000001_f012_authz.sql":      {"permissions", "portal_password_reset_tokens", "role_permissions", "roles", "user_roles"},
	"20261122000001_f013_game_login.sql": {"game_auth_sessions"},
}

// added lists, per migration file, the columns and indexes it adds to ent tables created by an earlier
// migration (ALTER TABLE … ADD COLUMN in its entddl block); earlier blocks are checked without them.
// dropped lists, per migration, the tables of its entddl block that a later contract migration dropped on purpose
// (ADR-0095 mục 8: org.classes and org.enrollments, 20261201000001_adr0095_drop_class_enrollment.sql); they are no
// longer in the ent schema and are left out of the comparison.
var dropped = map[string][]string{
	"20261119000001_f010_org.sql": {"org.classes", "org.enrollments"},
}

type addition struct {
	table            string
	columns, indexes []string
}

var added = map[string][]addition{
	"20261124000001_f017_sis_contract.sql": {
		{"sis_connections", []string{"webhook_secret_ref", "capabilities", "last_health_at", "last_success_at", "consecutive_failed_runs", "status_reason"},
			[]string{"sis_connections_schedule_idx"}},
		{"sync_runs", []string{"requested_by", "delivery_id"}, nil},
	},
}

// withoutAdded drops from tables the columns/indexes added by migrations other than file.
func withoutAdded(file string, tbs []*schema.Table) []*schema.Table {
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
	return map[string]string{
		"tenants": c.Tenant, "countries": c.Country, "provinces": c.Province, "province_successions": c.ProvinceSuccession,
		"schools": c.School, "academic_years": c.AcademicYear, "students": c.Student,
		"users": c.User, "external_identities": c.ExternalIdentity,
		"sis_connections": c.SisConnection, "sis_connection_schools": c.SisConnectionSchool, "sync_runs": c.SyncRun,
		"staging_records": c.StagingRecord, "sync_record_errors": c.SyncRecordError, "sis_imports": c.SisImport,
		"sis_links": c.SisLink, "synced_students": c.SyncedStudent, "student_login_keys": c.StudentLoginKey,
		"reconciliation_issues": c.ReconciliationIssue, "sis_identity_links": c.SisIdentityLink,
		"permissions": c.Permission, "roles": c.Role, "role_permissions": c.RolePermission, "user_roles": c.UserRole,
		"portal_password_reset_tokens": c.PortalPasswordResetToken, "login_events": c.LoginEvent, "game_auth_sessions": c.GameAuthSession,
	}
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
	listed := map[string]bool{"staging_records": true, "sync_record_errors": true, "login_events": true} // hand-written, partitioned
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
	files := map[string]bool{}
	for f := range migrations {
		files[f] = true
	}
	for f := range added {
		files[f] = true
	}
	for file := range files {
		want, err := entx.DDL(withoutAdded(file, tablesOf(t, migrations[file])), func(table string) string { return sc[table] })
		if err != nil {
			t.Fatal(err)
		}
		if len(migrations[file]) == 0 {
			want = ""
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
		if ok, got := entx.CheckDDLBlock(entx.WithoutTables(string(b), dropped[file]...), want); !ok {
			t.Fatalf("entddl block of %s differs from the ent schema; regenerate with -print=%s.\n--- want\n%s\n--- got\n%s", file, file, want, got)
		}
	}
}
