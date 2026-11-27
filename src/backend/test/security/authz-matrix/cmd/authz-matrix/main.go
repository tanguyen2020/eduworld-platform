// Command authz-matrix runs the authorization and tenant isolation matrix of every public operation against a
// running environment (F132 US3: F132-AC-15 matrix, F132-AC-16 SCOPE=internal, F132-AC-17 MODE=maxlength).
//
//	EW_ENV=staging EW_BASE_URL=https://api.staging.<domain>/api/v1 EW_INTERNAL_BASE_URL=https://admin-api.staging.<domain>/api/v1 \
//	EW_AUTHZ_FIXTURES=fixtures.yaml EW_AUTHZ_PASSWORD_FILE_STUDENT=… go run ./test/security/authz-matrix/cmd/authz-matrix -mode matrix
//
// Modes: matrix (operation × role × scope relation), internal (facade paths and x-internal routes not served over
// HTTP; direct ports with -direct), maxlength (maxLength+1 per body string → 422 MAX_LENGTH, missing X-Tenant-Id →
// 400 TENANT_HEADER_REQUIRED), gen (print the case counts, no request). Refuses EW_ENV outside local|dev|staging
// (F132-AC-07). Writes <out>/authz-matrix-<mode>.json and .md; exits 1 when a case fails (with -strict also on
// inconclusive or skipped cases — the staging gate).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/cases"
	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/runner"
	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/spec"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string) error {
	fs := flag.NewFlagSet("authz-matrix", flag.ContinueOnError)
	mode := fs.String("mode", "matrix", "matrix | internal | maxlength | gen")
	specPath := fs.String("spec", "../../specs/openapi/eduworld-api.v1.yaml", "merged public spec (relative to src/backend)")
	matrixPath := fs.String("matrix", "test/authz/matrix_gen.json", "routematrix output (make gen)")
	modulesDir := fs.String("modules", "../../specs/openapi/modules", "directory of the facade specs")
	overridesPath := fs.String("overrides", "test/security/authz-matrix/expectations.yaml", "reviewed class/skip overrides")
	out := fs.String("out", "../../tmp/security", "report directory (default: tmp/security of the repository, git-ignored)")
	direct := fs.String("direct", getenv("EW_AUTHZ_DIRECT_TARGETS"), "comma list of host:port of backend binaries (mode internal)")
	rps := fs.Float64("rps", 10, "requests per second towards the target")
	strict := fs.Bool("strict", false, "fail on inconclusive or skipped cases (staging gate)")
	only := fs.String("only", "", "comma list of operationIds to run (debugging)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ops, err := spec.Load(ctx, *specPath, *matrixPath)
	if err != nil {
		return err
	}
	ov, err := loadOverrides(*overridesPath)
	if err != nil {
		return err
	}
	ops = filter(ops, *only)
	if *mode == "gen" { // offline: no environment, no request
		cs := cases.Generate(ops, ov)
		ls, nc := cases.LengthCases(ops)
		paths, err := spec.InternalPaths(*modulesDir)
		if err != nil {
			return err
		}
		asserted := 0
		for _, c := range cs {
			if !c.Expect.Observe {
				asserted++
			}
		}
		fmt.Printf("authz-matrix gen: %d operations, %d matrix cases (%d asserted), %d internal probes, %d maxLength cases (%d composite strings not covered), %d missing X-Tenant-Id cases\n",
			len(ops), len(cs), asserted, len(cases.InternalProbes(ops, paths)), len(ls), nc, len(cases.HeaderCases(ops)))
		return nil
	}
	env, err := envguard.FromEnv()
	if err != nil {
		return err
	}
	base := getenv("EW_BASE_URL")
	if base == "" {
		return fmt.Errorf("authz-matrix: EW_BASE_URL is required (API base, …/api/v1)")
	}
	r := &runner.Runner{BaseURL: base, InternalURL: getenv("EW_INTERNAL_BASE_URL"), Creds: runner.EnvCredentials(getenv),
		Interval: time.Duration(float64(time.Second) / max(*rps, 0.1)), MaxRetries: 3}
	if p := getenv("EW_AUTHZ_FIXTURES"); p != "" {
		if r.Fixtures, err = runner.LoadFixtures(p); err != nil {
			return err
		}
	} else if *mode != "internal" {
		return fmt.Errorf("authz-matrix: EW_AUTHZ_FIXTURES is required for mode %s (see fixtures.example.yaml)", *mode)
	}
	rep := &runner.Report{RunID: uuid.NewString(), Env: env, Mode: *mode, Target: host(base), Commit: getenv("GITHUB_SHA"), StartedAt: time.Now()}
	switch *mode {
	case "matrix":
		rep.Results = r.RunMatrix(ctx, cases.Generate(ops, ov))
	case "internal":
		paths, err := spec.InternalPaths(*modulesDir)
		if err != nil {
			return err
		}
		var targets []runner.DirectTarget
		for _, d := range strings.Split(*direct, ",") {
			if d = strings.TrimSpace(d); d != "" {
				targets = append(targets, runner.DirectTarget(d))
			}
		}
		rep.Results = r.RunInternal(ctx, cases.InternalProbes(ops, paths), targets, paths)
	case "maxlength":
		ls, nc := cases.LengthCases(ops)
		rep.NotCovered = nc
		rep.Results = r.RunLength(ctx, ls, cases.HeaderCases(ops), ov)
	default:
		return fmt.Errorf("authz-matrix: unknown mode %q", *mode)
	}
	rep.FinishedAt = time.Now()
	rep.Summarise()
	if err := write(*out, rep); err != nil {
		return err
	}
	s := rep.Summary
	fmt.Printf("authz-matrix %s (%s): %d cases, %d pass, %d fail, %d inconclusive, %d skipped, %d observed (%.2f%%) → %s\n",
		*mode, env, s.Total, s.Pass, s.Fail, s.Inconclusive, s.Skipped, s.Observed, s.PassRate, *out)
	return rep.Gate(*strict)
}

func loadOverrides(path string) (cases.Overrides, error) {
	var ov cases.Overrides
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ov, nil
	}
	if err != nil {
		return ov, err
	}
	if err := yaml.Unmarshal(b, &ov); err != nil {
		return ov, fmt.Errorf("authz-matrix: %s: %w", path, err)
	}
	for id, reason := range ov.Skip {
		if strings.TrimSpace(reason) == "" {
			return ov, fmt.Errorf("authz-matrix: %s: skip of %s needs a reason", path, id)
		}
	}
	return ov, nil
}

func filter(ops []spec.Operation, only string) []spec.Operation {
	if only == "" {
		return ops
	}
	want := map[string]bool{}
	for _, id := range strings.Split(only, ",") {
		want[strings.TrimSpace(id)] = true
	}
	var out []spec.Operation
	for _, op := range ops {
		if want[op.ID] {
			out = append(out, op)
		}
	}
	return out
}

func write(dir string, rep *runner.Report) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := filepath.Join(dir, "authz-matrix-"+rep.Mode)
	jf, err := os.Create(name + ".json")
	if err != nil {
		return err
	}
	if err := rep.WriteJSON(jf); err != nil {
		_ = jf.Close()
		return err
	}
	if err := jf.Close(); err != nil {
		return err
	}
	mf, err := os.Create(name + ".md")
	if err != nil {
		return err
	}
	if err := rep.WriteMarkdown(mf); err != nil {
		_ = mf.Close()
		return err
	}
	return mf.Close()
}

func host(base string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 { // never report userinfo
		s = s[i+1:]
	}
	return s
}
