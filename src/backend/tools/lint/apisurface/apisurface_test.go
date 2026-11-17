// Package apisurface freezes the public API of pkg/tenancy and pkg/authz (F002 T057, ADR-0089 §3):
// changing an exported signature fails this test; update the golden file only together with an ADR.
//
//	go test ./tools/lint/apisurface -update
package apisurface

import (
	"flag"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

var update = flag.Bool("update", false, "rewrite golden files")

const base = "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/"

func surface(t *testing.T, path string) string {
	t.Helper()
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedTypes | packages.NeedName, Dir: "../../.."}, path)
	if err != nil || len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load %s: %v %v", path, err, pkgs)
	}
	scope := pkgs[0].Types.Scope()
	qual := types.RelativeTo(pkgs[0].Types)
	var blocks []string
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		lines := []string{types.ObjectString(obj, qual)}
		if tn, ok := obj.(*types.TypeName); ok {
			if named, ok := tn.Type().(*types.Named); ok {
				for i := 0; i < named.NumMethods(); i++ {
					if m := named.Method(i); m.Exported() {
						lines = append(lines, "  "+types.ObjectString(m, qual))
					}
				}
				if st, ok := named.Underlying().(*types.Struct); ok {
					for i := 0; i < st.NumFields(); i++ {
						if f := st.Field(i); f.Exported() {
							lines = append(lines, "  "+types.ObjectString(f, qual))
						}
					}
				}
			}
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	// scope.Names() is sorted, so blocks are in name order.
	return strings.Join(blocks, "\n") + "\n"
}

func TestAPISurface(t *testing.T) {
	for _, name := range []string{"tenancy", "authz"} {
		got := surface(t, base+name)
		golden := filepath.Join("testdata", name+".api")
		if *update {
			if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (run with -update to create)", err)
		}
		if strings.ReplaceAll(string(want), "\r\n", "\n") != got {
			t.Errorf("public API of pkg/%s changed (ADR-0089 §3 needs an ADR); diff against %s:\n%s", name, golden, got)
		}
	}
}
