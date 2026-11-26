package module

import (
	"database/sql"
	"errors"
	"log/slog"
	"sort"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// PoolHealth is the pool label of the pgx pool opened by Database (readiness, Atlas revision and seed checks); the
// database/sql pools of SQL carry their role (app, platform, read, relay, purger, dashboard…).
const PoolHealth = "health"

// registerDBMetrics adds the database collectors of MONITORING §3 to the admin registry: the acquire and query
// histograms (pkg/observability) and eduworld_db_pool_connections over every pool this registry opened.
func (r *Registry) registerDBMetrics(reg prometheus.Registerer) {
	for _, c := range append(observability.DBCollectors(), observability.NewDBPoolCollector(r.PoolStats)) {
		if err := reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				slog.Error("module: database metrics not registered", "error", err)
			}
		}
	}
}

// PoolStats lists the pools opened by the registry (each *sql.DB once: the read role falls back to the app pool), sorted
// by module and pool. A pool opened lazily without a connection yet reads total 0.
func (r *Registry) PoolStats() []observability.PoolStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]observability.PoolStats, 0, len(r.pools)+len(r.res.sqls))
	for module, p := range r.pools {
		s := p.Stat()
		out = append(out, observability.PoolStats{Module: module, Pool: PoolHealth, Total: int(s.TotalConns()), Idle: int(s.IdleConns()),
			InUse: int(s.AcquiredConns()), Max: int(s.MaxConns())})
	}
	seen := map[*sql.DB]bool{}
	for _, db := range r.res.sqls {
		if seen[db] {
			continue
		}
		seen[db] = true
		name, ok := observability.PoolOf(db)
		if !ok {
			continue
		}
		s := db.Stats()
		out = append(out, observability.PoolStats{Module: name.Module, Pool: name.Pool, Total: s.OpenConnections, Idle: s.Idle,
			InUse: s.InUse, Max: s.MaxOpenConnections})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Pool < out[j].Pool
	})
	return out
}
