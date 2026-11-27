// Command datagen generates the synthetic data of a load run (F132-AC-07, TEST_STRATEGY §5): N tenants perf-<n> ×
// M schools × students of grades 1–9 with fake Vietnamese names and one device_id each, written as csv_import files
// (schools.csv, students.csv per tenant) for the identity module, staff.csv for the staff API and bots.json for k6 and
// loadbot. Loading goes through the identity import/API (never straight into a database). It refuses to run unless
// EW_ENV is local, dev or staging.
//
//	EW_ENV=staging go run ./tools/datagen/cmd/datagen -tenants 20 -schools-per-tenant 5 -students 50000 -out tmp/datagen
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/datagen/internal/csv"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/datagen/internal/guard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/datagen/internal/org"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
)

func main() {
	if err := run(os.Args[1:], os.Getenv(envguard.Var), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "datagen:", err)
		if errors.Is(err, envguard.ErrRefused) {
			os.Exit(3)
		}
		os.Exit(1)
	}
}

func run(args []string, env string, stdout io.Writer) error {
	fs := flag.NewFlagSet("datagen", flag.ContinueOnError)
	o := org.Options{}
	fs.StringVar(&o.Prefix, "prefix", guard.TenantPrefix, "tenant code prefix (must start with perf)")
	fs.IntVar(&o.Tenants, "tenants", 1, "number of tenants")
	fs.IntVar(&o.SchoolsPerTenant, "schools-per-tenant", 1, "schools per tenant")
	fs.IntVar(&o.Students, "students", 100, "students over every tenant")
	fs.Uint64Var(&o.Seed, "seed", 1, "seed (same seed = same data)")
	out := fs.String("out", "tmp/datagen", "output directory")
	at := fs.String("updated-at", "2026-01-01T00:00:00Z", "updated_at of every record (RFC 3339)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := guard.Check(env, o.Prefix); err != nil {
		return err
	}
	ts, err := time.Parse(time.RFC3339, *at)
	if err != nil {
		return fmt.Errorf("updated-at: %w", err)
	}
	tenants, err := org.Build(o)
	if err != nil {
		return err
	}
	for _, t := range tenants {
		dir := filepath.Join(*out, t.Code)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
		files := []struct {
			name  string
			write func(io.Writer) error
		}{
			{"schools.csv", func(w io.Writer) error { return csv.Schools(w, t, ts) }},
			{"students.csv", func(w io.Writer) error { return csv.Students(w, t, ts) }},
			{"staff.csv", func(w io.Writer) error { return csv.Staff(w, t) }},
		}
		for _, f := range files {
			if err := writeFile(filepath.Join(dir, f.name), f.write); err != nil {
				return err
			}
		}
	}
	if err := writeFile(filepath.Join(*out, "bots.json"), func(w io.Writer) error { return csv.Bots(w, tenants) }); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "datagen: %d tenant(s), %d school(s), %d student(s) -> %s\n", len(tenants), o.Tenants*o.SchoolsPerTenant, o.Students, *out)
	return nil
}

func writeFile(path string, write func(io.Writer) error) (err error) {
	f, err := os.Create(path) // #nosec G304 -- a file under the output directory chosen by the operator
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return write(f)
}
