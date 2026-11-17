// Package health serves the admin port of every binary (F001 T008, FR-OPS-001, DEPLOYMENT §14):
// /livez, /readyz, /healthz, /metrics, /version. The admin port is never routed through APISIX.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
)

// Kind decides whether a check gates readiness or is only reported in /healthz.
type Kind int

const (
	// Readiness checks gate /readyz (module databases, Valkey, NATS, Atlas revision).
	Readiness Kind = iota
	// Info checks are reported in /healthz only (e.g. external integrations).
	Info
)

// CheckFunc returns nil when the dependency is usable.
type CheckFunc func(ctx context.Context) error

// DefaultTimeout bounds one /readyz evaluation; checks run in parallel.
const DefaultTimeout = 2 * time.Second

type check struct {
	name string
	kind Kind
	fn   CheckFunc
}

// Registry collects checks for one binary and exposes the admin HTTP handler.
type Registry struct {
	info    buildinfo.Info
	timeout time.Duration
	metrics *prometheus.Registry

	mu     sync.RWMutex
	checks []check
	names  map[string]struct{}
}

// NewRegistry creates a registry with Go/process collectors and eduworld_build_info{binary,profile,version,commit} = 1.
func NewRegistry(info buildinfo.Info) *Registry {
	m := prometheus.NewRegistry()
	m.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	build := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "eduworld_build_info",
		Help: "Build metadata of the running binary; value is always 1.",
	}, []string{"binary", "profile", "version", "commit"})
	build.WithLabelValues(info.Binary, info.Profile, info.Version, info.Commit).Set(1)
	m.MustRegister(build)
	return &Registry{info: info, timeout: DefaultTimeout, metrics: m, names: map[string]struct{}{}}
}

// Metrics returns the Prometheus registry so packages can add collectors under the same /metrics.
func (r *Registry) Metrics() *prometheus.Registry { return r.metrics }

// SetTimeout overrides the per-evaluation timeout (tests, slow environments).
func (r *Registry) SetTimeout(d time.Duration) { r.timeout = d }

// Register adds a named check. Names follow `db:<module>`, `atlas:<module>`, `valkey`, `nats`.
// Registering the same name twice is a wiring bug and panics.
func (r *Registry) Register(name string, kind Kind, fn CheckFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.names[name]; dup {
		panic(fmt.Sprintf("health: check %q registered twice", name))
	}
	r.names[name] = struct{}{}
	r.checks = append(r.checks, check{name: name, kind: kind, fn: fn})
}

// Result of one check in the JSON body. error_code is a stable code, never the raw error,
// so DSNs, hosts and credentials are not exposed.
type Result struct {
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
}

// Report is the body of /readyz and /healthz.
type Report struct {
	Status string            `json:"status"`
	Checks map[string]Result `json:"checks"`
}

// Evaluate runs the selected checks in parallel, bounded by the registry timeout.
func (r *Registry) Evaluate(ctx context.Context, includeInfo bool) Report {
	r.mu.RLock()
	selected := make([]check, 0, len(r.checks))
	for _, c := range r.checks {
		if c.kind == Readiness || includeInfo {
			selected = append(selected, c)
		}
	}
	r.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	type out struct {
		c   check
		err error
	}
	results := make(chan out, len(selected))
	for _, c := range selected {
		go func(c check) { results <- out{c: c, err: run(ctx, c.fn)} }(c)
	}

	rep := Report{Status: "ok", Checks: make(map[string]Result, len(selected))}
	for range selected {
		o := <-results
		if o.err == nil {
			rep.Checks[o.c.name] = Result{Status: "ok"}
			continue
		}
		rep.Checks[o.c.name] = Result{Status: "fail", ErrorCode: ErrorCode(o.err)}
		if o.c.kind == Readiness {
			rep.Status = "fail"
		}
	}
	return rep
}

// run executes fn but returns as soon as ctx expires, even if fn ignores ctx.
func run(ctx context.Context, fn CheckFunc) (err error) {
	done := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Errorf("check panicked: %v", p)
			}
		}()
		done <- fn(ctx)
	}()
	select {
	case err = <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Handler serves the admin endpoints. Mount it only on EW_ADMIN_ADDR.
func (r *Registry) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, req *http.Request) {
		rep := r.Evaluate(req.Context(), false)
		writeJSON(w, statusFor(rep), rep)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, req *http.Request) {
		rep := r.Evaluate(req.Context(), true)
		writeJSON(w, http.StatusOK, rep)
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, r.info)
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(r.metrics, promhttp.HandlerOpts{}))
	return mux
}

// CheckNames lists registered check names (sorted), used in startup logs.
func (r *Registry) CheckNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.checks))
	for _, c := range r.checks {
		names = append(names, c.name)
	}
	sort.Strings(names)
	return names
}

func statusFor(rep Report) int {
	if rep.Status == "ok" {
		return http.StatusOK
	}
	return http.StatusServiceUnavailable
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
