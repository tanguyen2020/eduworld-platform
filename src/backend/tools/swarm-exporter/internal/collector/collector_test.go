package collector_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/swarm-exporter/internal/collector"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/swarm-exporter/internal/dockerapi"
)

// MOCK (AGENTS R17): fakeDocker is an httptest stand-in for docker-api-proxy serving canned Engine API JSON.
// It records every request so the test can assert the exporter only sends GET /services, /tasks, /nodes.
type fakeDocker struct {
	mu       sync.Mutex
	services []any
	tasks    []any
	nodes    []any
	fail     string // path answering 500 ("" = none)
	requests []string
}

func (f *fakeDocker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	path := strings.TrimPrefix(r.URL.Path, "/"+dockerapi.APIVersion)
	if r.Method != http.MethodGet {
		http.Error(w, "denied", http.StatusForbidden)
		return
	}
	if path == f.fail {
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	}
	var body any
	switch path {
	case "/services":
		body = f.services
	case "/tasks":
		body = f.tasks
	case "/nodes":
		body = f.nodes
	default:
		http.Error(w, "denied", http.StatusForbidden)
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func replicated(id, name string, n int) any {
	return map[string]any{"ID": id, "Spec": map[string]any{"Name": name,
		"Labels": map[string]string{"com.docker.stack.namespace": "eduworld"},
		"Mode":   map[string]any{"Replicated": map[string]any{"Replicas": n}}}}
}

func global(id, name string) any {
	return map[string]any{"ID": id, "Spec": map[string]any{"Name": name,
		"Labels": map[string]string{"com.docker.stack.namespace": "eduworld"},
		"Mode":   map[string]any{"Global": map[string]any{}}}}
}

func task(id, svc, desired, state string, exit int) any {
	return map[string]any{"ID": id, "ServiceID": svc, "NodeID": "n1", "DesiredState": desired,
		"Status": map[string]any{"State": state, "ContainerStatus": map[string]any{"ExitCode": exit, "ContainerID": "c-" + id}}}
}

func node(id, host, role, label, state, reach string) any {
	n := map[string]any{"ID": id, "Spec": map[string]any{"Role": role, "Labels": map[string]string{}},
		"Description": map[string]any{"Hostname": host}, "Status": map[string]any{"State": state}}
	if label != "" {
		n["Spec"].(map[string]any)["Labels"] = map[string]string{"role": label}
	}
	if role == "manager" {
		n["ManagerStatus"] = map[string]any{"Reachability": reach}
	}
	return n
}

func setup(t *testing.T, f *fakeDocker) (*collector.Collector, *prometheus.Registry) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	reg := prometheus.NewRegistry()
	c, err := collector.New(dockerapi.New(srv.URL), "eduworld", reg)
	if err != nil {
		t.Fatal(err)
	}
	return c, reg
}

func gather(t *testing.T, reg *prometheus.Registry, names ...string) string {
	t.Helper()
	var b strings.Builder
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	for _, mf := range mfs {
		if !want[mf.GetName()] {
			continue
		}
		for _, m := range mf.GetMetric() {
			b.WriteString(mf.GetName())
			b.WriteString("{")
			for i, l := range m.GetLabel() {
				if i > 0 {
					b.WriteString(",")
				}
				b.WriteString(l.GetName() + "=" + l.GetValue())
			}
			b.WriteString("} ")
			switch {
			case m.GetGauge() != nil:
				b.WriteString(fmtF(m.GetGauge().GetValue()))
			case m.GetCounter() != nil:
				b.WriteString(fmtF(m.GetCounter().GetValue()))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func fmtF(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func baseCluster() *fakeDocker {
	return &fakeDocker{
		services: []any{
			replicated("s-api", "eduworld_core-api", 3),
			replicated("s-canary", "eduworld_core-api-canary", 1),
			replicated("s-chat", "eduworld_worker-chat", 2),
			replicated("s-pg", "eduworld_pg-1", 1),
			global("s-node", "eduworld_node-exporter"),
			// another stack: ignored (bounded labels, only the eduworld stack)
			map[string]any{"ID": "s-other", "Spec": map[string]any{"Name": "other_app",
				"Labels": map[string]string{"com.docker.stack.namespace": "other"},
				"Mode":   map[string]any{"Replicated": map[string]any{"Replicas": 9}}}},
		},
		tasks: []any{
			task("t1", "s-api", "running", "running", 0),
			task("t2", "s-api", "running", "running", 0),
			task("t3", "s-api", "running", "starting", 0),  // not running yet
			task("t4", "s-api", "shutdown", "shutdown", 0), // old task of a rolling update
			task("t5", "s-canary", "running", "running", 0),
			task("t6", "s-chat", "running", "running", 0),
			task("t7", "s-chat", "shutdown", "failed", 1), // already in the history at start: baseline
			task("t8", "s-pg", "running", "running", 0),
			task("g1", "s-node", "running", "running", 0),
			task("g2", "s-node", "running", "running", 0),
			task("g3", "s-node", "running", "pending", 0),
			task("g4", "s-node", "shutdown", "complete", 0),
			task("o1", "s-other", "running", "running", 0),
		},
		nodes: []any{
			node("n1", "mgr-1", "manager", "", "ready", "reachable"),
			node("n2", "mgr-2", "manager", "", "down", "unreachable"),
			node("n3", "app-1", "worker", "app", "ready", ""),
			node("n4", "ops-1", "worker", "ops", "down", ""),
		},
	}
}

func TestLabels(t *testing.T) {
	cases := []struct{ name, swarm, service, profile string }{
		{"eduworld_worker-chat", "worker-chat", "worker", "chat"},
		{"eduworld_worker-events", "worker-events", "worker", "events"},
		{"eduworld_core-api", "core-api", "core-api", ""},
		{"eduworld_core-api-canary", "core-api-canary", "core-api-canary", ""},
		{"eduworld_pg-1", "pg-1", "pg-1", ""},
		{"eduworld_worker-", "worker-", "worker-", ""},
		{"core-api", "core-api", "core-api", ""},
	}
	for _, c := range cases {
		sw, svc, prof := collector.Labels("eduworld", c.name)
		if sw != c.swarm || svc != c.service || prof != c.profile {
			t.Errorf("Labels(%q) = %q,%q,%q; want %q,%q,%q", c.name, sw, svc, prof, c.swarm, c.service, c.profile)
		}
	}
}

// F131-AC-01 (T009): the four MONITORING §1 metrics from a fake Docker API.
func TestPollExportsServiceAndNodeMetrics(t *testing.T) {
	f := baseCluster()
	c, reg := setup(t, f)
	if err := c.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := gather(t, reg, "swarm_service_replicas_desired", "swarm_service_replicas_running")
	want := []string{
		`swarm_service_replicas_desired{profile=,service=core-api,swarm_service=core-api} 3`,
		`swarm_service_replicas_running{profile=,service=core-api,swarm_service=core-api} 2`,
		`swarm_service_replicas_desired{profile=,service=core-api-canary,swarm_service=core-api-canary} 1`,
		`swarm_service_replicas_desired{profile=chat,service=worker,swarm_service=worker-chat} 2`,
		`swarm_service_replicas_running{profile=chat,service=worker,swarm_service=worker-chat} 1`,
		`swarm_service_replicas_desired{profile=,service=pg-1,swarm_service=pg-1} 1`,
		// global: desired = tasks with desired state running (one per eligible node)
		`swarm_service_replicas_desired{profile=,service=node-exporter,swarm_service=node-exporter} 3`,
		`swarm_service_replicas_running{profile=,service=node-exporter,swarm_service=node-exporter} 2`,
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %s in\n%s", w, got)
		}
	}
	if strings.Contains(got, "other") {
		t.Errorf("service of another stack exported:\n%s", got)
	}

	nodes := gather(t, reg, "swarm_node_ready", "swarm_node_manager_reachable")
	for _, w := range []string{
		`swarm_node_ready{manager=true,node=mgr-1,role=} 1`,
		`swarm_node_ready{manager=true,node=mgr-2,role=} 0`,
		`swarm_node_ready{manager=false,node=app-1,role=app} 1`,
		`swarm_node_ready{manager=false,node=ops-1,role=ops} 0`,
		`swarm_node_manager_reachable{node=mgr-1} 1`,
		`swarm_node_manager_reachable{node=mgr-2} 0`,
	} {
		if !strings.Contains(nodes, w) {
			t.Errorf("missing %s in\n%s", w, nodes)
		}
	}
	if strings.Contains(nodes, "swarm_node_manager_reachable{node=app-1}") {
		t.Error("manager reachability exported for a worker node")
	}

	// read-only: only GET of the three allowlisted paths
	for _, r := range f.requests {
		if !strings.HasPrefix(r, "GET /"+dockerapi.APIVersion+"/") {
			t.Errorf("unexpected request %s", r)
		}
	}
}

func TestLabelsAreBounded(t *testing.T) {
	c, reg := setup(t, baseCluster())
	if err := c.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := gather(t, reg, "swarm_service_replicas_desired", "swarm_service_replicas_running", "swarm_task_restarts_total", "swarm_node_ready", "swarm_node_manager_reachable")
	for _, bad := range []string{"t1", "c-t1", "s-api", "n1=", "image"} {
		if strings.Contains(out, bad) {
			t.Errorf("unbounded label value %q leaked:\n%s", bad, out)
		}
	}
}

func TestRestartsCountedOncePerTask(t *testing.T) {
	f := baseCluster()
	c, reg := setup(t, f)
	ctx := context.Background()
	restarts := func() string { return gather(t, reg, "swarm_task_restarts_total") }

	if err := c.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	// baseline: t7 was already failed in the history when the exporter started → not a new restart
	if !strings.Contains(restarts(), `swarm_task_restarts_total{profile=chat,service=worker,swarm_service=worker-chat} 0`) {
		t.Fatalf("baseline should be 0:\n%s", restarts())
	}
	if !strings.Contains(restarts(), `swarm_task_restarts_total{profile=,service=core-api,swarm_service=core-api} 0`) {
		t.Fatalf("every service has a restart series from the start:\n%s", restarts())
	}

	// two new crashes of worker-chat (failed) and one rejected core-api task
	f.mu.Lock()
	f.tasks = append(f.tasks,
		task("t9", "s-chat", "shutdown", "failed", 2),
		task("t10", "s-chat", "shutdown", "failed", 137),
		task("t11", "s-api", "shutdown", "rejected", 0),
		task("t12", "s-api", "shutdown", "shutdown", 143), // stopped by an update: not a restart
	)
	f.mu.Unlock()
	if err := c.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	// the same failed tasks are still in the history on the next polls: never counted twice
	for range 3 {
		if err := c.Poll(ctx); err != nil {
			t.Fatal(err)
		}
	}
	got := restarts()
	if !strings.Contains(got, `swarm_task_restarts_total{profile=chat,service=worker,swarm_service=worker-chat} 2`) {
		t.Errorf("worker-chat restarts:\n%s", got)
	}
	if !strings.Contains(got, `swarm_task_restarts_total{profile=,service=core-api,swarm_service=core-api} 1`) {
		t.Errorf("core-api restarts:\n%s", got)
	}

	// history trimmed by Swarm: the counter never goes down
	f.mu.Lock()
	f.tasks = f.tasks[:6]
	f.mu.Unlock()
	if err := c.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restarts(), `swarm_task_restarts_total{profile=chat,service=worker,swarm_service=worker-chat} 2`) {
		t.Errorf("counter decreased after the history was trimmed:\n%s", restarts())
	}
}

func TestAPIErrorKeepsLastGoodValues(t *testing.T) {
	for _, path := range []string{"/services", "/tasks", "/nodes"} {
		t.Run(path, func(t *testing.T) {
			f := baseCluster()
			c, reg := setup(t, f)
			ctx := context.Background()
			if err := c.Poll(ctx); err != nil {
				t.Fatal(err)
			}
			before := gather(t, reg, "swarm_service_replicas_desired", "swarm_service_replicas_running", "swarm_node_ready", "swarm_exporter_last_success_timestamp_seconds")

			f.mu.Lock()
			f.fail = path
			f.services = f.services[:1] // would change the snapshot if it were applied
			f.mu.Unlock()
			if err := c.Poll(ctx); err == nil {
				t.Fatal("want an error")
			}
			if err := c.Poll(ctx); err == nil {
				t.Fatal("want an error")
			}
			after := gather(t, reg, "swarm_service_replicas_desired", "swarm_service_replicas_running", "swarm_node_ready", "swarm_exporter_last_success_timestamp_seconds")
			if before != after {
				t.Errorf("snapshot changed on error:\nbefore\n%s\nafter\n%s", before, after)
			}
			if got := gather(t, reg, "swarm_exporter_scrape_errors_total"); !strings.Contains(got, "swarm_exporter_scrape_errors_total{} 2") {
				t.Errorf("scrape errors: %s", got)
			}
		})
	}
}

func TestLastSuccessTimestamp(t *testing.T) {
	c, reg := setup(t, baseCluster())
	if got := gather(t, reg, "swarm_exporter_last_success_timestamp_seconds"); !strings.Contains(got, "{} 0") {
		t.Fatalf("before first poll: %s", got)
	}
	if err := c.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := gather(t, reg, "swarm_exporter_last_success_timestamp_seconds"); strings.Contains(got, "{} 0\n") {
		t.Fatalf("after poll: %s", got)
	}
}

func TestRemovedServiceDropsSeries(t *testing.T) {
	f := baseCluster()
	c, reg := setup(t, f)
	ctx := context.Background()
	if err := c.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.services = f.services[:1]
	f.mu.Unlock()
	if err := c.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	got := gather(t, reg, "swarm_service_replicas_desired", "swarm_task_restarts_total")
	if strings.Contains(got, "worker-chat") || !strings.Contains(got, "core-api") {
		t.Errorf("series after removal:\n%s", got)
	}
}
