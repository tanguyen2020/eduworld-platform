// Package problem implements RFC 9457 Problem Details with the stable codes of API_DESIGN §4.1
// (F002 T042, F002-AC-20). Codes and their HTTP status are generated into codes_gen.go from the single
// catalog (`go run ./pkg/problem/gen`). Unknown errors become 500 INTERNAL and never leak details.
//
//go:generate go run ./gen -spec ../../../../specs/openapi/eduworld-api.v1.yaml -doc ../../../../docs/07-backend/API_DESIGN.md -out codes_gen.go
package problem

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Code is a stable error code (enum ProblemCode).
type Code string

// Sub-codes for errors[].code (not part of the ProblemCode catalog).
const (
	SubMaxLength = "MAX_LENGTH"
	SubRequired  = "REQUIRED"
	SubInvalid   = "INVALID"
)

// FieldError is one entry of errors[] (ValidationProblem).
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// Problem is the JSON body (schema Problem / ValidationProblem).
type Problem struct {
	Type       string         `json:"type"`
	Title      string         `json:"title"`
	Status     int            `json:"status"`
	Code       Code           `json:"code"`
	Detail     string         `json:"detail,omitempty"`
	Instance   string         `json:"instance,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
	Retryable  *bool          `json:"retryable,omitempty"`
	Errors     []FieldError   `json:"errors,omitempty"`
	Extensions map[string]any `json:"-"`
}

// MarshalJSON flattens Extensions (feature_key, reopen_at, policy_reason…) into the object.
func (p Problem) MarshalJSON() ([]byte, error) {
	type plain Problem
	base, err := json.Marshal(plain(p))
	if err != nil || len(p.Extensions) == 0 {
		return base, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	for k, v := range p.Extensions {
		if _, taken := m[k]; !taken {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

// Error is an application error carrying a catalog code. Wrap it freely; From unwraps it.
type Error struct {
	Code      Code
	Detail    string
	Retryable bool
	Fields    []FieldError
	Ext       map[string]any
	cause     error
}

// New creates an error with a catalog code and a client-safe detail.
func New(code Code, detail string) *Error { return &Error{Code: code, Detail: detail} }

// Invalid creates 422 VALIDATION_FAILED for one field.
func Invalid(field, sub, message string) *Error {
	return &Error{Code: ValidationFailed, Detail: "request validation failed", Fields: []FieldError{{Field: field, Code: sub, Message: message}}}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return string(e.Code) + ": " + e.Detail + ": " + e.cause.Error()
	}
	return string(e.Code) + ": " + e.Detail
}

// Unwrap exposes the internal cause (logged, never sent to clients).
func (e *Error) Unwrap() error { return e.cause }

// Wrap attaches an internal cause.
func (e *Error) Wrap(cause error) *Error { e.cause = cause; return e }

// Retry marks the error as retryable (transient).
func (e *Error) Retry() *Error { e.Retryable = true; return e }

// With adds an extension member (e.g. feature_key, reopen_at).
func (e *Error) With(key string, value any) *Error {
	if e.Ext == nil {
		e.Ext = map[string]any{}
	}
	e.Ext[key] = value
	return e
}

// AddField appends a field error.
func (e *Error) AddField(field, sub, message string) *Error {
	e.Fields = append(e.Fields, FieldError{Field: field, Code: sub, Message: message})
	return e
}

// Status returns the HTTP status of a code (500 for an unknown code).
func Status(c Code) int {
	if s, ok := statusByCode[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// AllCodes lists every catalog code.
func AllCodes() []Code { return append([]Code(nil), allCodes...) }

// From converts any error into a Problem. Non-catalog errors become 500 INTERNAL with a generic detail.
func From(err error, traceID string) Problem {
	var pe *Error
	if !errors.As(err, &pe) || pe == nil {
		pe = New(Internal, "internal error")
	}
	status := Status(pe.Code)
	p := Problem{
		Type:       "https://errors.eduworld.vn/" + strings.ToLower(strings.ReplaceAll(string(pe.Code), "_", "-")),
		Title:      http.StatusText(status),
		Status:     status,
		Code:       pe.Code,
		Detail:     pe.Detail,
		TraceID:    traceID,
		Errors:     pe.Fields,
		Extensions: pe.Ext,
	}
	if pe.Retryable {
		t := true
		p.Retryable = &t
	}
	return p
}

// Write sends err as application/problem+json.
func Write(w http.ResponseWriter, r *http.Request, err error, traceID string) {
	p := From(err, traceID)
	if r != nil {
		p.Instance = r.URL.Path
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
