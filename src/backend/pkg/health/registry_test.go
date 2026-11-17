package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
)

type readyBody struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Status    string `json:"status"`
		ErrorCode string `json:"error_code"`
	} `json:"checks"`
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// F001-AC-04: /livez never calls dependency checks.
func TestLivezDoesNotRunChecks(t *testing.T) {
	var calls atomic.Int32
	r := health.NewRegistry(buildinfo.Get("core-api", ""))
	r.Register("db:identity", health.Readiness, func(context.Context) error { calls.Add(1); return errors.New("down") })

	rec := get(t, r.Handler(), "/livez")
	if rec.Code != http.StatusOK {
		t.Fatalf("livez status = %d, want 200", rec.Code)
	}
	if calls.Load() != 0 {
		t.Fatalf("livez ran %d checks, want 0", calls.Load())
	}
}

// F001-AC-04, F001-AC-05: /readyz is 200 when every readiness check passes, 503 with the failing check name otherwise.
func TestReadyzReportsFailingCheck(t *testing.T) {
	r := health.NewRegistry(buildinfo.Get("core-api", ""))
	r.Register("db:identity", health.Readiness, func(context.Context) error { return nil })
	r.Register("valkey", health.Readiness, func(context.Context) error {
		return errors.New("dial tcp 10.0.0.1:6379: connection refused password=secret")
	})

	rec := get(t, r.Handler(), "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want 503", rec.Code)
	}
	var body readyBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("readyz body is not JSON: %v", err)
	}
	if body.Status != "fail" || body.Checks["valkey"].Status != "fail" || body.Checks["db:identity"].Status != "ok" {
		t.Fatalf("unexpected readyz body: %+v", body)
	}
	if body.Checks["valkey"].ErrorCode == "" {
		t.Fatalf("failing check has no error_code")
	}
	if strings.Contains(rec.Body.String(), "secret") || strings.Contains(rec.Body.String(), "10.0.0.1") {
		t.Fatalf("readyz leaks error details: %s", rec.Body.String())
	}
}

func TestReadyzOK(t *testing.T) {
	r := health.NewRegistry(buildinfo.Get("realtime", ""))
	r.Register("nats", health.Readiness, func(context.Context) error { return nil })
	rec := get(t, r.Handler(), "/readyz")
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, want 200", rec.Code)
	}
}

// Readiness checks run in parallel and are bounded by the 2 s timeout.
func TestReadyzTimeout(t *testing.T) {
	r := health.NewRegistry(buildinfo.Get("worker", "events"))
	r.SetTimeout(100 * time.Millisecond)
	r.Register("slow", health.Readiness, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	r.Register("slow2", health.Readiness, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })

	start := time.Now()
	rec := get(t, r.Handler(), "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want 503", rec.Code)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("checks not run in parallel/bounded: took %v", d)
	}
}

// Info checks appear in /healthz but never fail /readyz.
func TestHealthzIncludesInfoChecks(t *testing.T) {
	r := health.NewRegistry(buildinfo.Get("core-api", ""))
	r.Register("db:identity", health.Readiness, func(context.Context) error { return nil })
	r.Register("ext:keycloak", health.Info, func(context.Context) error { return errors.New("down") })

	if rec := get(t, r.Handler(), "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("info check failed readiness: %d", rec.Code)
	}
	rec := get(t, r.Handler(), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ext:keycloak") {
		t.Fatalf("healthz misses info check: %s", rec.Body.String())
	}
}

// F001-AC-04, F001-AC-19: /version returns build metadata; worker includes its profile.
func TestVersion(t *testing.T) {
	r := health.NewRegistry(buildinfo.Get("worker", "chat"))
	rec := get(t, r.Handler(), "/version")
	if rec.Code != http.StatusOK {
		t.Fatalf("version status = %d", rec.Code)
	}
	var info buildinfo.Info
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Binary != "worker" || info.Profile != "chat" || info.Version == "" || info.Commit == "" || info.BuildTime == "" {
		t.Fatalf("unexpected version: %+v", info)
	}
}

// F001-AC-04: /metrics exposes eduworld_build_info with binary and profile labels.
func TestMetricsBuildInfo(t *testing.T) {
	r := health.NewRegistry(buildinfo.Get("worker", "jobs"))
	rec := get(t, r.Handler(), "/metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "eduworld_build_info{") || !strings.Contains(body, `binary="worker"`) || !strings.Contains(body, `profile="jobs"`) {
		t.Fatalf("build info metric missing: %s", body)
	}
}

func TestDuplicateCheckPanics(t *testing.T) {
	r := health.NewRegistry(buildinfo.Get("core-api", ""))
	r.Register("valkey", health.Readiness, func(context.Context) error { return nil })
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate registration did not panic")
		}
	}()
	r.Register("valkey", health.Readiness, func(context.Context) error { return nil })
}
