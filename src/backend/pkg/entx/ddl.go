package entx

import (
	"fmt"
	"sort"
	"strings"

	"entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/schema/field"
)

// DDL renders the PostgreSQL CREATE TABLE / CREATE INDEX statements of ent tables (the generated
// migrate.Tables of a module) in a stable form. Modules embed the output between the markers
// "-- BEGIN entddl" / "-- END entddl" of a hand-written migration and keep it identical to the ent
// schema with a golden test (CheckDDLBlock); CHECK constraints, foreign keys, RLS, expression
// indexes and views are written by hand after the block (ADR-0089 §6, AGENTS R27).
// schemaOf returns the PostgreSQL schema of a table (the module's ent DefaultSchemaConfig, since the
// sql/schemaconfig feature keeps schema names out of migrate.Tables).
func DDL(tables []*schema.Table, schemaOf func(table string) string) (string, error) {
	var b strings.Builder
	for _, t := range tables {
		name := t.Name
		if s := schemaOf(t.Name); s != "" {
			name = s + "." + t.Name
		}
		fmt.Fprintf(&b, "CREATE TABLE %s (\n", name)
		lines := make([]string, 0, len(t.Columns)+1)
		for _, c := range t.Columns {
			typ, err := pgType(c)
			if err != nil {
				return "", fmt.Errorf("entx: %s.%s: %w", name, c.Name, err)
			}
			l := fmt.Sprintf("  %s %s", c.Name, typ)
			if !c.Nullable {
				l += " NOT NULL"
			}
			if d, ok := literalDefault(c); ok {
				l += " DEFAULT " + d
			}
			lines = append(lines, l)
		}
		pk := make([]string, 0, len(t.PrimaryKey))
		for _, c := range t.PrimaryKey {
			pk = append(pk, c.Name)
		}
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pk, ", ")))
		b.WriteString(strings.Join(lines, ",\n"))
		b.WriteString("\n);\n")
		idx := append([]*schema.Index(nil), t.Indexes...)
		sort.Slice(idx, func(i, j int) bool { return idx[i].Name < idx[j].Name })
		for _, ix := range idx {
			cols := make([]string, 0, len(ix.Columns))
			for _, c := range ix.Columns {
				cols = append(cols, c.Name)
			}
			kind := "INDEX"
			if ix.Unique {
				kind = "UNIQUE INDEX"
			}
			fmt.Fprintf(&b, "CREATE %s %s ON %s (%s)", kind, ix.Name, name, strings.Join(cols, ", "))
			if ix.Annotation != nil && ix.Annotation.Where != "" {
				b.WriteString(" WHERE " + ix.Annotation.Where)
			}
			b.WriteString(";\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func pgType(c *schema.Column) (string, error) {
	if st, ok := c.SchemaType["postgres"]; ok {
		return st, nil
	}
	switch c.Type {
	case field.TypeUUID:
		return "uuid", nil
	case field.TypeString:
		if c.Size <= 0 {
			return "", fmt.Errorf("string without MaxLen (ADR-0041)")
		}
		return fmt.Sprintf("varchar(%d)", c.Size), nil
	case field.TypeTime:
		return "timestamptz", nil
	case field.TypeJSON:
		return "jsonb", nil
	case field.TypeBool:
		return "boolean", nil
	case field.TypeInt16:
		return "smallint", nil
	case field.TypeInt32:
		return "integer", nil
	case field.TypeInt, field.TypeInt64:
		return "bigint", nil
	case field.TypeBytes:
		return "bytea", nil
	default:
		return "", fmt.Errorf("unsupported ent type %s", c.Type)
	}
}

// literalDefault returns the SQL default of string/bool/integer defaults declared in the ent schema
// (function defaults such as ids.New or time.Now stay in Go).
func literalDefault(c *schema.Column) (string, bool) {
	switch v := c.Default.(type) {
	case nil:
		return "", false
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'", true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case int, int16, int32, int64:
		return fmt.Sprintf("%d", v), true
	default:
		return "", false
	}
}

// CheckDDLBlock reports whether migration contains exactly want between "-- BEGIN entddl" and
// "-- END entddl" (line endings normalized). It returns the block found for diagnostics.
func CheckDDLBlock(migration, want string) (bool, string) {
	m := strings.ReplaceAll(migration, "\r\n", "\n")
	const begin, end = "-- BEGIN entddl\n", "-- END entddl"
	i := strings.Index(m, begin)
	j := strings.Index(m, end)
	if i < 0 || j < i {
		return false, ""
	}
	got := m[i+len(begin) : j]
	return got == want, got
}
