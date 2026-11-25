// Package bossschema checks boss payloads against the embedded copy of specs/schemas/boss.schema.json
// (with the common enums it references) — F103 T009, publish of the bosses on the release core. The copies are kept equal to the specs by
// TestEmbeddedCopiesMatchSpecs.
package bossschema

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

// Embedded copies of the specs (single source: specs/schemas, R33).
var (
	//go:embed boss.schema.json
	SchemaJSON []byte
	//go:embed enums.schema.json
	EnumsJSON []byte
)

const (
	schemaURL = "urn:eduworld:schema:boss:1"
	enumsURL  = "https://schemas.eduworld.vn/common/enums.schema.json"
)

// ErrInvalid is a payload outside boss.schema.json; the message lists the JSON pointers of the failing values.
var ErrInvalid = errors.New("outside boss.schema.json")

var (
	once   sync.Once
	schema *jsonschema.Schema
	errC   error
)

func compiled() (*jsonschema.Schema, error) {
	once.Do(func() {
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		for url, raw := range map[string][]byte{schemaURL: SchemaJSON, enumsURL: EnumsJSON} {
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				errC = fmt.Errorf("bossschema: parse %s: %w", url, err)
				return
			}
			if err := c.AddResource(url, doc); err != nil {
				errC = err
				return
			}
		}
		schema, errC = c.Compile(schemaURL)
	})
	return schema, errC
}

// Validator implements port.PayloadValidator.
type Validator struct{}

var _ port.PayloadValidator = Validator{}

// Validate checks a stored boss payload; the error lists the JSON pointers of the failing values (no values).
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
			return fmt.Errorf("%w: %s", ErrInvalid, strings.Join(Pointers(ve), ", "))
		}
		return ErrInvalid
	}
	return nil
}

// Pointers lists the JSON pointers of the leaf failures of a validation error.
func Pointers(ve *jsonschema.ValidationError) []string {
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
