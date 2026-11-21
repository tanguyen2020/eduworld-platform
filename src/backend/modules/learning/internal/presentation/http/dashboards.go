package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DashboardQueries are the dashboard reads of F043 (query.Dashboards).
type DashboardQueries interface {
	GetSchoolDashboard(ctx context.Context, in query.SchoolDashboardInput) (dto.SchoolDashboardView, error)
	GetGradeDashboard(ctx context.Context, in query.GradeDashboardInput) (dto.GradeDashboardView, error)
	GetSchoolSeries(ctx context.Context, in query.SeriesInput) (dto.AnalyticsSeriesView, error)
	GetStudentProfile(ctx context.Context, in query.ProfileInput) (dto.StudentLearningProfileView, error)
	GetTenantSummary(ctx context.Context, in query.TenantSummaryInput) (dto.LearningSummaryView, error)
	GetPlatformSummary(ctx context.Context, in query.PlatformSummaryInput) (dto.LearningSummaryView, error)
}

func init() {
	Operations["getSchoolDashboard"] = op("getSchoolDashboard", http.MethodGet, "/lms/dashboards/school", httpx.TenantRequired, httpx.IdempotencyNone, 200)
	Operations["getGradeDashboard"] = op("getGradeDashboard", http.MethodGet, "/lms/schools/{school_id}/grades/{grade_level}/dashboard",
		httpx.TenantRequired, httpx.IdempotencyNone, 200)
	Operations["getSchoolAnalyticsSeries"] = op("getSchoolAnalyticsSeries", http.MethodGet, "/lms/schools/{school_id}/analytics/series",
		httpx.TenantRequired, httpx.IdempotencyNone, 200)
	Operations["getStudentLearningProfile"] = op("getStudentLearningProfile", http.MethodGet, "/lms/students/{student_id}/analytics",
		httpx.TenantRequired, httpx.IdempotencyNone, 200)
	Operations["getTenantLearningSummary"] = op("getTenantLearningSummary", http.MethodGet, "/tenant/analytics/learning/summary",
		httpx.TenantRequired, httpx.IdempotencyNone, 200)
	// /admin/: internal gateway, platform role (httpx GuardPlatformRole, ADR-0027; no MFA, ADR-0094); no X-Tenant-Id.
	Operations["getPlatformLearningSummary"] = op("getPlatformLearningSummary", http.MethodGet, "/admin/analytics/learning/summary",
		httpx.TenantNone, httpx.IdempotencyNone, 200)
}

var dashboardRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"getSchoolDashboard":         Handlers.getSchoolDashboard,
	"getGradeDashboard":          Handlers.getGradeDashboard,
	"getSchoolAnalyticsSeries":   Handlers.getSchoolAnalyticsSeries,
	"getStudentLearningProfile":  Handlers.getStudentLearningProfile,
	"getTenantLearningSummary":   Handlers.getTenantLearningSummary,
	"getPlatformLearningSummary": Handlers.getPlatformLearningSummary,
}

// dashDates parses from/to (dates of the contract); a missing value stays zero and is refused by the use case.
func dashDates(r *http.Request) (time.Time, time.Time, error) {
	var out [2]time.Time
	for i, name := range []string{"from", "to"} {
		s := r.URL.Query().Get(name)
		if s == "" {
			continue
		}
		t, err := time.Parse(time.DateOnly, s)
		if err != nil {
			return out[0], out[1], problem.Invalid(name, problem.SubInvalid, "must be a date")
		}
		out[i] = t
	}
	return out[0], out[1], nil
}

func queryGrade(r *http.Request) *int {
	return queryInt(r, "grade_level")
}

func querySize(r *http.Request) int {
	if n := queryInt(r, "page_size"); n != nil {
		return *n
	}
	return 0
}

func queryUUIDs(r *http.Request, name string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, v := range r.URL.Query()[name] {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part == "" {
				continue
			}
			id, err := uuid.Parse(part)
			if err != nil {
				return nil, problem.Invalid(name, problem.SubInvalid, "must be UUIDs")
			}
			out = append(out, id)
		}
	}
	return out, nil
}

func (h Handlers) getSchoolDashboard(w http.ResponseWriter, r *http.Request) {
	in := query.SchoolDashboardInput{GradeLevel: queryGrade(r), PageSize: querySize(r)}
	school, err := queryUUID(r, "school_id")
	if err == nil && school == nil {
		err = problem.Invalid("school_id", problem.SubRequired, "is required")
	}
	if err == nil {
		in.SchoolID = *school
		in.SubjectID, err = queryUUID(r, "subject_id")
	}
	if err == nil {
		in.From, in.To, err = dashDates(r)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Dashboards.GetSchoolDashboard(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getGradeDashboard(w http.ResponseWriter, r *http.Request) {
	in := query.GradeDashboardInput{Grain: r.URL.Query().Get("grain"), PageSize: querySize(r), Cursor: r.URL.Query().Get("cursor")}
	var err error
	in.SchoolID, err = pathUUID(r, "school_id")
	if err == nil {
		in.GradeLevel, err = pathInt(r, "grade_level")
	}
	if err == nil {
		in.SubjectID, err = queryUUID(r, "subject_id")
	}
	if err == nil {
		in.From, in.To, err = dashDates(r)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Dashboards.GetGradeDashboard(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getSchoolAnalyticsSeries(w http.ResponseWriter, r *http.Request) {
	in := query.SeriesInput{Metric: r.URL.Query().Get("metric"), GradeLevel: queryGrade(r), Grain: r.URL.Query().Get("grain")}
	var err error
	in.SchoolID, err = pathUUID(r, "school_id")
	if err == nil {
		in.SubjectID, err = queryUUID(r, "subject_id")
	}
	if err == nil {
		in.From, in.To, err = dashDates(r)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Dashboards.GetSchoolSeries(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// getStudentLearningProfile reads the profile then records learning.profile_viewed (F043-AC-23, LEARNING_ANALYTICS §9).
func (h Handlers) getStudentLearningProfile(w http.ResponseWriter, r *http.Request) {
	var in query.ProfileInput
	var err error
	in.StudentID, err = pathUUID(r, "student_id")
	if err == nil {
		in.SubjectID, err = queryUUID(r, "subject_id")
	}
	if err == nil {
		in.From, in.To, err = dashDates(r)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Dashboards.GetStudentProfile(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.ProfileAudit.Record(r.Context(), in.StudentID, v.SchoolID, in.SubjectID); err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// getTenantLearningSummary ignores tenant_ids: a tenant user always reads the tenant of the session (F043-AC-27).
func (h Handlers) getTenantLearningSummary(w http.ResponseWriter, r *http.Request) {
	in := query.TenantSummaryInput{GradeLevel: queryGrade(r), Grain: r.URL.Query().Get("grain")}
	var err error
	in.SchoolIDs, err = queryUUIDs(r, "school_ids")
	if err == nil {
		in.From, in.To, err = dashDates(r)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Dashboards.GetTenantSummary(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// getPlatformLearningSummary: tenant_ids is read by the httpx scope (CrossTenantScope of System Admin).
func (h Handlers) getPlatformLearningSummary(w http.ResponseWriter, r *http.Request) {
	in := query.PlatformSummaryInput{GradeLevel: queryGrade(r), Grain: r.URL.Query().Get("grain")}
	var err error
	in.From, in.To, err = dashDates(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Dashboards.GetPlatformSummary(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
