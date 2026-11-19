// Command seed loads the dev seed of module identity (LOCAL_DEVELOPMENT §5): the demo tenants, schools, platform
// and tenant staff with their roles, and the students of the SIS CSV files of modules/identity/seed, through the use
// cases of the module — never SQL by hand. The first platform staff (sa.admin) is the bootstrap System Admin; every
// other write is made on its behalf. Demo users and passwords come from the realm import (make keycloak-import):
// the seed only links them (keycloak.DevAdopter). Running it again changes nothing.
//
//	go -C src/backend run ./modules/identity/cmd/seed [-dir modules/identity/seed]
//
// Environment (same names as the binaries, LOCAL_DEVELOPMENT §4): EW_DB_IDENTITY_DSN (identity_app),
// EW_DB_IDENTITY_PLATFORM_DSN (identity_platform) or their _URL_FILE variants; EW_OIDC_ISSUERS (the first issuer is
// the one of the tokens core-api verifies); EW_KC_ADMIN_BASE_URL, EW_KC_ADMIN_CLIENT_ID, EW_KC_ADMIN_CLIENT_SECRET;
// EW_SIS_ID_HASH_KEY (optional). Refuses to run unless EW_ENV=local.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/keycloak"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/devseed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func main() {
	dir := flag.String("dir", filepath.Join("modules", "identity", "seed"), "directory of the identity seed (dev.yaml, sis/)")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *dir); err != nil {
		slog.Error("identity seed failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dir string) error {
	if env := os.Getenv("EW_ENV"); env != "local" {
		return errors.New("the dev seed links the demo users of the local realm: set EW_ENV=local (got " + env + ")")
	}
	plan, err := devseed.Load(dir)
	if err != nil {
		return err
	}
	reg := module.NewRegistry("seed", "", health.NewRegistry(buildinfo.Get("seed", "")), os.Getenv)
	defer reg.Close()
	core, err := reg.Core(identity.Name)
	if err != nil {
		return err
	}
	cfg := reg.Config()
	if err := cfg.Require("EW_OIDC_ISSUERS"); err != nil {
		return err
	}
	// Composition-root hooks, as in the binaries: bypasses and denials are audited through the outbox.
	tenancy.Configure(tenancy.Hooks{Recorder: core.Recorder})
	authz.Configure(authz.Hooks{Recorder: core.Recorder})
	kcCfg, err := keycloak.ConfigFrom(cfg)
	if err != nil {
		return err
	}
	kc, err := keycloak.New(kcCfg, nil)
	if err != nil {
		return err
	}
	s, err := identity.NewDevSeed(identity.DevSeedDeps{UoW: core.UoW, Read: core.ReadSession, Writer: core.Writer, Recorder: core.Recorder,
		KC: kc, Issuers: cfg.List("EW_OIDC_ISSUERS"), IDHash: []byte(cfg.String("EW_SIS_ID_HASH_KEY"))})
	if err != nil {
		return err
	}
	rep, err := s.Run(ctx, plan)
	devseed.LogReport(ctx, rep)
	return err
}
