// Package contract checks implementations against the contracts in specs/ (F003 T055, NFR-MAINT-002):
// HTTP exchanges against the merged public OpenAPI spec, facade response objects against the module's
// internal spec, and event envelopes/payloads against their JSON Schemas. Caller-side facade tests use
// the generated api.Fake of each module.
package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Root returns the repository root (directory holding AGENTS.md) above the working directory.
func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("contract: repository root (AGENTS.md) not found")
		}
		dir = parent
	}
}

var (
	specMu sync.Mutex
	specs  = map[string]*openapi3.T{}
)

func load(rel string) (*openapi3.T, error) {
	specMu.Lock()
	defer specMu.Unlock()
	if d, ok := specs[rel]; ok {
		return d, nil
	}
	root, err := Root()
	if err != nil {
		return nil, err
	}
	l := openapi3.NewLoader()
	l.IsExternalRefsAllowed = true
	d, err := l.LoadFromFile(filepath.Join(root, rel))
	if err != nil {
		return nil, fmt.Errorf("contract: load %s: %w", rel, err)
	}
	specs[rel] = d
	return d, nil
}

func operation(d *openapi3.T, id string) (*routers.Route, error) {
	for path, item := range d.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == id {
				return &routers.Route{Spec: d, Path: path, PathItem: item, Method: method, Operation: op}, nil
			}
		}
	}
	return nil, fmt.Errorf("contract: operation %s not in the spec", id)
}

// Public checks HTTP responses of public operations against specs/openapi/eduworld-api.v1.yaml.
type Public struct{ doc *openapi3.T }

// LoadPublic loads the merged public spec.
func LoadPublic() (*Public, error) {
	d, err := load("specs/openapi/eduworld-api.v1.yaml")
	if err != nil {
		return nil, err
	}
	return &Public{doc: d}, nil
}

// CheckResponse validates status, headers and body of operationID's response for req.
func (p *Public) CheckResponse(operationID string, req *http.Request, status int, header http.Header, body []byte) error {
	route, err := operation(p.doc, operationID)
	if err != nil {
		return err
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}},
		Status: status, Header: header, Body: io.NopCloser(bytes.NewReader(body)),
		Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	return openapi3filter.ValidateResponse(context.Background(), in)
}

// Facade checks response objects of a module facade against specs/openapi/modules/<m>.internal.v1.yaml.
type Facade struct{ doc *openapi3.T }

// LoadFacade loads the internal spec of module.
func LoadFacade(module string) (*Facade, error) {
	d, err := load(filepath.Join("specs", "openapi", "modules", module+".internal.v1.yaml"))
	if err != nil {
		return nil, err
	}
	return &Facade{doc: d}, nil
}

// CheckResponse validates the JSON of v against the schema of operationID's response status.
func (f *Facade) CheckResponse(operationID string, status int, v any) error {
	route, err := operation(f.doc, operationID)
	if err != nil {
		return err
	}
	resp := route.Operation.Responses.Status(status)
	if resp == nil || resp.Value == nil {
		return fmt.Errorf("contract: %s has no %d response", operationID, status)
	}
	for _, mt := range resp.Value.Content {
		if mt.Schema == nil || mt.Schema.Value == nil {
			continue
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return err
		}
		return mt.Schema.Value.VisitJSON(doc, openapi3.MultiErrors())
	}
	return fmt.Errorf("contract: %s %d has no schema", operationID, status)
}

// Examples returns the example request bodies and response bodies (by status) declared for operationID.
func (f *Facade) Examples(operationID string) (requests []any, responses map[int][]any, err error) {
	route, err := operation(f.doc, operationID)
	if err != nil {
		return nil, nil, err
	}
	collect := func(c openapi3.Content) []any {
		var out []any
		for _, mt := range c {
			if mt.Example != nil {
				out = append(out, mt.Example)
			}
			for _, ex := range mt.Examples {
				if ex != nil && ex.Value != nil {
					out = append(out, ex.Value.Value)
				}
			}
		}
		return out
	}
	if rb := route.Operation.RequestBody; rb != nil && rb.Value != nil {
		requests = collect(rb.Value.Content)
	}
	responses = map[int][]any{}
	for code, r := range route.Operation.Responses.Map() {
		var status int
		if _, err := fmt.Sscanf(code, "%d", &status); err != nil || r.Value == nil {
			continue
		}
		if ex := collect(r.Value.Content); len(ex) > 0 {
			responses[status] = ex
		}
	}
	return requests, responses, nil
}

var (
	schemaMu sync.Mutex
	schemas  = map[string]*jsonschema.Schema{}
)

func schema(rel string) (*jsonschema.Schema, error) {
	schemaMu.Lock()
	defer schemaMu.Unlock()
	if s, ok := schemas[rel]; ok {
		return s, nil
	}
	root, err := Root()
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	s, err := c.Compile(filepath.Join(root, rel))
	if err != nil {
		return nil, fmt.Errorf("contract: schema %s: %w", rel, err)
	}
	schemas[rel] = s
	return s, nil
}

func validate(rel string, v any) error {
	s, err := schema(rel)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return s.Validate(doc)
}

// CheckEnvelope validates an event envelope (specs/events/envelope.schema.json).
func CheckEnvelope(v any) error {
	return validate(filepath.Join("specs", "events", "envelope.schema.json"), v)
}

// CheckPayload validates the payload of eventType v<version> owned by module (common for shared events).
func CheckPayload(module, eventType string, version int, v any) error {
	return validate(filepath.Join("specs", "events", "payloads", module, fmt.Sprintf("%s.v%d.schema.json", eventType, version)), v)
}
