package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/projection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// projectionJob is the worker profile of `worker projection rebuild` (one-off composition, module.Registry.Reconciles).
const projectionJob = "projection"

// runProjection implements `worker projection rebuild --module=<m> --source=<n> [--table=ref.<t>]`
// (F003-AC-39, BACKUP_RECOVERY §3.5). It returns an error (exit ≠ 0) when the source facade fails.
func runProjection(ctx context.Context, args []string, reg *projection.Registry, out io.Writer) error {
	if len(args) == 0 || args[0] != "rebuild" {
		return errors.New("usage: worker projection rebuild --module=<module> --source=<module> [--table=ref.<table>]")
	}
	fs := flag.NewFlagSet("projection rebuild", flag.ContinueOnError)
	fs.SetOutput(out)
	module := fs.String("module", "", "receiving module (owner of the projection)")
	source := fs.String("source", "", "source module (owner of the facade)")
	table := fs.String("table", "", "only this ref.<table>")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *module == "" || *source == "" {
		return errors.New("--module and --source are required")
	}
	// The system platform scope of the job; the source facade takes its own audited bypass platform_job (ADR-0089) to page
	// every tenant, and each page is written under the EventScope of its tenants (F003 Clarifications 2026-11-28).
	ps, err := authz.SystemPlatformScope(projectionJob + "-rebuild")
	if err != nil {
		return err
	}
	ctx = tenancy.WithScope(tenancy.WithModule(ctx, *module), ps)
	stats, err := reg.Rebuild(ctx, *module, *source, *table)
	names := make([]string, 0, len(stats))
	for name := range stats {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(out, "%s: %d pages, %d rows\n", name, stats[name].Pages, stats[name].Rows)
	}
	return err
}

// composeProjection composes the five modules in the one-off profile projection (no relay, consumer, job nor HTTP runs)
// and registers the ref.* rebuilders of the modules that keep projections of module identity (F003 Clarifications
// 2026-11-28). The returned function releases the pools and the lazy NATS connection.
func composeProjection(getenv func(string) string) (*projection.Registry, func(), error) {
	r := module.NewRegistry("worker", projectionJob, health.NewRegistry(buildinfo.Get("worker", projectionJob)), getenv)
	// Some modules take their JetStream context while composing; the connection is lazy and never used (nothing is
	// published or consumed: the rebuild writes the projections in the module transactions only).
	nc, err := connectLazyNATS(getenv)
	if err != nil {
		r.Close()
		return nil, nil, err
	}
	r.SetNATS(nc)
	closeAll := func() { r.Close(); nc.Close() }
	c, err := wiring.Compose(r)
	if err != nil {
		closeAll()
		return nil, nil, err
	}
	reg := &projection.Registry{}
	if _, err := wiring.RegisterIdentityProjections(reg, r, c); err != nil {
		closeAll()
		return nil, nil, err
	}
	return reg, closeAll, nil
}
