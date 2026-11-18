package http

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ---------- catalog ----------

func (h Handlers) listCountries(w http.ResponseWriter, r *http.Request) {
	out, err := h.Org.ListCountries(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h Handlers) listProvinces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.Org.ListProvinces(r.Context(), q.Get("country"), q.Get("status"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h Handlers) listCatalogSchools(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.Org.ListCatalogSchools(r.Context(), query.CatalogSchoolsInput{ListInput: list(r), ProvinceCode: q.Get("province"),
		Level: q.Get("school_level"), Q: q.Get("q")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- students and classes ----------

func (h Handlers) getMyStudentProfile(w http.ResponseWriter, r *http.Request) {
	out, err := h.Org.GetMyStudentProfile(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getStudentProfile(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "student_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Org.GetStudentProfile(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func optUUID(r *http.Request, name string) (*uuid.UUID, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, problem.Invalid(name, problem.SubInvalid, "must be a UUID")
	}
	return &id, nil
}

func (h Handlers) listClasses(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	year, err := optUUID(r, "academic_year_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Org.ListClasses(r.Context(), school, year, queryInt(r, "grade_level"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h Handlers) listTenantSchoolClasses(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	year, err := optUUID(r, "academic_year")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Org.ListTenantSchoolClasses(r.Context(), query.ClassesInput{ListInput: list(r), SchoolID: school, AcademicYearID: year,
		Grade: queryInt(r, "grade")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listUnassignedStudents(w http.ResponseWriter, r *http.Request) {
	school, err := optUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Org.ListUnassignedStudents(r.Context(), school, list(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- tenants (System Admin) ----------

func (h Handlers) listTenants(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.ListTenants.Handle(r.Context(), query.ListTenantsInput{Q: q.Get("q"), TenantType: q.Get("tenant_type"),
		Cursor: q.Get("cursor"), Limit: queryInt(r, "limit")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) createTenant(w http.ResponseWriter, r *http.Request) {
	var in dto.CreateTenant
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.CreateTenant.Handle(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusCreated, out)
}

func (h Handlers) updateTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "tenant_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := dto.UpdateTenant{ID: id, IfMatch: v}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.UpdateTenant.Handle(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, out)
}

// ---------- schools ----------

func (h Handlers) listAdminTenantSchools(w http.ResponseWriter, r *http.Request) {
	tid, err := pathUUID(r, "tenant_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Org.ListAdminTenantSchools(r.Context(), tid, r.URL.Query().Get("status"), list(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) createAdminTenantSchool(w http.ResponseWriter, r *http.Request) {
	tid, err := pathUUID(r, "tenant_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	in := dto.CreateSchool{TenantID: tid}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.CreateAdminSchool.Handle(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusCreated, out)
}

func (h Handlers) updateSchool(w http.ResponseWriter, r *http.Request, handler command.UpdateSchoolHandler, tenant uuid.UUID) {
	sid, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := dto.UpdateSchool{TenantID: tenant, SchoolID: sid, IfMatch: v}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := handler.Handle(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) updateAdminTenantSchool(w http.ResponseWriter, r *http.Request) {
	tid, err := pathUUID(r, "tenant_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	h.updateSchool(w, r, h.UpdateAdminSchool, tid)
}

func (h Handlers) updateTenantSchool(w http.ResponseWriter, r *http.Request) {
	h.updateSchool(w, r, h.UpdateTenantSchool, uuid.Nil)
}

func (h Handlers) approveTenantSchool(w http.ResponseWriter, r *http.Request) {
	sid, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.ApproveTenantSchool.Handle(r.Context(), sid, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getTenantSchool(w http.ResponseWriter, r *http.Request) {
	sid, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Org.GetTenantSchool(r.Context(), sid)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listTenantSchools(w http.ResponseWriter, r *http.Request) {
	out, err := h.Org.ListTenantSchools(r.Context(), r.URL.Query().Get("status"), list(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- common tenant ----------

func (h Handlers) createCommonTenantStudents(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.HeaderTenant(r.Context())
	if !ok {
		fail(w, r, problem.New(problem.TenantHeaderRequired, ""))
		return
	}
	in := command.CreateCommonStudentsInput{TenantID: tenant}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.CreateCommonStudents.Handle(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	for _, row := range out.Results {
		commonTenant.WithLabelValues(row.Status).Inc()
	}
	// The initial passwords are shown once: never cached anywhere (F010-AC-29).
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// commonTenant counts common-tenant accounts created by System Admin (ACCOUNT_PROVISIONING §3.4 step 4).
var commonTenant = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_auth_common_tenant_total",
	Help: "Rows of createCommonTenantStudents by result (created, failed)."}, []string{"result"})

// CommonTenantCounter exposes eduworld_auth_common_tenant_total for registration by the composition root.
func CommonTenantCounter() *prometheus.CounterVec { return commonTenant }
