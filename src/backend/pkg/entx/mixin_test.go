package entx_test

import (
	"regexp"
	"strings"
	"testing"

	"entgo.io/ent/dialect/sql/schema"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent/migrate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/fixture"
)

func cols(t *schema.Table) map[string]*schema.Column {
	m := map[string]*schema.Column{}
	for _, c := range t.Columns {
		m[c.Name] = c
	}
	return m
}

// F003-AC-01: each mixin yields exactly the columns of its table kind (BACKEND_DESIGN §5.2, ADR-0089 §2).
func TestMixinColumns(t *testing.T) {
	soft := []string{"updated_at", "updated_by", "deleted_at", "deleted_by", "delete_reason", "version"}
	cases := []struct {
		table          *schema.Table
		want           []string
		tenantNullable bool
		hasTenant      bool
	}{
		{migrate.WidgetsTable, append([]string{"id", "tenant_id", "created_at", "created_by", "code", "name"}, soft...), false, true},
		{migrate.WidgetLogsTable, []string{"id", "tenant_id", "created_at", "created_by", "message"}, false, true},
		{migrate.TemplatesTable, append([]string{"id", "tenant_id", "created_at", "created_by", "name"}, soft...), true, true},
		{migrate.UnitsTable, append([]string{"id", "created_at", "created_by", "symbol"}, soft...), false, false},
	}
	for _, c := range cases {
		got := cols(c.table)
		if len(got) != len(c.want) {
			t.Errorf("%s: columns %d, want %d", c.table.Name, len(got), len(c.want))
		}
		for _, w := range c.want {
			if got[w] == nil {
				t.Errorf("%s: missing column %s", c.table.Name, w)
			}
		}
		if tc := got["tenant_id"]; c.hasTenant && tc.Nullable != c.tenantNullable {
			t.Errorf("%s: tenant_id nullable=%v", c.table.Name, tc.Nullable)
		}
		if dr := got["delete_reason"]; dr != nil && dr.Size != 64 {
			t.Errorf("%s: delete_reason size %d, want 64", c.table.Name, dr.Size)
		}
	}
}

// The hand-written fixture DDL declares the same columns as the generated ent schema.
func TestFixtureMatchesEnt(t *testing.T) {
	createRe := regexp.MustCompile(`(?s)CREATE TABLE sample\.(\w+) \((.*?)\n\);`)
	ddl := map[string]string{}
	for _, m := range createRe.FindAllStringSubmatch(fixture.SQL, -1) {
		ddl[m[1]] = m[2]
	}
	for _, tbl := range migrate.Tables {
		body, ok := ddl[tbl.Name]
		if !ok {
			t.Errorf("fixture lacks table %s", tbl.Name)
			continue
		}
		for _, c := range tbl.Columns {
			if !regexp.MustCompile(`\b` + c.Name + `\b`).MatchString(body) {
				t.Errorf("fixture %s lacks column %s", tbl.Name, c.Name)
			}
		}
		if strings.Count(body, ",")+1 < len(tbl.Columns) {
			t.Errorf("fixture %s has fewer columns than ent", tbl.Name)
		}
	}
}
