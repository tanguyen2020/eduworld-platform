// Command realtime terminates WebSocket /realtime/v1 (ADR-0074 §2). It loads only module realtime,
// has no database and routes commands to core-api over NATS (F090): the gateway handler is installed by
// realtime.Register (Registry.SetPublic) and drains on SIGTERM (system.server.draining, close 1012).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/bootstrap"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

func main() {
	bootstrap.MaybeHealthcheck(":9106")
	authz.MustValidateAtStartup() // F002-AC-14: exit 1 if a role breaks the permission catalog
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, nil); err != nil {
		fmt.Fprintln(os.Stderr, "realtime:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, ready func(admin, http string)) error {
	return bootstrap.Run(ctx, bootstrap.Options{
		Binary:           "realtime",
		DefaultAdminAddr: ":9106",
		DefaultHTTPAddr:  ":8106",
		Public:           http.NotFoundHandler(), // replaced by the gateway of module realtime (Registry.SetPublic)
		Modules:          []string{realtime.Name},
		Register: func(r *module.Registry) error {
			_, err := realtime.Register(r)
			return err
		},
		Getenv: getenv,
		Stdout: os.Stdout,
		Ready:  ready,
	})
}
