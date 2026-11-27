package cases

import (
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/spec"
)

// LengthCase sends one string field of a request body at maxLength + 1 (F132-AC-17, TC-TEN-024, ADR-0041). The
// answer must be 422 VALIDATION_FAILED with errors[] naming Field (dot path, as pkg/httpx builds it from the JSON
// pointer) with code MAX_LENGTH.
type LengthCase struct {
	Op    spec.Operation `json:"-"`
	OpID  string         `json:"operation_id"`
	Field string         `json:"field"`
	Max   uint64         `json:"max_length"`
	Body  any            `json:"-"` // the oversized value at Field; sibling required fields are left out on purpose
}

// HeaderCase sends a tenant-scoped operation without X-Tenant-Id: 400 TENANT_HEADER_REQUIRED (ADR-0065).
type HeaderCase struct {
	Op   spec.Operation `json:"-"`
	OpID string         `json:"operation_id"`
}

// maxDepth bounds the walk of recursive schemas.
const maxDepth = 6

// LengthCases returns one case per bounded string field reachable through object properties and array items of
// every request body. Strings under oneOf/anyOf/allOf/not are skipped (the validator reports the composite schema,
// not the field); their number is returned so the report shows what was not covered.
func LengthCases(ops []spec.Operation) (out []LengthCase, skippedComposite int) {
	for _, op := range ops {
		if op.Body == nil {
			continue
		}
		var walk func(s *openapi3.Schema, path []string, depth int)
		walk = func(s *openapi3.Schema, path []string, depth int) {
			if s == nil || depth > maxDepth {
				return
			}
			if len(s.OneOf)+len(s.AnyOf)+len(s.AllOf) > 0 || s.Not != nil {
				skippedComposite += countStrings(s, depth)
				return
			}
			switch {
			case is(s, "string"):
				if s.MaxLength != nil && len(path) > 0 && len(s.Enum) == 0 && s.Const == nil {
					out = append(out, LengthCase{Op: op, OpID: op.ID, Field: strings.Join(path, "."), Max: *s.MaxLength,
						Body: nest(path, strings.Repeat("a", int(*s.MaxLength)+1))})
				}
			case is(s, "object") || len(s.Properties) > 0:
				names := make([]string, 0, len(s.Properties))
				for n := range s.Properties {
					names = append(names, n)
				}
				sort.Strings(names)
				for _, n := range names {
					if ref := s.Properties[n]; ref != nil && ref.Value != nil && !ref.Value.ReadOnly {
						walk(ref.Value, append(append([]string{}, path...), n), depth+1)
					}
				}
			case is(s, "array"):
				if s.Items != nil {
					walk(s.Items.Value, append(append([]string{}, path...), "0"), depth+1)
				}
			}
		}
		walk(op.Body, nil, 0)
	}
	return out, skippedComposite
}

// HeaderCases returns one case per operation whose X-Tenant-Id is required.
func HeaderCases(ops []spec.Operation) []HeaderCase {
	var out []HeaderCase
	for _, op := range ops {
		if op.TenantRequired && !op.Public {
			out = append(out, HeaderCase{Op: op, OpID: op.ID})
		}
	}
	return out
}

func is(s *openapi3.Schema, t string) bool { return s.Type != nil && s.Type.Includes(t) }

func countStrings(s *openapi3.Schema, depth int) int {
	if s == nil || depth > maxDepth {
		return 0
	}
	n := 0
	if is(s, "string") && s.MaxLength != nil {
		n++
	}
	for _, group := range []openapi3.SchemaRefs{s.OneOf, s.AnyOf, s.AllOf} {
		for _, r := range group {
			if r != nil {
				n += countStrings(r.Value, depth+1)
			}
		}
	}
	for _, r := range s.Properties {
		if r != nil {
			n += countStrings(r.Value, depth+1)
		}
	}
	if s.Items != nil {
		n += countStrings(s.Items.Value, depth+1)
	}
	return n
}

// nest builds {"a":{"b":[{"c":v}]}} for path a.b.0.c.
func nest(path []string, v any) any {
	if len(path) == 0 {
		return v
	}
	inner := nest(path[1:], v)
	if _, err := strconv.Atoi(path[0]); err == nil {
		return []any{inner}
	}
	return map[string]any{path[0]: inner}
}
