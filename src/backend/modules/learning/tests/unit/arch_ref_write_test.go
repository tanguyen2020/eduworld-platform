// Package unit holds architecture tests of module learning that read the source tree.
package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var refWrite = regexp.MustCompile(`(?is)\b(INSERT\s+INTO|UPDATE)\s+ref\.`)

// moduleFiles returns the non-test, non-generated Go files of module learning (relative paths, slash form).
func moduleFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	root := filepath.Join("..", "..")
	out := map[string]*ast.File{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if d.IsDir() {
			if rel == "tests" || rel == "ent" || rel == "api" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		out[rel] = f
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 10 {
		t.Fatalf("only %d files scanned", len(out))
	}
	return out
}

// F020-AC-05 (TC-TEN-017): only the projection consumer writes ref.*. SQL writing ref.* lives in the ref
// repository alone; the ProjectionWriter port is used only by the command UpsertProjection; that command is
// called only by the consumer learning.projection_identity (and wired by the module root).
func TestOnlyProjectionConsumerWritesRef(t *testing.T) {
	files := moduleFiles(t)
	const repo = "internal/infrastructure/persistence/write/ref_repo.go"
	sawRepo := false
	for rel, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err == nil && refWrite.MatchString(s) {
				if rel != repo {
					t.Errorf("%s writes ref.* (only %s may)", rel, repo)
				} else {
					sawRepo = true
				}
			}
			return true
		})
	}
	if !sawRepo {
		t.Fatal("the ref repository writes no ref.* table: the scan is broken")
	}
	uses := func(ident string, allowed ...string) {
		for rel, f := range files {
			found := false
			ast.Inspect(f, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == ident {
					found = true
				}
				if id, ok := n.(*ast.Ident); ok && id.Name == ident && id.Obj != nil {
					found = true
				}
				return !found
			})
			if !found {
				continue
			}
			ok := false
			for _, a := range allowed {
				if strings.HasPrefix(rel, a) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s uses %s (allowed only in %v)", rel, ident, allowed)
			}
		}
	}
	uses("ProjectionWriter", "internal/application/port/write/", "internal/application/command/upsert_projection.go",
		"internal/infrastructure/persistence/write/ref_repo.go")
	uses("UpsertProjection", "internal/application/command/upsert_projection.go", "internal/presentation/consumer/", "module.go")
	uses("RefRepo", "internal/infrastructure/persistence/write/ref_repo.go", "module.go")
}
