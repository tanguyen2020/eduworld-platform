//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/runtime" // mixin hooks, interceptors
)

// One migrated PostgreSQL for the package; each test works on its own copy of the databases.
func TestMain(m *testing.M) { os.Exit(testkit.RunWithSharedCluster(m)) }

// env is the learning integration environment of one test: the module pools, UnitOfWork, ReadSession, the
// outbox writer and audit recorder (hooked into tenancy), the fixture tables and tenants A, B.
type env struct {
	cl                                *testkit.Cluster
	app, platform, migrator, readonly *sql.DB
	relay                             *sql.DB
	uow                               *entx.UnitOfWork
	read                              *entx.ReadSession
	writer                            *outbox.Writer
	rec                               *outbox.AuditRecorder
	tenantA, tenantB, tenantOff       uuid.UUID
}

func open(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// setup builds the environment; withTenants seeds tenants A, B (active) and a disabled one in ref.tenants.
func setup(ctx context.Context, t *testing.T, withTenants bool) *env {
	t.Helper()
	cl := testkit.Isolated(ctx, t)
	e := &env{cl: cl, app: open(t, cl.ModuleDSN("learning", "app")), platform: open(t, cl.ModuleDSN("learning", "platform")),
		migrator: open(t, cl.ModuleDSN("learning", "migrator")), readonly: open(t, cl.ModuleDSN("learning", "readonly")),
		relay:   open(t, cl.ModuleDSN("learning", "relay")),
		tenantA: uuid.New(), tenantB: uuid.New(), tenantOff: uuid.New()}
	if err := fixtures.Install(ctx, e.migrator); err != nil {
		t.Fatalf("fixture tables: %v", err)
	}
	var err error
	if e.writer, err = outbox.NewWriter("learning", "core", nil); err != nil {
		t.Fatal(err)
	}
	e.rec = outbox.NewAuditRecorder(e.writer, e.app)
	tenancy.Configure(tenancy.Hooks{Recorder: e.rec})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	dbs := entx.DBs{App: e.app, Platform: e.platform}
	if e.uow, err = entx.NewUnitOfWork(entx.Options{Module: "learning", DBs: dbs, Events: e.writer, Audit: e.rec}); err != nil {
		t.Fatal(err)
	}
	if e.read, err = entx.NewReadSession("learning", dbs); err != nil {
		t.Fatal(err)
	}
	if withTenants {
		if err := fixtures.SeedTenants(ctx, e.migrator, fixtures.Tenant{ID: e.tenantA, Code: "t-a"},
			fixtures.Tenant{ID: e.tenantB, Code: "t-b"}, fixtures.Tenant{ID: e.tenantOff, Code: "t-off", Status: "disabled"}); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

// jetStream starts NATS with the identity and learning streams.
func jetStream(ctx context.Context, t *testing.T) jetstream.JetStream {
	t.Helper()
	_, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"identity", "learning"} {
		if err := natsx.EnsureStreams(ctx, js, m, natsx.StreamOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return js
}

type sqlQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func count(ctx context.Context, t *testing.T, q sqlQuerier, query string, args ...any) int {
	t.Helper()
	var n int
	if err := q.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// waitFor polls cond until it holds or the timeout ends.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) time.Duration {
	t.Helper()
	start := time.Now()
	for time.Since(start) < timeout {
		if cond() {
			return time.Since(start)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
	return 0
}

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// audits counts audit.recorded events of action in the outbox (optionally for one tenant).
func (e *env) audits(ctx context.Context, t *testing.T, action string, tenant *uuid.UUID) int {
	t.Helper()
	if tenant == nil {
		return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
			AND envelope->'payload'->>'action' = $1`, action)
	}
	return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = $1 AND tenant_id = $2`, action, *tenant)
}
