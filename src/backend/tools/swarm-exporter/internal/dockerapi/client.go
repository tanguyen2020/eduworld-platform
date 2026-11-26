// Package dockerapi is the read-only adapter of swarm-exporter to the Docker Engine API of the Swarm manager
// (F131 T016, MONITORING §1). It never talks to a Docker socket: every call goes through the HAProxy allowlist
// proxy docker-api-proxy (deploy/ops-scheduler/socket-proxy.cfg) and this client only ever sends
// GET /services, GET /tasks and GET /nodes — it has no write method at all.
package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// APIVersion is the Engine API version prefix (Docker Engine ≥ 25 serves it; same as ops-scheduler).
const APIVersion = "v1.44"

// maxBody bounds one answer of the Engine API (task history of a large swarm stays well below it).
const maxBody = 32 << 20

// Client calls the proxy at BaseURL (e.g. http://docker-api-proxy:2375).
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client with a bounded timeout (external integration: R29).
func New(base string) *Client {
	return &Client{BaseURL: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Service is the subset of a Swarm service the exporter needs.
type Service struct {
	ID    string
	Name  string
	Stack string // label com.docker.stack.namespace, empty for a service created outside a stack
	// Replicas is the desired replica count of a replicated service; nil for a global service.
	Replicas *uint64
}

// Task is the subset of a Swarm task the exporter needs.
type Task struct {
	ID           string
	ServiceID    string
	NodeID       string
	DesiredState string // running | shutdown | …
	State        string // new … running | complete | shutdown | failed | rejected | orphaned | remove
	ExitCode     int
}

// Node is the subset of a Swarm node the exporter needs.
type Node struct {
	ID       string
	Hostname string
	Role     string // manager | worker
	Labels   map[string]string
	State    string // ready | down | unknown | disconnected
	// Reachability is the manager reachability (reachable | unreachable | unknown), empty for a worker.
	Reachability string
}

// Error is a non-2xx answer of the Engine API (or of the proxy: 403 for a denied call).
type Error struct {
	Path   string
	Status int
}

func (e *Error) Error() string { return fmt.Sprintf("docker api GET %s: status %d", e.Path, e.Status) }

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/"+APIVersion+path, nil)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("docker api GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return &Error{Path: path, Status: resp.StatusCode}
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// ListServices returns every service of the swarm.
func (c *Client) ListServices(ctx context.Context) ([]Service, error) {
	var raw []struct {
		ID   string
		Spec struct {
			Name   string
			Labels map[string]string
			Mode   struct {
				Replicated *struct{ Replicas *uint64 }
				Global     *struct{}
			}
		}
	}
	if err := c.get(ctx, "/services", &raw); err != nil {
		return nil, err
	}
	out := make([]Service, 0, len(raw))
	for _, r := range raw {
		s := Service{ID: r.ID, Name: r.Spec.Name, Stack: r.Spec.Labels["com.docker.stack.namespace"]}
		if rep := r.Spec.Mode.Replicated; rep != nil {
			n := uint64(0)
			if rep.Replicas != nil {
				n = *rep.Replicas
			}
			s.Replicas = &n
		}
		out = append(out, s)
	}
	return out, nil
}

// ListTasks returns every task the manager still keeps (running tasks and the bounded task history).
func (c *Client) ListTasks(ctx context.Context) ([]Task, error) {
	var raw []struct {
		ID           string
		ServiceID    string
		NodeID       string
		DesiredState string
		Status       struct {
			State           string
			ContainerStatus *struct{ ExitCode int }
		}
	}
	if err := c.get(ctx, "/tasks", &raw); err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(raw))
	for _, r := range raw {
		t := Task{ID: r.ID, ServiceID: r.ServiceID, NodeID: r.NodeID, DesiredState: r.DesiredState, State: r.Status.State}
		if r.Status.ContainerStatus != nil {
			t.ExitCode = r.Status.ContainerStatus.ExitCode
		}
		out = append(out, t)
	}
	return out, nil
}

// ListNodes returns every node of the swarm.
func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	var raw []struct {
		ID   string
		Spec struct {
			Labels map[string]string
			Role   string
		}
		Description struct{ Hostname string }
		Status      struct{ State string }
		// ManagerStatus is only present on manager nodes.
		ManagerStatus *struct{ Reachability string }
	}
	if err := c.get(ctx, "/nodes", &raw); err != nil {
		return nil, err
	}
	out := make([]Node, 0, len(raw))
	for _, r := range raw {
		n := Node{ID: r.ID, Hostname: r.Description.Hostname, Role: r.Spec.Role, Labels: r.Spec.Labels, State: r.Status.State}
		if r.ManagerStatus != nil {
			n.Reachability = r.ManagerStatus.Reachability
		}
		out = append(out, n)
	}
	return out, nil
}
