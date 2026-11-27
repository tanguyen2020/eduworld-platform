package cases

import (
	"regexp"
	"sort"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/spec"
)

// Gateway names the entry point a probe goes through.
type Gateway string

// Entry points of SCOPE=internal (F132-AC-16).
const (
	PublicGateway   Gateway = "public"   // APISIX public (game client, Admin Portal shell)
	InternalGateway Gateway = "internal" // APISIX internal (Admin Portal workspaces, ADR-0027)
	DirectPort      Gateway = "direct"   // a host:port of a backend binary (must not be reachable from the runner)
)

// Probe is one request of SCOPE=internal. Pass when the answer is 404 (route unknown) or the connection is refused
// (Expect.Statuses is empty for direct ports: any HTTP answer on /internal fails, a refused connection passes).
type Probe struct {
	Gateway Gateway `json:"gateway"`
	Method  string  `json:"method"`
	Path    string  `json:"path"`
	Reason  string  `json:"reason"`
	// FromOrigin: Path is absolute from the gateway origin (facade paths /internal/v1/...); otherwise relative to the
	// API base URL (/api/v1).
	FromOrigin bool `json:"from_origin,omitempty"`
}

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// fill replaces every {param} by a syntactically valid id: the probe only checks routing.
func fill(p string) string {
	return pathParam.ReplaceAllString(p, "00000000-0000-4000-8000-000000000000")
}

// InternalProbes returns the probes of SCOPE=internal:
//   - every facade path (specs/openapi/modules/*.internal.v1.yaml) through both gateways: facades are in-process
//     calls (ADR-0074), never HTTP;
//   - every x-internal operation through the public gateway: routed by the internal gateway only (ADR-0027).
//
// Direct ports are probed by the runner with the facade paths (no answer expected at all).
func InternalProbes(ops []spec.Operation, internalPaths []string) []Probe {
	var out []Probe
	for _, p := range internalPaths {
		for _, g := range []Gateway{PublicGateway, InternalGateway} {
			out = append(out, Probe{Gateway: g, Method: "GET", Path: fill(p), Reason: "facade_path_not_http", FromOrigin: true},
				Probe{Gateway: g, Method: "POST", Path: fill(p), Reason: "facade_path_not_http", FromOrigin: true})
		}
	}
	for _, op := range ops {
		if op.Internal {
			out = append(out, Probe{Gateway: PublicGateway, Method: op.Method, Path: fill(op.Path), Reason: "x_internal_not_public"})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Gateway != out[j].Gateway {
			return out[i].Gateway < out[j].Gateway
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// PublicMirrorsInternal returns the facade paths that also exist in the public contract (must be none: a facade
// path published as a public route would expose the in-process API over HTTP).
func PublicMirrorsInternal(ops []spec.Operation, internalPaths []string) []string {
	pub := map[string]bool{}
	for _, op := range ops {
		pub["/api/v1"+op.Path] = true
		pub[op.Path] = true
	}
	var out []string
	for _, p := range internalPaths {
		if pub[p] {
			out = append(out, p)
		}
	}
	return out
}
