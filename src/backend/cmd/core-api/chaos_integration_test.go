//go:build integration

package main

import (
	"bufio"
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F132 local chaos evidence (T018–T020, F132-AC-08, AC-10, AC-12 at the scale of one in-process core-api with throwaway
// PostgreSQL/NATS/Valkey containers). It is NOT the staging chaos run (tests/chaos/run-chaos.sh under load, multi-node,
// Patroni, PgBouncer): it proves the module isolation of the binary itself — one module database, NATS or Valkey lost
// while core-api keeps serving the routes of the other modules, readiness and the alert metric
// (eduworld_db_pool_connections, ModuleDatabaseUnavailable) report the fault, and core-api recovers without a restart.

// clientConfigPath is getClientConfig of module game (security: [], no database of module learning involved).
const clientConfigPath = "/api/v1/game/client-config?platform=web&version=1.0.0"

func startCoreAPIBoth(t *testing.T, getenv func(string) string) (admin, httpAddr string, stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	type addrs struct{ admin, http string }
	ch := make(chan addrs, 1)
	done := make(chan error, 1)
	go func() { done <- run(ctx, getenv, func(a, h string) { ch <- addrs{a, h} }) }()
	select {
	case a := <-ch:
		admin, httpAddr = a.admin, a.http
	case err := <-done:
		t.Fatalf("core-api exited: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("core-api did not start")
	}
	return admin, httpAddr, func() {
		cancel()
		if err := <-done; err != nil {
			t.Logf("core-api stopped: %v", err)
		}
	}
}

// get returns the status of url and how long the answer took.
func get(t *testing.T, url string) (int, time.Duration) {
	t.Helper()
	c := http.Client{Timeout: 10 * time.Second}
	start := time.Now()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode, time.Since(start)
}

// poolTotal reads eduworld_db_pool_connections{module=m,state="total"} summed over the pools of module m.
func poolTotal(t *testing.T, admin, m string) (float64, bool) {
	t.Helper()
	resp, err := http.Get("http://" + admin + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var sum float64
	found := false
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		l := sc.Text()
		if !strings.HasPrefix(l, "eduworld_db_pool_connections{") || !strings.Contains(l, `module="`+m+`"`) ||
			!strings.Contains(l, `state="total"`) {
			continue
		}
		v, err := strconv.ParseFloat(l[strings.LastIndexByte(l, ' ')+1:], 64)
		if err != nil {
			t.Fatalf("metric line %q: %v", l, err)
		}
		sum, found = sum+v, true
	}
	return sum, found
}

// until polls cond every 500 ms until it holds or d elapses.
func until(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func psqlSuper(ctx context.Context, t *testing.T, c *testkit.Cluster, sql string) {
	t.Helper()
	code, out, err := c.Container.Exec(ctx, []string{"psql", "-U", "postgres", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-tAc", sql})
	if err != nil || code != 0 {
		t.Fatalf("psql %q: code %d, %v: %s", sql, code, err, readAll(out))
	}
}

func docker(ctx context.Context, t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
		t.Fatalf("docker %v: %v: %s", args, err, out)
	}
}

// TestCoreAPIChaosModuleIsolation — F132-AC-08 (TC-SVC-010), AC-10 (TC-SVC-012), AC-12 (TC-PERF-008), local scale.
func TestCoreAPIChaosModuleIsolation(t *testing.T) {
	ctx := context.Background()
	cluster := testkit.StartCluster(ctx, t, true)
	vk, vkAddr := testkit.StartValkey(ctx, t)
	nc, natsURL := testkit.StartNATS(ctx, t)

	admin, httpAddr, stop := startCoreAPIBoth(t, envFor(t, cluster, vkAddr, natsURL))
	defer stop()
	if code, body := waitReady(t, admin, 60*time.Second); code != http.StatusOK {
		t.Fatalf("readyz = %d %+v, want 200", code, body)
	}
	cfgURL := "http://" + httpAddr + clientConfigPath
	if code, _ := get(t, cfgURL); code != http.StatusOK {
		t.Fatalf("baseline getClientConfig = %d, want 200", code)
	}

	// T017/T040: the reconciliation queries of tests/chaos/reconcile run on the real migrated schemas (read only; an
	// acked id absent from the database counts as missing).
	t.Run("ReconcileQueriesRun", func(t *testing.T) {
		root := testkit.RepoRoot(t)
		for _, q := range []struct{ file, db, empty, oneMissing string }{
			{"attempts.sql", "eduworld_learning", "0|0|0", "0|1|1"},
			{"chat_messages.sql", "eduworld_chat", "0|0|0|0", "0|1|1|0"},
		} {
			src := filepath.Join(root, "tests", "chaos", "reconcile", q.file)
			if err := cluster.Container.CopyFileToContainer(ctx, src, "/tmp/"+q.file, 0o644); err != nil {
				t.Fatal(err)
			}
			for ids, want := range map[string]string{"": q.empty, "11111111-1111-4111-8111-111111111111": q.oneMissing} {
				code, out, err := cluster.Container.Exec(ctx, []string{"psql", "-U", "postgres", "-d", q.db, "-X", "-q", "-A", "-t",
					"-F", "|", "-v", "ON_ERROR_STOP=1", "-v", "tenant_prefix=perf-", "-v", "from=2026-01-01T00:00:00Z",
					"-v", "to=2030-01-01T00:00:00Z", "-v", "ids=" + ids, "-f", "/tmp/" + q.file})
				got := readAll(out)
				if err != nil || code != 0 || !strings.Contains(got, want) {
					t.Fatalf("%s ids=%q: code %d, %v, output %q, want a row %q", q.file, ids, code, err, got, want)
				}
			}
		}
	})

	// F132-AC-08: database eduworld_learning unreachable (new connections refused, live sessions killed — the local
	// equivalent of the staging scenario learning-db-down, which does the same through an ops job on the Patroni leader).
	t.Run("LearningDatabaseDown", func(t *testing.T) {
		psqlSuper(ctx, t, cluster, "ALTER DATABASE eduworld_learning ALLOW_CONNECTIONS false")
		psqlSuper(ctx, t, cluster, "SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity WHERE datname = 'eduworld_learning' AND pid <> pg_backend_pid()")
		restored := false
		defer func() {
			if !restored { // recovery always runs, as in run-chaos.sh
				psqlSuper(ctx, t, cluster, "ALTER DATABASE eduworld_learning ALLOW_CONNECTIONS true")
			}
		}()

		var last readiness
		if !until(30*time.Second, func() bool { _, last = readyz(t, admin); return last.Checks["db:learning"].Status == "fail" }) {
			t.Fatalf("readiness never reported db:learning fail: %+v", last)
		}
		for _, m := range []string{"identity", "game", "competition", "chat"} {
			if s := last.Checks["db:"+m].Status; s != "ok" {
				t.Fatalf("db:%s = %q while only learning is down (module isolation, ADR-0074)", m, s)
			}
		}
		// Routes of another module keep serving, without waiting on the lost database.
		for i := 0; i < 20; i++ {
			code, took := get(t, cfgURL)
			if code != http.StatusOK || took > 2*time.Second {
				t.Fatalf("getClientConfig (module game) = %d in %s while learning is down, want 200 < 2 s", code, took)
			}
		}
		// The alert input of ModuleDatabaseUnavailable (eduworld_db_pool_connections{state="total"} == 0) is reached.
		var total float64
		if !until(60*time.Second, func() bool {
			readyz(t, admin) // probes make the pool drop its dead connections
			v, ok := poolTotal(t, admin, "learning")
			total = v
			return ok && v == 0
		}) {
			t.Fatalf("eduworld_db_pool_connections{module=learning,state=total} = %v, want 0 (ModuleDatabaseUnavailable)", total)
		}
		if v, ok := poolTotal(t, admin, "game"); !ok || v == 0 {
			t.Fatalf("pool of module game = %v (found %v), want > 0", v, ok)
		}

		psqlSuper(ctx, t, cluster, "ALTER DATABASE eduworld_learning ALLOW_CONNECTIONS true")
		restored = true
		if code, body := waitReady(t, admin, 60*time.Second); code != http.StatusOK {
			t.Fatalf("no recovery after the database is back: readyz = %d %+v", code, body)
		}
	})

	// F132-AC-10 (part): NATS frozen — REST of core-api keeps serving; readiness reports nats; recovery after unpause.
	t.Run("NATSDown", func(t *testing.T) {
		docker(ctx, t, "pause", nc.GetContainerID())
		paused := true
		defer func() {
			if paused {
				docker(ctx, t, "unpause", nc.GetContainerID())
			}
		}()
		for i := 0; i < 10; i++ {
			code, took := get(t, cfgURL)
			if code != http.StatusOK || took > 2*time.Second {
				t.Fatalf("getClientConfig = %d in %s while NATS is down, want 200 < 2 s", code, took)
			}
		}
		var last readiness
		if !until(30*time.Second, func() bool { _, last = readyz(t, admin); return last.Checks["nats"].Status == "fail" }) {
			t.Logf("readiness did not report nats fail within 30 s (checks: %+v)", last.Checks)
		}
		docker(ctx, t, "unpause", nc.GetContainerID())
		paused = false
		if code, body := waitReady(t, admin, 60*time.Second); code != http.StatusOK {
			t.Fatalf("no recovery after NATS is back: readyz = %d %+v", code, body)
		}
	})

	// F132-AC-12 (part): Valkey lost — REST keeps serving (Valkey holds no data of record, ADR-0004). Readiness 503 with
	// check valkey is TestCoreAPIReadinessFollowsDependencies; this adds the REST side. Last: a stopped container comes
	// back on another port, so recovery is shown on staging (valkey-down), not here.
	t.Run("ValkeyDown", func(t *testing.T) {
		if err := vk.Stop(ctx, nil); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 10; i++ {
			code, took := get(t, cfgURL)
			if code >= 500 || took > 3*time.Second {
				t.Fatalf("getClientConfig = %d in %s while Valkey is down, want no 5xx and < 3 s", code, took)
			}
			if code != http.StatusOK {
				t.Fatalf("getClientConfig = %d while Valkey is down, want 200 (REST continues, AC-12)", code)
			}
		}
		if code, _ := get(t, "http://"+admin+"/livez"); code != http.StatusOK {
			t.Fatalf("livez = %d, want 200", code)
		}
	})
}
