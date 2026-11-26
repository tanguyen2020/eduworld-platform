package observability

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

// MONITORING §3: the query label is "<verb>:<table>" built from the statement only (bounded, no value).
func TestQueryName(t *testing.T) {
	for sql, want := range map[string]string{
		`SELECT "t1"."id", "t1"."name" FROM "assessment"."attempts" AS "t1" WHERE "t1"."id" = $1`: "select:assessment.attempts",
		"select school_id, grade_level FROM ref.students WHERE student_id = $1":                   "select:ref.students",
		`INSERT INTO "outbox"."events" ("id") VALUES ($1)`:                                        "insert:outbox.events",
		"UPDATE game.login_events SET logout_at = $3 WHERE tenant_id = $1":                        "update:game.login_events",
		`UPDATE ONLY "x" SET a = 1`:                                                               "update:x",
		"DELETE FROM chat.gameplay_messages WHERE id = $1":                                        "delete:chat.gameplay_messages",
		"/* trace_id=abc */ SELECT 1 FROM learning.lessons":                                       "select:learning.lessons",
		"-- comment\nselect * from game.sessions":                                                 "select:game.sessions",
		"SELECT set_config('app.tenant_id', $1, true)":                                            "select",
		"SELECT * FROM (SELECT 1) s":                                                              "select",
		"WITH a AS (SELECT 1) SELECT * FROM a":                                                    "with",
		"begin isolation level read committed read only":                                          "begin",
		"commit":                "commit",
		"SAVEPOINT ew_sp_1":     "savepoint",
		"VACUUM ANALYZE game.x": "other",
		"":                      "other",
		"SELECT 1 FROM " + strings.Repeat("a", 80): "select",
		"SELECT a FROM 1bad":                       "select",
	} {
		if got := QueryName(sql); got != want {
			t.Errorf("QueryName(%q) = %q, want %q", sql, got, want)
		}
	}
}

// The pgx tracer observes eduworld_db_query_duration_seconds{module,query} once per statement.
func TestQueryTracerObserves(t *testing.T) {
	tr := QueryTracer{Module: "obs_test_tracer"}
	before := testutil.CollectAndCount(dbQueryDuration)
	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT x FROM game.sessions WHERE id = $1"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	tr.TraceQueryEnd(context.Background(), nil, pgx.TraceQueryEndData{}) // no start in ctx: nothing observed
	if got := testutil.CollectAndCount(dbQueryDuration); got != before+1 {
		t.Fatalf("series %d, want %d", got, before+1)
	}
	if n := histogramCount(t, dbQueryDuration, "obs_test_tracer", "select:game.sessions"); n != 1 {
		t.Fatalf("count %d, want 1", n)
	}
}

// eduworld_db_pool_acquire_duration_seconds carries the module, database and role named by NamePool; an unnamed pool
// reads pool="unnamed" with the caller's module.
func TestObserveAcquireLabels(t *testing.T) {
	named, err := sql.Open("obs-noop", "")
	if err != nil {
		t.Fatal(err)
	}
	NamePool(named, "obs_acq", "platform")
	NamePool(named, "other", "app") // first name wins
	ObserveAcquire(named, "ignored", 3*time.Millisecond)
	if n := histogramCount(t, dbPoolAcquire, "obs_acq", "eduworld_obs_acq", "platform"); n != 1 {
		t.Fatalf("named count %d", n)
	}
	anon, _ := sql.Open("obs-noop", "x")
	ObserveAcquire(anon, "obs_acq", time.Millisecond)
	if n := histogramCount(t, dbPoolAcquire, "obs_acq", "eduworld_obs_acq", "unnamed"); n != 1 {
		t.Fatalf("unnamed count %d", n)
	}
}

// eduworld_db_pool_connections exposes total/idle/in_use/max for each listed pool with module, database and pool.
func TestDBPoolCollector(t *testing.T) {
	c := NewDBPoolCollector(func() []PoolStats {
		return []PoolStats{{Module: "game", Pool: "app", Total: 3, Idle: 2, InUse: 1, Max: 10}}
	})
	want := `
# HELP eduworld_db_pool_connections Connections of a module pool by state (total, idle, in_use, max); total = 0 means the database is not reachable.
# TYPE eduworld_db_pool_connections gauge
eduworld_db_pool_connections{database="eduworld_game",module="game",pool="app",state="idle"} 2
eduworld_db_pool_connections{database="eduworld_game",module="game",pool="app",state="in_use"} 1
eduworld_db_pool_connections{database="eduworld_game",module="game",pool="app",state="max"} 10
eduworld_db_pool_connections{database="eduworld_game",module="game",pool="app",state="total"} 3
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Fatal(err)
	}
}

func histogramCount(t *testing.T, h *prometheus.HistogramVec, labels ...string) uint64 {
	t.Helper()
	o, err := h.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := o.(prometheus.Metric)
	if !ok {
		t.Fatal("not a metric")
	}
	var pb dto.Metric
	if err := m.Write(&pb); err != nil {
		t.Fatal(err)
	}
	return pb.GetHistogram().GetSampleCount()
}

// noopDriver lets sql.Open build pools that never connect (pool naming only).
type noopDriver struct{}

func (noopDriver) Open(string) (driver.Conn, error) { return nil, driver.ErrBadConn }

func init() { sql.Register("obs-noop", noopDriver{}) }
