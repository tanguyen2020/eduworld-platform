// Package progressionschema checks the payloads of the F074 game content against the embedded copies of
// specs/schemas/progression-curve.schema.json and specs/schemas/reward-rule.schema.json (with the common enums they
// reference). The copies are kept equal to the specs by TestEmbeddedCopiesMatchSpecs.
package progressionschema

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
	//go:embed progression-curve.schema.json
	CurveJSON []byte
	//go:embed reward-rule.schema.json
	RuleJSON []byte
	//go:embed enums.schema.json
	EnumsJSON []byte
)

const (
	curveURL = "urn:eduworld:schema:progression-curve:1"
	ruleURL  = "urn:eduworld:schema:reward-rule:1"
	enumsURL = "https://schemas.eduworld.vn/common/enums.schema.json"
)

// ErrInvalid is a payload outside its JSON Schema; Error.Pointers lists the failing JSON pointers (no values).
var ErrInvalid = errors.New("outside the JSON Schema")

// Error is a schema failure with the JSON pointers of the failing values.
type Error struct {
	Schema   string
	Pointers []string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Schema, ErrInvalid, strings.Join(e.Pointers, ", "))
}
func (e *Error) Unwrap() error { return ErrInvalid }

var (
	once        sync.Once
	curve, rule *jsonschema.Schema
	errCompile  error
)

func compiled() error {
	once.Do(func() {
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		for url, raw := range map[string][]byte{curveURL: CurveJSON, ruleURL: RuleJSON, enumsURL: EnumsJSON} {
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				errCompile = fmt.Errorf("progressionschema: parse %s: %w", url, err)
				return
			}
			if err := c.AddResource(url, doc); err != nil {
				errCompile = err
				return
			}
		}
		if curve, errCompile = c.Compile(curveURL); errCompile != nil {
			return
		}
		rule, errCompile = c.Compile(ruleURL)
	})
	return errCompile
}

func validate(s func() *jsonschema.Schema, name string, raw []byte) error {
	if err := compiled(); err != nil {
		return err
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return &Error{Schema: name, Pointers: []string{"/"}}
	}
	if err := s().Validate(v); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return &Error{Schema: name, Pointers: pointers(ve)}
		}
		return &Error{Schema: name, Pointers: []string{"/"}}
	}
	return nil
}

// Curve implements port.PayloadValidator for a stored progression curve version.
type Curve struct{}

// Rule implements port.PayloadValidator for a stored reward rule version.
type Rule struct{}

var (
	_ port.PayloadValidator = Curve{}
	_ port.PayloadValidator = Rule{}
)

// Validate implements port.PayloadValidator.
func (Curve) Validate(raw []byte) error {
	return validate(func() *jsonschema.Schema { return curve }, "progression-curve.schema.json", raw)
}

// Validate implements port.PayloadValidator.
func (Rule) Validate(raw []byte) error {
	return validate(func() *jsonschema.Schema { return rule }, "reward-rule.schema.json", raw)
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
