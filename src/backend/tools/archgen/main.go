// Command archgen enforces and generates the module-boundary rules of ADR-0074 (F001 T045,
// F001-AC-21…23, AGENTS R13/R29). Rules are derived from `x-allowed-callers` of every operation in
// specs/openapi/modules/*.internal.v1.yaml:
//
//   - modules/<A>/... may import modules/<B>/... (B ≠ A) only as modules/<B>/api, and only when A is
//     an allowed caller of at least one operation of B;
//   - pkg/... never imports modules/...;
//   - cmd/... imports only module root packages modules/<m> (which expose Register);
//   - layers inside a module: domain imports no application/infrastructure/presentation;
//     application imports no infrastructure/presentation; application/query never imports
//     application/command or application/port/write (ADR-0087);
//   - domain imports no infrastructure library (ent, pgx, database/sql, net/http, nats, valkey) nor the
//     module ent package (F002-AC-26, BACKEND_DESIGN §3);
//   - pkg/authz, pkg/problem, pkg/tenancy never depend on ent or pgx, even transitively (F002-AC-27).
//
// Go's internal/ rule already stops imports of modules/<B>/internal/... at compile time.
//
//	go run ./tools/archgen           # write .golangci.arch.yml (depguard rules)
//	go run ./tools/archgen --check   # verify imports of every package; fail on violations or a stale file
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const generatedFile = ".golangci.arch.yml"

func main() {
	check := flag.Bool("check", false, "verify imports and that the generated file is up to date")
	root := flag.String("root", ".", "Go module root (src/backend)")
	specs := flag.String("specs", filepath.Join("..", "..", "specs", "openapi", "modules"), "directory of *.internal.v1.yaml")
	flag.Parse()
	if err := run(*root, *specs, *check, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "archgen:", err)
		os.Exit(1)
	}
}

func run(root, specsDir string, check bool, out io.Writer) error {
	modPath, err := modulePath(root)
	if err != nil {
		return err
	}
	allowed, err := loadAllowedCallers(specsDir)
	if err != nil {
		return err
	}
	modules, err := listModules(filepath.Join(root, "modules"))
	if err != nil {
		return err
	}
	gen := renderDepguard(modPath, modules, allowed)
	target := filepath.Join(root, generatedFile)
	if !check {
		return os.WriteFile(target, gen, 0o644)
	}

	var problems []string
	if cur, err := os.ReadFile(target); err != nil || !bytes.Equal(normalize(cur), normalize(gen)) {
		problems = append(problems, generatedFile+" is stale: run `make gen`")
	}
	pkgs, err := goList(root)
	if err != nil {
		return err
	}
	problems = append(problems, Violations(modPath, pkgs, allowed)...)
	sort.Strings(problems)
	for _, p := range problems {
		fmt.Fprintln(out, p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d architecture violation(s)", len(problems))
	}
	fmt.Fprintf(out, "archgen: %d packages, %d modules, 0 violations\n", len(pkgs), len(modules))
	return nil
}

func normalize(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

// Package is the subset of `go list -json` used here.
type Package struct {
	ImportPath string
	Imports    []string
	Deps       []string
}

// domainForbidden are import prefixes a domain package must never use (F002-AC-26).
var domainForbidden = []string{"entgo.io/", "github.com/jackc/pgx", "database/sql", "net/http", "github.com/nats-io/",
	"github.com/valkey-io/"}

// dataLayerFree are pkg/ packages (and their sub-packages) without ent/pgx in their dependency graph.
var dataLayerFree = []string{"pkg/authz", "pkg/problem", "pkg/tenancy"}

func externalViolations(modPath, rel, owner, layer string, p Package) []string {
	var out []string
	if owner != "" && (layer == "domain" || strings.HasPrefix(layer, "domain/")) {
		for _, imp := range p.Imports {
			for _, f := range domainForbidden {
				if strings.HasPrefix(imp, f) {
					out = append(out, fmt.Sprintf("%s: domain must not import %s (F002-AC-26)", rel, imp))
					break
				}
			}
			if strings.HasPrefix(imp, modPath+"/modules/"+owner+"/ent") {
				out = append(out, fmt.Sprintf("%s: domain must not import %s (F002-AC-26)", rel, strings.TrimPrefix(imp, modPath+"/")))
			}
		}
	}
	for _, d := range dataLayerFree {
		if rel != d && !strings.HasPrefix(rel, d+"/") {
			continue
		}
		for _, dep := range append(append([]string(nil), p.Imports...), p.Deps...) {
			if strings.HasPrefix(dep, "entgo.io/") || strings.HasPrefix(dep, "github.com/jackc/pgx") {
				out = append(out, fmt.Sprintf("%s: must not depend on %s (F002-AC-27)", rel, dep))
				break
			}
		}
	}
	return out
}

// Violations returns one message per forbidden import.
func Violations(modPath string, pkgs []Package, allowed map[string]map[string]bool) []string {
	var out []string
	modPrefix := modPath + "/modules/"
	for _, p := range pkgs {
		rel := strings.TrimPrefix(p.ImportPath, modPath+"/")
		if rel == p.ImportPath {
			continue
		}
		owner, layer := ownerAndLayer(rel)
		out = append(out, externalViolations(modPath, rel, owner, layer, p)...)
		for _, imp := range p.Imports {
			if !strings.HasPrefix(imp, modPath+"/") {
				continue
			}
			irel := strings.TrimPrefix(imp, modPath+"/")
			switch {
			case strings.HasPrefix(rel, "pkg/") && strings.HasPrefix(imp, modPrefix):
				out = append(out, fmt.Sprintf("%s: pkg/ must not import modules/ (%s)", rel, irel))
			case strings.HasPrefix(rel, "cmd/") && strings.HasPrefix(imp, modPrefix):
				parts := strings.Split(strings.TrimPrefix(imp, modPrefix), "/")
				if len(parts) != 1 {
					out = append(out, fmt.Sprintf("%s: cmd/ may import only module root packages (Register), not %s", rel, irel))
				}
			case owner != "" && strings.HasPrefix(imp, modPrefix):
				parts := strings.Split(strings.TrimPrefix(imp, modPrefix), "/")
				callee := parts[0]
				if callee == owner {
					if msg := layerViolation(layer, irel, owner); msg != "" {
						out = append(out, fmt.Sprintf("%s: %s", rel, msg))
					}
					continue
				}
				if len(parts) != 2 || parts[1] != "api" {
					out = append(out, fmt.Sprintf("%s: module %s may import only modules/%s/api, not %s", rel, owner, callee, irel))
				} else if !allowed[callee][owner] {
					out = append(out, fmt.Sprintf("%s: module %s is not in x-allowed-callers of any %s operation (imports %s)", rel, owner, callee, irel))
				}
			}
		}
	}
	return out
}

// ownerAndLayer: modules/<m>/internal/<layer>/... → (m, layer path after internal/).
func ownerAndLayer(rel string) (string, string) {
	if !strings.HasPrefix(rel, "modules/") {
		return "", ""
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return "", ""
	}
	if len(parts) >= 4 && parts[2] == "internal" {
		return parts[1], strings.Join(parts[3:], "/")
	}
	return parts[1], ""
}

func layerViolation(layer, importRel, owner string) string {
	base := "modules/" + owner + "/internal/"
	if !strings.HasPrefix(importRel, base) {
		return ""
	}
	target := strings.TrimPrefix(importRel, base)
	has := func(s, p string) bool { return s == p || strings.HasPrefix(s, p+"/") }
	switch {
	case has(layer, "domain") && (has(target, "application") || has(target, "infrastructure") || has(target, "presentation")):
		return "domain must not import " + target
	case has(layer, "application") && (has(target, "infrastructure") || has(target, "presentation")):
		return "application must not import " + target
	case has(layer, "application/query") && (has(target, "application/command") || has(target, "application/port/write")):
		return "query (read context) must not import " + target
	}
	return ""
}

func loadAllowedCallers(dir string) (map[string]map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.internal.v1.yaml"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no internal specs in %s", dir)
	}
	allowed := map[string]map[string]bool{}
	for _, f := range files {
		callee := strings.SplitN(filepath.Base(f), ".", 2)[0]
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Paths map[string]map[string]struct {
				Callers []string `yaml:"x-allowed-callers"`
			} `yaml:"paths"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		set := map[string]bool{}
		for _, item := range doc.Paths {
			for _, op := range item {
				for _, c := range op.Callers {
					set[c] = true
				}
			}
		}
		allowed[callee] = set
	}
	return allowed, nil
}

func listModules(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func renderDepguard(modPath string, modules []string, allowed map[string]map[string]bool) []byte {
	var b strings.Builder
	b.WriteString("# Code generated by tools/archgen from x-allowed-callers in specs/openapi/modules/*.internal.v1.yaml. DO NOT EDIT.\n")
	b.WriteString("# Used by `make lint`; `make arch-check` (tools/archgen --check) is the authoritative check.\n")
	b.WriteString("version: \"2\"\nlinters:\n  default: none\n  enable: [depguard]\n  settings:\n    depguard:\n      rules:\n")
	b.WriteString("        pkg-no-modules:\n          files: [\"**/pkg/**\"]\n          deny:\n")
	fmt.Fprintf(&b, "            - pkg: %q\n              desc: \"pkg/ must not import modules/ (ADR-0074)\"\n", modPath+"/modules")
	for _, caller := range modules {
		var denies []string
		for _, callee := range modules {
			if callee == caller {
				continue
			}
			if !allowed[callee][caller] {
				denies = append(denies, callee)
			}
		}
		if len(denies) == 0 {
			continue
		}
		fmt.Fprintf(&b, "        module-%s:\n          files: [\"**/modules/%s/**\"]\n          deny:\n", caller, caller)
		for _, callee := range denies {
			fmt.Fprintf(&b, "            - pkg: %q\n              desc: \"%s is not in x-allowed-callers of %s\"\n", modPath+"/modules/"+callee+"/api", caller, callee)
		}
	}
	return []byte(b.String())
}

func modulePath(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", errors.New("module line not found in go.mod")
}

func goList(root string) ([]Package, error) {
	cmd := exec.Command("go", "list", "-e", "-tags", "integration", "-json=ImportPath,Imports,Deps", "./...")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list: %w: %s", err, stderr.String())
	}
	var pkgs []Package
	dec := json.NewDecoder(&stdout)
	for dec.More() {
		var p Package
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}
