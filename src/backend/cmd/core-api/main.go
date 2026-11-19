// Command core-api is the composition root of the REST binary (ADR-0074 §2): it loads the five domain modules,
// wires their facades in process and serves the public API of every module on one pkg/httpx server (OQ F010-2),
// plus the admin port. It runs no relay, consumer or job (worker profiles).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/bootstrap"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

func main() {
	bootstrap.MaybeHealthcheck(":9100")
	authz.MustValidateAtStartup() // F002-AC-14: exit 1 if a role breaks the permission catalog
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, nil); err != nil {
		fmt.Fprintln(os.Stderr, "core-api:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, ready func(admin, http string)) error {
	return bootstrap.Run(ctx, bootstrap.Options{
		Binary:           "core-api",
		DefaultAdminAddr: ":9100",
		DefaultHTTPAddr:  ":8100",
		Modules:          wiring.Modules,
		Register: func(r *module.Registry) error {
			_, err := wiring.ComposeCoreAPI(ctx, r)
			return err
		},
		Getenv: getenv,
		Stdout: os.Stdout,
		Ready:  ready,
	})
}
