package httpx_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

const preSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /auth/x:
    post:
      operationId: refreshX
      security: []
      parameters:
        - { name: X-EW-CSRF, in: header, required: true, schema: { type: string, minLength: 43, maxLength: 64 } }
      responses: { "204": { description: ok } }
`

// noKeys is a key source for servers with public operations only.
type noKeys struct{}

func (noKeys) Key(context.Context, string, string) (any, error) { return nil, errors.New("no keys") }

// PreValidate answers before contract validation (ADR-0091 §4: a missing CSRF header is 403, not 422) and
// is refused on authenticated operations.
func TestPreValidateRunsBeforeValidation(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(preSpec))
	if err != nil {
		t.Fatal(err)
	}
	p := newIDP(t)
	s, err := httpx.NewServer(httpx.Deps{Auth: authCfg(p), Keys: noKeys{}, Resolver: &fakeResolver{}, Spec: doc})
	if err != nil {
		t.Fatal(err)
	}
	pre := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-EW-CSRF") == "" {
				httpx.WriteError(w, r, problem.New(problem.CsrfTokenInvalid, ""))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	op := httpx.Operation{ID: "refreshX", Method: http.MethodPost, Pattern: "/auth/x", Public: true, Tenant: httpx.TenantNone,
		SuccessStatus: 204, PreValidate: pre}
	if err := s.Handle(op, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })); err != nil {
		t.Fatal(err)
	}
	call := func(h string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/x", nil)
		if h != "" {
			req.Header.Set("X-EW-CSRF", h)
		}
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := call(""); c != http.StatusForbidden {
		t.Fatalf("missing header = %d, want 403 before validation", c)
	}
	if c := call("short"); c != http.StatusUnprocessableEntity {
		t.Fatalf("invalid header = %d, want 422 from validation", c)
	}
	op.Public = false
	op.ID = "refreshX"
	s2, _ := httpx.NewServer(httpx.Deps{Auth: authCfg(p), Keys: noKeys{}, Resolver: &fakeResolver{}, Spec: doc})
	if err := s2.Handle(op, http.NotFoundHandler()); err == nil {
		t.Fatal("PreValidate on an authenticated operation must be refused")
	}
}
