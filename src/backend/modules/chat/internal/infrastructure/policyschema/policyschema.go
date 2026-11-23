// Package policyschema checks chat policy documents against the embedded copy of specs/schemas/chat-policy.schema.json v5
// (F120-AC-06, AC-08). The copy is kept equal to the spec by TestEmbeddedCopyMatchesSpec.
package policyschema

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// SchemaJSON is the embedded copy of specs/schemas/chat-policy.schema.json.
//
//go:embed chat-policy.schema.json
var SchemaJSON []byte

const schemaURL = "urn:eduworld:schema:chat-policy:5"

var (
	once   sync.Once
	schema *jsonschema.Schema
	errC   error
)

func compiled() (*jsonschema.Schema, error) {
	once.Do(func() {
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaJSON))
		if err != nil {
			errC = fmt.Errorf("policyschema: parse: %w", err)
			return
		}
		if err := c.AddResource(schemaURL, doc); err != nil {
			errC = err
			return
		}
		schema, errC = c.Compile(schemaURL)
	})
	return schema, errC
}

// Validator implements port.PolicySchema.
type Validator struct{}

var _ port.PolicySchema = Validator{}

// Validate checks a policy document; the error lists the JSON pointers of the failing values (never the values).
func (Validator) Validate(raw []byte) ([]string, error) {
	s, err := compiled()
	if err != nil {
		return nil, err
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil { // the use case marshals the document itself: never user JSON
		return nil, fmt.Errorf("policyschema: not JSON: %w", err)
	}
	if err := s.Validate(v); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return pointers(ve), nil
		}
		return []string{"/"}, nil
	}
	return nil, nil
}

func pointers(ve *jsonschema.ValidationError) []string {
	seen := map[string]bool{}
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			seen["/"+strings.Join(e.InstanceLocation, "/")] = true
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
