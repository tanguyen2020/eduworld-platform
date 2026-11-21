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

// AddedDDL renders the ALTER TABLE … ADD COLUMN and CREATE INDEX statements of columns and indexes that a
// later migration adds to an ent table created by an earlier one (expand step, AGENTS R12). Columns and
// indexes are rendered in the order given, with the same types and defaults as DDL; a NOT NULL column
// needs a literal default so existing rows stay valid.
func AddedDDL(t *schema.Table, columns, indexes []string, schemaOf func(table string) string) (string, error) {
	name := t.Name
	if s := schemaOf(t.Name); s != "" {
		name = s + "." + t.Name
	}
	var b strings.Builder
	for _, cn := range columns {
		var c *schema.Column
		for _, x := range t.Columns {
			if x.Name == cn {
				c = x
			}
		}
		if c == nil {
			return "", fmt.Errorf("entx: %s has no column %s", name, cn)
		}
		typ, err := pgType(c)
		if err != nil {
			return "", fmt.Errorf("entx: %s.%s: %w", name, c.Name, err)
		}
		fmt.Fprintf(&b, "ALTER TABLE %s ADD COLUMN %s %s", name, c.Name, typ)
		d, hasDefault := literalDefault(c)
		if !c.Nullable {
			if !hasDefault {
				return "", fmt.Errorf("entx: %s.%s: an added NOT NULL column needs a literal default", name, c.Name)
			}
			b.WriteString(" NOT NULL")
		}
		if hasDefault {
			b.WriteString(" DEFAULT " + d)
		}
		b.WriteString(";\n")
	}
	for _, in := range indexes {
		var ix *schema.Index
		for _, x := range t.Indexes {
			if x.Name == in {
				ix = x
			}
		}
		if ix == nil {
			return "", fmt.Errorf("entx: %s has no index %s", name, in)
		}
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
	return b.String(), nil
}

// Without returns a copy of t without the given columns and indexes (the shape of the table before a
// later migration added them), for the golden test of the earlier migration.
func Without(t *schema.Table, columns, indexes []string) *schema.Table {
	drop := map[string]bool{}
	for _, n := range append(append([]string{}, columns...), indexes...) {
		drop[n] = true
	}
	c := &schema.Table{Name: t.Name, Schema: t.Schema, PrimaryKey: t.PrimaryKey, ForeignKeys: t.ForeignKeys, Annotation: t.Annotation}
	for _, x := range t.Columns {
		if !drop[x.Name] {
			c.Columns = append(c.Columns, x)
		}
	}
	c.Indexes = nil
	for _, x := range t.Indexes {
		if !drop[x.Name] {
			c.Indexes = append(c.Indexes, x)
		}
	}
	return c
}

// WithoutTables returns migration with the CREATE TABLE statement (and the CREATE INDEX statements that follow it)
// of each given schema-qualified table removed from its entddl block: the tables a later contract migration dropped
// on purpose (ADR-0095 mục 8), which no longer exist in the ent schema. The applied migration itself never changes
// (AGENTS R12); the golden test compares the rest of the block with the ent schema.
func WithoutTables(migration string, tables ...string) string {
	m := strings.ReplaceAll(migration, "\r\n", "\n")
	const begin, end = "-- BEGIN entddl\n", "-- END entddl"
	i := strings.Index(m, begin)
	j := strings.Index(m, end)
	if i < 0 || j < i {
		return m
	}
	drop := map[string]bool{}
	for _, t := range tables {
		drop["CREATE TABLE "+t+" ("] = true
	}
	chunks := strings.Split(strings.TrimRight(m[i+len(begin):j], "\n"), "\n\n")
	kept := make([]string, 0, len(chunks))
	for _, c := range chunks {
		first, _, _ := strings.Cut(c, "\n")
		if !drop[first] {
			kept = append(kept, c)
		}
	}
	block := strings.Join(kept, "\n\n")
	if block != "" {
		block += "\n"
	}
	return m[:i+len(begin)] + block + m[j:]
}
