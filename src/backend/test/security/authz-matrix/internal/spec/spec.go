// Package spec loads the public contract for the authz-matrix (F132 T005): every operation of the merged public
// spec specs/openapi/eduworld-api.v1.yaml with its classification from test/authz/matrix_gen.json.
//
// Why no oapi-codegen client: the matrix is generic over ~600 operations (method, path, parameters, request
// body schema); a typed client would add nothing the runner uses. The spec is loaded with httpx.LoadSpec — the
// very loader core-api validates requests with — and the classification (public, tenant-scoped, id parameter)
// is the routematrix output (`make gen`, checked by `make gen-check`/`routematrix check`), so the matrix can never
// drift from the contract or from the per-PR authorization tests (F002-AC-28).
package spec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// Operation is one public operation with what the matrix needs.
type Operation struct {
	ID             string
	Module         string
	Method         string
	Path           string // relative to /api/v1, OpenAPI syntax ({param})
	Public         bool   // no token (login, refresh, client-config)
	TenantScoped   bool   // takes X-Tenant-Id (required or optional)
	TenantRequired bool   // X-Tenant-Id is required (TenantIdHeader → 400 TENANT_HEADER_REQUIRED when missing)
	HasIDParam     bool
	Internal       bool // x-internal: routed only by the internal gateway (ADR-0027)
	AdminBearer    bool // security keycloakAdminBearer (Admin Portal token of a platform role)
	PathParams     []string
	NeedsIfMatch   bool
	Idempotent     bool // Idempotency-Key parameter
	NeedsDeviceID  bool
	Body           *openapi3.Schema // application/json request body, nil when none
	BodyRequired   bool
}

// matrixEntry mirrors routematrix Entry (test/authz/matrix_gen.json).
type matrixEntry struct {
	OperationID  string `json:"operation_id"`
	Module       string `json:"module"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Public       bool   `json:"public"`
	TenantScoped bool   `json:"tenant_scoped"`
	HasIDParam   bool   `json:"has_id_param"`
}

// Load reads the public spec and the routematrix classification and returns the operations sorted by ID. It fails
// when the two disagree (stale matrix: run `make gen`).
func Load(ctx context.Context, specPath, matrixPath string) ([]Operation, error) {
	doc, err := httpx.LoadSpec(ctx, specPath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(matrixPath)
	if err != nil {
		return nil, fmt.Errorf("spec: read matrix: %w", err)
	}
	var entries []matrixEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("spec: decode matrix: %w", err)
	}
	byID := make(map[string]matrixEntry, len(entries))
	for _, e := range entries {
		byID[e.OperationID] = e
	}
	var out []Operation
	seen := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, o := range item.Operations() {
			e, ok := byID[o.OperationID]
			if !ok || !strings.EqualFold(e.Method, method) || e.Path != path {
				return nil, fmt.Errorf("spec: operation %s (%s %s) is not in %s as such: run `make gen`", o.OperationID, method, path, filepath.Base(matrixPath))
			}
			seen[o.OperationID] = true
			out = append(out, build(e, item, o))
		}
	}
	for id := range byID {
		if !seen[id] {
			return nil, fmt.Errorf("spec: %s lists %s which the spec no longer has: run `make gen`", filepath.Base(matrixPath), id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func build(e matrixEntry, item *openapi3.PathItem, o *openapi3.Operation) Operation {
	op := Operation{ID: e.OperationID, Module: e.Module, Method: strings.ToUpper(e.Method), Path: e.Path, Public: e.Public,
		TenantScoped: e.TenantScoped, HasIDParam: e.HasIDParam}
	if v, ok := o.Extensions["x-internal"].(bool); ok && v {
		op.Internal = true
	}
	if o.Security != nil {
		for _, req := range *o.Security {
			if _, ok := req["keycloakAdminBearer"]; ok {
				op.AdminBearer = true
			}
		}
	}
	params := append(slices.Clone(item.Parameters), o.Parameters...)
	for _, pr := range params {
		if pr == nil || pr.Value == nil {
			continue
		}
		p := pr.Value
		switch {
		case p.In == "path":
			op.PathParams = append(op.PathParams, p.Name)
		case p.In == "header" && strings.EqualFold(p.Name, "X-Tenant-Id") && p.Required:
			op.TenantRequired = true
		case p.In == "header" && strings.EqualFold(p.Name, "If-Match") && p.Required:
			op.NeedsIfMatch = true
		case p.In == "header" && strings.EqualFold(p.Name, "Idempotency-Key"):
			op.Idempotent = true
		case p.In == "header" && strings.EqualFold(p.Name, "X-Device-Id") && p.Required:
			op.NeedsDeviceID = true
		}
	}
	if rb := o.RequestBody; rb != nil && rb.Value != nil {
		if mt := rb.Value.Content.Get("application/json"); mt != nil && mt.Schema != nil {
			op.Body, op.BodyRequired = mt.Schema.Value, rb.Value.Required
		}
	}
	return op
}

// InternalPaths returns the paths of every facade spec (specs/openapi/modules/*.internal.v1.yaml). Facades are
// called in process (ADR-0074): none of these paths may answer over HTTP (F132-AC-16).
func InternalPaths(modulesDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(modulesDir, "*.internal.v1.yaml"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("spec: no *.internal.v1.yaml in %s", modulesDir)
	}
	var out []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Paths map[string]any `yaml:"paths"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("spec: %s: %w", f, err)
		}
		for p := range doc.Paths {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}
