// Command migrate applies Atlas migrations of one module database (F001 T025, F001-AC-03):
//
//	migrate --module=learning      # one database, role learning_migrator
//	migrate --all                  # every module, in order
//
// It is the only way schemas change; core-api, realtime and worker never migrate on startup.
// Before applying, it refuses to run when the database is at a newer revision than the code
// (edge case of spec F001: never touch a schema written by a newer release).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Modules with a database, in apply order.
var Modules = []string{"identity", "learning", "game", "competition", "chat"}

func main() {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	mod := fs.String("module", "", "module to migrate (identity|learning|game|competition|chat)")
	all := fs.Bool("all", false, "migrate every module")
	root := fs.String("root", envOr("EW_MIGRATIONS_ROOT", "."), "repository root containing modules/<module>/migrations")
	atlas := fs.String("atlas", envOr("EW_ATLAS_BIN", "atlas"), "atlas CLI binary")
	_ = fs.Parse(os.Args[1:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	targets, err := selectModules(*mod, *all)
	if err == nil {
		for _, m := range targets {
			if err = migrate(ctx, m, *root, *atlas, os.Getenv); err != nil {
				break
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func selectModules(mod string, all bool) ([]string, error) {
	switch {
	case all && mod != "":
		return nil, errors.New("use either --module or --all")
	case all:
		return Modules, nil
	case mod == "":
		return nil, errors.New("--module or --all is required")
	}
	for _, m := range Modules {
		if m == mod {
			return []string{m}, nil
		}
	}
	return nil, fmt.Errorf("unknown module %q", mod)
}

func migrate(ctx context.Context, module, root, atlasBin string, getenv func(string) string) error {
	dsn, err := migratorDSN(module, getenv)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "modules", module, "migrations")
	latest, err := latestVersion(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", module, err)
	}
	applied, err := appliedVersion(ctx, dsn)
	if err != nil {
		return fmt.Errorf("%s: read applied revision: %w", module, err)
	}
	if applied > latest {
		return fmt.Errorf("%s: database revision %s is newer than code (%s); refusing to migrate", module, applied, latest)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	argv := strings.Fields(atlasBin) // allows a wrapper such as "bash scripts/dev/atlas.sh"
	if len(argv) == 0 {
		return fmt.Errorf("%s: empty atlas command", module)
	}
	argv = append(argv, "migrate", "apply", "--dir", "file://"+filepath.ToSlash(abs), "--url", dsn)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	fmt.Printf("migrate: %s: applying up to %s (current %q)\n", module, latest, applied)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: atlas migrate apply: %w", module, err)
	}
	return nil
}

// migratorDSN reads EW_DB_<MODULE>_MIGRATOR_DSN (local) or EW_DB_<MODULE>_MIGRATOR_URL_FILE (secret).
func migratorDSN(module string, getenv func(string) string) (string, error) {
	key := "EW_DB_" + strings.ToUpper(module) + "_MIGRATOR"
	if v := strings.TrimSpace(getenv(key + "_DSN")); v != "" {
		return v, nil
	}
	if f := strings.TrimSpace(getenv(key + "_URL_FILE")); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("read %s_URL_FILE: %w", key, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return "", fmt.Errorf("%s: set %s_DSN or %s_URL_FILE", module, key, key)
}

func latestVersion(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read migration dir: %w", err)
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		if v, _, ok := strings.Cut(e.Name(), "_"); ok {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return "", errors.New("no migration files")
	}
	sort.Strings(versions)
	return versions[len(versions)-1], nil
}

// appliedVersion returns the newest applied Atlas revision, or "" for a fresh database.
func appliedVersion(ctx context.Context, dsn string) (string, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)
	var v string
	err = conn.QueryRow(ctx,
		`SELECT version FROM atlas_schema_revisions.atlas_schema_revisions
		 WHERE type <> 1 ORDER BY version DESC LIMIT 1`).Scan(&v)
	var pgErr *pgconn.PgError
	switch {
	case err == nil:
		return v, nil
	case errors.Is(err, pgx.ErrNoRows):
		return "", nil
	case errors.As(err, &pgErr) && (pgErr.Code == "42P01" || pgErr.Code == "3F000"):
		return "", nil
	default:
		return "", err
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
