// Package bootstrap runs one EduWorld binary (F001 T014): admin port with health/metrics/version,
// optional public HTTP listener, Valkey and NATS readiness, module registration and graceful stop.
// Logging, tracing and HTTP metrics come from pkg/observability (F002); F003 adds the full shutdown
// sequence (drain consumers, outbox relay).
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/lifecycle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Options describes one binary.
type Options struct {
	Binary  string // core-api, realtime, worker
	Profile string // worker only

	// DefaultAdminAddr is used when EW_ADMIN_ADDR is unset (native run: 9100/9106/9200).
	DefaultAdminAddr string
	// DefaultHTTPAddr enables the public listener (core-api :8100, realtime :8106); empty = none.
	DefaultHTTPAddr string
	// Public is the public handler (routes are added by features; F001 serves 404 only).
	Public http.Handler

	// Modules are the module names loaded by this binary (startup log only).
	Modules []string
	// Register calls modules/<module>.Register for every loaded module.
	Register func(r *module.Registry) error

	Getenv func(string) string
	Stdout io.Writer

	// Workers run until shutdown (outbox relay, consumers, jobs); they must return after finishing
	// in-flight work when their context is cancelled (F003-AC-30).
	Workers []Worker

	// Ready, when set, receives the bound admin and public addresses once listening (tests).
	Ready func(adminAddr, httpAddr string)
}

// Worker is a named background loop.
type Worker struct {
	Name string
	Run  func(ctx context.Context) error
}

// Run blocks until ctx is cancelled or a listener fails.
func Run(ctx context.Context, o Options) error {
	if o.Getenv == nil {
		return errors.New("bootstrap: Getenv is required")
	}
	info := buildinfo.Get(o.Binary, o.Profile)
	log := newLogger(o, info)
	slog.SetDefault(log)

	tracker, err := newErrorTracker(o, info)
	if err != nil {
		return err
	}
	observability.SetErrorTracker(tracker)
	defer func() {
		tracker.Flush(2 * time.Second)
		observability.SetErrorTracker(nil)
	}()

	hr := health.NewRegistry(info)
	obs, err := newObs(ctx, o, log, hr)
	if err != nil {
		return fmt.Errorf("observability: %w", err)
	}
	defer obs.Shutdown()
	reg := module.NewRegistry(o.Binary, o.Profile, hr, o.Getenv)
	defer reg.Close()

	if addr := strings.TrimSpace(o.Getenv("EW_VALKEY_ADDR")); addr != "" {
		v := NewValkeyPinger(addr)
		defer v.Close()
		hr.Register("valkey", health.Readiness, health.ValkeyCheck(v))
	} else {
		hr.Register("valkey", health.Readiness, health.ValkeyCheck(nil))
	}

	nc, err := connectNATS(o)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer nc.Close()
	hr.Register("nats", health.Readiness, health.NATSCheck(nc))
	// Shared adapter metrics (MONITORING §3): NATS connection (alert NatsClusterDown), NATS request/reply (alert
	// NatsRequestTimeouts) and circuit breakers of the external integrations (alert CircuitBreakerOpen).
	hr.Metrics().MustRegister(health.NATSConnectedGauge(nc))
	hr.Metrics().MustRegister(natsx.RequestCollectors()...)
	hr.Metrics().MustRegister(observability.ExternalCollectors()...)

	workers := lifecycle.NewGroup(ctx)
	for _, w := range o.Workers {
		workers.Go(w.Name, w.Run)
	}

	reg.SetNATS(nc)
	if o.Register != nil {
		if err := o.Register(reg); err != nil {
			return fmt.Errorf("register modules: %w", err)
		}
	}
	// Module workers (outbox relays, consumers, jobs) start once every module is registered and wired.
	for _, w := range reg.Workers() {
		workers.Go(w.Name, w.Run)
	}

	var lc net.ListenConfig
	adminLn, err := lc.Listen(ctx, "tcp", envOr(o.Getenv, "EW_ADMIN_ADDR", o.DefaultAdminAddr))
	if err != nil {
		return fmt.Errorf("admin listener: %w", err)
	}
	admin := &http.Server{Handler: hr.Handler(), ReadHeaderTimeout: 5 * time.Second}

	var public *http.Server
	var publicLn net.Listener
	if o.DefaultHTTPAddr != "" {
		publicLn, err = lc.Listen(ctx, "tcp", envOr(o.Getenv, "EW_HTTP_ADDR", o.DefaultHTTPAddr))
		if err != nil {
			_ = adminLn.Close()
			return fmt.Errorf("public listener: %w", err)
		}
		h := o.Public
		if p := reg.Public(); p != nil {
			h = p
		}
		if h == nil {
			h = http.NotFoundHandler()
		}
		public = &http.Server{Handler: obs.HTTP(h), ReadHeaderTimeout: 5 * time.Second}
	}

	errc := make(chan error, 2)
	go func() { errc <- serve(admin, adminLn) }()
	httpAddr := ""
	if public != nil {
		httpAddr = publicLn.Addr().String()
		go func() { errc <- serve(public, publicLn) }()
	}

	log.Info("started",
		"binary", info.Binary, "profile", info.Profile, "modules", o.Modules,
		"version", info.Version, "commit", info.Commit, "error_tracking", tracker != nil,
		"admin_addr", adminLn.Addr().String(), "http_addr", httpAddr,
		"checks", hr.CheckNames())
	if o.Ready != nil {
		o.Ready(adminLn.Addr().String(), httpAddr)
	}

	select {
	case <-ctx.Done():
	case err := <-errc:
		if err != nil {
			log.Error("listener failed", "error", err)
			return err
		}
	}

	log.Info("stopping")
	plan := lifecycle.Plan{Readiness: hr, Workers: workers, Admin: admin, NATS: nc, Close: []func(){reg.Close}, Logger: log,
		Timeout: envDuration(o.Getenv, "EW_WORKER_SHUTDOWN_TIMEOUT", 60*time.Second)}
	if public != nil {
		plan.Public = []*http.Server{public}
		plan.DrainDelay = envDuration(o.Getenv, "EW_SHUTDOWN_DRAIN_DELAY", 5*time.Second)
		plan.Timeout = envDuration(o.Getenv, "EW_HTTP_SHUTDOWN_TIMEOUT", 20*time.Second)
	}
	return plan.Run()
}

func envDuration(getenv func(string) string, key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(getenv(key))); err == nil && d >= 0 {
		return d
	}
	return def
}

func serve(s *http.Server, ln net.Listener) error {
	if err := s.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func connectNATS(o Options) (*nats.Conn, error) {
	urls := envOr(o.Getenv, "EW_NATS_URLS", nats.DefaultURL)
	opts := []nats.Option{
		nats.Name(o.Binary),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
	}
	if creds := strings.TrimSpace(o.Getenv("EW_NATS_CREDS_FILE")); creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	}
	return nats.Connect(urls, opts...)
}

func envOr(getenv func(string) string, key, def string) string {
	if v := strings.TrimSpace(getenv(key)); v != "" {
		return v
	}
	return def
}

func newLogger(o Options, info buildinfo.Info) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(o.Getenv("EW_LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	return observability.NewLogger(out, observability.LoggerOptions{
		Binary: o.Binary, Profile: o.Profile, Env: o.Getenv("EW_ENV"), Version: info.Version, Level: level,
		Text: strings.EqualFold(o.Getenv("EW_LOG_FORMAT"), "text"),
	})
}

// newObs exports spans with OTLP/HTTP when OTEL_EXPORTER_OTLP_ENDPOINT is set and registers the HTTP
// metrics on the admin registry.
func newObs(ctx context.Context, o Options, log *slog.Logger, hr *health.Registry) (*observability.Obs, error) {
	opts := observability.Options{Logger: log, Binary: o.Binary, Registerer: hr.Metrics()}
	if strings.TrimSpace(o.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" {
		p, err := observability.NewOTLPProcessor(ctx)
		if err != nil {
			return nil, err
		}
		opts.SpanProcessor = p
	}
	return observability.New(opts)
}

// newErrorTracker enables error tracking (GlitchTip, F131-AC-17) when EW_ERROR_TRACKING_DSN_FILE names the Docker
// secret holding the DSN; unset = disabled, no Sentry client and no network. An unreadable file stops the binary
// with an error naming the variable only (F130-AC-11).
func newErrorTracker(o Options, info buildinfo.Info) (*observability.ErrorTracker, error) {
	path := strings.TrimSpace(o.Getenv("EW_ERROR_TRACKING_DSN_FILE"))
	if path == "" {
		return nil, nil
	}
	dsn, err := config.ReadSecretFile("EW_ERROR_TRACKING_DSN_FILE", path)
	if err != nil {
		return nil, err
	}
	return observability.NewErrorTracker(observability.ErrorTrackingOptions{DSN: dsn, Binary: info.Binary, Profile: info.Profile,
		Env: o.Getenv("EW_ENV"), Version: info.Version})
}
