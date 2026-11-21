package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/reporting"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ReportCommands are the write use cases of F044 (command.Reports, command.Quality).
type ReportCommands interface {
	CreateReport(ctx context.Context, in command.ReportInput) (dto.ReportJobView, error)
}

// CalibrationCommands decide on the calibration proposals (command.Quality, F044 US5).
type CalibrationCommands interface {
	AcceptCalibration(ctx context.Context, id uuid.UUID) (dto.CalibrationProposalView, error)
	DismissCalibration(ctx context.Context, id uuid.UUID) (dto.CalibrationProposalView, error)
}

// ReportQueries are the reads of F044 (query.Reports).
type ReportQueries interface {
	GetReport(ctx context.Context, scope reporting.Scope, id uuid.UUID) (dto.ReportJobView, error)
	GetCourseResults(ctx context.Context, in query.CourseResultsInput) (query.CourseResultsPage, error)
	GetReferenceReport(ctx context.Context, in query.ReferenceInput) (query.ReferencePage, error)
	GetQuestionStats(ctx context.Context, questionID uuid.UUID) (dto.QuestionStatsView, error)
	ListCalibrationProposals(ctx context.Context, status string, in query.PageInput) (query.Page[dto.CalibrationProposalView], error)
	GetExamSessionReport(ctx context.Context, id uuid.UUID) (dto.ExamSessionReportView, error)
}

// ReportHandlers are the use cases of F044 served over HTTP (set on Handlers.Reports).
type ReportHandlers struct {
	Commands    ReportCommands
	Calibration CalibrationCommands
	Queries     ReportQueries
}

func init() {
	Operations["createLmsReport"] = lmsOp("createLmsReport", http.MethodPost, "/lms/reports", httpx.IdempotencyRequired, 202)
	Operations["getLmsReport"] = lmsOp("getLmsReport", http.MethodGet, "/lms/reports/{report_id}", httpx.IdempotencyNone, 200)
	Operations["createTenantReport"] = lmsOp("createTenantReport", http.MethodPost, "/tenant/reports", httpx.IdempotencyRequired, 202)
	Operations["getTenantReport"] = lmsOp("getTenantReport", http.MethodGet, "/tenant/reports/{report_id}", httpx.IdempotencyNone, 200)
	// /admin/: internal gateway, System Admin (httpx GuardPlatformRole, ADR-0027; no MFA, ADR-0094).
	Operations["createPlatformReport"] = op("createPlatformReport", http.MethodPost, "/admin/reports", httpx.TenantOptional,
		httpx.IdempotencyRequired, 202)
	Operations["getPlatformReport"] = op("getPlatformReport", http.MethodGet, "/admin/reports/{report_id}", httpx.TenantNone,
		httpx.IdempotencyNone, 200)
	Operations["getCourseResults"] = lmsOp("getCourseResults", http.MethodGet, "/lms/courses/{course_id}/results", httpx.IdempotencyNone, 200)
	Operations["getRegulationReferenceReport"] = lmsOp("getRegulationReferenceReport", http.MethodGet,
		"/lms/schools/{school_id}/reference-levels", httpx.IdempotencyNone, 200)
	Operations["getQuestionStats"] = op("getQuestionStats", http.MethodGet, "/cms/learning/questions/{question_id}/stats",
		httpx.TenantOptional, httpx.IdempotencyNone, 200)
	Operations["listCalibrationProposals"] = op("listCalibrationProposals", http.MethodGet, "/cms/learning/calibration-proposals",
		httpx.TenantOptional, httpx.IdempotencyNone, 200)
	Operations["acceptCalibrationProposal"] = op("acceptCalibrationProposal", http.MethodPost,
		"/cms/learning/calibration-proposals/{proposal_id}:accept", httpx.TenantOptional, httpx.IdempotencyRequired, 200)
	Operations["dismissCalibrationProposal"] = op("dismissCalibrationProposal", http.MethodPost,
		"/cms/learning/calibration-proposals/{proposal_id}:dismiss", httpx.TenantOptional, httpx.IdempotencyRequired, 200)
	Operations["getExamSessionReport"] = lmsOp("getExamSessionReport", http.MethodGet, "/exams/sessions/{exam_session_id}/report",
		httpx.IdempotencyNone, 200)
}

var reportRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"createLmsReport":    func(h Handlers, w http.ResponseWriter, r *http.Request) { h.createReport(w, r, reporting.ScopeLMS) },
	"createTenantReport": func(h Handlers, w http.ResponseWriter, r *http.Request) { h.createReport(w, r, reporting.ScopeTenant) },
	"createPlatformReport": func(h Handlers, w http.ResponseWriter, r *http.Request) {
		h.createReport(w, r, reporting.ScopePlatform)
	},
	"getLmsReport":                 func(h Handlers, w http.ResponseWriter, r *http.Request) { h.getReport(w, r, reporting.ScopeLMS) },
	"getTenantReport":              func(h Handlers, w http.ResponseWriter, r *http.Request) { h.getReport(w, r, reporting.ScopeTenant) },
	"getPlatformReport":            func(h Handlers, w http.ResponseWriter, r *http.Request) { h.getReport(w, r, reporting.ScopePlatform) },
	"getCourseResults":             Handlers.getCourseResults,
	"getRegulationReferenceReport": Handlers.getRegulationReferenceReport,
	"getQuestionStats":             Handlers.getQuestionStats,
	"listCalibrationProposals":     Handlers.listCalibrationProposals,
	"acceptCalibrationProposal":    func(h Handlers, w http.ResponseWriter, r *http.Request) { h.decideCalibration(w, r, true) },
	"dismissCalibrationProposal":   func(h Handlers, w http.ResponseWriter, r *http.Request) { h.decideCalibration(w, r, false) },
	"getExamSessionReport":         Handlers.getExamSessionReport,
}

// reportBody is the union of LmsReportRequest and ReportRequest (the OpenAPI middleware validated the route's schema).
type reportBody struct {
	ReportType   string      `json:"report_type"`
	Format       string      `json:"format"`
	AssignmentID *uuid.UUID  `json:"assignment_id"`
	CourseID     *uuid.UUID  `json:"course_id"`
	SchoolID     *uuid.UUID  `json:"school_id"`
	ClassID      *uuid.UUID  `json:"class_id"`
	SubjectID    *uuid.UUID  `json:"subject_id"`
	Period       string      `json:"period"`
	AcademicYear int         `json:"academic_year"`
	From         string      `json:"from"`
	To           string      `json:"to"`
	SchoolIDs    []uuid.UUID `json:"school_ids"`
	TenantIDs    []uuid.UUID `json:"tenant_ids"`
	GradeLevel   *int        `json:"grade_level"`
	Params       struct {
		StudyID *uuid.UUID `json:"study_id"`
	} `json:"params"`
}

func bodyDate(field, s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, problem.Invalid("/"+field, problem.SubInvalid, "must be a date")
	}
	return &t, nil
}

func (h Handlers) createReport(w http.ResponseWriter, r *http.Request, scope reporting.Scope) {
	var b reportBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	req := reporting.Request{Scope: scope, Type: b.ReportType, Format: b.Format, SchoolID: b.SchoolID, ClassID: b.ClassID,
		CourseID: b.CourseID, AssignmentID: b.AssignmentID, SubjectID: b.SubjectID, Period: b.Period, AcademicYear: b.AcademicYear,
		SchoolIDs: b.SchoolIDs, TenantIDs: b.TenantIDs, GradeLevel: b.GradeLevel, StudyID: b.Params.StudyID}
	var err error
	if req.From, err = bodyDate("from", b.From); err == nil {
		req.To, err = bodyDate("to", b.To)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Reports.Commands.CreateReport(r.Context(), command.ReportInput{Scope: scope, Request: req,
		IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, v)
}

func (h Handlers) getReport(w http.ResponseWriter, r *http.Request, scope reporting.Scope) {
	id, err := pathUUID(r, "report_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Reports.Queries.GetReport(r.Context(), scope, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getCourseResults(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "course_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Reports.Queries.GetCourseResults(r.Context(), query.CourseResultsInput{CourseID: id, Page: pageIn(r)})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"course_id": v.CourseID, "columns": v.Columns, "rows": v.Rows,
		"page": pageJSON{NextCursor: v.NextCursor, HasMore: v.HasMore, Limit: v.Limit}})
}

func (h Handlers) getRegulationReferenceReport(w http.ResponseWriter, r *http.Request) {
	in := query.ReferenceInput{Period: r.URL.Query().Get("period"), Page: pageIn(r)}
	var err error
	in.SchoolID, err = pathUUID(r, "school_id")
	var subject *uuid.UUID
	if err == nil {
		subject, err = queryUUID(r, "subject_id")
	}
	if err == nil && subject == nil {
		err = problem.Invalid("subject_id", problem.SubRequired, "is required")
	}
	if err == nil {
		in.SubjectID = *subject
		in.ClassID, err = queryUUID(r, "class_id")
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	if y := queryInt(r, "academic_year"); y != nil {
		in.AcademicYear = *y
	}
	v, err := h.Reports.Queries.GetReferenceReport(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	items := v.Items
	if items == nil {
		items = []dto.ReferenceItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"school_id": v.SchoolID, "subject_id": v.SubjectID, "academic_year": v.AcademicYear,
		"period": v.Period, "disclaimer": v.Disclaimer, "items": items,
		"page": pageJSON{NextCursor: v.NextCursor, HasMore: v.HasMore, Limit: v.Limit}})
}

func (h Handlers) getQuestionStats(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "question_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Reports.Queries.GetQuestionStats(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) listCalibrationProposals(w http.ResponseWriter, r *http.Request) {
	p, err := h.Reports.Queries.ListCalibrationProposals(r.Context(), r.URL.Query().Get("status"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.CalibrationProposalView) dto.CalibrationProposalView { return v })
}

func (h Handlers) decideCalibration(w http.ResponseWriter, r *http.Request, accept bool) {
	id, err := pathUUID(r, "proposal_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var v dto.CalibrationProposalView
	if accept {
		v, err = h.Reports.Calibration.AcceptCalibration(r.Context(), id)
	} else {
		v, err = h.Reports.Calibration.DismissCalibration(r.Context(), id)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getExamSessionReport(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "exam_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Reports.Queries.GetExamSessionReport(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
