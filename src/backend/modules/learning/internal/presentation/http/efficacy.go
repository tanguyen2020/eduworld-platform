package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// EfficacyCommands are the write use cases of the efficacy study (command.Efficacy, F035).
type EfficacyCommands interface {
	CreateResearchAgreement(ctx context.Context, in command.AgreementInput) (dto.ResearchAgreementView, error)
	RevokeResearchAgreement(ctx context.Context, id uuid.UUID, reason string) (dto.ResearchAgreementView, error)
	CreateEfficacyStudy(ctx context.Context, in command.StudyInput) (dto.EfficacyStudyView, error)
	UpdateEfficacyStudy(ctx context.Context, id uuid.UUID, ifMatch int64, in command.StudyPatch) (dto.EfficacyStudyView, error)
	AddEfficacyStudyCohort(ctx context.Context, studyID uuid.UUID, in command.CohortInput) (dto.EfficacyStudyCohortView, error)
	RemoveEfficacyStudyCohort(ctx context.Context, studyID, schoolID uuid.UUID) error
	ActivateEfficacyStudy(ctx context.Context, id uuid.UUID) (dto.EfficacyStudyView, error)
	ScheduleEfficacyWave(ctx context.Context, studyID uuid.UUID, in command.WaveInput) (dto.EfficacyWaveView, error)
	WithdrawEfficacyParticipants(ctx context.Context, studyID uuid.UUID, in command.WithdrawInput) (dto.EfficacyWithdrawalView, error)
	CompleteEfficacyStudy(ctx context.Context, id uuid.UUID) (dto.EfficacyStudyView, error)
	CreateEfficacyStudyReport(ctx context.Context, studyID uuid.UUID, excluded []uuid.UUID, key string) (dto.EfficacyReportView, error)
}

// EfficacyQueries are the reads of the efficacy study (query.Efficacy).
type EfficacyQueries interface {
	ListResearchAgreements(ctx context.Context, status string, in query.PageInput) (query.Page[dto.ResearchAgreementView], error)
	ListEfficacyStudies(ctx context.Context, status string, in query.PageInput) (query.Page[dto.EfficacyStudyView], error)
	GetEfficacyStudy(ctx context.Context, id uuid.UUID) (dto.EfficacyStudyView, error)
	ListEfficacyStudyCohorts(ctx context.Context, studyID uuid.UUID, in query.PageInput) (query.Page[dto.EfficacyStudyCohortView], error)
	ListEfficacyWaves(ctx context.Context, studyID uuid.UUID, in query.PageInput) (query.Page[dto.EfficacyWaveView], error)
	GetEfficacyStudyReport(ctx context.Context, studyID, reportID uuid.UUID) (dto.EfficacyReportView, error)
}

// EfficacyHandlers are the use cases of F035 served over HTTP (set on Handlers.Efficacy).
type EfficacyHandlers struct {
	Commands EfficacyCommands
	Queries  EfficacyQueries
}

const (
	agreementsPath = "/admin/learning/research-agreements"
	studiesPath    = "/admin/learning/efficacy-studies"
	studyPath      = studiesPath + "/{study_id}"
)

func init() {
	// /admin/: internal gateway, platform role (httpx GuardPlatformRole, ADR-0027; no MFA, ADR-0094); platform.efficacy_study.manage in
	// the use cases. Writes into a tenant take X-Tenant-Id (TenantRequired); platform writes and lists take none.
	for id, o := range map[string]httpx.Operation{
		"listResearchAgreements":    op("listResearchAgreements", http.MethodGet, agreementsPath, httpx.TenantNone, httpx.IdempotencyNone, 200),
		"createResearchAgreement":   op("createResearchAgreement", http.MethodPost, agreementsPath, httpx.TenantRequired, httpx.IdempotencyRequired, 201),
		"revokeResearchAgreement":   op("revokeResearchAgreement", http.MethodPost, agreementsPath+"/{agreement_id}:revoke", httpx.TenantRequired, httpx.IdempotencyRequired, 200),
		"listEfficacyStudies":       op("listEfficacyStudies", http.MethodGet, studiesPath, httpx.TenantNone, httpx.IdempotencyNone, 200),
		"createEfficacyStudy":       op("createEfficacyStudy", http.MethodPost, studiesPath, httpx.TenantNone, httpx.IdempotencyRequired, 201),
		"getEfficacyStudy":          op("getEfficacyStudy", http.MethodGet, studyPath, httpx.TenantNone, httpx.IdempotencyNone, 200),
		"updateEfficacyStudy":       op("updateEfficacyStudy", http.MethodPatch, studyPath, httpx.TenantNone, httpx.IdempotencyNone, 200),
		"listEfficacyStudyCohorts":  op("listEfficacyStudyCohorts", http.MethodGet, studyPath+"/cohorts", httpx.TenantNone, httpx.IdempotencyNone, 200),
		"addEfficacyStudyCohort":    op("addEfficacyStudyCohort", http.MethodPost, studyPath+"/cohorts", httpx.TenantRequired, httpx.IdempotencyRequired, 201),
		"removeEfficacyStudyCohort": op("removeEfficacyStudyCohort", http.MethodDelete, studyPath+"/cohorts/{school_id}", httpx.TenantRequired, httpx.IdempotencyNone, 204),
		"activateEfficacyStudy":     op("activateEfficacyStudy", http.MethodPost, studyPath+":activate", httpx.TenantNone, httpx.IdempotencyRequired, 200),
		"listEfficacyWaves":         op("listEfficacyWaves", http.MethodGet, studyPath+"/waves", httpx.TenantNone, httpx.IdempotencyNone, 200),
		"scheduleEfficacyWave":      op("scheduleEfficacyWave", http.MethodPost, studyPath+"/waves", httpx.TenantNone, httpx.IdempotencyRequired, 201),
		"withdrawEfficacyParticipants": op("withdrawEfficacyParticipants", http.MethodPost, studyPath+":withdraw", httpx.TenantRequired,
			httpx.IdempotencyRequired, 200),
		"completeEfficacyStudy":     op("completeEfficacyStudy", http.MethodPost, studyPath+":complete", httpx.TenantNone, httpx.IdempotencyRequired, 200),
		"createEfficacyStudyReport": op("createEfficacyStudyReport", http.MethodPost, studyPath+"/reports", httpx.TenantNone, httpx.IdempotencyRequired, 202),
		"getEfficacyStudyReport":    op("getEfficacyStudyReport", http.MethodGet, studyPath+"/reports/{report_id}", httpx.TenantNone, httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var efficacyRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listResearchAgreements":       Handlers.listResearchAgreements,
	"createResearchAgreement":      Handlers.createResearchAgreement,
	"revokeResearchAgreement":      Handlers.revokeResearchAgreement,
	"listEfficacyStudies":          Handlers.listEfficacyStudies,
	"createEfficacyStudy":          Handlers.createEfficacyStudy,
	"getEfficacyStudy":             Handlers.getEfficacyStudy,
	"updateEfficacyStudy":          Handlers.updateEfficacyStudy,
	"listEfficacyStudyCohorts":     Handlers.listEfficacyStudyCohorts,
	"addEfficacyStudyCohort":       Handlers.addEfficacyStudyCohort,
	"removeEfficacyStudyCohort":    Handlers.removeEfficacyStudyCohort,
	"activateEfficacyStudy":        Handlers.activateEfficacyStudy,
	"listEfficacyWaves":            Handlers.listEfficacyWaves,
	"scheduleEfficacyWave":         Handlers.scheduleEfficacyWave,
	"withdrawEfficacyParticipants": Handlers.withdrawEfficacyParticipants,
	"completeEfficacyStudy":        Handlers.completeEfficacyStudy,
	"createEfficacyStudyReport":    Handlers.createEfficacyStudyReport,
	"getEfficacyStudyReport":       Handlers.getEfficacyStudyReport,
}

// listItems writes a page of the admin lists (items + page).
func listItems[T any](w http.ResponseWriter, p query.Page[T]) {
	writeJSON(w, http.StatusOK, map[string]any{"items": p.Data, "page": pageJSON{NextCursor: p.NextCursor, HasMore: p.HasMore, Limit: p.Limit}})
}

func (h Handlers) listResearchAgreements(w http.ResponseWriter, r *http.Request) {
	p, err := h.Efficacy.Queries.ListResearchAgreements(r.Context(), r.URL.Query().Get("status"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	listItems(w, p)
}

func (h Handlers) createResearchAgreement(w http.ResponseWriter, r *http.Request) {
	var b struct {
		AgreementRef string   `json:"agreement_ref"`
		Covers       []string `json:"covers"`
		AgreedOn     string   `json:"agreed_on"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	on, err := time.Parse(time.DateOnly, b.AgreedOn)
	if err != nil {
		fail(w, r, problem.Invalid("/agreed_on", problem.SubInvalid, "must be a date"))
		return
	}
	v, err := h.Efficacy.Commands.CreateResearchAgreement(r.Context(), command.AgreementInput{Ref: b.AgreementRef, Covers: b.Covers, AgreedOn: on})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h Handlers) revokeResearchAgreement(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "agreement_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Efficacy.Commands.RevokeResearchAgreement(r.Context(), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) listEfficacyStudies(w http.ResponseWriter, r *http.Request) {
	p, err := h.Efficacy.Queries.ListEfficacyStudies(r.Context(), r.URL.Query().Get("status"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	listItems(w, p)
}

func (h Handlers) createEfficacyStudy(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Code         string      `json:"code"`
		Title        string      `json:"title"`
		SubjectID    uuid.UUID   `json:"subject_id"`
		GradeLevel   int         `json:"grade_level"`
		ObjectiveIDs []uuid.UUID `json:"objective_ids"`
		FormA        uuid.UUID   `json:"form_a_blueprint_version_id"`
		FormB        uuid.UUID   `json:"form_b_blueprint_version_id"`
		MinWeeks     int         `json:"min_intervention_weeks"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Efficacy.Commands.CreateEfficacyStudy(r.Context(), command.StudyInput{StudyInput: efficacy.StudyInput{Code: b.Code, Title: b.Title,
		SubjectID: b.SubjectID, GradeLevel: b.GradeLevel, ObjectiveIDs: b.ObjectiveIDs, FormA: b.FormA, FormB: b.FormB, MinWeeks: b.MinWeeks},
		IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.Version)
	writeJSON(w, http.StatusCreated, v)
}

func (h Handlers) getEfficacyStudy(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Efficacy.Queries.GetEfficacyStudy(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.Version)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) updateEfficacyStudy(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	pt, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in command.StudyPatch
	if in.Title, err = optionalOf[string](pt, "title"); err == nil {
		if _, err = pt.into("objective_ids", &in.ObjectiveIDs); err == nil {
			if in.FormA, err = optionalOf[uuid.UUID](pt, "form_a_blueprint_version_id"); err == nil {
				if in.FormB, err = optionalOf[uuid.UUID](pt, "form_b_blueprint_version_id"); err == nil {
					in.MinWeeks, err = optionalOf[int](pt, "min_intervention_weeks")
				}
			}
		}
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Efficacy.Commands.UpdateEfficacyStudy(r.Context(), id, v, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listEfficacyStudyCohorts(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Efficacy.Queries.ListEfficacyStudyCohorts(r.Context(), id, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	listItems(w, p)
}

func (h Handlers) addEfficacyStudyCohort(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		SchoolID  uuid.UUID `json:"school_id"`
		PairNo    *int      `json:"pair_no"`
		Arm       string    `json:"arm"`
		FormOrder string    `json:"form_order"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Efficacy.Commands.AddEfficacyStudyCohort(r.Context(), id, command.CohortInput{SchoolID: b.SchoolID, PairNo: b.PairNo, Arm: b.Arm,
		FormOrder: b.FormOrder})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h Handlers) removeEfficacyStudyCohort(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Efficacy.Commands.RemoveEfficacyStudyCohort(r.Context(), id, school); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) studyAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, uuid.UUID) (dto.EfficacyStudyView, error)) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := fn(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.Version)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) activateEfficacyStudy(w http.ResponseWriter, r *http.Request) {
	h.studyAction(w, r, h.Efficacy.Commands.ActivateEfficacyStudy)
}

func (h Handlers) completeEfficacyStudy(w http.ResponseWriter, r *http.Request) {
	h.studyAction(w, r, h.Efficacy.Commands.CompleteEfficacyStudy)
}

func (h Handlers) listEfficacyWaves(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Efficacy.Queries.ListEfficacyWaves(r.Context(), id, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	listItems(w, p)
}

func (h Handlers) scheduleEfficacyWave(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Kind            string    `json:"kind"`
		Form            string    `json:"form"`
		WindowStart     time.Time `json:"window_start"`
		WindowEnd       time.Time `json:"window_end"`
		DurationSeconds int       `json:"duration_seconds"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Efficacy.Commands.ScheduleEfficacyWave(r.Context(), id, command.WaveInput{WaveRequest: efficacy.WaveRequest{Kind: b.Kind,
		Form: b.Form, WindowStart: b.WindowStart, WindowEnd: b.WindowEnd, DurationSec: b.DurationSeconds},
		IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h Handlers) withdrawEfficacyParticipants(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Scope            string      `json:"scope"`
		SchoolID         *uuid.UUID  `json:"school_id"`
		StudentIDs       []uuid.UUID `json:"student_ids"`
		Reason           string      `json:"reason"`
		SchoolRequestRef *string     `json:"school_request_ref"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Efficacy.Commands.WithdrawEfficacyParticipants(r.Context(), id, command.WithdrawInput{Scope: b.Scope, SchoolID: b.SchoolID,
		StudentIDs: b.StudentIDs, Reason: b.Reason, SchoolRequestRef: b.SchoolRequestRef,
		IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) createEfficacyStudyReport(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Excluded []uuid.UUID `json:"excluded_question_version_ids"`
	}
	if r.ContentLength != 0 {
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
	}
	v, err := h.Efficacy.Commands.CreateEfficacyStudyReport(r.Context(), id, b.Excluded, r.Header.Get("Idempotency-Key"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, v)
}

func (h Handlers) getEfficacyStudyReport(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "study_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	report, err := pathUUID(r, "report_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Efficacy.Queries.GetEfficacyStudyReport(r.Context(), id, report)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
