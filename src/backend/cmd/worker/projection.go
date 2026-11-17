package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/projection"
)

// projections holds the ref.* rebuilders; modules register their tables in it when they own a
// projection (F010 onward). Empty until then: the command fails with "nothing registered".
var projections projection.Registry

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
	stats, err := reg.Rebuild(ctx, *module, *source, *table)
	for name, s := range stats {
		fmt.Fprintf(out, "%s: %d pages, %d rows\n", name, s.Pages, s.Rows)
	}
	return err
}
