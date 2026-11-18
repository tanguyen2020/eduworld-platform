package ent_test

import (
	"flag"
	"os"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/migrate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// F010 T005: the generated block of the F010 migration equals the DDL of the ent schema (AGENTS R27).
// go test ./modules/identity/ent -run TestMigrationMatchesEnt -print prints the block to paste.
var printDDL = flag.Bool("print", false, "print the entddl block")

const migration = "../migrations/20261119000001_f010_org.sql"

func TestMigrationMatchesEnt(t *testing.T) {
	c := ent.DefaultSchemaConfig
	schemas := map[string]string{
		"tenants": c.Tenant, "countries": c.Country, "provinces": c.Province, "province_successions": c.ProvinceSuccession,
		"schools": c.School, "academic_years": c.AcademicYear, "classes": c.Class, "students": c.Student,
		"enrollments": c.Enrollment, "users": c.User, "external_identities": c.ExternalIdentity,
	}
	for _, tb := range migrate.Tables {
		if schemas[tb.Name] == "" {
			t.Fatalf("table %s has no schema in the test map", tb.Name)
		}
	}
	want, err := entx.DDL(migrate.Tables, func(table string) string { return schemas[table] })
	if err != nil {
		t.Fatal(err)
	}
	if *printDDL {
		_, _ = os.Stdout.WriteString(want)
		return
	}
	b, err := os.ReadFile(migration)
	if err != nil {
		t.Fatal(err)
	}
	if ok, got := entx.CheckDDLBlock(string(b), want); !ok {
		t.Fatalf("entddl block of %s differs from the ent schema; regenerate with -print.\n--- want\n%s\n--- got\n%s", migration, want, got)
	}
}
