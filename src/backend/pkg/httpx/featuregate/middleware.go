package featuregate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// OpenAPI extensions of ADR-0090.
const (
	ExtAction = "x-portal-action"
	ExtShell  = "x-portal-shell"
)

// Rule is the route → action mapping of one operation, read from its x-portal-action: one action key, or
// {param, map} where the action depends on the value of a path (else query) parameter.
type Rule struct {
	action  string
	param   string
	byValue map[string]string
}

// ActionFor returns the action of the request. A {param, map} value outside the map yields false: the
// value is outside the parameter enum (check-portal-catalog keeps map = enum), so OpenAPI validation,
// which runs after the gate and before the handler, refuses the request.
func (r Rule) ActionFor(req *http.Request) (string, bool) {
	if r.param == "" {
		return r.action, true
	}
	v := chi.URLParam(req, r.param)
	if v == "" {
		v = req.URL.Query().Get(r.param)
	}
	a, ok := r.byValue[v]
	return a, ok
}

func (r Rule) actions() []string {
	if r.param == "" {
		return []string{r.action}
	}
	out := make([]string, 0, len(r.byValue))
	for _, a := range r.byValue {
		out = append(out, a)
	}
	return out
}

type paramMap struct {
	Param string            `json:"param"`
	Map   map[string]string `json:"map"`
}

// Bind reads the rule of a spec operation. gated is false for x-portal-shell operations, operations
// without x-portal-action and operations whose actions all have can_disable = false (nothing to gate).
// Malformed extensions and actions missing from the catalog are errors (the binary refuses to start).
func (g *Gate) Bind(op *openapi3.Operation) (rule Rule, gated bool, err error) {
	raw, has := op.Extensions[ExtAction]
	if shell, _ := op.Extensions[ExtShell].(bool); shell {
		if has {
			return Rule{}, false, fmt.Errorf("featuregate: operation %s has both %s and %s", op.OperationID, ExtShell, ExtAction)
		}
		return Rule{}, false, nil
	}
	if !has {
		return Rule{}, false, nil
	}
	rule, err = parseRule(raw)
	if err != nil {
		return Rule{}, false, fmt.Errorf("featuregate: operation %s: %w", op.OperationID, err)
	}
	for _, a := range rule.actions() {
		ca, ok := g.o.Catalog.Action(a)
		if !ok {
			return Rule{}, false, fmt.Errorf("featuregate: operation %s: action %s is not in the menu catalog", op.OperationID, a)
		}
		gated = gated || ca.CanDisable
	}
	return rule, gated, nil
}

func parseRule(raw any) (Rule, error) {
	if s, ok := raw.(string); ok {
		if s == "" {
			return Rule{}, errors.New("empty " + ExtAction)
		}
		return Rule{action: s}, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return Rule{}, fmt.Errorf("%s: %w", ExtAction, err)
	}
	var pm paramMap
	if err := json.Unmarshal(b, &pm); err != nil {
		return Rule{}, fmt.Errorf("%s must be an action key or {param, map}: %w", ExtAction, err)
	}
	if pm.Param == "" || len(pm.Map) == 0 {
		return Rule{}, fmt.Errorf("%s {param, map} needs a param and a non-empty map", ExtAction)
	}
	for v, a := range pm.Map {
		if a == "" {
			return Rule{}, fmt.Errorf("%s map value %q is empty", ExtAction, v)
		}
	}
	return Rule{param: pm.Param, byValue: pm.Map}, nil
}

// Middleware gates one operation. It needs the Principal in the context (pkg/httpx installs it after
// authentication, tenant and guards) and applies only to the portal channel (azp = portal client); the
// scope is the session tenant (Principal.TenantID; nil = platform default). writeErr renders Problem
// Details (httpx.WriteError).
func (g *Gate) Middleware(rule Rule, writeErr func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := authz.PrincipalFrom(r.Context())
			if !ok || p.Channel != authz.ChannelPortal {
				next.ServeHTTP(w, r)
				return
			}
			action, ok := rule.ActionFor(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			if err := g.Check(r.Context(), p.TenantID, action); err != nil {
				writeErr(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
