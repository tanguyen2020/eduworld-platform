// Package module is the minimal composition-root registry passed to every modules/<module>.Register
// (F001 T012/T024, BACKEND_DESIGN §3.3). It owns resources shared by the modules a binary loads:
// the health registry and one pgx pool per module database. F002/F003 extend it with HTTP routes,
// NATS handlers, relays, consumers and jobs.
//
// pkg/ must never import modules/ (F001-AC-23); modules depend on this package, not the reverse.
package module

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
)

// Registry is created once per binary by its composition root (cmd/<binary>).
type Registry struct {
	binary  string
	profile string
	health  *health.Registry
	getenv  func(string) string

	mu    sync.Mutex
	pools map[string]*pgxpool.Pool

	res resources // resources.go: SQL pools by role, NATS, Valkey, background workers, public handler
}

// NewRegistry creates the registry. getenv is os.Getenv in production and a map lookup in tests.
func NewRegistry(binary, profile string, h *health.Registry, getenv func(string) string) *Registry {
	if getenv == nil {
		getenv = os.Getenv
	}
	return &Registry{binary: binary, profile: profile, health: h, getenv: getenv, pools: map[string]*pgxpool.Pool{},
		res: resources{sqls: map[string]*sql.DB{}, cores: map[string]*Core{}}}
}

// Binary is the running binary name (core-api, realtime, worker).
func (r *Registry) Binary() string { return r.binary }

// Profile is the worker profile (events, chat, jobs, all); empty for other binaries.
func (r *Registry) Profile() string { return r.profile }

// Health returns the binary's health registry; modules register `db:<module>` and `atlas:<module>`.
func (r *Registry) Health() *health.Registry { return r.health }

// ErrDatabaseNotConfigured is returned when neither EW_DB_<MODULE>_DSN nor EW_DB_<MODULE>_URL_FILE is set.
var ErrDatabaseNotConfigured = errors.New("module database not configured")

// Database returns the pool of the module's own database (role <module>_app), created once.
// The pool connects lazily, so a binary starts even when the database is down and reports
// not ready instead (F001-AC-05). A module may only open its own database (ADR-0074 §4).
func (r *Registry) Database(module string) (*pgxpool.Pool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.pools[module]; ok {
		return p, nil
	}
	dsn, err := r.dsn(module)
	if err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse %s database config: %w", module, err)
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = r.binary + "/" + module
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("create %s pool: %w", module, err)
	}
	r.pools[module] = pool
	return pool, nil
}

// dsn reads EW_DB_<MODULE>_DSN (local) or the secret file EW_DB_<MODULE>_URL_FILE (staging/production).
func (r *Registry) dsn(module string) (string, error) {
	key := "EW_DB_" + strings.ToUpper(module)
	if v := strings.TrimSpace(r.getenv(key + "_DSN")); v != "" {
		return v, nil
	}
	if f := strings.TrimSpace(r.getenv(key + "_URL_FILE")); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("read %s_URL_FILE: %w", key, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return "", fmt.Errorf("%s: %w (set %s_DSN or %s_URL_FILE)", module, ErrDatabaseNotConfigured, key, key)
}

// Close releases every pool opened through the registry.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeResources()
	for name, p := range r.pools {
		p.Close()
		delete(r.pools, name)
	}
}
