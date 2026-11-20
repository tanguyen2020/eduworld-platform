package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator checks client frames against the AsyncAPI (F090-AC-20): JSON object, known type, ClientEnvelope +
// payload schema (maxLength, enum, required — ADR-0041).
type Validator struct {
	schemas map[string]*jsonschema.Schema
}

// NewValidator compiles the generated schemas of every client command type.
func NewValidator() (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(clientSchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("protocol: schema bundle: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	const url = "mem://eduworld-realtime/client.json"
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	v := &Validator{schemas: make(map[string]*jsonschema.Schema, len(clientTypes))}
	for _, t := range clientTypes {
		s, err := c.Compile(url + "#/$defs/msg." + t)
		if err != nil {
			return nil, fmt.Errorf("protocol: compile %s: %w", t, err)
		}
		v.schemas[t] = s
	}
	return v, nil
}

// Known reports a type with a schema (and so a route).
func (v *Validator) Known(t string) bool { _, ok := v.schemas[t]; return ok }

// Rejection is a frame that is not forwarded: code VALIDATION_FAILED or UNKNOWN_TYPE, ref = the id when usable.
type Rejection struct {
	Code string
	Ref  string
}

// Parse decodes and validates one frame. The payload is never logged (R14).
func (v *Validator) Parse(frame []byte) (ClientEnvelope, *Rejection) {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(frame))
	obj, ok := inst.(map[string]any)
	if err != nil || !ok {
		return ClientEnvelope{}, &Rejection{Code: CodeValidationFailed}
	}
	ref := ""
	if id, ok := obj["id"].(string); ok && len(id) >= 10 && len(id) <= 40 {
		ref = id
	}
	typ, _ := obj["type"].(string)
	s, ok := v.schemas[typ]
	if !ok {
		return ClientEnvelope{}, &Rejection{Code: CodeUnknownType, Ref: ref}
	}
	if err := s.Validate(inst); err != nil {
		return ClientEnvelope{}, &Rejection{Code: CodeValidationFailed, Ref: ref}
	}
	var env ClientEnvelope
	if err := json.Unmarshal(frame, &env); err != nil {
		return ClientEnvelope{}, &Rejection{Code: CodeValidationFailed, Ref: ref}
	}
	return env, nil
}

// RefOf extracts a usable id of a frame without validating it (rate-limited frames).
func RefOf(frame []byte) string {
	var x struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(frame, &x) == nil && len(x.ID) >= 10 && len(x.ID) <= 40 {
		return x.ID
	}
	return ""
}
