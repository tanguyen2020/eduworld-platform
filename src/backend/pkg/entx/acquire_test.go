package entx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// fakeDriver hands out connections whose transactions only count commits; the first badBegins BeginTx calls answer
// driver.ErrBadConn (a connection broken while idle in the pool).
type fakeDriver struct {
	badBegins atomic.Int32
	commits   atomic.Int32
	rollbacks atomic.Int32
}

type fakeConn struct{ d *fakeDriver }
type fakeTx struct{ d *fakeDriver }

func (d *fakeDriver) Open(string) (driver.Conn, error) { return &fakeConn{d: d}, nil }
func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not supported")
}
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if c.d.badBegins.Add(-1) >= 0 {
		return nil, driver.ErrBadConn
	}
	return &fakeTx{d: c.d}, nil
}
func (t *fakeTx) Commit() error   { t.d.commits.Add(1); return nil }
func (t *fakeTx) Rollback() error { t.d.rollbacks.Add(1); return nil }

type fakeConnector struct{ d *fakeDriver }

func (c fakeConnector) Connect(context.Context) (driver.Conn, error) { return &fakeConn{d: c.d}, nil }
func (c fakeConnector) Driver() driver.Driver                        { return c.d }

func acquireCount(t *testing.T, module, pool string) uint64 {
	t.Helper()
	o, err := observability.DBPoolAcquire().GetMetricWithLabelValues(module, observability.DatabaseOf(module), pool)
	if err != nil {
		t.Fatal(err)
	}
	var m dto.Metric
	if err := o.(prometheus.Metric).Write(&m); err != nil {
		t.Fatal(err)
	}
	return m.GetHistogram().GetSampleCount()
}

// MONITORING §3: each UnitOfWork/ReadSession transaction observes one eduworld_db_pool_acquire_duration_seconds sample
// with the module, database and role of the pool, and gives its connection back to the pool after commit/rollback.
func TestTransactionObservesAcquire(t *testing.T) {
	d := &fakeDriver{}
	app, platform := sql.OpenDB(fakeConnector{d: d}), sql.OpenDB(fakeConnector{d: d})
	t.Cleanup(func() { _ = app.Close(); _ = platform.Close() })
	observability.NamePool(app, "acqtest", "app")
	observability.NamePool(platform, "acqtest", "platform")
	u, err := NewUnitOfWork(Options{Module: "acqtest", DBs: DBs{App: app, Platform: platform}})
	if err != nil {
		t.Fatal(err)
	}
	before := acquireCount(t, "acqtest", "app")
	if err := u.Do(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	failed := errors.New("use case failed")
	if err := u.Do(context.Background(), func(context.Context) error { return failed }); !errors.Is(err, failed) {
		t.Fatalf("err %v", err)
	}
	rs, err := NewReadSession("acqtest", DBs{App: app, Platform: platform})
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Read(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got := acquireCount(t, "acqtest", "app") - before; got != 3 {
		t.Fatalf("acquire samples %d, want 3", got)
	}
	if d.commits.Load() != 2 || d.rollbacks.Load() != 1 {
		t.Fatalf("commits %d rollbacks %d", d.commits.Load(), d.rollbacks.Load())
	}
	if s := app.Stats(); s.InUse != 0 {
		t.Fatalf("connections still in use: %+v", s)
	}
}

// Like db.BeginTx, a connection that is broken when the transaction starts is dropped and another one is taken.
func TestBeginRetriesBadConnection(t *testing.T) {
	d := &fakeDriver{}
	d.badBegins.Store(2)
	db := sql.OpenDB(fakeConnector{d: d})
	t.Cleanup(func() { _ = db.Close() })
	conn, tx, err := begin(context.Background(), db, "acqtest", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	d.badBegins.Store(5)
	if _, _, err := begin(context.Background(), db, "acqtest", false); !errors.Is(err, driver.ErrBadConn) {
		t.Fatalf("err %v, want ErrBadConn after 3 attempts", err)
	}
}
