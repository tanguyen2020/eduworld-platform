package httpx

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
)

func TestNeedsIfMatch(t *testing.T) {
	header := func(name string, required bool) *openapi3.ParameterRef {
		return &openapi3.ParameterRef{Value: &openapi3.Parameter{Name: name, In: openapi3.ParameterInHeader, Required: required}}
	}
	cases := []struct {
		name string
		op   openapi3.Parameters
		item openapi3.Parameters
		want bool
	}{
		{"required on operation", openapi3.Parameters{header("If-Match", true)}, nil, true},
		{"required on path item", nil, openapi3.Parameters{header("if-match", true)}, true},
		{"optional", openapi3.Parameters{header("If-Match", false)}, nil, false},
		{"absent", openapi3.Parameters{header("X-Tenant-Id", true)}, nil, false},
	}
	for _, c := range cases {
		r := &routers.Route{Operation: &openapi3.Operation{Parameters: c.op}, PathItem: &openapi3.PathItem{Parameters: c.item}}
		if got := needsIfMatch(r); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	if needsIfMatch(nil) {
		t.Fatal("nil route")
	}
}
