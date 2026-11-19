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
	"20261119000001_f010_org.sql": {"academic_years", "classes", "countries", "enrollments", "external_identities", "provinces",
		"province_successions", "schools", "students", "tenants", "users"},
	"20261120000001_f011_sis.sql": {"reconciliation_issues", "sis_connection_schools", "sis_connections", "sis_identity_links",
		"sis_imports", "sis_links", "student_login_keys", "sync_runs", "synced_students"},
	"20261121000001_f012_authz.sql":      {"permissions", "portal_password_reset_tokens", "role_permissions", "roles", "user_roles"},
	"20261122000001_f013_game_login.sql": {"game_auth_sessions"},
}

func schemas() map[string]string {
	c := ent.DefaultSchemaConfig
	return map[string]string{
		"tenants": c.Tenant, "countries": c.Country, "provinces": c.Province, "province_successions": c.ProvinceSuccession,
		"schools": c.School, "academic_years": c.AcademicYear, "classes": c.Class, "students": c.Student,
		"enrollments": c.Enrollment, "users": c.User, "external_identities": c.ExternalIdentity,
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
