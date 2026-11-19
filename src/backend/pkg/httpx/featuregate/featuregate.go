// Package featuregate blocks Admin Portal requests to menus/actions that System Admin turned off
// (ADR-0090 §3, F012 US9, F012-AC-40, F012-AC-41). pkg/httpx runs the middleware after authentication,
// X-Tenant-Id, principal and portal_staff and before the handler (and so before authz.Require, which it
// never replaces), only for tokens of the portal client and operations declaring x-portal-action.
//
// The effective state comes from the port Source (implemented by module identity, F015) and is cached
// in process per scope (tenant id or platform) for feature_toggle_cache_ttl, never longer than
// principal_cache_ttl; Invalidate applies platform_settings.changed (scope feature_flag) at once. When the
// state cannot be read and no valid cache entry exists the request fails with 503 UNAVAILABLE (never
// fail-open). Actions with can_disable = false and x-portal-shell operations are never looked up.
package featuregate

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Set is the effective disabled state of one scope: menu keys and action keys turned off (platform default
// merged with the tenant override). An action is blocked when it or its parent menu is in the set.
type Set struct {
	menus   map[string]struct{}
	actions map[string]struct{}
}

// NewSet builds a Set from menu and action keys.
func NewSet(menus, actions []string) Set {
	s := Set{menus: make(map[string]struct{}, len(menus)), actions: make(map[string]struct{}, len(actions))}
	for _, m := range menus {
		s.menus[m] = struct{}{}
	}
	for _, a := range actions {
		s.actions[a] = struct{}{}
	}
	return s
}

// MenuDisabled reports whether the menu key is turned off.
func (s Set) MenuDisabled(key string) bool { _, ok := s.menus[key]; return ok }

// ActionDisabled reports whether the action key itself is turned off (its parent menu is not considered).
func (s Set) ActionDisabled(key string) bool { _, ok := s.actions[key]; return ok }

// Menus returns the disabled menu keys, sorted.
func (s Set) Menus() []string { return sortedKeys(s.menus) }

// Actions returns the disabled action keys, sorted.
func (s Set) Actions() []string { return sortedKeys(s.actions) }

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Source reads the effective disabled state of a scope: tenantID nil = platform default (System Admin
// session without X-Tenant-Id), otherwise the tenant of the session. Implemented by module identity from
// platform.feature_flags and platform.portal_menu_catalog (F015); getMyPermissions reads the same Source
// (F012-AC-39). Errors mean "state unknown" and make the gate answer 503.
type Source interface {
	DisabledFor(ctx context.Context, tenantID *uuid.UUID) (Set, error)
}

// Options configure a Gate; the TTLs come from configuration (CONFIG_DEFAULTS), never from code.
type Options struct {
	Source  Source
	Catalog *Catalog
	// TTL is feature_toggle_cache_ttl; it must be positive and not above PrincipalTTL.
	TTL time.Duration
	// PrincipalTTL is principal_cache_ttl, the upper bound of TTL (ADR-0090 §3).
	PrincipalTTL time.Duration
	Clock        clock.Clock           // nil = system clock
	Registerer   prometheus.Registerer // nil = metrics not registered
}

var (
	blocked = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_featuregate_blocked_total",
		Help: "Admin Portal requests refused with FEATURE_DISABLED, by disabled menu/action key.",
	}, []string{"feature_key"})
	sourceErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_featuregate_source_errors_total",
		Help: "featuregate.Source reads that failed (the request got 503 UNAVAILABLE).",
	})
)

// BlockedCounter exposes eduworld_featuregate_blocked_total (tests).
func BlockedCounter() *prometheus.CounterVec { return blocked }

// SourceErrorsCounter exposes eduworld_featuregate_source_errors_total (tests).
func SourceErrorsCounter() prometheus.Counter { return sourceErrors }

type entry struct {
	set     Set
	expires time.Time
}

type call struct {
	done chan struct{}
	set  Set
	err  error
}

// Gate decides whether an action may run for a scope; it holds the per-scope cache.
type Gate struct {
	o        Options
	mu       sync.Mutex
	entries  map[string]entry
	inflight map[string]*call
	gen      uint64 // bumped by Invalidate; a read started before it is not cached
}

// New validates the options and registers the metrics.
func New(o Options) (*Gate, error) {
	switch {
	case o.Source == nil || o.Catalog == nil:
		return nil, errors.New("featuregate: needs a source and a catalog")
	case o.TTL <= 0 || o.PrincipalTTL <= 0:
		return nil, errors.New("featuregate: needs feature_toggle_cache_ttl and principal_cache_ttl")
	case o.TTL > o.PrincipalTTL:
		return nil, errors.New("featuregate: feature_toggle_cache_ttl must not exceed principal_cache_ttl")
	}
	if o.Clock == nil {
		o.Clock = clock.System()
	}
	if o.Registerer != nil {
		for _, c := range []prometheus.Collector{blocked, sourceErrors} {
			if err := o.Registerer.Register(c); err != nil {
				var are prometheus.AlreadyRegisteredError
				if !errors.As(err, &are) {
					return nil, err
				}
			}
		}
	}
	return &Gate{o: o, entries: map[string]entry{}, inflight: map[string]*call{}}, nil
}

func scopeKey(tenantID *uuid.UUID) string {
	if tenantID == nil {
		return "platform"
	}
	return tenantID.String()
}

// Check returns nil when action may run in the scope, 403 FEATURE_DISABLED (extension feature_key = the
// disabled menu, else the disabled action) when it or its parent menu is off, and 503 UNAVAILABLE when
// the state cannot be read. can_disable = false actions are allowed without reading the state.
func (g *Gate) Check(ctx context.Context, tenantID *uuid.UUID, action string) error {
	a, ok := g.o.Catalog.Action(action)
	if !ok {
		// Bind refuses unknown actions at registration; reaching this is a wiring bug: fail closed.
		return problem.New(problem.Internal, "").Wrap(errors.New("featuregate: action not in catalog: " + action))
	}
	if !a.CanDisable {
		return nil
	}
	set, err := g.lookup(ctx, tenantID)
	if err != nil {
		return problem.New(problem.Unavailable, "feature state unavailable").Retry().Wrap(err)
	}
	key := ""
	switch {
	case set.MenuDisabled(a.Menu):
		key = a.Menu
	case set.ActionDisabled(a.Key):
		key = a.Key
	default:
		return nil
	}
	blocked.WithLabelValues(key).Inc()
	return problem.New(problem.FeatureDisabled, "").With("feature_key", key)
}

// lookup returns the cached state of the scope or reads it once for every concurrent caller.
func (g *Gate) lookup(ctx context.Context, tenantID *uuid.UUID) (Set, error) {
	k := scopeKey(tenantID)
	g.mu.Lock()
	if e, ok := g.entries[k]; ok && g.o.Clock.Now().Before(e.expires) {
		g.mu.Unlock()
		return e.set, nil
	}
	if c, ok := g.inflight[k]; ok {
		g.mu.Unlock()
		select {
		case <-c.done:
			return c.set, c.err
		case <-ctx.Done():
			return Set{}, ctx.Err()
		}
	}
	c := &call{done: make(chan struct{})}
	g.inflight[k] = c
	gen := g.gen
	g.mu.Unlock()

	c.set, c.err = g.o.Source.DisabledFor(ctx, tenantID)
	g.mu.Lock()
	delete(g.inflight, k)
	if c.err == nil && gen == g.gen {
		g.entries[k] = entry{set: c.set, expires: g.o.Clock.Now().Add(g.o.TTL)}
	}
	g.mu.Unlock()
	close(c.done)
	if c.err != nil {
		sourceErrors.Inc()
		return Set{}, c.err
	}
	return c.set, nil
}

// Invalidate drops cached state on platform_settings.changed with scope feature_flag (consumer wiring:
// F015): a tenant override drops that tenant; tenantID nil (platform default changed) drops every scope,
// because every tenant inherits the platform default.
func (g *Gate) Invalidate(tenantID *uuid.UUID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.gen++
	if tenantID == nil {
		clear(g.entries)
		return
	}
	delete(g.entries, scopeKey(tenantID))
}
