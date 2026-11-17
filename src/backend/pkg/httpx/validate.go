package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/go-chi/chi/v5"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LoadSpec loads and validates an OpenAPI document (the merged public spec for core-api).
func LoadSpec(ctx context.Context, path string) (*openapi3.T, error) {
	l := openapi3.NewLoader()
	l.Context = ctx
	doc, err := l.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("httpx: load spec: %w", err)
	}
	if err := doc.Validate(ctx); err != nil {
		return nil, fmt.Errorf("httpx: invalid spec: %w", err)
	}
	return doc, nil
}

// specRoute finds operationId in doc and checks method and path against the registration.
func specRoute(doc *openapi3.T, op Operation) (*routers.Route, error) {
	for path, item := range doc.Paths.Map() {
		for method, o := range item.Operations() {
			if o.OperationID != op.ID {
				continue
			}
			if !strings.EqualFold(method, op.Method) || path != op.Pattern {
				return nil, fmt.Errorf("httpx: operation %s is %s %s in the spec, registered as %s %s", op.ID, method, path, op.Method, op.Pattern)
			}
			return &routers.Route{Spec: doc, Path: path, PathItem: item, Method: method, Operation: o}, nil
		}
	}
	return nil, fmt.Errorf("httpx: operation %s is not in the spec", op.ID)
}

var validateOpts = &openapi3filter.Options{
	AuthenticationFunc: openapi3filter.NoopAuthenticationFunc, // authentication is the auth middleware's job
	MultiError:         true,
}

// validate checks parameters and body against the contract before the handler (ADR-0041, F002-AC-25):
// maxLength → MAX_LENGTH, required → REQUIRED, other schema violations → INVALID, all as 422
// VALIDATION_FAILED; undecodable bodies → 400 BAD_REQUEST. Rejected values are never echoed.
func validate(route *routers.Route, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := map[string]string{}
		if rc := chi.RouteContext(r.Context()); rc != nil {
			for i, k := range rc.URLParams.Keys {
				if k != "*" {
					params[k] = rc.URLParams.Values[i]
				}
			}
		}
		in := &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route, Options: validateOpts}
		if err := openapi3filter.ValidateRequest(r.Context(), in); err != nil {
			WriteError(w, r, toProblem(err))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func toProblem(err error) error {
	pe := problem.New(problem.ValidationFailed, "request validation failed")
	badBody := false
	collect(err, nil, pe, &badBody)
	if len(pe.Fields) == 0 {
		if badBody {
			return problem.New(problem.BadRequest, "request body cannot be decoded for this operation")
		}
		pe.AddField("request", problem.SubInvalid, "invalid request")
	}
	return pe
}

// collect walks kin-openapi errors (type switches, not errors.As: RequestError unwraps to its inner
// MultiError and would lose the parameter name) and adds one FieldError per violation.
func collect(err error, prefix []string, pe *problem.Error, badBody *bool) {
	add := func(path []string, sub, msg string) {
		pe.AddField(orDefault(strings.Join(path, "."), "body"), sub, msg)
	}
	switch e := err.(type) {
	case openapi3.MultiError:
		for _, x := range e {
			collect(x, prefix, pe, badBody)
		}
	case *openapi3filter.RequestError:
		p := prefix
		if e.Parameter != nil {
			p = []string{e.Parameter.Name}
		}
		switch e.Err.(type) {
		case openapi3.MultiError, *openapi3.SchemaError:
			collect(e.Err, p, pe, badBody)
		default:
			if e.Parameter == nil && e.RequestBody != nil {
				*badBody = true
				return
			}
			add(p, problem.SubInvalid, "invalid value")
		}
	case *openapi3.SchemaError:
		path := append(slices.Clone(prefix), e.JSONPointer()...)
		switch e.Origin.(type) {
		case openapi3.MultiError, *openapi3.SchemaError:
			collect(e.Origin, path, pe, badBody)
			return
		}
		sub, msg := problem.SubInvalid, "invalid value"
		switch e.SchemaField {
		case "maxLength", "maxItems", "maxProperties":
			sub, msg = problem.SubMaxLength, "too long"
		case "required":
			missing := missingProperties(e.Reason)
			if len(missing) == 0 || (len(path) > 0 && slices.Contains(missing, path[len(path)-1])) {
				add(path, problem.SubRequired, "required")
				return
			}
			for _, m := range missing {
				add(append(slices.Clone(path), m), problem.SubRequired, "required")
			}
			return
		}
		add(path, sub, msg)
	default:
		if inner := errors.Unwrap(err); inner != nil {
			collect(inner, prefix, pe, badBody)
			return
		}
		add(prefix, problem.SubInvalid, "invalid value")
	}
}

var (
	missingDoubleRe = regexp.MustCompile(`property "([^"]+)" is missing`)
	missingSingleRe = regexp.MustCompile(`missing propert(?:y|ies) ((?:'[^']+'(?:, )?)+)`)
	quotedRe        = regexp.MustCompile(`'([^']+)'`)
)

// missingProperties reads missing property names from kin-openapi (3.0) and jsonschema (3.1) messages.
func missingProperties(reason string) []string {
	if m := missingDoubleRe.FindStringSubmatch(reason); m != nil {
		return []string{m[1]}
	}
	var out []string
	if m := missingSingleRe.FindStringSubmatch(reason); m != nil {
		for _, q := range quotedRe.FindAllStringSubmatch(m[1], -1) {
			out = append(out, q[1])
		}
	}
	return out
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
