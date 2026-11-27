package scaler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/dockerapi"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/scaler"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/schedule"
)

const sched = `version: 1
stack: eduworld
lead_minutes: 15
windows:
  - { name: morning, start: "06:45", end: "08:00" }
services:
  - { name: core-api, baseline: 2, peak: 4 }
  - { name: keycloak, baseline: 2, peak: 3 }
`

// fakeEngine is an in-memory Docker Engine API behind an allowlist like deploy/ops-scheduler/socket-proxy.cfg.
type fakeEngine struct {
	mu       sync.Mutex
	replicas map[string]uint64
	updates  []string
	failNext bool
	cpuFree  int64 // NanoCPUs of the single app node
}

func (f *fakeEngine) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerapi.APIVersion)
		switch {
		case r.Method == http.MethodGet && path == "/services":
			var out []map[string]any
			for _, name := range []string{"core-api", "keycloak"} {
				role := "app"
				if name == "keycloak" {
					role = "idp"
				}
				out = append(out, map[string]any{"ID": "id-" + name, "Version": map[string]any{"Index": 7}, "Spec": map[string]any{
					"Name": "eduworld_" + name, "Labels": map[string]any{"com.docker.stack.namespace": "eduworld"},
					"Mode": map[string]any{"Replicated": map[string]any{"Replicas": f.replicas[name]}},
					"TaskTemplate": map[string]any{
						"Resources": map[string]any{"Reservations": map[string]any{"NanoCPUs": 2e9, "MemoryBytes": 1 << 30}},
						"Placement": map[string]any{"Constraints": []string{"node.labels.role == " + role}},
					},
				}})
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodGet && path == "/nodes":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"ID": "n-app", "Spec": map[string]any{"Labels": map[string]string{"role": "app"}, "Role": "worker", "Availability": "active"},
					"Description": map[string]any{"Resources": map[string]any{"NanoCPUs": f.cpuFree, "MemoryBytes": 64 << 30}},
					"Status":      map[string]any{"State": "ready"}},
				{"ID": "n-idp", "Spec": map[string]any{"Labels": map[string]string{"role": "idp"}, "Role": "worker", "Availability": "active"},
					"Description": map[string]any{"Resources": map[string]any{"NanoCPUs": 1e9, "MemoryBytes": 64 << 30}},
					"Status":      map[string]any{"State": "ready"}},
			})
		case r.Method == http.MethodGet && path == "/tasks":
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPost && strings.HasPrefix(path, "/services/") && strings.HasSuffix(path, "/update"):
			if f.failNext {
				f.failNext = false
				http.Error(w, `{"message":"rpc error: update out of sequence"}`, http.StatusInternalServerError)
				return
			}
			if r.URL.Query().Get("version") != "7" {
				t.Errorf("update without the version read: %s", r.URL.RawQuery)
			}
			var spec map[string]any
			_ = json.NewDecoder(r.Body).Decode(&spec)
			name := strings.TrimPrefix(spec["Name"].(string), "eduworld_")
			n := spec["Mode"].(map[string]any)["Replicated"].(map[string]any)["Replicas"].(float64)
			f.replicas[name] = uint64(n)
			f.updates = append(f.updates, name)
		default:
			// Everything else is denied, like the HAProxy allowlist (POST /services/create, DELETE, exec…).
			http.Error(w, "forbidden by ops-scheduler allowlist", http.StatusForbidden)
		}
	})
}

func newScaler(t *testing.T, eng *fakeEngine) (*scaler.Scaler, *metrics.Metrics) {
	t.Helper()
	s, err := schedule.Parse([]byte(sched), nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := schedule.NewPlanner(s)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(eng.handler(t))
	t.Cleanup(srv.Close)
	m := metrics.New(prometheus.NewRegistry())
	return &scaler.Scaler{Stack: "eduworld", Planner: p, Docker: dockerapi.New(srv.URL), Metrics: m}, m
}

func at(t *testing.T, hhmm string) time.Time {
	loc, _ := time.LoadLocation(schedule.Timezone)
	ts, err := time.ParseInLocation("2006-01-02 15:04", "2026-11-26 "+hhmm, loc)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// F130-AC-22: 15 minutes before the window every scheduled service goes to peak, after it back to baseline; the
// counter eduworld_ops_scale_actions_total{service,action,result} counts each action.
func TestTickScalesUpBeforeWindowAndDownAfter(t *testing.T) {
	eng := &fakeEngine{replicas: map[string]uint64{"core-api": 2, "keycloak": 2}, cpuFree: 32e9}
	eng.replicas["keycloak"] = 3 // already at peak capacity for its tiny idp node: no action needed at peak
	sc, m := newScaler(t, eng)
	ctx := context.Background()

	if out, err := sc.Tick(ctx, at(t, "06:20")); err != nil || len(out) != 1 || out[0].Service != "keycloak" || out[0].To != 2 {
		t.Fatalf("06:20 outcomes = %+v, %v (keycloak 3 → baseline 2 expected)", out, err)
	}
	out, err := sc.Tick(ctx, at(t, "06:30"))
	if err != nil {
		t.Fatal(err)
	}
	if eng.replicas["core-api"] != 4 {
		t.Fatalf("core-api replicas at 06:30 = %d, want peak 4 (%+v)", eng.replicas["core-api"], out)
	}
	if got := testutil.ToFloat64(m.ScaleActions.WithLabelValues("core-api", metrics.ActionScaleUp, metrics.ResultOK)); got != 1 {
		t.Fatalf("scale_up ok counter = %v", got)
	}
	if _, err := sc.Tick(ctx, at(t, "08:00")); err != nil {
		t.Fatal(err)
	}
	if eng.replicas["core-api"] != 2 {
		t.Fatalf("core-api replicas at 08:00 = %d, want baseline 2", eng.replicas["core-api"])
	}
	if got := testutil.ToFloat64(m.ScaleActions.WithLabelValues("core-api", metrics.ActionScaleDown, metrics.ResultOK)); got != 1 {
		t.Fatalf("scale_down ok counter = %v", got)
	}
	// Idempotent: a second tick in the same state changes nothing.
	before := len(eng.updates)
	if _, err := sc.Tick(ctx, at(t, "08:01")); err != nil || len(eng.updates) != before {
		t.Fatalf("idle tick updated services: %v %v", eng.updates, err)
	}
}

// F130-AC-23: reservations beyond the role capacity → no scale, result="rejected"; Docker API error → "error".
func TestTickRejectsOverCapacityAndRecordsErrors(t *testing.T) {
	eng := &fakeEngine{replicas: map[string]uint64{"core-api": 2, "keycloak": 2}, cpuFree: 6e9}
	sc, m := newScaler(t, eng)
	out, err := sc.Tick(context.Background(), at(t, "07:00"))
	if err != nil {
		t.Fatal(err)
	}
	// core-api +2 × 2 CPU = 4 CPU ≤ 6: ok; keycloak +1 × 2 CPU > 1 CPU of the idp node: rejected.
	if eng.replicas["core-api"] != 4 || eng.replicas["keycloak"] != 2 {
		t.Fatalf("replicas = %v (%+v)", eng.replicas, out)
	}
	if got := testutil.ToFloat64(m.ScaleActions.WithLabelValues("keycloak", metrics.ActionScaleUp, metrics.ResultRejected)); got != 1 {
		t.Fatalf("rejected counter = %v", got)
	}
	eng.replicas["core-api"] = 2
	eng.failNext = true
	if _, err := sc.Tick(context.Background(), at(t, "07:01")); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(m.ScaleActions.WithLabelValues("core-api", metrics.ActionScaleUp, metrics.ResultError)); got != 1 {
		t.Fatalf("error counter = %v", got)
	}
}

// The client only uses allowlisted calls; anything else answers 403 from the proxy and is surfaced as an error.
func TestClientSurfacesProxyDenial(t *testing.T) {
	eng := &fakeEngine{replicas: map[string]uint64{}}
	srv := httptest.NewServer(eng.handler(t))
	defer srv.Close()
	c := dockerapi.New(srv.URL)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/"+dockerapi.APIVersion+"/services/create", strings.NewReader("{}"))
	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("create = %d, want 403", resp.StatusCode)
	}
	err = c.ScaleService(context.Background(), dockerapi.Service{Name: "eduworld_x"}, 3)
	if err == nil || !strings.Contains(err.Error(), "not replicated") {
		t.Fatalf("scale of a global service = %v", err)
	}
}
