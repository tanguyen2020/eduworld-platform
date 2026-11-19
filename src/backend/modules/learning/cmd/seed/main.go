// Command seed loads the dev/pilot seed of module learning (F021-AC-23, FR-CUR-004, LOCAL_DEVELOPMENT §5): the subject
// catalog (seed/curriculum/subjects.yaml) and every curriculum file of seed/curriculum (GDPT2018_SAMPLE Toán 1–5)
// through the same import use case as the CMS — never SQL by hand — then publishes them. Running it again creates
// nothing (the version label of each file is already there).
//
//	go -C src/backend run ./modules/learning/cmd/seed [-dir modules/learning/seed/curriculum]
//
// Connections: EW_DB_LEARNING_DSN (learning_app) and EW_DB_LEARNING_PLATFORM_DSN (learning_platform, platform rows),
// or their _URL_FILE variants (LOCAL_DEVELOPMENT §3).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/runtime" // mixin hooks, interceptors
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/importer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func main() {
	dir := flag.String("dir", filepath.Join("modules", "learning", "seed", "curriculum"), "directory of the curriculum seed files")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *dir); err != nil {
		slog.Error("learning seed failed", "error", err)
		os.Exit(1)
	}
}

func dsn(key string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(key + "_DSN")); v != "" {
		return v, nil
	}
	if f := strings.TrimSpace(os.Getenv(key + "_URL_FILE")); f != "" {
		b, err := os.ReadFile(f) //nolint:gosec // path from the operator's environment
		if err != nil {
			return "", fmt.Errorf("read %s_URL_FILE: %w", key, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return "", fmt.Errorf("set %s_DSN or %s_URL_FILE", key, key)
}

func open(key string) (*sql.DB, error) {
	d, err := dsn(key)
	if err != nil {
		return nil, err
	}
	return sql.Open("pgx", d)
}

func run(ctx context.Context, dir string) error {
	app, err := open("EW_DB_LEARNING")
	if err != nil {
		return err
	}
	defer func() { _ = app.Close() }()
	platform, err := open("EW_DB_LEARNING_PLATFORM")
	if err != nil {
		return err
	}
	defer func() { _ = platform.Close() }()
	w, err := outbox.NewWriter("learning", "core", nil)
	if err != nil {
		return err
	}
	rec := outbox.NewAuditRecorder(w, app)
	tenancy.Configure(tenancy.Hooks{Recorder: rec})
	uow, err := entx.NewUnitOfWork(entx.Options{Module: "learning", DBs: entx.DBs{App: app, Platform: platform}, Events: w, Audit: rec})
	if err != nil {
		return err
	}
	h := command.Curriculum{UoW: uow, Store: write.CurriculumRepo{}, Catalog: write.CatalogRepo{}, Imports: write.ImportJobRepo{},
		Events: write.Publisher{W: w}, Audit: write.Auditor{R: rec}, Codec: importer.Codec{}}
	res, err := job.Seed(ctx, h, dir)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "learning seed done", "subjects_created", res.SubjectsCreated, "curricula", len(res.Curricula))
	return nil
}
