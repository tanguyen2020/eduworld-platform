// Command entlint checks ent schemas (F003 T018/T014, ADR-0041, ADR-0089 §2):
//
//   - every field.String has MaxLen(n); field.Text needs a "// maxlen-justified:" comment;
//
//   - every schema declares exactly one entx mixin in Mixin().
//
//     go run ./tools/entlint [dir...]   # default: modules/*/ent/schema and pkg/testkit/sample/ent/schema
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Mixins of pkg/entx (one per table kind).
var Mixins = map[string]bool{"Base": true, "TenantAppendOnly": true, "TenantOptional": true, "TenantOptionalAppendOnly": true,
	"HybridContent": true, "GlobalBase": true, "AppendOnly": true}

func main() {
	flag.Parse()
	dirs := flag.Args()
	if len(dirs) == 0 {
		m, _ := filepath.Glob(filepath.Join("modules", "*", "ent", "schema"))
		dirs = append(m, filepath.Join("pkg", "testkit", "sample", "ent", "schema"))
	}
	var problems []string
	for _, d := range dirs {
		p, err := CheckDir(d)
		if err != nil {
			fmt.Fprintln(os.Stderr, "entlint:", err)
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

// CheckDir returns one message (file:line: …) per violation in the Go files of dir.
func CheckDir(dir string) ([]string, error) {
	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	var out []string
	schemas := map[string]token.Pos{}
	mixins := map[string]int{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		entxName := importName(file, "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx")
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSpec:
				if st, ok := x.Type.(*ast.StructType); ok && embedsSchema(st) {
					schemas[x.Name.Name] = x.Pos()
				}
			case *ast.FuncDecl:
				if x.Recv != nil && x.Name.Name == "Mixin" && len(x.Recv.List) == 1 {
					mixins[recvName(x.Recv.List[0].Type)] = countMixins(x.Body, entxName)
				}
			case *ast.CallExpr:
				if msg := checkField(x, file, fset); msg != "" {
					out = append(out, fmt.Sprintf("%s: %s", fset.Position(x.Pos()), msg))
				}
			}
			return true
		})
	}
	for name, pos := range schemas {
		if n := mixins[name]; n != 1 {
			out = append(out, fmt.Sprintf("%s: schema %s declares %d entx mixins, want exactly one (ADR-0089 §2)", fset.Position(pos), name, n))
		}
	}
	return out, nil
}

func importName(f *ast.File, path string) string {
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) == path {
			if imp.Name != nil {
				return imp.Name.Name
			}
			return path[strings.LastIndex(path, "/")+1:]
		}
	}
	return ""
}

func embedsSchema(st *ast.StructType) bool {
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			if sel, ok := f.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Schema" {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "ent" {
					return true
				}
			}
		}
	}
	return false
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return recvName(t.X)
	}
	return ""
}

func countMixins(body *ast.BlockStmt, entxName string) int {
	n := 0
	if entxName == "" || body == nil {
		return 0
	}
	ast.Inspect(body, func(node ast.Node) bool {
		if cl, ok := node.(*ast.CompositeLit); ok {
			if sel, ok := cl.Type.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == entxName && Mixins[sel.Sel.Name] {
					n++
				}
			}
		}
		return true
	})
	return n
}

// checkField inspects the outermost call of a field chain: field.String("x")…MaxLen(n)….
func checkField(call *ast.CallExpr, file *ast.File, fset *token.FileSet) string {
	// Only the outermost call of a chain: its Fun is a selector whose X is another call (or the root).
	root, methods := chain(call)
	if root == nil {
		return ""
	}
	sel, ok := root.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "field" {
		return ""
	}
	switch sel.Sel.Name {
	case "String":
		if !methods["MaxLen"] {
			return "field.String without MaxLen (ADR-0041)"
		}
	case "Text":
		if !justified(file, fset, call) {
			return "field.Text without a // maxlen-justified: comment and a CHECK (ADR-0041)"
		}
	}
	return ""
}

var seen = map[*ast.CallExpr]bool{}

// chain returns the root field.X(...) call and the method names of the chain when call is the outermost
// call (the root is returned only once per chain).
func chain(call *ast.CallExpr) (*ast.CallExpr, map[string]bool) {
	if seen[call] {
		return nil, nil
	}
	methods := map[string]bool{}
	cur := call
	for {
		seen[cur] = true
		sel, ok := cur.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, nil
		}
		inner, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return cur, methods
		}
		methods[sel.Sel.Name] = true
		cur = inner
	}
}

func justified(file *ast.File, fset *token.FileSet, call *ast.CallExpr) bool {
	line := fset.Position(call.Pos()).Line
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			l := fset.Position(c.Pos()).Line
			if (l == line || l == line-1) && strings.Contains(c.Text, "maxlen-justified:") {
				return true
			}
		}
	}
	return false
}
