// Command tenancyguard checks SQL against the tenancy rules (F003 T019, F003-AC-07, AGENTS R21/R27,
// ADR-0089):
//  1. no RLS policy references deleted_at of the protected table (migrations);
//  2. every table with tenant_id enables RLS, except schema infra (ADR-0089 §4);
//  3. no DELETE FROM / TRUNCATE outside the chat gameplay purge package (Go string literals, migrations);
//  4. explicit SQL in Go reading or updating a business table filters tenant_id and deleted_at.
//
// A Go literal may be exempted with a "tenancyguard:ok <reason>" comment on the line above.
//
//	go run ./tools/tenancyguard            # scans modules/ and pkg/
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	createRe   = regexp.MustCompile(`(?is)CREATE TABLE (?:IF NOT EXISTS )?([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)\s*\((.*?)\)\s*(?:PARTITION BY [^;]*)?;`)
	rlsRe      = regexp.MustCompile(`(?i)ALTER TABLE ([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*) ENABLE ROW LEVEL SECURITY`)
	policyRe   = regexp.MustCompile(`(?is)CREATE POLICY \w+ ON [a-z_][a-z0-9_.]*.*?;`)
	policyDel  = regexp.MustCompile(`(^|[^.a-z_])deleted_at`)
	deleteRe   = regexp.MustCompile(`(?i)\b(DELETE\s+FROM|TRUNCATE\s+(TABLE\s+)?[a-z_])`)
	deleteStmt = regexp.MustCompile(`(?im)^\s*(DELETE\s+FROM|TRUNCATE)\b`)
	tableUseRe = regexp.MustCompile(`(?i)\b(FROM|UPDATE|JOIN)\s+([a-z_][a-z0-9_]*)\.([a-z_][a-z0-9_]*)`)
	tenantCol  = regexp.MustCompile(`(^|[\s,(])tenant_id\s+uuid`)
	// The tenant registry (platform.tenants) has tenant_id as its primary key: a G table, not tenant-scoped.
	tenantPK  = regexp.MustCompile(`(?i)PRIMARY KEY\s*\(\s*tenant_id\s*\)|tenant_id\s+uuid\s+(NOT NULL\s+)?PRIMARY KEY`)
	commentRe = regexp.MustCompile(`--[^\n]*`)
)

// Schemas whose explicit SQL is exempt from rule 4 (no business rows): infra tables, Atlas, catalog.
var noTenantSchemas = map[string]bool{"infra": true, "atlas_schema_revisions": true, "pg_catalog": true, "information_schema": true}

func main() {
	var problems []string
	for _, root := range []string{"modules", "pkg"} {
		p, err := Scan(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tenancyguard:", err)
			os.Exit(1)
		}
		problems = append(problems, p...)
	}
	sort.Strings(problems)
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}

// Scan checks .sql migrations and Go sources under root.
func Scan(root string) ([]string, error) {
	var out []string
	sqlByDir := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".sql"):
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sqlByDir[filepath.Dir(path)] = append(sqlByDir[filepath.Dir(path)], path+"\x00"+string(b))
		case strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") && !strings.HasSuffix(path, "_gen.go"):
			p, err := scanGo(path)
			if err != nil {
				return err
			}
			out = append(out, p...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, files := range sqlByDir {
		out = append(out, scanMigrations(files)...)
	}
	return out, nil
}

func line(src string, idx int) int { return strings.Count(src[:idx], "\n") + 1 }

// scanMigrations checks one migration directory (rules 1–3).
func scanMigrations(files []string) []string {
	var out []string
	tables := map[string]string{} // table → file:line
	rls := map[string]bool{}
	for _, f := range files {
		path, raw, _ := strings.Cut(f, "\x00")
		src := commentRe.ReplaceAllStringFunc(raw, func(s string) string { return strings.Repeat(" ", len(s)) })
		for _, m := range createRe.FindAllStringSubmatchIndex(src, -1) {
			table, body := src[m[2]:m[3]], src[m[4]:m[5]]
			if tenantCol.MatchString(body) && !tenantPK.MatchString(body) && !strings.HasPrefix(table, "infra.") {
				tables[table] = fmt.Sprintf("%s:%d", path, line(src, m[0]))
			}
		}
		for _, m := range rlsRe.FindAllStringSubmatch(src, -1) {
			rls[m[1]] = true
		}
		for _, m := range policyRe.FindAllStringIndex(src, -1) {
			if policyDel.MatchString(src[m[0]:m[1]]) {
				out = append(out, fmt.Sprintf("%s:%d: RLS policy references deleted_at (ADR-0089 §1)", path, line(src, m[0])))
			}
		}
		if !strings.Contains(filepath.ToSlash(path), "/purge/") {
			for _, m := range deleteStmt.FindAllStringIndex(src, -1) {
				out = append(out, fmt.Sprintf("%s:%d: DELETE/TRUNCATE in a migration needs an ADR and a backup plan (R12, R21)", path, line(src, m[0])))
			}
		}
	}
	for t, pos := range tables {
		if !rls[t] {
			out = append(out, fmt.Sprintf("%s: table %s has tenant_id but no ENABLE ROW LEVEL SECURITY (ADR-0005, ADR-0089)", pos, t))
		}
	}
	return out
}

// scanGo checks string literals of one Go file (rules 3–4).
func scanGo(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	exempt := map[int]bool{}
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "tenancyguard:ok") {
				exempt[fset.Position(c.Pos()).Line+1] = true
				exempt[fset.Position(c.Pos()).Line] = true
			}
		}
	}
	purge := strings.Contains(filepath.ToSlash(path), "/internal/infrastructure/purge/")
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil || !looksLikeSQL(s) {
			return true
		}
		pos := fset.Position(lit.Pos())
		if exempt[pos.Line] {
			return true
		}
		if !purge && deleteRe.MatchString(s) {
			out = append(out, fmt.Sprintf("%s: DELETE/TRUNCATE in application code (soft-delete only, R21)", pos))
		}
		for _, m := range tableUseRe.FindAllStringSubmatch(s, -1) {
			if noTenantSchemas[strings.ToLower(m[2])] {
				continue
			}
			if !strings.Contains(s, "tenant_id") || !strings.Contains(s, "deleted_at") {
				out = append(out, fmt.Sprintf("%s: explicit SQL on %s.%s must filter tenant_id and deleted_at IS NULL (R27)", pos, m[2], m[3]))
				break
			}
		}
		return true
	})
	return out, nil
}

var sqlStart = regexp.MustCompile(`(?is)^\s*(SELECT|INSERT|UPDATE|DELETE|TRUNCATE|WITH)\b`)

func looksLikeSQL(s string) bool { return sqlStart.MatchString(s) }
