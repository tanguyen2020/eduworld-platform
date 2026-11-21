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
	CreateSisImport      command.CreateSisImportHandler // F011
	SisImports           query.SisImports               // F011
	SisAdmin             query.SisAdmin                 // F011
	SisConnections       SisConnectionsHandler          // F011 (command.SisConnections)
	Me                   query.Me                       // F012
	UserRoles            command.UserRoles              // F012
	UserRoleList         query.UserRoles                // F012
	Staff                command.Staff                  // F012
	UserList             query.TenantUsers              // F012
	Roles                command.Roles                  // F012
	RoleList             query.Roles                    // F012
}

func op(id, method, pattern string, tenant httpx.TenantMode, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return httpx.Operation{ID: id, Module: "identity", Method: method, Pattern: pattern, Tenant: tenant, Idempotency: idem, SuccessStatus: status}
}

// Operations of F010 (method, path, X-Tenant-Id mode, Idempotency-Key and success status of the spec;
// the route test checks them against specs/openapi/eduworld-api.v1.yaml).
var Operations = map[string]httpx.Operation{
	"listCountries":              op("listCountries", http.MethodGet, "/catalog/countries", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"listProvinces":              op("listProvinces", http.MethodGet, "/catalog/provinces", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"listCatalogSchools":         op("listCatalogSchools", http.MethodGet, "/catalog/schools", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"getMyStudentProfile":        withGuard(op("getMyStudentProfile", http.MethodGet, "/students/me", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardGamePlayer),
	"getStudentProfile":          withGuard(op("getStudentProfile", http.MethodGet, "/students/{student_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"listClasses":                op("listClasses", http.MethodGet, "/schools/{school_id}/classes", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listSchoolAcademicYears":    op("listSchoolAcademicYears", http.MethodGet, "/schools/{school_id}/academic-years", httpx.TenantRequired, httpx.IdempotencyNone, 200),
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
	"createSisImport":               op("createSisImport", http.MethodPost, "/schools/{school_id}/sis-imports", httpx.TenantRequired, httpx.IdempotencyRequired, 202),
	"getSisImport":                  op("getSisImport", http.MethodGet, "/schools/{school_id}/sis-imports/{import_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"createPlatformSisImport":       op("createPlatformSisImport", http.MethodPost, "/sis/imports", httpx.TenantRequired, httpx.IdempotencyRequired, 202),
	"listSisConnections":            op("listSisConnections", http.MethodGet, "/sis/connections", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"createSisConnection":           op("createSisConnection", http.MethodPost, "/sis/connections", httpx.TenantRequired, httpx.IdempotencyRequired, 201),
	"listSisConnectionSchools":      op("listSisConnectionSchools", http.MethodGet, "/admin/sis/connections/{connection_id}/schools", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"setSisConnectionSchools":       op("setSisConnectionSchools", http.MethodPut, "/admin/sis/connections/{connection_id}/schools", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"listSisSyncRuns":               op("listSisSyncRuns", http.MethodGet, "/sis/connections/{connection_id}/sync-runs", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"getSisSyncRun":                 op("getSisSyncRun", http.MethodGet, "/sis/runs/{run_id}", httpx.TenantNone, httpx.IdempotencyNone, 200),
	// F052-1: school-scoped public routes of Workspace C (owners F010, F011)
	"listTenantClassStudents": op("listTenantClassStudents", http.MethodGet, "/tenant/schools/{school_id}/classes/{class_id}/students",
		httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listSchoolReconciliationIssues": op("listSchoolReconciliationIssues", http.MethodGet, "/schools/{school_id}/reconciliation-issues",
		httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"resolveSchoolReconciliationIssue": op("resolveSchoolReconciliationIssue", http.MethodPost,
		"/schools/{school_id}/reconciliation-issues/{issue_id}:resolve", httpx.TenantRequired, httpx.IdempotencyRequired, 200),
	// F012
	"listMyTenants":          op("listMyTenants", http.MethodGet, "/me/tenants", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"getMyPermissions":       withGuard(op("getMyPermissions", http.MethodGet, "/me/permissions", httpx.TenantOptional, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"listTenantUserRoles":    withGuard(op("listTenantUserRoles", http.MethodGet, "/tenant/user-roles", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"grantTenantUserRole":    withGuard(op("grantTenantUserRole", http.MethodPost, "/tenant/user-roles", httpx.TenantRequired, httpx.IdempotencyRequired, 201), httpx.GuardPortalStaff),
	"revokeTenantUserRole":   withGuard(op("revokeTenantUserRole", http.MethodDelete, "/tenant/user-roles/{user_role_id}", httpx.TenantRequired, httpx.IdempotencyNone, 204), httpx.GuardPortalStaff),
	"listSchoolUserRoles":    withGuard(op("listSchoolUserRoles", http.MethodGet, "/schools/{school_id}/user-roles", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"grantSchoolUserRole":    withGuard(op("grantSchoolUserRole", http.MethodPost, "/schools/{school_id}/user-roles", httpx.TenantRequired, httpx.IdempotencyRequired, 201), httpx.GuardPortalStaff),
	"listAllUserRoles":       op("listAllUserRoles", http.MethodGet, "/admin/user-roles", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"grantPlatformUserRole":  op("grantPlatformUserRole", http.MethodPost, "/admin/user-roles", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"revokePlatformUserRole": op("revokePlatformUserRole", http.MethodDelete, "/admin/user-roles/{user_role_id}", httpx.TenantOptional, httpx.IdempotencyNone, 204),
	"listTenantPermissions":  withGuard(op("listTenantPermissions", http.MethodGet, "/tenant/permissions", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"listTenantRoles":        withGuard(op("listTenantRoles", http.MethodGet, "/tenant/roles", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"createTenantRole":       withGuard(op("createTenantRole", http.MethodPost, "/tenant/roles", httpx.TenantRequired, httpx.IdempotencyRequired, 201), httpx.GuardPortalStaff),
	"getTenantRole":          withGuard(op("getTenantRole", http.MethodGet, "/tenant/roles/{role_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"updateTenantRole":       withGuard(op("updateTenantRole", http.MethodPatch, "/tenant/roles/{role_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"deleteTenantRole":       withGuard(op("deleteTenantRole", http.MethodDelete, "/tenant/roles/{role_id}", httpx.TenantRequired, httpx.IdempotencyNone, 204), httpx.GuardPortalStaff),
	"listPlatformRoles":      op("listPlatformRoles", http.MethodGet, "/admin/roles", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"createPlatformRole":     op("createPlatformRole", http.MethodPost, "/admin/roles", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"getPlatformRole":        op("getPlatformRole", http.MethodGet, "/admin/roles/{role_id}", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"updatePlatformRole":     op("updatePlatformRole", http.MethodPatch, "/admin/roles/{role_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"deletePlatformRole":     op("deletePlatformRole", http.MethodDelete, "/admin/roles/{role_id}", httpx.TenantOptional, httpx.IdempotencyNone, 204),
	"listAllPermissions":     op("listAllPermissions", http.MethodGet, "/admin/permissions", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"listTenantUsers":        withGuard(op("listTenantUsers", http.MethodGet, "/tenant/users", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"createTenantUser":       withGuard(op("createTenantUser", http.MethodPost, "/tenant/users", httpx.TenantRequired, httpx.IdempotencyRequired, 201), httpx.GuardPortalStaff),
	"updateTenantUser":       withGuard(op("updateTenantUser", http.MethodPatch, "/tenant/users/{user_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"getMe":                  withGuard(op("getMe", http.MethodGet, "/auth/me", httpx.TenantOptional, httpx.IdempotencyNone, 200), httpx.GuardPortalStaff),
	"listSisSyncRunErrors":   op("listSisSyncRunErrors", http.MethodGet, "/sis/runs/{run_id}/errors", httpx.TenantNone, httpx.IdempotencyNone, 200),
}

// portalStaffOps are the operations of the Admin Portal (x-portal-action) declared before F012: every portal
// route requires the portal_staff guard (F012-AC-05; tests/contract checks it against the spec).
var portalStaffOps = []string{
	"approveTenantSchool",
	"createAdminTenantSchool",
	"createCommonTenantStudents",
	"createCountry",
	"createPlatformRole",
	"createPlatformSisImport",
	"createProvince",
	"createProvinceSuccession",
	"createSisConnection",
	"createSisImport",
	"createTenant",
	"deletePlatformRole",
	"getPlatformRole",
	"getSisImport",
	"getSisSyncRun",
	"getTenantSchool",
	"grantPlatformUserRole",
	"listAdminTenantSchools",
	"listAllPermissions",
	"listAllUserRoles",
	"listClasses",
	"listPlatformRoles",
	"listSisConnectionSchools",
	"listSisConnections",
	"listSchoolReconciliationIssues",
	"listSchoolAcademicYears",
	"listSisReconciliationIssues",
	"listSisSyncRunErrors",
	"listSisSyncRuns",
	"listTenantSchoolClasses",
	"listTenantSchools",
	"listTenants",
	"listTenantClassStudents",
	"listUnassignedStudents",
	"resolveSchoolReconciliationIssue",
	"resolveSisReconciliationIssue",
	"revokePlatformUserRole",
	"setSisConnectionSchools",
	"updateAdminTenantSchool",
	"updateCountry",
	"updatePlatformRole",
	"updateProvince",
	"updateTenant",
	"updateTenantSchool",
}

func init() {
	for _, id := range portalStaffOps {
		o := Operations[id]
		o.Guard |= httpx.GuardPortalStaff
		Operations[id] = o
	}
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
		"listSchoolAcademicYears":    h.listSchoolAcademicYears,
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
		"listTenantClassStudents":    h.listTenantClassStudents,
		"createCountry":              h.createCountry,
		"updateCountry":              h.updateCountry,
		"createProvince":             h.createProvince,
		"updateProvince":             h.updateProvince,
		"createProvinceSuccession":   h.createProvinceSuccession,
	}
	for id, fn := range sisOperations {
		routes[id] = fn(h)
	}
	for id, fn := range sisAdminOperations {
		routes[id] = fn(h)
	}
	for id, fn := range meOperations {
		routes[id] = fn(h)
	}
	for id, fn := range userRoleOperations {
		routes[id] = fn(h)
	}
	for id, fn := range roleOperations {
		routes[id] = fn(h)
	}
	for id, fn := range userOperations {
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
