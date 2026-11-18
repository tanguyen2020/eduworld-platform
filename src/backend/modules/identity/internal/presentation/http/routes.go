// Package http is the public HTTP presentation of module identity: thin handlers that decode the request,
// call one use case and encode the response; authentication, X-Tenant-Id, principal, guards, OpenAPI
// validation and idempotency are the pkg/httpx chain declared per operation (Operations).
package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Handlers are the use cases served over HTTP (F010).
type Handlers struct {
	CreateTenant         command.CreateTenantHandler
	UpdateTenant         command.UpdateTenantHandler
	ListTenants          query.ListTenantsHandler
	CreateAdminSchool    command.CreateAdminTenantSchoolHandler
	UpdateAdminSchool    command.UpdateSchoolHandler
	UpdateTenantSchool   command.UpdateSchoolHandler
	ApproveTenantSchool  command.ApproveTenantSchoolHandler
	CreateCommonStudents command.CreateCommonStudentsHandler
	Catalog              command.Catalog
	Org                  query.Org
	Sis                  query.Sis                      // F011
	ResolveSisIssue      command.ResolveSisIssueHandler // F011
}

func op(id, method, pattern string, tenant httpx.TenantMode, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return httpx.Operation{ID: id, Module: "identity", Method: method, Pattern: pattern, Tenant: tenant, Idempotency: idem, SuccessStatus: status}
}

// Operations of F010 (method, path, X-Tenant-Id mode, Idempotency-Key and success status of the spec;
// the route test checks them against specs/openapi/eduworld-api.v1.yaml).
var Operations = map[string]httpx.Operation{
	"listCountries":              op("listCountries", http.MethodGet, "/catalog/countries", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"listProvinces":              op("listProvinces", http.MethodGet, "/catalog/provinces", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"listCatalogSchools":         op("listCatalogSchools", http.MethodGet, "/catalog/schools", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"getMyStudentProfile":        withGuard(op("getMyStudentProfile", http.MethodGet, "/students/me", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardGamePlayer),
	"getStudentProfile":          withGuard(op("getStudentProfile", http.MethodGet, "/students/{student_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"listClasses":                op("listClasses", http.MethodGet, "/schools/{school_id}/classes", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listTenants":                op("listTenants", http.MethodGet, "/admin/tenants", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"createTenant":               op("createTenant", http.MethodPost, "/admin/tenants", httpx.TenantNone, httpx.IdempotencyRequired, 201),
	"updateTenant":               op("updateTenant", http.MethodPatch, "/admin/tenants/{tenant_id}", httpx.TenantNone, httpx.IdempotencyRequired, 200),
	"listAdminTenantSchools":     op("listAdminTenantSchools", http.MethodGet, "/admin/tenants/{tenant_id}/schools", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"createAdminTenantSchool":    op("createAdminTenantSchool", http.MethodPost, "/admin/tenants/{tenant_id}/schools", httpx.TenantRequired, httpx.IdempotencyRequired, 201),
	"updateAdminTenantSchool":    op("updateAdminTenantSchool", http.MethodPatch, "/admin/tenants/{tenant_id}/schools/{school_id}", httpx.TenantRequired, httpx.IdempotencyRequired, 200),
	"createCommonTenantStudents": op("createCommonTenantStudents", http.MethodPost, "/admin/common-tenant/students", httpx.TenantRequired, httpx.IdempotencyRequired, 200),
	"listTenantSchools":          op("listTenantSchools", http.MethodGet, "/tenant/schools", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"getTenantSchool":            op("getTenantSchool", http.MethodGet, "/tenant/schools/{school_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"updateTenantSchool":         op("updateTenantSchool", http.MethodPatch, "/tenant/schools/{school_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"approveTenantSchool":        op("approveTenantSchool", http.MethodPost, "/tenant/schools/{school_id}:approve", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listTenantSchoolClasses":    op("listTenantSchoolClasses", http.MethodGet, "/tenant/schools/{school_id}/classes", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listUnassignedStudents":     op("listUnassignedStudents", http.MethodGet, "/tenant/students/unassigned", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"createCountry":              op("createCountry", http.MethodPost, "/admin/catalog/countries", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"updateCountry":              op("updateCountry", http.MethodPatch, "/admin/catalog/countries/{code}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createProvince":             op("createProvince", http.MethodPost, "/admin/catalog/provinces", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"updateProvince":             op("updateProvince", http.MethodPatch, "/admin/catalog/provinces/{province_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createProvinceSuccession":   op("createProvinceSuccession", http.MethodPost, "/admin/catalog/province-successions", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	// F011
	"listSisReconciliationIssues":   op("listSisReconciliationIssues", http.MethodGet, "/sis/reconciliation-issues", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"resolveSisReconciliationIssue": op("resolveSisReconciliationIssue", http.MethodPost, "/sis/reconciliation-issues/{issue_id}:resolve", httpx.TenantRequired, httpx.IdempotencyRequired, 200),
}

func withGuard(o httpx.Operation, g httpx.Guard) httpx.Operation {
	o.Guard |= g
	return o
}

// Register mounts every F010 and F011 operation on the server.
func Register(s *httpx.Server, h Handlers) error {
	routes := map[string]http.HandlerFunc{
		"listCountries":              h.listCountries,
		"listProvinces":              h.listProvinces,
		"listCatalogSchools":         h.listCatalogSchools,
		"getMyStudentProfile":        h.getMyStudentProfile,
		"getStudentProfile":          h.getStudentProfile,
		"listClasses":                h.listClasses,
		"listTenants":                h.listTenants,
		"createTenant":               h.createTenant,
		"updateTenant":               h.updateTenant,
		"listAdminTenantSchools":     h.listAdminTenantSchools,
		"createAdminTenantSchool":    h.createAdminTenantSchool,
		"updateAdminTenantSchool":    h.updateAdminTenantSchool,
		"createCommonTenantStudents": h.createCommonTenantStudents,
		"listTenantSchools":          h.listTenantSchools,
		"getTenantSchool":            h.getTenantSchool,
		"updateTenantSchool":         h.updateTenantSchool,
		"approveTenantSchool":        h.approveTenantSchool,
		"listTenantSchoolClasses":    h.listTenantSchoolClasses,
		"listUnassignedStudents":     h.listUnassignedStudents,
		"createCountry":              h.createCountry,
		"updateCountry":              h.updateCountry,
		"createProvince":             h.createProvince,
		"updateProvince":             h.updateProvince,
		"createProvinceSuccession":   h.createProvinceSuccession,
	}
	for id, fn := range sisOperations {
		routes[id] = fn(h)
	}
	for id, fn := range routes {
		o, ok := Operations[id]
		if !ok {
			return problem.New(problem.Internal, "no operation "+id)
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

// ---------- encoding helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func etag(w http.ResponseWriter, version int64) {
	w.Header().Set("ETag", `W/"`+strconv.FormatInt(version, 10)+`"`)
}

// ifMatch parses If-Match (W/"n", "n" or n): missing → 428 PRECONDITION_REQUIRED, malformed → 412.
func ifMatch(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return 0, problem.New(problem.PreconditionRequired, "")
	}
	v := strings.Trim(strings.TrimPrefix(raw, "W/"), `"`)
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 1 {
		return 0, problem.New(problem.VersionMismatch, "")
	}
	return n, nil
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return problem.New(problem.BadRequest, "invalid JSON body")
	}
	return nil
}

func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, problem.Invalid(name, problem.SubInvalid, "must be a UUID")
	}
	return id, nil
}

func queryInt(r *http.Request, name string) int {
	n, _ := strconv.Atoi(r.URL.Query().Get(name))
	return n
}

func list(r *http.Request) query.ListInput {
	return query.ListInput{Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")}
}

func fail(w http.ResponseWriter, r *http.Request, err error) { httpx.WriteError(w, r, err) }
