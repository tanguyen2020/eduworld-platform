// Command glitchtip-bridge receives GlitchTip alert webhooks and opens one GitHub Issue per new production
// GlitchTip issue for the owning team (F131 T037, F131-AC-18, MONITORING §9). It is an ops tool of the Go
// module src/backend (no module import, no database) and runs on the ops node next to GlitchTip.
//
//	glitchtip-bridge run           webhook POST /hook/<token> on EW_HTTP_ADDR (default :8080);
//	                               /metrics and /healthz on EW_ADMIN_ADDR (default :9090)
//	glitchtip-bridge healthcheck   Swarm healthcheck: GET /healthz on the admin port
//
// Environment (secrets only as Docker secret files, AGENTS R3/R4):
//
//	EW_GLITCHTIP_BRIDGE_TOKEN_FILE     webhook token (path segment of the webhook URL configured in GlitchTip)
//	EW_GLITCHTIP_URL                   GlitchTip API base URL (e.g. http://glitchtip:8000)
//	EW_GLITCHTIP_API_TOKEN_FILE        GlitchTip API token (scope event:read)
//	EW_GITHUB_REPO                     owner/name of the repository receiving tickets
//	EW_GITHUB_TOKEN_FILE               GitHub token (issues: read and write on that repository)
//	EW_GITHUB_API_URL                  GitHub REST base URL (default https://api.github.com)
//	EW_GLITCHTIP_BRIDGE_ENVIRONMENTS   comma-separated GlitchTip environments that create tickets (default production)
//	EW_HTTP_ADDR, EW_ADMIN_ADDR
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
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/glitchtip-bridge/internal/bridge"
)

const apiTimeout = 10 * time.Second

var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)

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
		fmt.Fprintf(stderr, "glitchtip-bridge: unknown command %q (usage: glitchtip-bridge run | healthcheck)\n", cmd)
		return 2
	}
}

func healthcheck(getenv func(string) string) int {
	addr := envOr(getenv, "EW_ADMIN_ADDR", ":9090")
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

type settings struct {
	hookToken, glitchTipURL, glitchTipToken, repo, githubToken, githubURL string
	envs                                                                  []string
}

// load reads the configuration; errors name variables only, never values or file contents (F130-AC-11).
func load(getenv func(string) string) (settings, error) {
	var missing []string
	need := func(k string) string {
		v := strings.TrimSpace(getenv(k))
		if v == "" {
			missing = append(missing, k)
		}
		return v
	}
	s := settings{glitchTipURL: need("EW_GLITCHTIP_URL"), repo: need("EW_GITHUB_REPO"),
		githubURL: envOr(getenv, "EW_GITHUB_API_URL", "https://api.github.com")}
	files := map[string]*string{"EW_GLITCHTIP_BRIDGE_TOKEN_FILE": &s.hookToken, "EW_GLITCHTIP_API_TOKEN_FILE": &s.glitchTipToken,
		"EW_GITHUB_TOKEN_FILE": &s.githubToken}
	paths := map[string]string{}
	for k := range files {
		paths[k] = need(k)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return s, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	for k, dst := range files {
		v, err := config.ReadSecretFile(k, paths[k])
		if err != nil {
			return s, err
		}
		*dst = v
	}
	if len(s.hookToken) < 24 {
		return s, errors.New("EW_GLITCHTIP_BRIDGE_TOKEN_FILE: webhook token must be at least 24 characters")
	}
	if !repoName.MatchString(s.repo) {
		return s, errors.New("EW_GITHUB_REPO must be owner/name")
	}
	s.envs = strings.Split(envOr(getenv, "EW_GLITCHTIP_BRIDGE_ENVIRONMENTS", "production"), ",")
	return s, nil
}

func serve(getenv func(string) string, stderr io.Writer) int {
	log := slog.New(slog.NewJSONHandler(stderr, nil))
	s, err := load(getenv)
	if err != nil {
		log.Error("invalid configuration: refusing to start", "err", err)
		return 1
	}
	reg := prometheus.NewRegistry()
	b, err := bridge.New(bridge.Options{
		Token: s.hookToken, Environments: s.envs, Registerer: reg, Logger: log,
		Tracker: bridge.NewGlitchTip(s.glitchTipURL, s.glitchTipToken, apiTimeout),
		Tickets: bridge.NewGitHub(s.githubURL, s.repo, s.githubToken, apiTimeout),
	})
	if err != nil {
		log.Error("bridge", "err", err)
		return 1
	}
	admin := http.NewServeMux()
	admin.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	admin.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	hook := http.NewServeMux()
	hook.Handle("/hook/", b)
	servers := []*http.Server{
		{Addr: envOr(getenv, "EW_ADMIN_ADDR", ":9090"), Handler: admin, ReadHeaderTimeout: 5 * time.Second},
		// the handler calls GlitchTip and GitHub (≤ 3 GETs with retries + 1 POST per issue)
		{Addr: envOr(getenv, "EW_HTTP_ADDR", ":8080"), Handler: hook, ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout: 15 * time.Second, WriteTimeout: 2 * time.Minute},
	}
	errc := make(chan error, len(servers))
	for _, srv := range servers {
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Info("glitchtip-bridge started", "repo", s.repo, "environments", s.envs)
	code := 0
	select {
	case <-ctx.Done():
	case err := <-errc:
		log.Error("listener failed", "err", err)
		code = 1
	}
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, srv := range servers {
		_ = srv.Shutdown(sctx)
	}
	return code
}

func envOr(getenv func(string) string, k, def string) string {
	if v := strings.TrimSpace(getenv(k)); v != "" {
		return v
	}
	return def
}
