package entx_test

import (
	"strings"
	"testing"

	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/schema/field"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent/migrate"
)

// F010 T005: PK names a key column other than "id", as UUID or varchar (DATABASE_DESIGN keys).
func TestMixinPK(t *testing.T) {
	uuidPK := entx.Base{PK: entx.PK{Column: "school_id"}}.Fields()[0].Descriptor()
	if uuidPK.Name != "id" || uuidPK.StorageKey != "school_id" || uuidPK.Info.Type != field.TypeUUID || uuidPK.Default == nil {
		t.Fatalf("uuid pk = %+v", uuidPK)
	}
	strPK := entx.GlobalBase{PK: entx.PK{Column: "country_code", StringLen: 2}}.Fields()[0].Descriptor()
	if strPK.StorageKey != "country_code" || strPK.Info.Type != field.TypeString || strPK.Size != 2 || strPK.Default != nil {
		t.Fatalf("string pk = %+v", strPK)
	}
	def := entx.TenantAppendOnly{}.Fields()[0].Descriptor()
	if def.StorageKey != "" || def.Name != "id" {
		t.Fatalf("default pk = %+v", def)
	}
}

func TestDDL(t *testing.T) {
	out, err := entx.DDL(migrate.Tables, func(string) string { return "sample" })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"CREATE TABLE sample.widgets (\n  id uuid NOT NULL,\n  tenant_id uuid NOT NULL,",
		"  version bigint NOT NULL DEFAULT 1,",
		"  code varchar(32) NOT NULL,",
		"  PRIMARY KEY (id)\n);",
		"CREATE UNIQUE INDEX widget_tenant_id_code ON sample.widgets (tenant_id, code) WHERE deleted_at IS NULL;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DDL lacks %q:\n%s", want, out)
		}
	}
	bad := &schema.Table{Name: "t", Columns: []*schema.Column{{Name: "s", Type: field.TypeString}}}
	bad.PrimaryKey = bad.Columns
	if _, err := entx.DDL([]*schema.Table{bad}, func(string) string { return "" }); err == nil {
		t.Fatal("string without MaxLen must fail (ADR-0041)")
	}
	dated := &schema.Table{Name: "d", Columns: []*schema.Column{
		{Name: "id", Type: field.TypeUUID},
		{Name: "day", Type: field.TypeTime, Nullable: true, SchemaType: map[string]string{"postgres": "date"}},
		{Name: "flag", Type: field.TypeBool, Default: true},
		{Name: "kind", Type: field.TypeString, Size: 8, Default: "it's"},
	}, Indexes: []*schema.Index{{Name: "d_day_idx", Annotation: &entsql.IndexAnnotation{Where: "day IS NOT NULL"}}}}
	dated.PrimaryKey = dated.Columns[:1]
	dated.Indexes[0].Columns = dated.Columns[1:2]
	out, err = entx.DDL([]*schema.Table{dated}, func(string) string { return "x" })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"  day date,", "  flag boolean NOT NULL DEFAULT true,", "  kind varchar(8) NOT NULL DEFAULT 'it''s',",
		"CREATE INDEX d_day_idx ON x.d (day) WHERE day IS NOT NULL;"} {
		if !strings.Contains(out, want) {
			t.Errorf("DDL lacks %q:\n%s", want, out)
		}
	}
}

func TestCheckDDLBlock(t *testing.T) {
	m := "-- header\r\n-- BEGIN entddl\r\nCREATE TABLE a ();\r\n-- END entddl\r\nALTER TABLE a;\r\n"
	if ok, _ := entx.CheckDDLBlock(m, "CREATE TABLE a ();\n"); !ok {
		t.Fatal("matching block (CRLF) must pass")
	}
	if ok, got := entx.CheckDDLBlock(m, "CREATE TABLE b ();\n"); ok || got != "CREATE TABLE a ();\n" {
		t.Fatalf("different block: ok=%v got=%q", ok, got)
	}
	if ok, _ := entx.CheckDDLBlock("no markers", ""); ok {
		t.Fatal("missing markers must fail")
	}
}

func TestWithoutTables(t *testing.T) {
	m := "-- BEGIN entddl\r\nCREATE TABLE s.a (\r\n  x int\r\n);\r\nCREATE INDEX a_idx ON s.a (x);\r\n\r\nCREATE TABLE s.b (\r\n  y int\r\n);\r\n\r\n" +
		"CREATE TABLE s.c (\r\n  z int\r\n);\r\n-- END entddl\r\nALTER TABLE s.b;\r\n"
	got := entx.WithoutTables(m, "s.b")
	want := "-- BEGIN entddl\nCREATE TABLE s.a (\n  x int\n);\nCREATE INDEX a_idx ON s.a (x);\n\nCREATE TABLE s.c (\n  z int\n);\n-- END entddl\nALTER TABLE s.b;\n"
	if got != want {
		t.Fatalf("WithoutTables:\n%q\nwant\n%q", got, want)
	}
	if ok, _ := entx.CheckDDLBlock(entx.WithoutTables(m, "s.a", "s.b", "s.c"), ""); !ok {
		t.Fatal("dropping every table leaves an empty block")
	}
	if got := entx.WithoutTables("no markers", "s.a"); got != "no markers" {
		t.Fatalf("no markers: %q", got)
	}
}
