// Package scopeguard reports scope creation outside package authz (F002-AC-11, ADR-0089 §3): calls or
// references to tenancy.New*ForAuthz and composite literals of tenancy.TenantScope, CrossTenantScope
// and PlatformScope. Package tenancy and package authz (with sub-packages such as authztest) are exempt.
package scopeguard

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

// Analyzer is the scopeguard analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "scopeguard",
	Doc:      "only package authz creates tenancy scopes (ADR-0089 §3)",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var scopeTypes = map[string]bool{"TenantScope": true, "CrossTenantScope": true, "PlatformScope": true}

func exempt(path string) bool {
	path = strings.TrimSuffix(path, "_test")
	return path == tenancyPath || path == authzPath || strings.HasPrefix(path, authzPath+"/")
}

func run(pass *analysis.Pass) (any, error) {
	if exempt(pass.Pkg.Path()) {
		return nil, nil
	}
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.Preorder([]ast.Node{(*ast.Ident)(nil), (*ast.CompositeLit)(nil)}, func(n ast.Node) {
		switch n := n.(type) {
		case *ast.Ident:
			fn, ok := pass.TypesInfo.Uses[n].(*types.Func)
			if ok && fn.Pkg() != nil && fn.Pkg().Path() == tenancyPath && strings.HasSuffix(fn.Name(), "ForAuthz") {
				pass.Reportf(n.Pos(), "tenancy.%s is reserved for package authz: use authz.ScopeFor/CrossTenantScope/PlatformScope/SystemPlatformScope", fn.Name())
			}
		case *ast.CompositeLit:
			tv, ok := pass.TypesInfo.Types[n]
			if !ok {
				return
			}
			named, ok := types.Unalias(tv.Type).(*types.Named)
			if ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == tenancyPath && scopeTypes[named.Obj().Name()] {
				pass.Reportf(n.Pos(), "composite literal of tenancy.%s outside package authz (scopes come from authz)", named.Obj().Name())
			}
		}
	})
	return nil, nil
}
