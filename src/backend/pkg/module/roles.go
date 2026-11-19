package module

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Entry points a module declares on the registry (BACKEND_DESIGN §3.3, ADR-0074 §2). A declaration is
// recorded in every binary (wiring tests compare it with the specs) but only runs where its role is on:
//
//	HTTP     → core-api
//	Relay    → worker --profile=events|all
//	Consumer → worker --profile=events|all
//	Job      → worker --profile=jobs|all
const (
	KindHTTP     = "http"
	KindRelay    = "relay"
	KindConsumer = "consumer"
	KindJob      = "job"
)

// Declaration is one entry point declared by a module.
type Declaration struct {
	Module, Kind, Name string
	Active             bool          // runs in this binary/profile
	Interval           time.Duration // jobs
}

// Mount mounts the public operations of one module on the shared pkg/httpx server.
type Mount func(s *httpx.Server) error

// Core holds the write and read contexts of one module database (ADR-0087): role pools, outbox writer,
// audit recorder, UnitOfWork, ReadSession and the idempotency store of the module.
type Core struct {
	Module              string
	App, Platform, Read *sql.DB // Read falls back to App when no replica is configured
	Writer              *outbox.Writer
	Recorder            *outbox.AuditRecorder
	UoW                 *entx.UnitOfWork
	ReadSession         *entx.ReadSession
	Idempotency         *idempotency.Store
}

// ServesHTTP reports whether the binary serves the public API (core-api).
func (r *Registry) ServesHTTP() bool { return r.binary == "core-api" }

// RunsEvents reports whether the binary runs relays and consumers (worker events|all).
func (r *Registry) RunsEvents() bool {
	return r.binary == "worker" && (r.profile == "events" || r.profile == "all")
}

// RunsJobs reports whether the binary runs scheduled jobs (worker jobs|all).
func (r *Registry) RunsJobs() bool {
	return r.binary == "worker" && (r.profile == "jobs" || r.profile == "all")
}

// Core opens the app and platform pools (and the read pool) of module and builds its write/read contexts,
// once. The audit recorder of the module is registered for the tenancy/authz hooks (AuditRecorder).
func (r *Registry) Core(module string) (*Core, error) {
	r.mu.Lock()
	if c, ok := r.res.cores[module]; ok {
		r.mu.Unlock()
		return c, nil
	}
	r.mu.Unlock()
	app, err := r.SQL(module, RoleApp)
	if err != nil {
		return nil, err
	}
	platform, err := r.SQL(module, RolePlatform)
	if err != nil {
		return nil, err
	}
	readDB, err := r.SQL(module, RoleRead)
	if err != nil {
		return nil, err
	}
	w, err := outbox.NewWriter(module, "core", nil)
	if err != nil {
		return nil, err
	}
	rec := outbox.NewAuditRecorder(w, app)
	dbs := entx.DBs{App: app, Platform: platform, Read: readDB}
	uow, err := entx.NewUnitOfWork(entx.Options{Module: module, DBs: dbs, Events: w, Audit: rec})
	if err != nil {
		return nil, err
	}
	rs, err := entx.NewReadSession(module, dbs)
	if err != nil {
		return nil, err
	}
	c := &Core{Module: module, App: app, Platform: platform, Read: readDB, Writer: w, Recorder: rec, UoW: uow, ReadSession: rs,
		Idempotency: idempotency.NewStore(app, 0, nil)}
	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.res.cores[module]; ok {
		return prev, nil
	}
	r.res.cores[module] = c
	return c, nil
}

func (r *Registry) declare(d Declaration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.res.decls = append(r.res.decls, d)
}

// Declarations returns every entry point declared so far, sorted by module, kind and name.
func (r *Registry) Declarations() []Declaration {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]Declaration(nil), r.res.decls...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	return out
}

// HTTP declares the public operations of module; core-api mounts them with MountHTTP.
func (r *Registry) HTTP(module string, m Mount) {
	r.declare(Declaration{Module: module, Kind: KindHTTP, Name: module, Active: r.ServesHTTP()})
	if !r.ServesHTTP() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.res.mounts = append(r.res.mounts, namedMount{module: module, mount: m})
}

type namedMount struct {
	module string
	mount  Mount
}

// MountHTTP mounts every declared module on s (core-api composition root, after the facades are wired).
func (r *Registry) MountHTTP(s *httpx.Server) error {
	r.mu.Lock()
	mounts := append([]namedMount(nil), r.res.mounts...)
	r.mu.Unlock()
	for _, m := range mounts {
		if err := m.mount(s); err != nil {
			return fmt.Errorf("%s: mount http: %w", m.module, err)
		}
	}
	return nil
}

// IdempotencyStores returns the idempotency store of every module with a Core (httpx.Deps.IdempotencyStores).
func (r *Registry) IdempotencyStores() map[string]*idempotency.Store {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]*idempotency.Store, len(r.res.cores))
	for m, c := range r.res.cores {
		out[m] = c.Idempotency
	}
	return out
}

// AuditRecorder records an audit event with the recorder of the module of ctx (tenancy.ModuleFrom: set by
// httpx per operation, by consumers and by ReadSession/UnitOfWork), so tenancy.Bypass and authz.denied of a
// module are written into that module's outbox. An event without a known module is refused (never written
// into another module's database).
func (r *Registry) AuditRecorder() audit.Recorder { return moduleRecorder{r: r} }

type moduleRecorder struct{ r *Registry }

func (m moduleRecorder) Record(ctx context.Context, e audit.Event) error {
	mod := tenancy.ModuleFrom(ctx)
	m.r.mu.Lock()
	c, ok := m.r.res.cores[mod]
	m.r.mu.Unlock()
	if !ok {
		return fmt.Errorf("module: no audit recorder for module %q", mod)
	}
	return c.Recorder.Record(ctx, e)
}

// Relay declares the outbox relay of module: it ensures EVT_<MODULE>/DLQ_<MODULE> and publishes
// infra.outbox_events with role <module>_relay (EW_DB_<MODULE>_RELAY_DSN).
func (r *Registry) Relay(module string) error {
	r.declare(Declaration{Module: module, Kind: KindRelay, Name: module + ".outbox-relay", Active: r.RunsEvents()})
	if !r.RunsEvents() {
		return nil
	}
	db, err := r.SQL(module, RoleRelay)
	if err != nil {
		return err
	}
	reg := r.Metrics()
	r.Go(module+".outbox-relay", func(ctx context.Context) error {
		js, err := r.JetStream()
		if err != nil {
			return err
		}
		if err := natsx.EnsureStreams(ctx, js, module, natsx.StreamOptions{Replicas: r.Config().Int("EW_NATS_STREAM_REPLICAS", 1)}); err != nil {
			return err
		}
		relay, err := outbox.NewRelay(outbox.RelayOptions{Module: module, DB: db, JS: js, Registerer: reg})
		if err != nil {
			return err
		}
		return relay.Run(ctx)
	})
	return nil
}

// ConsumerFactory builds the options of a consumer once JetStream is available.
type ConsumerFactory func(js jetstream.JetStream) outbox.ConsumerOptions

// Consumer declares a durable outbox consumer (pkg/outbox: processed_events, DLQ). The stream of the producer
// is created by the producer's relay; until it exists the consumer is retried.
func (r *Registry) Consumer(name string, build ConsumerFactory) {
	module, _, _ := strings.Cut(name, ".")
	r.declare(Declaration{Module: module, Kind: KindConsumer, Name: name, Active: r.RunsEvents()})
	if !r.RunsEvents() {
		return
	}
	reg := r.Metrics()
	r.Go(name, func(ctx context.Context) error {
		js, err := r.JetStream()
		if err != nil {
			return err
		}
		o := build(js)
		if base, _, _ := strings.Cut(name, "@"); o.Name != base { // "<consumer>@<stream>": one durable per stream
			return fmt.Errorf("consumer %s: options name %s", name, o.Name)
		}
		if o.Registerer == nil {
			o.Registerer = reg
		}
		c, err := outbox.NewConsumer(ctx, o)
		if err != nil {
			return err
		}
		return c.Run(ctx)
	})
}

// Background declares a worker outside the outbox (e.g. a JetStream work-queue consumer) that runs where
// active is true.
func (r *Registry) Background(module, kind, name string, active bool, run func(ctx context.Context) error) {
	r.declare(Declaration{Module: module, Kind: kind, Name: name, Active: active})
	if active {
		r.Go(name, run)
	}
}

// JobFunc is one run of a scheduled job; it returns the number of items handled.
type JobFunc func(ctx context.Context) (int, error)

// Job declares a periodic job: it runs at start and then every interval (worker jobs|all). A failed run is
// logged and retried at the next tick. Leader election (DEPLOYMENT §7 singleton locks) is [Proposed]: the
// jobs are safe to run concurrently (FOR UPDATE re-checks, idempotent sagas) until it exists.
func (r *Registry) Job(module, name string, interval time.Duration, run JobFunc) {
	r.declare(Declaration{Module: module, Kind: KindJob, Name: module + "." + name, Active: r.RunsJobs(), Interval: interval})
	if !r.RunsJobs() || interval <= 0 {
		return
	}
	_ = r.Metrics().Register(jobRuns)
	r.Go(module+"."+name, func(ctx context.Context) error {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			ctx := tenancy.WithModule(ctx, module)
			n, err := run(ctx)
			jobRuns.WithLabelValues(module, name, result(err)).Inc()
			if err != nil {
				slog.WarnContext(ctx, "job failed", "module", module, "job", name, "error", err)
			} else {
				slog.DebugContext(ctx, "job tick", "module", module, "job", name, "count", n)
				if n > 0 {
					slog.InfoContext(ctx, "job done", "module", module, "job", name, "count", n)
				}
			}
			select {
			case <-ctx.Done():
				return nil
			case <-t.C:
			}
		}
	})
}

var jobRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "eduworld_job_runs_total", Help: "Runs of scheduled jobs by module, job and result (ok, error).",
}, []string{"module", "job", "result"})

func result(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
