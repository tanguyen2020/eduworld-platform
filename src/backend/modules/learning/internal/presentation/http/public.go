package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
)

// Public reads of /learning/* (F021 US4): published versions only and no editorial metadata (author, reviewer,
// change note, draft status) — F021-AC-20.
var publicRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listCurricula":          Handlers.listCurricula,
	"listSubjects":           Handlers.listSubjects,
	"listTopics":             Handlers.listTopics,
	"listLearningObjectives": Handlers.listLearningObjectives,
	"getLearningObjective":   Handlers.getLearningObjective,
}

// curriculumSummary is CurriculumSummary.
type curriculumSummary struct {
	CurriculumID        uuid.UUID         `json:"curriculum_id"`
	Code                string            `json:"code"`
	Name                map[string]string `json:"name"`
	Version             int               `json:"version"`
	CurriculumVersionID uuid.UUID         `json:"curriculum_version_id"`
	VersionLabel        string            `json:"version_label"`
}

func contentLanguage(w http.ResponseWriter, lang string) {
	if lang == "" {
		lang = "vi"
	}
	w.Header().Set("Content-Language", lang)
}

func (h Handlers) listCurricula(w http.ResponseWriter, r *http.Request) {
	subject, err := queryUUID(r, "subject_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	cs, err := h.Queries.PublicCurricula(r.Context(), subject, queryInt(r, "grade_level"))
	if err != nil {
		fail(w, r, err)
		return
	}
	items := make([]curriculumSummary, len(cs))
	for i, a := range cs {
		items[i] = curriculumSummary{CurriculumID: a.Curriculum.ID, Code: a.Curriculum.Code, Name: a.Curriculum.Name, Version: a.Version.VersionNo,
			CurriculumVersionID: a.Version.ID, VersionLabel: a.Version.Label}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) listSubjects(w http.ResponseWriter, r *http.Request) {
	ss, err := h.Queries.PublicSubjects(r.Context(), queryInt(r, "grade_level"))
	if err != nil {
		fail(w, r, err)
		return
	}
	items := make([]dto.SubjectView, len(ss))
	for i, s := range ss {
		items[i] = dto.SubjectJSON(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) listTopics(w http.ResponseWriter, r *http.Request) {
	cid, err := queryUUID(r, "curriculum_id")
	var subject *uuid.UUID
	if err == nil {
		subject, err = queryUUID(r, "subject_id")
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	ts, lang, err := h.Queries.PublicTopics(r.Context(), cid, subject, queryInt(r, "grade_level"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]dto.TopicView, len(ts))
	for i, t := range ts {
		data[i] = dto.TopicJSON(t)
	}
	contentLanguage(w, lang)
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h Handlers) listLearningObjectives(w http.ResponseWriter, r *http.Request) {
	in := query.PublicObjectiveInput{Grade: queryInt(r, "grade_level"), Page: pageIn(r)}
	var err error
	if in.CurriculumID, err = queryUUID(r, "curriculum_id"); err == nil {
		if in.TopicID, err = queryUUID(r, "topic_id"); err == nil {
			in.SubjectID, err = queryUUID(r, "subject_id")
		}
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	p, lang, err := h.Queries.PublicObjectives(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang)
	list(w, p, dto.ObjectiveJSON)
}

func (h Handlers) getLearningObjective(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "objective_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	o, lang, err := h.Queries.PublicObjective(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang)
	writeJSON(w, http.StatusOK, dto.ObjectiveJSON(o))
}
