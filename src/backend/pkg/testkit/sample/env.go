//go:build integration

// Package sample is the sample module of F003 (entity Widget and friends, one per entx kind) and the
// integration environment its tests share: PostgreSQL with migrations, the sample tables, the
// learning_* pools, UnitOfWork, ReadSession, outbox Writer and AuditRecorder.
package sample

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent"
	_ "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent/runtime" // mixin hooks, interceptors, defaults
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/fixture"
)

// Env is the shared integration environment.
type Env struct {
	Cluster  *testkit.Cluster
	App      *sql.DB
	Platform *sql.DB
	Migrator *sql.DB
	Relay    *sql.DB
	UoW      *entx.UnitOfWork
	Read     *entx.ReadSession
	Writer   *outbox.Writer
	Audit    *outbox.AuditRecorder
}

// Open opens a database/sql pool for dsn and closes it at the end of the test.
func Open(t testing.TB, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// Setup starts a migrated cluster and creates the sample tables (with their RLS policies).
func Setup(ctx context.Context, t testing.TB) *Env {
	t.Helper()
	cl := testkit.StartCluster(ctx, t, true)
	e := &Env{Cluster: cl,
		App:      Open(t, cl.ModuleDSN("learning", "app")),
		Platform: Open(t, cl.ModuleDSN("learning", "platform")),
		Migrator: Open(t, cl.ModuleDSN("learning", "migrator")),
		Relay:    Open(t, cl.ModuleDSN("learning", "relay")),
	}
	if _, err := e.Migrator.ExecContext(ctx, fixture.SQL); err != nil {
		t.Fatalf("sample fixture: %v", err)
	}
	var err error
	if e.Writer, err = outbox.NewWriter("learning", "sample", nil); err != nil {
		t.Fatal(err)
	}
	e.Audit = outbox.NewAuditRecorder(e.Writer, e.App)
	dbs := entx.DBs{App: e.App, Platform: e.Platform}
	if e.UoW, err = entx.NewUnitOfWork(entx.Options{Module: "learning", DBs: dbs, Events: e.Writer, Audit: e.Audit}); err != nil {
		t.Fatal(err)
	}
	if e.Read, err = entx.NewReadSession("learning", dbs); err != nil {
		t.Fatal(err)
	}
	return e
}

// Client returns the ent client bound to the transaction of ctx.
func Client(ctx context.Context) *ent.Client { return ent.NewClient(ent.Driver(entx.Driver(ctx))) }
