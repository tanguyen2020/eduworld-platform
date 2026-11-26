// Package collector turns the Docker Engine API view of the swarm into the Prometheus metrics named by
// MONITORING §1 (F131-AC-01, T016): swarm_service_replicas_desired, swarm_service_replicas_running,
// swarm_task_restarts_total, swarm_node_ready, plus swarm_node_manager_reachable (manager quorum) and the
// exporter's own swarm_exporter_scrape_errors_total and swarm_exporter_last_success_timestamp_seconds.
//
// Labels stay bounded (MONITORING §2): Swarm service name, binary (service) and worker profile for services;
// hostname, node label role and manager flag for nodes. Never task IDs, container IDs or image names.
//
// The collector polls on its own interval (Poll) and serves the last good snapshot on every scrape (Collect),
// so a scrape never calls the Docker API and an API error keeps the previous values.
package collector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/swarm-exporter/internal/dockerapi"
)

// Source is the read-only Docker API the collector needs (dockerapi.Client in production).
type Source interface {
	ListServices(ctx context.Context) ([]dockerapi.Service, error)
	ListTasks(ctx context.Context) ([]dockerapi.Task, error)
	ListNodes(ctx context.Context) ([]dockerapi.Node, error)
}

var (
	serviceLabels  = []string{"swarm_service", "service", "profile"}
	descDesired    = prometheus.NewDesc("swarm_service_replicas_desired", "Desired tasks of a Swarm service (replicated: spec replicas; global: tasks with desired state running).", serviceLabels, nil)
	descRunning    = prometheus.NewDesc("swarm_service_replicas_running", "Tasks of a Swarm service whose state and desired state are both running.", serviceLabels, nil)
	descRestarts   = prometheus.NewDesc("swarm_task_restarts_total", "Tasks of a Swarm service newly observed in a failed or rejected state since the exporter started (each task counted once).", serviceLabels, nil)
	descNodeReady  = prometheus.NewDesc("swarm_node_ready", "1 when the Swarm node status is ready, else 0.", []string{"node", "role", "manager"}, nil)
	descReachable  = prometheus.NewDesc("swarm_node_manager_reachable", "1 when a manager node is reachable in the Raft quorum, else 0 (managers only).", []string{"node"}, nil)
	workerPrefix   = "worker-"
	restartedState = map[string]bool{"failed": true, "rejected": true}
)

type serviceKey struct{ swarm, service, profile string }

func (k serviceKey) values() []string { return []string{k.swarm, k.service, k.profile} }

type serviceSample struct {
	key              serviceKey
	desired, running float64
}

type nodeSample struct {
	node, role string
	manager    bool
	ready      bool
	reachable  bool
}

// Collector is a prometheus.Collector fed by Poll.
type Collector struct {
	src   Source
	stack string
	now   func() time.Time

	scrapeErrors prometheus.Counter
	lastSuccess  prometheus.Gauge

	mu       sync.Mutex
	services []serviceSample
	nodes    []nodeSample
	restarts map[serviceKey]float64
	// seen holds the IDs of terminal tasks already accounted for. After every poll it is pruned to the task
	// IDs the manager still returns, so its size is bounded by the Swarm task history (task-history-limit per
	// slot); a task that left the history can never come back.
	seen      map[string]struct{}
	baselined bool
}

// New returns a collector for the services of stack (label com.docker.stack.namespace; empty = every
// service). It registers its own scrape error and last-success metrics on reg together with itself.
func New(src Source, stack string, reg prometheus.Registerer) (*Collector, error) {
	c := &Collector{
		src: src, stack: stack, now: time.Now,
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "swarm_exporter_scrape_errors_total", Help: "Polls of the Docker API that failed (the previous values are kept).",
		}),
		lastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "swarm_exporter_last_success_timestamp_seconds", Help: "Unix time of the last successful poll of the Docker API.",
		}),
		restarts: map[serviceKey]float64{},
		seen:     map[string]struct{}{},
	}
	for _, m := range []prometheus.Collector{c, c.scrapeErrors, c.lastSuccess} {
		if err := reg.Register(m); err != nil {
			return nil, fmt.Errorf("register swarm-exporter metrics: %w", err)
		}
	}
	return c, nil
}

// Labels maps a Swarm service name to the bounded label set: the stack prefix "<stack>_" is stripped,
// worker-<p> becomes service="worker", profile="<p>" (ADR-0074 profiles), anything else keeps its name as
// service with an empty profile.
func Labels(stack, name string) (swarm, service, profile string) {
	swarm = name
	if stack != "" {
		swarm = strings.TrimPrefix(name, stack+"_")
	}
	if p, ok := strings.CutPrefix(swarm, workerPrefix); ok && p != "" {
		return swarm, "worker", p
	}
	return swarm, swarm, ""
}

// Poll reads services, tasks and nodes once and replaces the snapshot. On any API error nothing changes
// except swarm_exporter_scrape_errors_total.
func (c *Collector) Poll(ctx context.Context) error {
	services, err := c.src.ListServices(ctx)
	if err == nil {
		var tasks []dockerapi.Task
		if tasks, err = c.src.ListTasks(ctx); err == nil {
			var nodes []dockerapi.Node
			if nodes, err = c.src.ListNodes(ctx); err == nil {
				c.apply(services, tasks, nodes)
				c.lastSuccess.Set(float64(c.now().Unix()))
				return nil
			}
		}
	}
	c.scrapeErrors.Inc()
	return err
}

// apply computes the snapshot. Restart semantics (swarm_task_restarts_total): Swarm replaces a crashed
// container by a new task and keeps the old one in its bounded history in state failed (non-zero exit) or
// rejected (could not start). The counter is incremented once per task ID the first time it is seen in such a
// state — never derived from the raw list, whose length is capped by the history and would go down. A task
// stopped by a rolling update or scale-down ends in shutdown/complete and is not a restart. The first
// successful poll only records the terminal tasks already in the history (baseline) so an exporter restart
// does not report the whole history as new restarts; failures that happen while the exporter is down are
// therefore not counted.
func (c *Collector) apply(services []dockerapi.Service, tasks []dockerapi.Task, nodes []dockerapi.Node) {
	type acc struct {
		key              serviceKey
		replicated       bool
		desired, running float64
	}
	byID := make(map[string]*acc, len(services))
	order := make([]string, 0, len(services))
	for _, s := range services {
		if c.stack != "" && s.Stack != c.stack {
			continue
		}
		sw, svc, prof := Labels(c.stack, s.Name)
		a := &acc{key: serviceKey{sw, svc, prof}}
		if s.Replicas != nil {
			a.replicated, a.desired = true, float64(*s.Replicas)
		}
		byID[s.ID] = a
		order = append(order, s.ID)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	present := make(map[string]struct{}, len(tasks))
	for _, t := range tasks {
		a, ok := byID[t.ServiceID]
		if !ok {
			continue
		}
		if t.DesiredState == "running" {
			if !a.replicated {
				a.desired++
			}
			if t.State == "running" {
				a.running++
			}
		}
		if !restartedState[t.State] {
			continue
		}
		present[t.ID] = struct{}{}
		if _, done := c.seen[t.ID]; done {
			continue
		}
		c.seen[t.ID] = struct{}{}
		if c.baselined {
			c.restarts[a.key]++
		}
	}
	for id := range c.seen {
		if _, ok := present[id]; !ok {
			delete(c.seen, id)
		}
	}
	c.baselined = true

	c.services = c.services[:0]
	keep := make(map[serviceKey]bool, len(order))
	for _, id := range order {
		a := byID[id]
		c.services = append(c.services, serviceSample{key: a.key, desired: a.desired, running: a.running})
		keep[a.key] = true
		if _, ok := c.restarts[a.key]; !ok {
			c.restarts[a.key] = 0 // the series exists from the start so increase() sees the first restart
		}
	}
	for k := range c.restarts {
		if !keep[k] {
			delete(c.restarts, k) // service removed from the stack
		}
	}

	c.nodes = c.nodes[:0]
	for _, n := range nodes {
		name := n.Hostname
		if name == "" {
			name = n.ID
		}
		c.nodes = append(c.nodes, nodeSample{node: name, role: n.Labels["role"], manager: n.Role == "manager",
			ready: n.State == "ready", reachable: n.Reachability == "reachable"})
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{descDesired, descRunning, descRestarts, descNodeReady, descReachable} {
		ch <- d
	}
}

// Collect implements prometheus.Collector: the last good snapshot.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.services {
		ch <- prometheus.MustNewConstMetric(descDesired, prometheus.GaugeValue, s.desired, s.key.values()...)
		ch <- prometheus.MustNewConstMetric(descRunning, prometheus.GaugeValue, s.running, s.key.values()...)
	}
	for k, v := range c.restarts {
		ch <- prometheus.MustNewConstMetric(descRestarts, prometheus.CounterValue, v, k.values()...)
	}
	for _, n := range c.nodes {
		ch <- prometheus.MustNewConstMetric(descNodeReady, prometheus.GaugeValue, b2f(n.ready), n.node, n.role, fmt.Sprint(n.manager))
		if n.manager {
			ch <- prometheus.MustNewConstMetric(descReachable, prometheus.GaugeValue, b2f(n.reachable), n.node)
		}
	}
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
