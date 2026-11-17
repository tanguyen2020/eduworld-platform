// Package nointernalhttp reports uses of the HTTP server helpers that oapi-codegen generates into the
// module facades (modules/<m>/api): Handler*, NewStrictHandler*. Facades are called in process through
// pkg/modcall; no route, listener or port serves /internal/v1 (ADR-0074 §3, F002-AC-05).
package nointernalhttp

import (
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var facadeRe = regexp.MustCompile(`/src/backend/modules/[a-z]+/api$`)

// Analyzer is the nointernalhttp analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "nointernalhttp",
	Doc:      "module facades are never served over HTTP (ADR-0074)",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func banned(name string) bool {
	return strings.HasPrefix(name, "Handler") || strings.HasPrefix(name, "NewStrictHandler")
}

func run(pass *analysis.Pass) (any, error) {
	if facadeRe.MatchString(pass.Pkg.Path()) {
		return nil, nil // the generated file itself
	}
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.Preorder([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node) {
		id := n.(*ast.Ident)
		fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
		if ok && fn.Pkg() != nil && facadeRe.MatchString(fn.Pkg().Path()) && banned(fn.Name()) {
			pass.Reportf(id.Pos(), "%s.%s would serve a module facade over HTTP; call it in process through pkg/modcall (ADR-0074)",
				fn.Pkg().Name(), fn.Name())
		}
	})
	return nil, nil
}
