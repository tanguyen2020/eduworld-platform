// Package contract validates SIS records against the EduWorld SIS Contract (ADR-0021): the JSON Schema of
// the DTOs is generated from specs/sis/eduworld-sis-contract.v1.yaml by tools/sisgen and embedded here, so
// csv_import (F011) and contract_v1 (F017) share one validator.
package contract

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed contract.schema.json
var schemaJSON []byte

// Kinds are the DTOs a SIS import carries.
const (
	KindSchool     = "School"
	KindClass      = "Class"
	KindStudent    = "Student"
	KindEnrollment = "Enrollment"
	// Kinds of contract_v1 responses (F017).
	KindAcademicYear = "AcademicYear"
	KindLogin        = "Login"
)

// Violation is one contract violation of a record (SIS_CONTRACT_VIOLATION).
type Violation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validator validates records of each DTO kind.
type Validator struct {
	schemas map[string]*jsonschema.Schema
	props   map[string]map[string]bool
	types   map[string]map[string]string
	version string
}

// New compiles the embedded contract.
func New() (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("contract: parse: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	const url = "https://schemas.eduworld.vn/sis/contract.v1.json"
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	var raw struct {
		Version string `json:"x-contract-version"`
		Defs    map[string]struct {
			Properties map[string]struct {
				Type any `json:"type"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(schemaJSON, &raw); err != nil {
		return nil, err
	}
	v := &Validator{schemas: map[string]*jsonschema.Schema{}, props: map[string]map[string]bool{}, types: map[string]map[string]string{}, version: raw.Version}
	for _, k := range []string{KindSchool, KindClass, KindStudent, KindEnrollment, KindAcademicYear, KindLogin} {
		s, err := c.Compile(url + "#/$defs/" + k)
		if err != nil {
			return nil, fmt.Errorf("contract: compile %s: %w", k, err)
		}
		v.schemas[k] = s
		v.props[k] = map[string]bool{}
		v.types[k] = map[string]string{}
		for p, d := range raw.Defs[k].Properties {
			v.props[k][p] = true
			switch t := d.Type.(type) {
			case string:
				v.types[k][p] = t
			case []any: // ["string", "null"]
				if len(t) > 0 {
					v.types[k][p], _ = t[0].(string)
				}
			}
		}
	}
	return v, nil
}

// Version is the contract version of the embedded schema.
func (v *Validator) Version() string { return v.version }

// Known reports whether field is a property of the DTO (columns outside the contract are ignored, F011-AC-03).
func (v *Validator) Known(kind, field string) bool { return v.props[kind][field] }

// Type is the JSON type of a DTO property ("string", "integer", "object"…; "" for $ref properties).
func (v *Validator) Type(kind, field string) string { return v.types[kind][field] }

// Validate checks one record (already typed: integers as numbers, login as an object).
func (v *Validator) Validate(kind string, record map[string]any) []Violation {
	s, ok := v.schemas[kind]
	if !ok {
		return []Violation{{Field: "", Message: "unknown record kind " + kind}}
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return []Violation{{Message: "record is not JSON"}}
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return []Violation{{Message: "record is not JSON"}}
	}
	err = s.Validate(doc)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []Violation{{Message: err.Error()}}
	}
	var out []Violation
	collect(ve, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

func collect(ve *jsonschema.ValidationError, out *[]Violation) {
	if len(ve.Causes) == 0 {
		field := strings.Join(ve.InstanceLocation, ".")
		msg := ve.ErrorKind.LocalizedString(printer)
		if missing := missingFields(msg); len(missing) > 0 {
			for _, m := range missing { // one violation per missing column (F011-AC-02: row and column)
				*out = append(*out, Violation{Field: strings.Trim(field+"."+m, "."), Message: "required"})
			}
			return
		}
		*out = append(*out, Violation{Field: field, Message: msg})
		return
	}
	for _, c := range ve.Causes {
		collect(c, out)
	}
}

// missingFields extracts the properties of "missing property 'x'" / "missing properties 'x', 'y'".
func missingFields(msg string) []string {
	i := strings.Index(msg, "missing propert")
	if i < 0 {
		return nil
	}
	var out []string
	parts := strings.Split(msg[i:], "'")
	for j := 1; j < len(parts); j += 2 {
		out = append(out, parts[j])
	}
	return out
}

// printer renders violation messages (English, stable for logs and sync_record_errors).
var printer = message.NewPrinter(language.English)

// Strip removes the fields outside the contract from a response record (the top level and the nested
// login object of a Student) and returns the clean copy with the number of fields dropped: unknown fields
// are never stored (SIS_INTEGRATION §2.5, F017-AC-06).
func (v *Validator) Strip(kind string, record map[string]any) (map[string]any, int) {
	out := make(map[string]any, len(record))
	dropped := 0
	for k, x := range record {
		if !v.Known(kind, k) {
			dropped++
			continue
		}
		if kind == KindStudent && k == "login" {
			if m, ok := x.(map[string]any); ok {
				clean, n := v.Strip(KindLogin, m)
				dropped += n
				x = clean
			}
		}
		out[k] = x
	}
	return out, dropped
}
