package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

const miniSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /lms/classes:
    post:
      operationId: createClass
      parameters:
        - { name: q, in: query, schema: { type: string, maxLength: 5 } }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              additionalProperties: false
              properties:
                name: { type: string, maxLength: 10 }
                grade: { type: integer, minimum: 1, maximum: 9 }
                tags: { type: array, maxItems: 3, items: { type: string, maxLength: 4 } }
      responses: { "201": { description: ok } }
`

func loadMini(t *testing.T) *openapi3.T {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromData([]byte(miniSpec))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

type fieldErr struct{ field, code string }

func fieldErrors(r resp) []fieldErr {
	var out []fieldErr
	list, _ := r.raw["errors"].([]any)
	for _, e := range list {
		m, _ := e.(map[string]any)
		f, _ := m["field"].(string)
		c, _ := m["code"].(string)
		out = append(out, fieldErr{f, c})
	}
	return out
}

func (e *env) post(t *testing.T, path, token, body string, hdr map[string]string) resp {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	out := resp{status: rec.Code}
	if rec.Code >= 400 {
		_ = jsonUnmarshal(rec.Body.Bytes(), &out.raw)
		out.code, _ = out.raw["code"].(string)
	}
	return out
}

// F002-AC-25 (TC-TEN-024 API part): maxLength violations → 422 VALIDATION_FAILED, errors[].code MAX_LENGTH
// with the field name, before the handler runs.
func TestValidateMaxLength(t *testing.T) {
	e := newEnv(t)
	srv, err := httpx.NewServer(httpx.Deps{Auth: authCfg(e.idp), Keys: e.jwks, Resolver: e.res, Clock: e.clk, Spec: loadMini(t)})
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	op := httpx.Operation{ID: "createClass", Module: "learning", Method: http.MethodPost, Pattern: "/lms/classes", Tenant: httpx.TenantRequired}
	if err := srv.Handle(op, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called++; w.WriteHeader(201) })); err != nil {
		t.Fatal(err)
	}
	e.srv = srv
	ta := uuid.New()
	c := e.idp.baseClaims(portalClient)
	e.res.set(c["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: &ta})
	tk := e.idp.sign(tok{claims: c})
	h := map[string]string{"X-Tenant-Id": ta.String()}

	cases := []struct {
		name, path, body string
		status           int
		code             string
		want             fieldErr
	}{
		{"body too long", "/api/v1/lms/classes", `{"name":"` + strings.Repeat("x", 11) + `"}`, 422, "VALIDATION_FAILED", fieldErr{"name", "MAX_LENGTH"}},
		{"nested item too long", "/api/v1/lms/classes", `{"name":"a","tags":["ok","toolong"]}`, 422, "VALIDATION_FAILED", fieldErr{"tags.1", "MAX_LENGTH"}},
		{"query too long", "/api/v1/lms/classes?q=abcdef", `{"name":"a"}`, 422, "VALIDATION_FAILED", fieldErr{"q", "MAX_LENGTH"}},
		{"missing required", "/api/v1/lms/classes", `{"grade":3}`, 422, "VALIDATION_FAILED", fieldErr{"name", "REQUIRED"}},
		{"out of range", "/api/v1/lms/classes", `{"name":"a","grade":12}`, 422, "VALIDATION_FAILED", fieldErr{"grade", "INVALID"}},
		{"malformed json", "/api/v1/lms/classes", `{"name":`, 400, "BAD_REQUEST", fieldErr{}},
		{"valid", "/api/v1/lms/classes", `{"name":"Lớp 6A","grade":6}`, 201, "", fieldErr{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := called
			r := e.post(t, tc.path, tk, tc.body, h)
			if r.status != tc.status || r.code != tc.code {
				t.Fatalf("status %d code %q (%v)", r.status, r.code, r.raw)
			}
			if tc.want.code != "" {
				found := false
				for _, f := range fieldErrors(r) {
					if f == tc.want {
						found = true
					}
				}
				if !found {
					t.Fatalf("errors %v, want %v", fieldErrors(r), tc.want)
				}
				if called != before {
					t.Fatal("handler ran on invalid input")
				}
				if strings.Contains(r.rawString(), strings.Repeat("x", 11)) {
					t.Fatal("problem echoes the rejected value")
				}
			}
		})
	}
}

// Every registered operation must exist in the contract when a spec is configured.
func TestOperationMustBeInSpec(t *testing.T) {
	e := newEnv(t)
	srv, err := httpx.NewServer(httpx.Deps{Auth: authCfg(e.idp), Keys: e.jwks, Resolver: e.res, Clock: e.clk, Spec: loadMini(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Handle(httpx.Operation{ID: "notInSpec", Method: http.MethodGet, Pattern: "/lms/x"}, http.HandlerFunc(echo)); err == nil {
		t.Fatal("operation outside the spec accepted")
	}
	if err := srv.Handle(httpx.Operation{ID: "createClass", Method: http.MethodGet, Pattern: "/lms/classes"}, http.HandlerFunc(echo)); err == nil {
		t.Fatal("method mismatch with spec accepted")
	}
}

// The merged public spec (OpenAPI 3.1) loads and validates.
func TestLoadMergedSpec(t *testing.T) {
	doc, err := httpx.LoadSpec(context.Background(), "../../../../specs/openapi/eduworld-api.v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Paths.Map()) < 100 {
		t.Fatalf("paths = %d", len(doc.Paths.Map()))
	}
}

var jsonUnmarshal = json.Unmarshal

func (r resp) rawString() string {
	b, _ := json.Marshal(r.raw)
	return string(b)
}
