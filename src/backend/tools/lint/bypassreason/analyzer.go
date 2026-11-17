// Package bypassreason reports tenancy.Bypass and authz.CrossTenantScope calls whose reason is not one
// of the tenancy.Reason* constants of the closed list (F002 T058, F002-AC-30, ADR-0089 §3, AGENTS R27).
package bypassreason

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	tenancyPath = "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	authzPath   = "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// Analyzer is the bypassreason analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "bypassreason",
	Doc:      "reason of tenancy.Bypass / authz.CrossTenantScope must be a tenancy.Reason* constant (ADR-0089 §3)",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// reasonArg maps each guarded function to the index of its reason argument.
var reasonArg = map[string]int{tenancyPath + ".Bypass": 1, authzPath + ".CrossTenantScope": 2}

func run(pass *analysis.Pass) (any, error) {
	// Packages tenancy and authz (and their external tests) define and exercise the gate itself.
	if p := strings.TrimSuffix(pass.Pkg.Path(), "_test"); p == tenancyPath || p == authzPath {
		return nil, nil
	}
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.WithStack([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		id := n.(*ast.Ident)
		fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
		if !ok || fn.Pkg() == nil {
			return true
		}
		idx, guarded := reasonArg[fn.Pkg().Path()+"."+fn.Name()]
		if !guarded {
			return true
		}
		// The expression naming the function: pkg.F (selector) or F (identifier).
		var expr ast.Expr = id
		parent := 1
		if len(stack) >= 2 {
			if sel, ok := stack[len(stack)-2].(*ast.SelectorExpr); ok && sel.Sel == id {
				expr, parent = sel, 2
			}
		}
		if len(stack) > parent {
			if call, ok := stack[len(stack)-1-parent].(*ast.CallExpr); ok && call.Fun == expr {
				if len(call.Args) > idx && !isReasonConst(pass, call.Args[idx]) {
					pass.Reportf(call.Args[idx].Pos(), "Bypass reason must be a tenancy.Reason* constant of the closed list (ADR-0089 §3)")
				}
				return true
			}
		}
		pass.Reportf(expr.Pos(), "%s.%s must be called directly so its reason can be checked", fn.Pkg().Name(), fn.Name())
		return true
	})
	return nil, nil
}

// isReasonConst: an identifier or selector naming a constant Reason* declared in package tenancy.
func isReasonConst(pass *analysis.Pass, e ast.Expr) bool {
	var id *ast.Ident
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	default:
		return false
	}
	c, ok := pass.TypesInfo.Uses[id].(*types.Const)
	return ok && c.Pkg() != nil && c.Pkg().Path() == tenancyPath && strings.HasPrefix(c.Name(), "Reason")
}
