// Package projection rebuilds ref.* projections from the source module's paginated facade (F003 T076,
// AC-39, DATABASE_DESIGN §2.17 rule 3, BACKUP_RECOVERY): pages are read through pkg/modcall and
// upserted by source_version so a newer row written concurrently by the event consumer is never
// overwritten; nothing is truncated, so a failed run leaves the projection as it was.
package projection

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Row is one projected record of the source.
type Row struct {
	Key           string // primary key value of the projection row
	SourceVersion int64
	Values        map[string]any // column → value (without key and source_version)
}

// FetchPage reads one page of the source facade (through modcall) after cursor; "" starts, next == ""
// ends.
type FetchPage func(ctx context.Context, cursor string) (rows []Row, next string, err error)

// UpsertPage writes a page into the projection (typically entx.UnitOfWork + Upsert).
type UpsertPage func(ctx context.Context, rows []Row) error

// Table registers the rebuild of one projection table.
type Table struct {
	Module string // receiving module (owner of the projection)
	Source string // source module (owner of the facade)
	Name   string // ref.<table>
	Fetch  FetchPage
	Upsert UpsertPage
}

// Registry of rebuildable tables.
type Registry struct {
	mu     sync.Mutex
	tables []Table
}

// Register adds a table; modules call it from their Register.
func (r *Registry) Register(t Table) error {
	if t.Module == "" || t.Source == "" || !strings.HasPrefix(t.Name, "ref.") || t.Fetch == nil || t.Upsert == nil {
		return errors.New("projection: table needs module, source, ref.<name>, Fetch and Upsert")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tables = append(r.tables, t)
	return nil
}

// Select returns the tables of module/source (and table when not empty) in registration order: a module registers a
// referenced table before the tables that reference it (ref.tenants before ref.schools and ref.students, FK), so a full
// rebuild of an empty projection never hits a missing parent.
func (r *Registry) Select(module, source, table string) []Table {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Table
	for _, t := range r.tables {
		if t.Module == module && t.Source == source && (table == "" || t.Name == table) {
			out = append(out, t)
		}
	}
	return out
}

// Stats of a rebuild.
type Stats struct {
	Pages, Rows int
}

// Rebuild runs every selected table; the first error stops the run (exit ≠ 0) with the projection
// untouched beyond the pages already upserted (upserts are idempotent; nothing is deleted).
func (r *Registry) Rebuild(ctx context.Context, module, source, table string) (map[string]Stats, error) {
	tables := r.Select(module, source, table)
	if len(tables) == 0 {
		return nil, fmt.Errorf("projection: nothing registered for module=%s source=%s table=%s", module, source, table)
	}
	out := map[string]Stats{}
	for _, t := range tables {
		var st Stats
		cursor := ""
		for {
			rows, next, err := t.Fetch(ctx, cursor)
			if err != nil {
				return out, fmt.Errorf("projection: %s page after %q: %w", t.Name, cursor, err)
			}
			if len(rows) > 0 {
				if err := t.Upsert(ctx, rows); err != nil {
					return out, fmt.Errorf("projection: %s upsert: %w", t.Name, err)
				}
			}
			st.Pages++
			st.Rows += len(rows)
			if next == "" {
				break
			}
			if next == cursor {
				return out, fmt.Errorf("projection: %s cursor did not advance", t.Name)
			}
			cursor = next
		}
		out[t.Name] = st
	}
	return out, nil
}

var identRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*(\.[a-z_][a-z0-9_]*)?$`)

// UpsertSQL returns INSERT … ON CONFLICT (key) DO UPDATE … WHERE source_version is older, for table with
// the given value columns (fixed order). Arguments: key, source_version, then columns.
func UpsertSQL(table, key string, columns []string) (string, error) {
	for _, id := range append([]string{table, key}, columns...) {
		if !identRe.MatchString(id) {
			return "", fmt.Errorf("projection: invalid identifier %q", id)
		}
	}
	cols := append([]string{key, "source_version"}, columns...)
	ph := make([]string, len(cols))
	set := []string{"source_version = EXCLUDED.source_version"}
	for i := range cols {
		ph[i] = fmt.Sprintf("$%d", i+1)
	}
	for _, c := range columns {
		set = append(set, c+" = EXCLUDED."+c)
	}
	short := table[strings.LastIndex(table, ".")+1:]
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s WHERE %s.source_version < EXCLUDED.source_version",
		table, strings.Join(cols, ", "), strings.Join(ph, ", "), key, strings.Join(set, ", "), short), nil
}

// Args orders the arguments of UpsertSQL for row.
func Args(row Row, columns []string) []any {
	out := []any{row.Key, row.SourceVersion}
	for _, c := range columns {
		out = append(out, row.Values[c])
	}
	return out
}
