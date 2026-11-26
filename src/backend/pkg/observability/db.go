package observability

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// Database metrics of the module pools (MONITORING §3 "Database Performance"): one pool per module database and role
// (ADR-0074 §4, ADR-0089). Labels are bounded — module, database (eduworld_<module>), pool (role of the pool), state and
// query (verb + table, never the SQL text nor a value); `service`/`profile` are attached by Prometheus at scrape
// (MONITORING §2). Never a tenant, user or argument label (AGENTS R14).
var (
	dbPoolAcquire = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_db_pool_acquire_duration_seconds",
		Help:    "Time to get a connection from a module pool before a transaction (UnitOfWork/ReadSession) starts.",
		Buckets: []float64{.0001, .0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"module", "database", "pool"})
	dbQueryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "eduworld_db_query_duration_seconds",
		Help:    "Statement latency on a module pool by query name (<verb>:<table>, e.g. select:assessment.attempts).",
		Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"module", "query"})
	dbPoolConnectionsDesc = prometheus.NewDesc("eduworld_db_pool_connections",
		"Connections of a module pool by state (total, idle, in_use, max); total = 0 means the database is not reachable.",
		[]string{"module", "database", "pool", "state"}, nil)
)

// DBCollectors are the database collectors registered on the admin registry of a binary (pkg/module).
func DBCollectors() []prometheus.Collector {
	return []prometheus.Collector{dbPoolAcquire, dbQueryDuration}
}

// DBPoolAcquire exposes eduworld_db_pool_acquire_duration_seconds (tests).
func DBPoolAcquire() *prometheus.HistogramVec { return dbPoolAcquire }

// DBQueryDuration exposes eduworld_db_query_duration_seconds (tests).
func DBQueryDuration() *prometheus.HistogramVec { return dbQueryDuration }

// DatabaseOf is the database of a module (one database per module, ADR-0074 §4).
func DatabaseOf(module string) string { return "eduworld_" + module }

// DBPool names one pool: the module of its database and its role (app, platform, read, relay, purger…).
type DBPool struct{ Module, Pool string }

var namedPools sync.Map // *sql.DB → DBPool

// NamePool records the module and role of a database/sql pool (pkg/module.Registry.SQL) so the acquire histogram of the
// transaction helpers (pkg/entx) carries them. The first name wins (the read role falls back to the app pool).
func NamePool(db *sql.DB, module, pool string) {
	if db != nil {
		namedPools.LoadOrStore(db, DBPool{Module: module, Pool: pool})
	}
}

// PoolOf returns the name recorded by NamePool.
func PoolOf(db *sql.DB) (DBPool, bool) {
	v, ok := namedPools.Load(db)
	if !ok {
		return DBPool{}, false
	}
	p, _ := v.(DBPool)
	return p, true
}

// ObserveAcquire records one connection acquisition of db; an unnamed pool (tests, ad-hoc pools) is labelled with
// module and pool "unnamed".
func ObserveAcquire(db *sql.DB, module string, d time.Duration) {
	p, ok := PoolOf(db)
	if !ok {
		p = DBPool{Module: module, Pool: "unnamed"}
	}
	dbPoolAcquire.WithLabelValues(p.Module, DatabaseOf(p.Module), p.Pool).Observe(d.Seconds())
}

// PoolStats is the snapshot of one pool at scrape time.
type PoolStats struct {
	Module, Pool            string
	Total, Idle, InUse, Max int
}

type poolCollector struct{ list func() []PoolStats }

// NewDBPoolCollector returns the collector of eduworld_db_pool_connections{module,database,pool,state}; list is called
// at each scrape (pkg/module lists the pgx and database/sql pools it opened).
func NewDBPoolCollector(list func() []PoolStats) prometheus.Collector {
	return poolCollector{list: list}
}

func (c poolCollector) Describe(ch chan<- *prometheus.Desc) { ch <- dbPoolConnectionsDesc }

func (c poolCollector) Collect(ch chan<- prometheus.Metric) {
	for _, s := range c.list() {
		db := DatabaseOf(s.Module)
		for _, st := range []struct {
			state string
			v     int
		}{{"total", s.Total}, {"idle", s.Idle}, {"in_use", s.InUse}, {"max", s.Max}} {
			ch <- prometheus.MustNewConstMetric(dbPoolConnectionsDesc, prometheus.GaugeValue, float64(st.v), s.Module, db, s.Pool, st.state)
		}
	}
}

// QueryTracer is the pgx tracer of a module pool: it observes eduworld_db_query_duration_seconds for every statement
// (ent and explicit SQL, BEGIN/COMMIT included) under the bounded name of QueryName.
type QueryTracer struct{ Module string }

var _ pgx.QueryTracer = QueryTracer{}

type queryStartKey struct{}

type queryStart struct {
	at   time.Time
	name string
}

// TraceQueryStart implements pgx.QueryTracer.
func (t QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryStartKey{}, queryStart{at: time.Now(), name: QueryName(d.SQL)})
}

// TraceQueryEnd implements pgx.QueryTracer.
func (t QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if s, ok := ctx.Value(queryStartKey{}).(queryStart); ok {
		dbQueryDuration.WithLabelValues(t.Module, s.name).Observe(time.Since(s.at).Seconds())
	}
}

// QueryName is the bounded name of a statement: "<verb>:<table>" for select/insert/update/delete with a plain
// (schema-qualified) table, the verb alone otherwise (begin, commit, with, select without FROM…), "other" for anything
// else. Values never come from the statement arguments, so the label set is bounded by the tables of the module.
func QueryName(sql string) string {
	s := skipNoise(sql)
	verb, rest := nextWord(s)
	verb = strings.ToLower(verb)
	var keyword string
	switch verb {
	case "select", "delete":
		keyword = "from"
	case "insert":
		keyword = "into"
	case "update":
		if t := tableName(rest); t != "" {
			return verb + ":" + t
		}
		return verb
	case "with", "begin", "commit", "rollback", "savepoint", "release", "set", "show", "lock", "copy", "values", "start", "end":
		return verb
	default:
		return "other"
	}
	if t := tableName(afterKeyword(rest, keyword)); t != "" {
		return verb + ":" + t
	}
	return verb
}

// skipNoise drops leading blanks and comments (/* … */ and -- …).
func skipNoise(s string) string {
	for {
		s = strings.TrimLeft(s, " \t\r\n(")
		switch {
		case strings.HasPrefix(s, "/*"):
			i := strings.Index(s, "*/")
			if i < 0 {
				return ""
			}
			s = s[i+2:]
		case strings.HasPrefix(s, "--"):
			i := strings.IndexByte(s, '\n')
			if i < 0 {
				return ""
			}
			s = s[i+1:]
		default:
			return s
		}
	}
}

func nextWord(s string) (string, string) {
	s = skipNoise(s)
	i := strings.IndexAny(s, " \t\r\n(;")
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i:]
}

// afterKeyword returns the text after the first standalone keyword (case-insensitive), "" when absent. Only the first
// 2 KiB are scanned: the table of a statement comes early.
func afterKeyword(s, kw string) string {
	if len(s) > 2048 {
		s = s[:2048]
	}
	low := strings.ToLower(s)
	for from := 0; ; {
		i := strings.Index(low[from:], kw)
		if i < 0 {
			return ""
		}
		i += from
		end := i + len(kw)
		if (i == 0 || isSpace(low[i-1])) && end < len(low) && isSpace(low[end]) {
			return s[end:]
		}
		from = end
	}
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

// tableName reads a plain, optionally schema-qualified and quoted identifier ("schema"."table", schema.table, table);
// anything else (subquery, function, ONLY, too long) gives "".
func tableName(s string) string {
	s = strings.TrimLeft(s, " \t\r\n")
	if strings.HasPrefix(strings.ToLower(s), "only ") {
		s = strings.TrimLeft(s[5:], " \t\r\n")
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			continue
		case c == '.' || c == '_' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			b.WriteByte(c)
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c + ('a' - 'A'))
		default:
			i = len(s)
		}
	}
	t := b.String()
	if t == "" || len(t) > 63 || t[0] == '.' || t[len(t)-1] == '.' || strings.Count(t, ".") > 1 || (t[0] >= '0' && t[0] <= '9') {
		return ""
	}
	return t
}
