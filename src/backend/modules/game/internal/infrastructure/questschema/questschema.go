// Package questschema checks quest payloads against the embedded copy of specs/schemas/quest.schema.json v2 (F072
// T044, F072-AC-27). The copy is kept equal to the spec by TestEmbeddedCopyMatchesSpec.
package questschema

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// SchemaJSON is the embedded copy of specs/schemas/quest.schema.json.
//
//go:embed quest.schema.json
var SchemaJSON []byte

const schemaURL = "urn:eduworld:schema:quest-version:2"

// ErrInvalid is a payload outside quest.schema.json v2.
var ErrInvalid = errors.New("outside quest.schema.json v2")

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
			errC = fmt.Errorf("questschema: parse: %w", err)
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

// Validator implements port.PayloadValidator.
type Validator struct{}

var _ port.PayloadValidator = Validator{}

// Validate checks a stored quest payload; the error lists the JSON pointers of the failing values (no values).
func (Validator) Validate(raw []byte) error {
	s, err := compiled()
	if err != nil {
		return err
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("%w: not JSON", ErrInvalid)
	}
	if err := s.Validate(v); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return fmt.Errorf("%w: %s", ErrInvalid, strings.Join(pointers(ve), ", "))
		}
		return ErrInvalid
	}
	return nil
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
