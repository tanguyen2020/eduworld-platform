package module

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib" // database/sql over pgx for the role pools
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// Roles of a module database (LOCAL_DEVELOPMENT §4, ADR-0089): each role pool is opened from its own
// variable, never derived from another role's credentials.
const (
	RoleApp      = "app"      // EW_DB_<MODULE>_DSN: <module>_app (RLS, no DELETE)
	RolePlatform = "platform" // EW_DB_<MODULE>_PLATFORM_DSN: <module>_platform (BYPASSRLS, audited scopes)
	RoleRead     = "read"     // EW_DB_<MODULE>_READ_DSN: read replica of the read context; empty = primary (app)
	RoleRelay    = "relay"    // EW_DB_<MODULE>_RELAY_DSN: <module>_relay (outbox relay only, profile events)
)

// Worker is a named background loop registered by a module (relay, consumer, job); it runs until its
// context ends and must return after finishing in-flight work (F003-AC-30).
type Worker struct {
	Name string
	Run  func(ctx context.Context) error
}

type resources struct {
	sqls    map[string]*sql.DB
	nc      *nats.Conn
	js      jetstream.JetStream
	valkey  valkey.Client
	workers []Worker
	public  http.Handler
	cores   map[string]*Core
	decls   []Declaration
	mounts  []namedMount
}

// Config is the configuration of the binary (the getenv of bootstrap.Options, X_FILE secrets).
func (r *Registry) Config() *config.Config { return config.FromGetenv(r.getenv) }

// Metrics is the Prometheus registerer of the admin port (/metrics).
func (r *Registry) Metrics() prometheus.Registerer { return r.health.Metrics() }

// SQL returns the database/sql pool of the module database for role (RoleApp, RolePlatform, RoleRead,
// RoleRelay), created once. RoleRead falls back to RoleApp when no replica is configured. Pools connect
// lazily: a binary starts while the database is down and reports not ready (F001-AC-05).
func (r *Registry) SQL(module, role string) (*sql.DB, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := module + "/" + role
	if db, ok := r.res.sqls[key]; ok {
		return db, nil
	}
	prefix := "EW_DB_" + strings.ToUpper(module)
	if role != RoleApp {
		prefix += "_" + strings.ToUpper(role)
	}
	dsn, err := r.secret(prefix+"_DSN", prefix+"_URL_FILE")
	if err != nil {
		return nil, err
	}
	if dsn == "" && role == RoleRead {
		r.mu.Unlock()
		db, err := r.SQL(module, RoleApp)
		r.mu.Lock()
		if err == nil {
			r.res.sqls[key] = db
		}
		return db, err
	}
	if dsn == "" {
		return nil, fmt.Errorf("%s %s: %w (set %s_DSN or %s_URL_FILE)", module, role, ErrDatabaseNotConfigured, prefix, prefix)
	}
	cc, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s %s pool: parse config: %w", module, role, err)
	}
	cc.Tracer = observability.QueryTracer{Module: module} // eduworld_db_query_duration_seconds (MONITORING §3)
	db := stdlib.OpenDB(*cc)                              // lazy: no connection until first use (F001-AC-05)
	observability.NamePool(db, module, role)
	r.res.sqls[key] = db
	return db, nil
}

func (r *Registry) secret(direct, file string) (string, error) {
	if v := strings.TrimSpace(r.getenv(direct)); v != "" {
		return v, nil
	}
	if f := strings.TrimSpace(r.getenv(file)); f != "" {
		return config.ReadSecretFile(file, f) // missing or empty secret file: start-up error naming the variable (F130-AC-11)
	}
	return "", nil
}

// SetNATS installs the NATS connection of the binary (bootstrap, before Register).
func (r *Registry) SetNATS(nc *nats.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.res.nc, r.res.js = nc, nil
}

// NATS returns the NATS connection of the binary (nil when bootstrap has none, e.g. unit tests).
func (r *Registry) NATS() *nats.Conn {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.res.nc
}

// ErrNoNATS: JetStream requested but the binary has no NATS connection.
var ErrNoNATS = errors.New("module: no NATS connection")

// JetStream returns the JetStream context over the binary's NATS connection, created once.
func (r *Registry) JetStream() (jetstream.JetStream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.res.js != nil {
		return r.res.js, nil
	}
	if r.res.nc == nil {
		return nil, ErrNoNATS
	}
	js, err := jetstream.New(r.res.nc)
	if err != nil {
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	r.res.js = js
	return js, nil
}

// Valkey returns the Valkey client of the binary (EW_VALKEY_ADDR, optional EW_VALKEY_USERNAME and
// EW_VALKEY_PASSWORD(_FILE)), created once. Per-module ACL users (VALKEY_DESIGN) are [Proposed]: one client
// per binary until the ACL users exist in every environment.
func (r *Registry) Valkey() (valkey.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.res.valkey != nil {
		return r.res.valkey, nil
	}
	addr := strings.TrimSpace(r.getenv("EW_VALKEY_ADDR"))
	if addr == "" {
		return nil, errors.New("module: EW_VALKEY_ADDR is not set")
	}
	pw, err := r.secret("EW_VALKEY_PASSWORD", "EW_VALKEY_PASSWORD_FILE")
	if err != nil {
		return nil, err
	}
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, Username: strings.TrimSpace(r.getenv("EW_VALKEY_USERNAME")),
		Password: pw, DisableCache: true})
	if err != nil {
		return nil, fmt.Errorf("valkey: %w", err)
	}
	r.res.valkey = c
	return c, nil
}

// Go registers a background worker; bootstrap starts every worker after Register and drains them on shutdown.
func (r *Registry) Go(name string, run func(ctx context.Context) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.res.workers = append(r.res.workers, Worker{Name: name, Run: supervise(name, run)})
}

// Workers returns the registered background workers in registration order.
func (r *Registry) Workers() []Worker {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Worker(nil), r.res.workers...)
}

// SetPublic installs the public HTTP handler (core-api: the pkg/httpx server of every module).
func (r *Registry) SetPublic(h http.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.res.public = h
}

// Public returns the handler installed by SetPublic (nil when none).
func (r *Registry) Public() http.Handler {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.res.public
}

// closeResources is called by Close with r.mu held.
func (r *Registry) closeResources() {
	for k, db := range r.res.sqls {
		_ = db.Close()
		delete(r.res.sqls, k)
	}
	if r.res.valkey != nil {
		r.res.valkey.Close()
		r.res.valkey = nil
	}
}

// supervise reruns a worker that fails before its context ends (NATS or a database not reachable yet), with a
// capped backoff, so a transient start-up failure never leaves a relay or consumer silently stopped.
func supervise(name string, run func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		delay := time.Second
		for {
			err := run(ctx)
			if ctx.Err() != nil || err == nil {
				return err
			}
			slog.WarnContext(ctx, "worker failed, restarting", "worker", name, "error", err, "retry_in", delay.String())
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
			delay = min(2*delay, 30*time.Second)
		}
	}
}

// Cursors returns the keyset cursor codec of the binary: HMAC key EW_PAGE_CURSOR_KEY(_FILE), ≥ 32 bytes
// (pkg/page, API_DESIGN §5). Every replica of every binary must use the same key.
func (r *Registry) Cursors() (*page.Codec, error) {
	key, err := r.secret("EW_PAGE_CURSOR_KEY", "EW_PAGE_CURSOR_KEY_FILE")
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, errors.New("module: EW_PAGE_CURSOR_KEY is not set")
	}
	return page.NewCodec([]byte(key))
}

// SetValkey installs the Valkey client of the binary (composition-root tests; production uses Valkey()).
func (r *Registry) SetValkey(c valkey.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.res.valkey = c
}
