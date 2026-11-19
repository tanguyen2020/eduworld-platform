//go:build integration

package testkit

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // driver "pgx"
)

// Shared cluster: one PostgreSQL container migrated once per test package (TestMain); every test gets
// its own copy of the module databases (CREATE DATABASE … TEMPLATE, about a second) instead of a new
// container plus five Atlas runs (about a minute). Tests stay isolated: each sees fresh databases.

var shared struct {
	mu      sync.Mutex
	cluster *Cluster
	n       atomic.Int64
}

// mainTB lets StartCluster run from TestMain (no *testing.T there): it records cleanups and turns a
// fatal error into a panic. Methods other than the overridden ones are never called by StartCluster.
type mainTB struct {
	testing.TB
	cleanups []func()
}

func (m *mainTB) Helper()                           {}
func (m *mainTB) Cleanup(f func())                  { m.cleanups = append(m.cleanups, f) }
func (m *mainTB) Fatal(args ...any)                 { panic(fmt.Sprint(args...)) }
func (m *mainTB) Fatalf(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }
func (m *mainTB) Logf(format string, args ...any)   { fmt.Fprintf(os.Stderr, format+"\n", args...) }

// RunWithSharedCluster is a TestMain body: it starts and migrates one cluster, runs the tests (which get
// copies through Isolated) and removes the containers.
//
//	func TestMain(m *testing.M) { os.Exit(testkit.RunWithSharedCluster(m)) }
func RunWithSharedCluster(m *testing.M) int {
	tb := &mainTB{}
	defer func() {
		for i := len(tb.cleanups) - 1; i >= 0; i-- {
			tb.cleanups[i]()
		}
	}()
	shared.cluster = StartCluster(context.Background(), tb, true)
	return m.Run()
}

// Isolated returns a cluster view whose module databases are fresh copies of the migrated templates
// (dropped at the end of the test). Without a shared cluster (no TestMain) it starts a new cluster.
func Isolated(ctx context.Context, t testing.TB) *Cluster {
	t.Helper()
	if shared.cluster == nil {
		return StartCluster(ctx, t, true)
	}
	base := shared.cluster
	suffix := fmt.Sprintf("_t%d", shared.n.Add(1))
	admin, err := sql.Open("pgx", base.DSN("postgres", "postgres"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	// A template must have no other session while it is copied: copies are made one at a time.
	shared.mu.Lock()
	for _, m := range Modules {
		if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE eduworld_%s%s TEMPLATE eduworld_%s`, m, suffix, m)); err != nil {
			shared.mu.Unlock()
			t.Fatalf("clone eduworld_%s: %v", m, err)
		}
	}
	shared.mu.Unlock()
	t.Cleanup(func() {
		for _, m := range Modules {
			_, _ = admin.ExecContext(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS eduworld_%s%s WITH (FORCE)`, m, suffix))
		}
	})
	c := *base
	c.suffix = suffix
	return &c
}
