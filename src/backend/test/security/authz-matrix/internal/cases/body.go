package cases

import (
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// MinimalBody returns a best-effort valid body of schema s: required properties only, values from const, example,
// default or enum, then from format/bounds. A cross-tenant case needs a body the validator accepts, otherwise the
// 422 of the contract check answers before the handler could leak the resource (pkg/httpx validates after the
// guards). Fixture bodies (fixtures.yaml `bodies`) take precedence; a 422 is reported as inconclusive.
func MinimalBody(s *openapi3.Schema) any { return minimal(s, 0) }

func minimal(s *openapi3.Schema, depth int) any {
	if s == nil || depth > maxDepth {
		return nil
	}
	switch {
	case s.Const != nil:
		return s.Const
	case s.Example != nil:
		return s.Example
	case len(s.Examples) > 0:
		return s.Examples[0]
	case s.Default != nil:
		return s.Default
	case len(s.Enum) > 0:
		return s.Enum[0]
	}
	if len(s.OneOf) > 0 && s.OneOf[0] != nil {
		return minimal(s.OneOf[0].Value, depth+1)
	}
	if len(s.AnyOf) > 0 && s.AnyOf[0] != nil {
		return minimal(s.AnyOf[0].Value, depth+1)
	}
	if len(s.AllOf) > 0 {
		merged := map[string]any{}
		for _, r := range s.AllOf {
			if r == nil {
				continue
			}
			if m, ok := minimal(r.Value, depth+1).(map[string]any); ok {
				for k, v := range m {
					merged[k] = v
				}
			}
		}
		if own, ok := object(s, depth).(map[string]any); ok {
			for k, v := range own {
				merged[k] = v
			}
		}
		return merged
	}
	switch {
	case is(s, "object") || len(s.Properties) > 0:
		return object(s, depth)
	case is(s, "array"):
		n := int(s.MinItems)
		out := make([]any, 0, n)
		for i := 0; i < n && s.Items != nil; i++ {
			out = append(out, minimal(s.Items.Value, depth+1))
		}
		return out
	case is(s, "integer"):
		if s.Min != nil {
			return int64(*s.Min) + 1
		}
		return 1
	case is(s, "number"):
		if s.Min != nil {
			return *s.Min + 1
		}
		return 1
	case is(s, "boolean"):
		return false
	case is(s, "string"):
		return str(s)
	case is(s, "null"):
		return nil
	}
	return nil
}

func object(s *openapi3.Schema, depth int) any {
	out := map[string]any{}
	req := append([]string{}, s.Required...)
	sort.Strings(req)
	for _, n := range req {
		if r := s.Properties[n]; r != nil && r.Value != nil {
			out[n] = minimal(r.Value, depth+1)
		}
	}
	return out
}

func str(s *openapi3.Schema) string {
	var v string
	switch s.Format {
	case "uuid":
		v = "00000000-0000-4000-8000-000000000001"
	case "date":
		v = "2026-01-01"
	case "date-time":
		v = "2026-01-01T00:00:00Z"
	case "time":
		v = "08:00:00"
	case "email":
		v = "authz-matrix@example.invalid"
	case "uri", "url":
		v = "https://example.invalid/authz-matrix"
	default:
		v = "x"
	}
	if n := int(s.MinLength); len(v) < n {
		v += strings.Repeat("x", n-len(v))
	}
	if s.MaxLength != nil && uint64(len(v)) > *s.MaxLength {
		v = v[:*s.MaxLength]
	}
	return v
}
