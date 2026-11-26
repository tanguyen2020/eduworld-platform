// Command swarm-exporter exports the Swarm service, task and node state as Prometheus metrics (F131 T016,
// MONITORING §1, research R2). It is an ops tool of the Go module src/backend (no module import, no database)
// and reads the Docker Engine API of a manager only through the HAProxy allowlist proxy docker-api-proxy, with
// GET /services, GET /tasks and GET /nodes — never a Docker socket, never a write.
//
//	swarm-exporter run           poll loop + /metrics, /livez on EW_ADMIN_ADDR (default :9090)
//	swarm-exporter healthcheck   Swarm healthcheck: GET /livez on the admin port
//
// Environment: EW_SWARM_EXPORTER_DOCKER_API (required, e.g. http://docker-api-proxy:2375),
// EW_SWARM_EXPORTER_INTERVAL (poll interval, default 15s), EW_SWARM_EXPORTER_STACK (stack namespace whose
// services are exported and whose "<stack>_" prefix is stripped, default eduworld), EW_ADMIN_ADDR.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/swarm-exporter/internal/collector"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/swarm-exporter/internal/dockerapi"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, os.Getenv))
}

func run(args []string, stderr io.Writer, getenv func(string) string) int {
	cmd := "run"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "healthcheck":
		return healthcheck(getenv)
	case "run":
		return serve(getenv, stderr)
	default:
		fmt.Fprintf(stderr, "swarm-exporter: unknown command %q (usage: swarm-exporter run | healthcheck)\n", cmd)
		return 2
	}
}

func healthcheck(getenv func(string) string) int {
	addr := envOr(getenv, "EW_ADMIN_ADDR", ":9090")
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://" + addr + "/livez")
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func serve(getenv func(string) string, stderr io.Writer) int {
	log := slog.New(slog.NewJSONHandler(stderr, nil))
	api := getenv("EW_SWARM_EXPORTER_DOCKER_API")
	if api == "" {
		log.Error("missing required environment variables: EW_SWARM_EXPORTER_DOCKER_API")
		return 1
	}
	interval, err := time.ParseDuration(envOr(getenv, "EW_SWARM_EXPORTER_INTERVAL", "15s"))
	if err != nil || interval < time.Second {
		log.Error("EW_SWARM_EXPORTER_INTERVAL must be a duration ≥ 1s")
		return 1
	}
	stack := envOr(getenv, "EW_SWARM_EXPORTER_STACK", "eduworld")

	reg := prometheus.NewRegistry()
	col, err := collector.New(dockerapi.New(api), stack, reg)
	if err != nil {
		log.Error("collector", "err", err)
		return 1
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: envOr(getenv, "EW_ADMIN_ADDR", ":9090"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("admin server", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Info("swarm-exporter started", "stack", stack, "interval", interval.String())
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		pctx, cancel := context.WithTimeout(ctx, interval)
		if err := col.Poll(pctx); err != nil {
			log.Warn("docker api poll failed; keeping the last values", "err", err)
		}
		cancel()
		select {
		case <-ctx.Done():
			sctx, c := context.WithTimeout(context.Background(), 5*time.Second)
			_ = srv.Shutdown(sctx)
			c()
			return 0
		case <-t.C:
		}
	}
}

func envOr(getenv func(string) string, k, def string) string {
	if v := strings.TrimSpace(getenv(k)); v != "" {
		return v
	}
	return def
}
