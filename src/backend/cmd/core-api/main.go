// Command core-api is the composition root of the REST binary (ADR-0074 §2): it loads the five
// domain modules and serves the public API (routes are added by later features) plus the admin port.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning"
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
		Public:           publicRouter(),
		Modules:          []string{identity.Name, learning.Name, game.Name, competition.Name, chat.Name},
		Register:         registerModules,
		Getenv:           getenv,
		Stdout:           os.Stdout,
		Ready:            ready,
	})
}

func registerModules(r *module.Registry) error {
	if _, err := identity.Register(r); err != nil {
		return err
	}
	if _, err := learning.Register(r); err != nil {
		return err
	}
	if _, err := game.Register(r); err != nil {
		return err
	}
	if _, err := competition.Register(r); err != nil {
		return err
	}
	if _, err := chat.Register(r); err != nil {
		return err
	}
	return nil
}

// publicRouter is the /api/v1 router; F002 adds the pkg/httpx middleware chain and features add routes.
func publicRouter() http.Handler {
	r := chi.NewRouter()
	r.NotFound(http.NotFound)
	return r
}
