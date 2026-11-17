// Package bootstrap runs one EduWorld binary (F001 T014): admin port with health/metrics/version,
// optional public HTTP listener, Valkey and NATS readiness, module registration and graceful stop.
// F002 replaces the env reading/logging here with pkg/config and pkg/observability; F003 adds the
// full shutdown sequence (drain consumers, outbox relay).
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
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
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

	// Ready, when set, receives the bound admin and public addresses once listening (tests).
	Ready func(adminAddr, httpAddr string)
}

// ShutdownTimeout bounds the graceful stop of the listeners.
const ShutdownTimeout = 10 * time.Second

// Run blocks until ctx is cancelled or a listener fails.
func Run(ctx context.Context, o Options) error {
	if o.Getenv == nil {
		return errors.New("bootstrap: Getenv is required")
	}
	log := newLogger(o)
	info := buildinfo.Get(o.Binary, o.Profile)

	hr := health.NewRegistry(info)
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

	if o.Register != nil {
		if err := o.Register(reg); err != nil {
			return fmt.Errorf("register modules: %w", err)
		}
	}

	adminLn, err := net.Listen("tcp", envOr(o.Getenv, "EW_ADMIN_ADDR", o.DefaultAdminAddr))
	if err != nil {
		return fmt.Errorf("admin listener: %w", err)
	}
	admin := &http.Server{Handler: hr.Handler(), ReadHeaderTimeout: 5 * time.Second}

	var public *http.Server
	var publicLn net.Listener
	if o.DefaultHTTPAddr != "" {
		publicLn, err = net.Listen("tcp", envOr(o.Getenv, "EW_HTTP_ADDR", o.DefaultHTTPAddr))
		if err != nil {
			_ = adminLn.Close()
			return fmt.Errorf("public listener: %w", err)
		}
		h := o.Public
		if h == nil {
			h = http.NotFoundHandler()
		}
		public = &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
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
		"version", info.Version, "commit", info.Commit,
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
	sctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	if public != nil {
		_ = public.Shutdown(sctx)
	}
	_ = admin.Shutdown(sctx)
	return nil
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

func newLogger(o Options) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(o.Getenv("EW_LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	hopts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(o.Getenv("EW_LOG_FORMAT"), "text") {
		return slog.New(slog.NewTextHandler(out, hopts))
	}
	return slog.New(slog.NewJSONHandler(out, hopts))
}
