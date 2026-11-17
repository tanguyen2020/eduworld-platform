//go:build integration

package testkit

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Kit is the integration environment of one module (F003 T054, AC-31): PostgreSQL 17 with the module's
// Atlas migrations, NATS JetStream with the module streams, Valkey, the module pools, UnitOfWork,
// ReadSession, outbox writer and audit recorder, and tenant A/B fixtures.
type Kit struct {
	Module   string
	Cluster  *Cluster
	App      *sql.DB
	Platform *sql.DB
	Migrator *sql.DB
	UoW      *entx.UnitOfWork
	Read     *entx.ReadSession
	Writer   *outbox.Writer
	Audit    *outbox.AuditRecorder
	NATS     *nats.Conn
	JS       jetstream.JetStream
	Valkey   string // host:port
}

func open(t testing.TB, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// New starts the containers for module and wires the data-access kit; tenancy hooks use the kit's
// audit recorder for the duration of the test.
func New(t testing.TB, module string) *Kit {
	t.Helper()
	ctx := context.Background()
	cl := StartCluster(ctx, t, true)
	_, natsURL := StartNATS(ctx, t)
	_, valkeyAddr := StartValkey(ctx, t)
	k := &Kit{Module: module, Cluster: cl, Valkey: valkeyAddr,
		App: open(t, cl.ModuleDSN(module, "app")), Platform: open(t, cl.ModuleDSN(module, "platform")),
		Migrator: open(t, cl.ModuleDSN(module, "migrator"))}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	k.NATS = nc
	if k.JS, err = jetstream.New(nc); err != nil {
		t.Fatal(err)
	}
	if err := natsx.EnsureStreams(ctx, k.JS, module, natsx.StreamOptions{}); err != nil {
		t.Fatal(err)
	}
	if k.Writer, err = outbox.NewWriter(module, "testkit", nil); err != nil {
		t.Fatal(err)
	}
	k.Audit = outbox.NewAuditRecorder(k.Writer, k.App)
	dbs := entx.DBs{App: k.App, Platform: k.Platform}
	if k.UoW, err = entx.NewUnitOfWork(entx.Options{Module: module, DBs: dbs, Events: k.Writer, Audit: k.Audit}); err != nil {
		t.Fatal(err)
	}
	if k.Read, err = entx.NewReadSession(module, dbs); err != nil {
		t.Fatal(err)
	}
	tenancy.Configure(tenancy.Hooks{Recorder: k.Audit})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	return k
}

// As returns a context of p with its scope and the kit's module (authztest fixtures: Staff, Student,
// Platform, NoRole on authztest.TenantA / TenantB).
func (k *Kit) As(p authz.Principal) context.Context {
	return tenancy.WithModule(authztest.Ctx(context.Background(), p), k.Module)
}
