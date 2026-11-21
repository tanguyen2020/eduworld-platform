package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SchoolCurriculumCommands are the writes of the curriculum of a school (command.SchoolCurriculum, F021 FR-CUR-008).
type SchoolCurriculumCommands interface {
	PutCurriculumSettings(ctx context.Context, school uuid.UUID, items []command.CurriculumSettingInput, ifMatch int64) (dto.CurriculumSettingsView, error)
}

// SchoolCurriculumQueries are the reads of the curriculum of a school (query.SchoolCurriculum).
type SchoolCurriculumQueries interface {
	CurriculumSettings(ctx context.Context, school uuid.UUID, yearID *uuid.UUID) (dto.CurriculumSettingsView, error)
}

// lmsOp is a staff operation of the LMS (tenant required).
func lmsOp(id, method, pattern string, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return op(id, method, pattern, httpx.TenantRequired, idem, status)
}

func init() {
	settings := "/lms/schools/{school_id}/curriculum-settings"
	Operations["getSchoolCurriculumSettings"] = lmsOp("getSchoolCurriculumSettings", http.MethodGet, settings, httpx.IdempotencyNone, 200)
	Operations["putSchoolCurriculumSettings"] = lmsOp("putSchoolCurriculumSettings", http.MethodPut, settings, httpx.IdempotencyNone, 200)
}

var schoolCurriculumRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"getSchoolCurriculumSettings": Handlers.getSchoolCurriculumSettings, "putSchoolCurriculumSettings": Handlers.putSchoolCurriculumSettings,
}

func (h Handlers) getSchoolCurriculumSettings(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	year, err := queryUUID(r, "academic_year_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.SchoolCurriculumQueries.CurriculumSettings(r.Context(), school, year)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.Version)
	writeJSON(w, http.StatusOK, s)
}

func (h Handlers) putSchoolCurriculumSettings(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Items []command.CurriculumSettingInput `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	s, err := h.SchoolCurriculum.PutCurriculumSettings(r.Context(), school, b.Items, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.Version)
	writeJSON(w, http.StatusOK, s)
}
