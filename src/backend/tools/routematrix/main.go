// Command routematrix builds the authorization test matrix from the merged public spec and checks that
// every implemented operation has its required authorization tests (F002 T049, F002-AC-28, NFR-SEC-006).
//
//	go run ./tools/routematrix gen     # write test/authz/matrix_gen.json
//	go run ./tools/routematrix check   # fail when matrix is stale or an implemented operation lacks tests
//
// Required cases: role_denied for every authenticated operation; cross_tenant (tenant A/B → 404) when
// the operation takes X-Tenant-Id or an id in its path. An operation is implemented when a non-test Go
// file has an httpx.Operation literal with that ID; a test covers cases by calling
// authztest.Cover(t, "<operationId>", authztest.CrossTenant, authztest.RoleDenied) with literals.
// Facade operations (internal specs) are covered by facade tests (make test-contract), not here.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	httpxPath     = "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	authztestPath = "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	matrixFile    = "test/authz/matrix_gen.json"
)

var caseConsts = map[string]string{"CrossTenant": "cross_tenant", "RoleDenied": "role_denied"}

// Entry is one row of the matrix.
type Entry struct {
	OperationID  string   `json:"operation_id"`
	Module       string   `json:"module"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	Public       bool     `json:"public,omitempty"`
	TenantScoped bool     `json:"tenant_scoped,omitempty"`
	HasIDParam   bool     `json:"has_id_param,omitempty"`
	Required     []string `json:"required"`
}

func main() {
	root := "."
	specPath := filepath.Join("..", "..", "specs", "openapi", "eduworld-api.v1.yaml")
	if len(os.Args) < 2 || (os.Args[1] != "gen" && os.Args[1] != "check") {
		fmt.Fprintln(os.Stderr, "usage: routematrix gen|check")
		os.Exit(2)
	}
	if err := run(os.Args[1], root, specPath); err != nil {
		fmt.Fprintln(os.Stderr, "routematrix:", err)
		os.Exit(1)
	}
}

func run(cmd, root, specPath string) error {
	b, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	m, err := Gen(b)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	target := filepath.Join(root, matrixFile)
	if cmd == "gen" {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, out, 0o644)
	}
	var problems []string
	if cur, err := os.ReadFile(target); err != nil || !bytes.Equal(bytes.ReplaceAll(cur, []byte("\r\n"), []byte("\n")), out) {
		problems = append(problems, matrixFile+" is stale: run `make gen`")
	}
	more, err := Check(m, root)
	if err != nil {
		return err
	}
	problems = append(problems, more...)
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problem(s)", len(problems))
	}
	fmt.Printf("routematrix: %d operations in the contract, 0 problems\n", len(m))
	return nil
}

type param struct {
	Ref      string `yaml:"$ref"`
	Name     string `yaml:"name"`
	In       string `yaml:"in"`
	Required bool   `yaml:"required"`
}

var methods = []string{"get", "put", "post", "delete", "patch"}

// Gen builds the sorted matrix from the merged public spec.
func Gen(spec []byte) ([]Entry, error) {
	var doc struct {
		Paths      map[string]map[string]yaml.Node `yaml:"paths"`
		Components struct {
			Parameters map[string]param `yaml:"parameters"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		return nil, err
	}
	resolve := func(p param) param {
		if name, ok := strings.CutPrefix(p.Ref, "#/components/parameters/"); ok {
			return doc.Components.Parameters[name]
		}
		return p
	}
	tenantHeader := func(ps []param) bool {
		for _, p := range ps {
			p = resolve(p)
			if p.In == "header" && strings.EqualFold(p.Name, "X-Tenant-Id") {
				return true
			}
		}
		return false
	}
	var out []Entry
	for path, item := range doc.Paths {
		var shared []param
		if n, ok := item["parameters"]; ok {
			if err := n.Decode(&shared); err != nil {
				return nil, fmt.Errorf("%s parameters: %w", path, err)
			}
		}
		for method, node := range item {
			if !slices.Contains(methods, method) {
				continue
			}
			var op struct {
				ID       string            `yaml:"operationId"`
				Service  yaml.Node         `yaml:"x-service"` // one module, or a list routed by x-service-routing
				Params   []param           `yaml:"parameters"`
				Security *[]map[string]any `yaml:"security"`
			}
			if err := node.Decode(&op); err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			if op.ID == "" {
				return nil, fmt.Errorf("%s %s: missing operationId", method, path)
			}
			e := Entry{OperationID: op.ID, Module: serviceName(op.Service), Method: strings.ToUpper(method), Path: path,
				Public: op.Security != nil && len(*op.Security) == 0, TenantScoped: tenantHeader(append(shared, op.Params...)),
				HasIDParam: strings.Contains(path, "{"), Required: []string{}}
			if !e.Public {
				if e.TenantScoped || e.HasIDParam {
					e.Required = append(e.Required, "cross_tenant")
				}
				e.Required = append(e.Required, "role_denied")
			}
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OperationID < out[j].OperationID })
	for i := 1; i < len(out); i++ {
		if out[i].OperationID == out[i-1].OperationID {
			return nil, fmt.Errorf("duplicate operationId %s", out[i].OperationID)
		}
	}
	return out, nil
}

// Check compares implemented operations under root/modules and root/cmd with the Cover calls in tests.
func Check(m []Entry, root string) ([]string, error) {
	byID := map[string]Entry{}
	for _, e := range m {
		byID[e.OperationID] = e
	}
	implemented := map[string]string{} // id → file
	covered := map[string]map[string]bool{}
	var problems []string
	for _, dir := range []string{"modules", "cmd"} {
		base := filepath.Join(root, dir)
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return filepath.SkipDir
				}
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			rel, _ := filepath.Rel(root, path)
			if strings.HasSuffix(path, "_test.go") {
				problems = append(problems, scanCover(f, filepath.ToSlash(rel), covered)...)
			} else {
				scanOperations(f, filepath.ToSlash(rel), implemented)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	ids := make([]string, 0, len(implemented))
	for id := range implemented {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		e, ok := byID[id]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: operation %s is implemented but not in the public spec", implemented[id], id))
			continue
		}
		var missing []string
		for _, c := range e.Required {
			if !covered[id][c] {
				missing = append(missing, c)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("%s: operation %s lacks authorization tests %v (authztest.Cover)", implemented[id], id, missing))
		}
	}
	return problems, nil
}

// importName returns the local name of import path in f ("" when not imported).
func importName(f *ast.File, path string) string {
	for _, imp := range f.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == path {
			if imp.Name != nil {
				return imp.Name.Name
			}
			return path[strings.LastIndex(path, "/")+1:]
		}
	}
	return ""
}

func scanOperations(f *ast.File, rel string, implemented map[string]string) {
	name := importName(f, httpxPath)
	if name == "" {
		return // only files that use httpx can declare operations
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if !isOperationType(lit.Type, name) && lit.Type != nil {
			return true
		}
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "ID" {
				if s, ok := kv.Value.(*ast.BasicLit); ok && s.Kind == token.STRING {
					if id, err := strconv.Unquote(s.Value); err == nil {
						implemented[id] = rel
					}
				}
			}
		}
		return true
	})
}

// isOperationType: httpx.Operation (or an element of []httpx.Operation, whose type is elided).
func isOperationType(t ast.Expr, httpxName string) bool {
	if httpxName == "" {
		return false
	}
	switch x := t.(type) {
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		return ok && id.Name == httpxName && x.Sel.Name == "Operation"
	case *ast.ArrayType:
		return isOperationType(x.Elt, httpxName)
	}
	return false
}

func scanCover(f *ast.File, rel string, covered map[string]map[string]bool) []string {
	name := importName(f, authztestPath)
	if name == "" {
		return nil
	}
	var problems []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Cover" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != name {
			return true
		}
		if len(call.Args) < 3 {
			return true
		}
		lit, ok := call.Args[1].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			problems = append(problems, fmt.Sprintf("%s: authztest.Cover operation id must be a string literal", rel))
			return true
		}
		op, _ := strconv.Unquote(lit.Value)
		if covered[op] == nil {
			covered[op] = map[string]bool{}
		}
		for _, a := range call.Args[2:] {
			s, ok := a.(*ast.SelectorExpr)
			if c, known := caseConsts[selName(s, ok)]; known {
				covered[op][c] = true
			} else {
				problems = append(problems, fmt.Sprintf("%s: authztest.Cover cases must be authztest.CrossTenant/RoleDenied", rel))
			}
		}
		return true
	})
	return problems
}

func selName(s *ast.SelectorExpr, ok bool) string {
	if !ok {
		return ""
	}
	return s.Sel.Name
}

// serviceName renders x-service: "learning", or "game|competition|chat" for parameter-routed operations.
func serviceName(n yaml.Node) string {
	var one string
	if n.Decode(&one) == nil {
		return one
	}
	var many []string
	if n.Decode(&many) == nil {
		return strings.Join(many, "|")
	}
	return ""
}
