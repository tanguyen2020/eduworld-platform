//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring/wiringtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

type readiness struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Status    string `json:"status"`
		ErrorCode string `json:"error_code"`
	} `json:"checks"`
}

// envFor is the complete configuration of core-api on the test cluster, with a fake OIDC issuer (httpxtest.IDP)
// so the readiness check jwks passes (OQ F010-2: core-api serves every module and needs its whole configuration).
func envFor(t *testing.T, c *testkit.Cluster, valkey, nats string) func(string) string {
	t.Helper()
	env := wiringtest.Env(wiringtest.Options{RepoRoot: testkit.RepoRoot(t), DSN: c.ModuleDSN, Issuer: httpxtest.NewIDP(t).Issuer(),
		ValkeyAddr: valkey, NATSURL: nats, SecretsDir: t.TempDir()})
	return func(k string) string { return env[k] }
}

func startCoreAPI(t *testing.T, getenv func(string) string) (admin string, stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	addrs := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- run(ctx, getenv, func(a, _ string) { addrs <- a }) }()
	select {
	case admin = <-addrs:
	case err := <-done:
		t.Fatalf("core-api exited: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("core-api did not start")
	}
	return admin, func() { cancel(); <-done }
}

func readyz(t *testing.T, admin string) (int, readiness) {
	t.Helper()
	resp, err := http.Get("http://" + admin + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r readiness
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, r
}

func status(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// F001-AC-04, F001-AC-05: ready with every module database, Valkey and NATS; Valkey down → 503
// with check "valkey" failing while /livez stays 200.
func TestCoreAPIReadinessFollowsDependencies(t *testing.T) {
	ctx := context.Background()
	cluster := testkit.StartCluster(ctx, t, true)
	vk, vkAddr := testkit.StartValkey(ctx, t)
	_, natsURL := testkit.StartNATS(ctx, t)

	admin, stop := startCoreAPI(t, envFor(t, cluster, vkAddr, natsURL))
	defer stop()

	code, body := waitReady(t, admin, 30*time.Second)
	if code != http.StatusOK {
		t.Fatalf("readyz = %d %+v, want 200", code, body)
	}
	for _, m := range testkit.Modules {
		for _, name := range []string{"db:" + m, "atlas:" + m} {
			if body.Checks[name].Status != "ok" {
				t.Fatalf("check %s = %+v", name, body.Checks[name])
			}
		}
	}

	if err := vk.Stop(ctx, nil); err != nil {
		t.Fatal(err)
	}
	code, body = readyz(t, admin)
	if code != http.StatusServiceUnavailable || body.Checks["valkey"].Status != "fail" {
		t.Fatalf("after valkey stop: readyz = %d %+v, want 503 with valkey fail", code, body)
	}
	if body.Checks["db:identity"].Status != "ok" {
		t.Fatalf("unrelated checks must stay ok: %+v", body.Checks)
	}
	if got := status(t, "http://"+admin+"/livez"); got != http.StatusOK {
		t.Fatalf("livez = %d, want 200", got)
	}
}

func waitReady(t *testing.T, admin string, d time.Duration) (int, readiness) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		code, body := readyz(t, admin)
		if code == http.StatusOK || time.Now().After(deadline) {
			return code, body
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// F001-AC-03 (T018): on an unmigrated database core-api reports atlas:<module> not_migrated and
// does not create any schema itself.
func TestCoreAPIDoesNotAutoMigrate(t *testing.T) {
	ctx := context.Background()
	cluster := testkit.StartCluster(ctx, t, false)
	_, vkAddr := testkit.StartValkey(ctx, t)
	_, natsURL := testkit.StartNATS(ctx, t)

	admin, stop := startCoreAPI(t, envFor(t, cluster, vkAddr, natsURL))
	defer stop()

	time.Sleep(time.Second)
	code, body := readyz(t, admin)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("readyz = %d, want 503", code)
	}
	for _, m := range testkit.Modules {
		c := body.Checks["atlas:"+m]
		if c.Status != "fail" || c.ErrorCode != "not_migrated" {
			t.Fatalf("atlas:%s = %+v, want fail/not_migrated", m, c)
		}
	}
	// No module schema exists: the binary never ran DDL.
	_, out, err := cluster.Container.Exec(ctx, []string{"psql", "-U", "postgres", "-d", "eduworld_identity", "-tAc",
		"SELECT count(*) FROM pg_namespace WHERE nspname IN ('identity','infra','atlas_schema_revisions')"})
	if err != nil {
		t.Fatal(err)
	}
	buf := new(strings.Builder)
	if _, err := fmt.Fprint(buf, readAll(out)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(buf.String()), "0") {
		t.Fatalf("schemas created without migrate: %q", buf.String())
	}
}
