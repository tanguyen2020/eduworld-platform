// Command mastery is the operations CLI of the mastery model (F031 US5, LEARNING_ANALYTICS §3.4): there is no public API
// or portal screen to change the model in use.
//
//	go -C src/backend run ./modules/learning/cmd/mastery add-model -version we-1.1 -params params.json
//	go -C src/backend run ./modules/learning/cmd/mastery recompute -version we-1.1 [-by <operator user id>]
//	go -C src/backend run ./modules/learning/cmd/mastery activate -run <run id> -by <System Admin user id>
//
// recompute writes a shadow run and prints its report (JSON) without changing the mastery in use; activate makes the
// model of a completed run active, applies it tenant by tenant and records the audit mastery_model.activated.
// Connections: EW_DB_LEARNING_DSN (learning_app) and EW_DB_LEARNING_PLATFORM_DSN (learning_platform), or their _URL_FILE
// variants (LOCAL_DEVELOPMENT §3).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/runtime" // mixin hooks, interceptors
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("mastery command failed", "error", err)
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

// handler builds the use case on the two pools.
func handler(app, platform *sql.DB) (command.Recompute, error) {
	w, err := outbox.NewWriter("learning", "core", nil)
	if err != nil {
		return command.Recompute{}, err
	}
	rec := outbox.NewAuditRecorder(w, app)
	tenancy.Configure(tenancy.Hooks{Recorder: rec})
	uow, err := entx.NewUnitOfWork(entx.Options{Module: "learning", DBs: entx.DBs{App: app, Platform: platform}, Events: w, Audit: rec})
	if err != nil {
		return command.Recompute{}, err
	}
	repo := write.MasteryRepo{}
	return command.Recompute{UoW: uow, Store: write.RecomputeRepo{}, Audit: write.Auditor{R: rec},
		Mastery: command.Mastery{Store: repo, Evidence: repo, Learning: write.LearningEventWriter{}, Events: write.Publisher{W: w}}}, nil
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mastery add-model|recompute|activate [flags]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	version := fs.String("version", "", "model version (add-model, recompute)")
	params := fs.String("params", "", "JSON file of the model parameters (add-model)")
	runID := fs.String("run", "", "recompute run id (activate)")
	by := fs.String("by", "", "user id of the operator (System Admin for activate)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
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
	h, err := handler(app, platform)
	if err != nil {
		return err
	}
	var operator *uuid.UUID
	if *by != "" {
		id, err := uuid.Parse(*by)
		if err != nil {
			return fmt.Errorf("-by: %w", err)
		}
		operator = &id
	}
	var out any
	switch args[0] {
	case "add-model":
		b, err := os.ReadFile(*params) //nolint:gosec // path given by the operator
		if err != nil {
			return err
		}
		if err := h.AddModel(ctx, *version, b); err != nil {
			return err
		}
		out = map[string]string{"model_version": *version, "status": "draft"}
	case "recompute":
		if out, err = h.Shadow(ctx, *version, operator); err != nil {
			return err
		}
	case "activate":
		id, err := uuid.Parse(*runID)
		if err != nil {
			return fmt.Errorf("-run: %w", err)
		}
		if operator == nil {
			return errors.New("-by is required to activate a model")
		}
		if out, err = h.Activate(ctx, id, *operator); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
