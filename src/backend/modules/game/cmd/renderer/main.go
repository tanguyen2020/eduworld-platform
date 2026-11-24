// Command renderer is the entrypoint of the game-renderer container (F078 US5, FR-CMS-017, research R2,
// DESIGN_SYSTEM §7): it pulls render jobs from the JetStream work queue GAME_RENDER_JOBS (durable consumer
// game_renderer, subject ew.job.game.render.request), downloads the 3D source of the published version from object
// storage (SHA-256 checked), runs Godot with src/game/tools/thumbnail_renderer/render_thumbnails.gd, checks the PNG
// (512×512 thumbnail or 3072×2048 turntable sheet), uploads it content-addressed to ew-media-public
// (media/<sha256>/<kind>.png, immutable) and answers one Result on ew.job.game.render.result. It never touches the
// database of module game: the worker records the result (R13).
//
//	go -C src/backend run ./modules/game/cmd/renderer        (needs Godot with a renderer: see GodotRunner)
//	docker compose -f deploy/compose/docker-compose.yml -f deploy/compose/game-renderer.yml up game-renderer
//
// Configuration (environment, AGENTS R3; X may be given as a file through X_FILE):
//
//	EW_NATS_URL (or EW_NATS_URLS)  NATS server(s), required; EW_NATS_CREDS_FILE optional
//	EW_S3_ENDPOINT, EW_S3_REGION, EW_S3_ACCESS_KEY, EW_S3_SECRET_KEY   object storage (endpoint and keys required)
//	EW_RENDERER_GODOT    Godot command, split on blanks (default "godot"; the image sets its xvfb wrapper)
//	EW_RENDERER_PROJECT  Godot project path (default /opt/eduworld/game)
//	EW_RENDERER_TIMEOUT  Godot run per job (default 120s)
//	EW_RENDERER_WORKDIR  parent of the per-job temporary directories (default: system temp dir)
//	EW_ADMIN_ADDR        health and metrics (default :9090); EW_LOG_LEVEL, EW_LOG_FORMAT, EW_ENV as every binary
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/bootstrap"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/lifecycle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

const (
	binaryName       = "game-renderer"
	defaultAdminAddr = ":9090"
	defaultGodot     = "godot"
	defaultProject   = "/opt/eduworld/game"
	defaultTimeout   = 120 * time.Second
	s3Timeout        = 60 * time.Second
)

func main() {
	bootstrap.MaybeHealthcheck(defaultAdminAddr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, os.Stdout, nil); err != nil {
		fmt.Fprintln(os.Stderr, binaryName+":", err)
		stop()
		os.Exit(1)
	}
}

// Config of the container. Secrets (S3 keys, NATS credentials) are never logged.
type Config struct {
	NATSURL   string
	NATSCreds string
	S3        storage.Config
	Godot     []string
	Project   string
	Timeout   time.Duration
	WorkDir   string
	AdminAddr string
	Shutdown  time.Duration
}

// LoadConfig reads the environment; missing required variables are reported by name only.
func LoadConfig(getenv func(string) string) (Config, error) {
	c := config.FromGetenv(getenv)
	cfg := Config{
		NATSURL:   c.StringOr("EW_NATS_URL", c.String("EW_NATS_URLS")),
		NATSCreds: c.String("EW_NATS_CREDS_FILE"),
		S3: storage.Config{Endpoint: c.String("EW_S3_ENDPOINT"), Region: c.String("EW_S3_REGION"),
			AccessKey: c.String("EW_S3_ACCESS_KEY"), SecretKey: c.String("EW_S3_SECRET_KEY"), Timeout: s3Timeout},
		Godot:     strings.Fields(c.StringOr("EW_RENDERER_GODOT", defaultGodot)),
		Project:   c.StringOr("EW_RENDERER_PROJECT", defaultProject),
		Timeout:   c.Duration("EW_RENDERER_TIMEOUT", defaultTimeout),
		WorkDir:   c.StringOr("EW_RENDERER_WORKDIR", os.TempDir()),
		AdminAddr: c.StringOr("EW_ADMIN_ADDR", defaultAdminAddr),
		Shutdown:  c.Duration("EW_WORKER_SHUTDOWN_TIMEOUT", 30*time.Second),
	}
	required := []string{"EW_S3_ENDPOINT", "EW_S3_ACCESS_KEY", "EW_S3_SECRET_KEY"}
	if cfg.NATSURL == "" {
		required = append([]string{"EW_NATS_URL"}, required...)
	}
	if err := c.Require(required...); err != nil {
		return Config{}, err
	}
	if len(cfg.Godot) == 0 {
		cfg.Godot = []string{defaultGodot}
	}
	if cfg.Timeout <= 0 {
		return Config{}, errors.New("EW_RENDERER_TIMEOUT must be a positive duration")
	}
	return cfg, nil
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer, ready func(adminAddr string)) error {
	cfg, err := LoadConfig(getenv)
	if err != nil {
		return err
	}
	info := buildinfo.Get(binaryName, "")
	level := slog.LevelInfo
	if strings.EqualFold(getenv("EW_LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}
	log := observability.NewLogger(stdout, observability.LoggerOptions{Binary: binaryName, Env: getenv("EW_ENV"),
		Version: info.Version, Level: level, Text: strings.EqualFold(getenv("EW_LOG_FORMAT"), "text")})
	slog.SetDefault(log)
	hr := health.NewRegistry(info)

	if err := os.MkdirAll(cfg.WorkDir, 0o700); err != nil {
		return fmt.Errorf("workdir: %w", err)
	}
	s3, err := storage.New(cfg.S3, &http.Client{})
	if err != nil {
		return err
	}
	opts := []nats.Option{nats.Name(binaryName), nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second)}
	if cfg.NATSCreds != "" {
		opts = append(opts, nats.UserCredentials(cfg.NATSCreds))
	}
	nc, err := nats.Connect(cfg.NATSURL, opts...)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer nc.Close()
	hr.Register("nats", health.Readiness, health.NATSCheck(nc))
	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	worker := &Worker{
		Processor: &Processor{Storage: s3, Runner: GodotRunner{Command: cfg.Godot, Project: cfg.Project},
			WorkDir: cfg.WorkDir, Timeout: cfg.Timeout, Log: log},
		Publisher: JetStreamPublisher{JS: js}, Log: log, Metrics: NewMetrics(hr.Metrics()), RepublishDelay: 30 * time.Second,
	}
	workers := lifecycle.NewGroup(ctx)
	workers.Go("render", supervise(log, "render", func(ctx context.Context) error {
		return Consume(ctx, js, worker, cfg.Timeout)
	}))

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.AdminAddr)
	if err != nil {
		_ = workers.Stop(cfg.Shutdown)
		return fmt.Errorf("admin listener: %w", err)
	}
	admin := &http.Server{Handler: hr.Handler(), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() {
		if err := admin.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	log.Info("started", "binary", binaryName, "version", info.Version, "commit", info.Commit,
		"admin_addr", ln.Addr().String(), "s3_endpoint", cfg.S3.Endpoint, "godot", cfg.Godot[0],
		"project", cfg.Project, "timeout", cfg.Timeout.String())
	if ready != nil {
		ready(ln.Addr().String())
	}

	select {
	case <-ctx.Done():
	case err := <-errc:
		log.Error("admin listener failed", "error", err)
		_ = lifecycle.Plan{Readiness: hr, Workers: workers, Admin: admin, NATS: nc, Logger: log, Timeout: cfg.Shutdown}.Run()
		return err
	}
	log.Info("stopping")
	return lifecycle.Plan{Readiness: hr, Workers: workers, Admin: admin, NATS: nc, Logger: log, Timeout: cfg.Shutdown}.Run()
}

// supervise reruns the consumer when it fails before shutdown (NATS not reachable yet), with a capped backoff.
func supervise(log *slog.Logger, name string, run func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		delay := time.Second
		for {
			err := run(ctx)
			if err == nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err() // shutdown: ignored by lifecycle.Group
			}
			log.WarnContext(ctx, "worker failed, restarting", "worker", name, "error", err, "retry_in", delay.String())
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
			delay = min(2*delay, 30*time.Second)
		}
	}
}
