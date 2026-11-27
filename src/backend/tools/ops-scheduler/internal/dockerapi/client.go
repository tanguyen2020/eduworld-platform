// Package dockerapi is the infrastructure adapter of ops-scheduler to the Docker Engine API of the Swarm
// manager. It never talks to a Docker socket: every call goes through the HAProxy allowlist proxy
// (deploy/ops-scheduler/socket-proxy.cfg), which permits only GET /services, GET /nodes, GET /tasks and
// POST /services/{id}/update (F130-AC-23, DEPLOYMENT §4.4).
package dockerapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// APIVersion is the Engine API version prefix (Docker Engine ≥ 25 serves it).
const APIVersion = "v1.44"

// Client calls the proxy at BaseURL (e.g. http://docker-api-proxy:2375).
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client with a bounded timeout (external integration: R29).
func New(base string) *Client {
	return &Client{BaseURL: base, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Resources are the reservations of a task (NanoCPUs, MemoryBytes).
type Resources struct {
	NanoCPUs    int64 `json:"NanoCPUs,omitempty"`
	MemoryBytes int64 `json:"MemoryBytes,omitempty"`
}

// Service is the subset of a Swarm service used by the scheduler. Spec is kept raw so an update sends back the
// exact spec read (only the replica count changes).
type Service struct {
	ID      string
	Name    string
	Version uint64
	Labels  map[string]string
	// Replicas is nil for a global service.
	Replicas     *uint64
	Reservations Resources
	Constraints  []string
	Spec         map[string]any
}

// Node is a Swarm node.
type Node struct {
	ID           string
	Labels       map[string]string
	Role         string // manager | worker
	Availability string // active | pause | drain
	State        string // ready | down | …
	Capacity     Resources
}

// Task is a running task of a service.
type Task struct {
	ID           string
	ServiceID    string
	NodeID       string
	Reservations Resources
}

// Error is a non-2xx answer of the Engine API (or the proxy: 403 for a denied call).
type Error struct {
	Method, Path string
	Status       int
	Body         string
}

func (e *Error) Error() string {
	return fmt.Sprintf("docker api %s %s: %d %s", e.Method, e.Path, e.Status, e.Body)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	u := c.BaseURL + "/" + APIVersion + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s: %w", path, err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("docker api %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }() // read fully below; a close error after the read changes nothing
	b, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode/100 != 2 {
		msg := string(b) // what could be read: the status is the error the callers check
		if len(msg) > 256 {
			msg = msg[:256]
		}
		return &Error{Method: method, Path: path, Status: resp.StatusCode, Body: msg}
	}
	if readErr != nil {
		return fmt.Errorf("docker api %s %s: read body: %w", method, path, readErr)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return nil
}

type rawService struct {
	ID      string
	Version struct{ Index uint64 }
	Spec    map[string]any
}

// ListServices returns the services of the stack (label com.docker.stack.namespace).
func (c *Client) ListServices(ctx context.Context, stack string) ([]Service, error) {
	filters, _ := json.Marshal(map[string][]string{"label": {"com.docker.stack.namespace=" + stack}})
	var raw []rawService
	if err := c.do(ctx, http.MethodGet, "/services", url.Values{"filters": {string(filters)}}, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Service, 0, len(raw))
	for _, r := range raw {
		out = append(out, decodeService(r))
	}
	return out, nil
}

func decodeService(r rawService) Service {
	s := Service{ID: r.ID, Version: r.Version.Index, Spec: r.Spec, Labels: map[string]string{}}
	s.Name, _ = r.Spec["Name"].(string)
	if l, ok := r.Spec["Labels"].(map[string]any); ok {
		for k, v := range l {
			s.Labels[k], _ = v.(string)
		}
	}
	if mode, ok := r.Spec["Mode"].(map[string]any); ok {
		if rep, ok := mode["Replicated"].(map[string]any); ok {
			n := uint64(0)
			if f, ok := rep["Replicas"].(float64); ok {
				n = uint64(f)
			}
			s.Replicas = &n
		}
	}
	if tt, ok := r.Spec["TaskTemplate"].(map[string]any); ok {
		if res, ok := tt["Resources"].(map[string]any); ok {
			s.Reservations = decodeResources(res["Reservations"])
		}
		if pl, ok := tt["Placement"].(map[string]any); ok {
			if cs, ok := pl["Constraints"].([]any); ok {
				for _, c := range cs {
					if str, ok := c.(string); ok {
						s.Constraints = append(s.Constraints, str)
					}
				}
			}
		}
	}
	return s
}

func decodeResources(v any) Resources {
	m, ok := v.(map[string]any)
	if !ok {
		return Resources{}
	}
	var r Resources
	if f, ok := m["NanoCPUs"].(float64); ok {
		r.NanoCPUs = int64(f)
	}
	if f, ok := m["MemoryBytes"].(float64); ok {
		r.MemoryBytes = int64(f)
	}
	return r
}

// ListNodes returns every node of the swarm.
func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	var raw []struct {
		ID   string
		Spec struct {
			Labels       map[string]string
			Role         string
			Availability string
		}
		Description struct{ Resources Resources }
		Status      struct{ State string }
	}
	if err := c.do(ctx, http.MethodGet, "/nodes", nil, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Node, 0, len(raw))
	for _, r := range raw {
		out = append(out, Node{ID: r.ID, Labels: r.Spec.Labels, Role: r.Spec.Role, Availability: r.Spec.Availability,
			State: r.Status.State, Capacity: r.Description.Resources})
	}
	return out, nil
}

// ListRunningTasks returns tasks whose desired state is running.
func (c *Client) ListRunningTasks(ctx context.Context) ([]Task, error) {
	filters, _ := json.Marshal(map[string][]string{"desired-state": {"running"}})
	var raw []struct {
		ID        string
		ServiceID string
		NodeID    string
		Spec      struct {
			Resources struct{ Reservations Resources }
		}
	}
	if err := c.do(ctx, http.MethodGet, "/tasks", url.Values{"filters": {string(filters)}}, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(raw))
	for _, r := range raw {
		out = append(out, Task{ID: r.ID, ServiceID: r.ServiceID, NodeID: r.NodeID, Reservations: r.Spec.Resources.Reservations})
	}
	return out, nil
}

// ScaleService sets the replica count of a replicated service (POST /services/{id}/update with the spec read at
// version), the only write the proxy allows.
func (c *Client) ScaleService(ctx context.Context, s Service, replicas uint64) error {
	if s.Replicas == nil {
		return fmt.Errorf("service %s is not replicated", s.Name)
	}
	spec := cloneMap(s.Spec)
	mode, _ := spec["Mode"].(map[string]any)
	mode = cloneMap(mode)
	rep, _ := mode["Replicated"].(map[string]any)
	rep = cloneMap(rep)
	rep["Replicas"] = replicas
	mode["Replicated"] = rep
	spec["Mode"] = mode
	q := url.Values{"version": {strconv.FormatUint(s.Version, 10)}}
	return c.do(ctx, http.MethodPost, "/services/"+url.PathEscape(s.ID)+"/update", q, spec, nil)
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
