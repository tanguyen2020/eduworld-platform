package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

var cmsStructureRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listCmsSubjects":              Handlers.listCmsSubjects,
	"createSubject":                Handlers.createSubject,
	"updateSubject":                Handlers.updateSubject,
	"listCmsTopics":                Handlers.listCmsTopics,
	"createTopic":                  Handlers.createTopic,
	"updateTopic":                  Handlers.updateTopic,
	"deleteTopic":                  Handlers.deleteTopic,
	"listCmsObjectives":            Handlers.listCmsObjectives,
	"createObjective":              Handlers.createObjective,
	"updateObjective":              Handlers.updateObjective,
	"deleteObjective":              Handlers.deleteObjective,
	"listCmsCompetencies":          Handlers.listCmsCompetencies,
	"createCompetency":             Handlers.createCompetency,
	"updateCompetency":             Handlers.updateCompetency,
	"listTextbookSeries":           Handlers.listTextbookSeries,
	"createTextbookSeries":         Handlers.createTextbookSeries,
	"listTextbooks":                Handlers.listTextbooks,
	"createTextbook":               Handlers.createTextbook,
	"listTextbookSections":         Handlers.listTextbookSections,
	"createTextbookSection":        Handlers.createTextbookSection,
	"updateTextbookSection":        Handlers.updateTextbookSection,
	"putTextbookSectionObjectives": Handlers.putTextbookSectionObjectives,
}

// ---------- subjects

func (h Handlers) listCmsSubjects(w http.ResponseWriter, r *http.Request) {
	p, err := h.Queries.ListSubjects(r.Context(), queryInt(r, "grade_level"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.SubjectJSON)
}

func (h Handlers) createSubject(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Code        string            `json:"code"`
		Name        map[string]string `json:"name"`
		GradeLevels []int             `json:"grade_levels"`
		IconKey     *string           `json:"icon_key"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.CreateSubject(r.Context(), command.SubjectInput{Code: b.Code, Name: b.Name, GradeLevels: b.GradeLevels, IconKey: b.IconKey})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.RowVersion)
	writeJSON(w, http.StatusCreated, dto.SubjectJSON(s))
}

func (h Handlers) updateSubject(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "subject_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.UpdateSubjectInput{ID: id, IfMatch: v}
	if _, err = p.into("name", &in.Name); err == nil {
		if _, err = p.into("grade_levels", &in.GradeLevels); err == nil {
			in.IconKey, err = nullable[string](p, "icon_key")
		}
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.UpdateSubject(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.RowVersion)
	writeJSON(w, http.StatusOK, dto.SubjectJSON(s))
}

// ---------- topics

func (h Handlers) listCmsTopics(w http.ResponseWriter, r *http.Request) {
	vid, err := queryUUID(r, "curriculum_version_id")
	if err == nil && vid == nil {
		err = problem.Invalid("curriculum_version_id", problem.SubRequired, "is required")
	}
	var subject *uuid.UUID
	if err == nil {
		subject, err = queryUUID(r, "subject_id")
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListTopics(r.Context(), *vid, subject, queryInt(r, "grade_level"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.TopicJSON)
}

func (h Handlers) createTopic(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CurriculumVersionID uuid.UUID  `json:"curriculum_version_id"`
		Code                string     `json:"code"`
		Name                string     `json:"name"`
		SubjectID           uuid.UUID  `json:"subject_id"`
		GradeLevel          int        `json:"grade_level"`
		ParentTopicID       *uuid.UUID `json:"parent_topic_id"`
		SortOrder           int        `json:"sort_order"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	t, err := h.Commands.CreateTopic(r.Context(), command.TopicInput{VersionID: b.CurriculumVersionID, Code: b.Code, Name: b.Name, SubjectID: b.SubjectID,
		GradeLevel: b.GradeLevel, ParentID: b.ParentTopicID, SortOrder: b.SortOrder})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, t.RowVersion)
	writeJSON(w, http.StatusCreated, dto.TopicJSON(t))
}

func (h Handlers) updateTopic(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "topic_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.UpdateTopicInput{ID: id, IfMatch: v}
	if in.Code, err = optionalOf[string](p, "code"); err == nil {
		if in.Name, err = optionalOf[string](p, "name"); err == nil {
			if in.SortOrder, err = optionalOf[int](p, "sort_order"); err == nil {
				in.Parent, err = nullable[uuid.UUID](p, "parent_topic_id")
			}
		}
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	t, err := h.Commands.UpdateTopic(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, t.RowVersion)
	writeJSON(w, http.StatusOK, dto.TopicJSON(t))
}

func (h Handlers) deleteChild(w http.ResponseWriter, r *http.Request, param string, del func(command.DeleteInput) error) {
	id, err := pathUUID(r, param)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	reason, err := readReason(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := del(command.DeleteInput{ID: id, IfMatch: v, Reason: reason}); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) deleteTopic(w http.ResponseWriter, r *http.Request) {
	h.deleteChild(w, r, "topic_id", func(in command.DeleteInput) error { return h.Commands.DeleteTopic(r.Context(), in) })
}

// ---------- objectives

func (h Handlers) listCmsObjectives(w http.ResponseWriter, r *http.Request) {
	in := query.ObjectiveListInput{Grade: queryInt(r, "grade_level"), Page: pageIn(r)}
	var err error
	for _, x := range []struct {
		name string
		dst  **uuid.UUID
	}{{"curriculum_version_id", &in.VersionID}, {"topic_id", &in.TopicID}, {"subject_id", &in.SubjectID}, {"textbook_section_id", &in.SectionID}} {
		if *x.dst, err = queryUUID(r, x.name); err != nil {
			fail(w, r, err)
			return
		}
	}
	if s := r.URL.Query().Get("yccd_ref"); s != "" {
		in.YccdRef = &s
	}
	p, err := h.Queries.ListObjectives(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.ObjectiveJSON)
}

func (h Handlers) createObjective(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Code            string      `json:"code"`
		Description     string      `json:"description"`
		TopicID         uuid.UUID   `json:"topic_id"`
		SortOrder       int         `json:"sort_order"`
		BloomLevel      *string     `json:"bloom_level"`
		YccdRef         *string     `json:"yccd_ref"`
		CompetencyIDs   []uuid.UUID `json:"competency_ids"`
		PrerequisiteIDs []uuid.UUID `json:"prerequisite_ids"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	o, err := h.Commands.CreateObjective(r.Context(), command.ObjectiveInput{TopicID: b.TopicID, Code: b.Code, Description: b.Description,
		SortOrder: b.SortOrder, BloomLevel: b.BloomLevel, YccdRef: b.YccdRef, CompetencyIDs: b.CompetencyIDs, PrerequisiteIDs: b.PrerequisiteIDs})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, o.RowVersion)
	writeJSON(w, http.StatusCreated, dto.ObjectiveJSON(o))
}

func (h Handlers) updateObjective(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "objective_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.UpdateObjectiveInput{ID: id, IfMatch: v}
	steps := []func() error{
		func() (e error) { in.Code, e = optionalOf[string](p, "code"); return },
		func() (e error) { in.Description, e = optionalOf[string](p, "description"); return },
		func() (e error) { in.TopicID, e = optionalOf[uuid.UUID](p, "topic_id"); return },
		func() (e error) { in.SortOrder, e = optionalOf[int](p, "sort_order"); return },
		func() (e error) { in.BloomLevel, e = optionalOf[string](p, "bloom_level"); return },
		func() (e error) { in.YccdRef, e = nullable[string](p, "yccd_ref"); return },
		func() (e error) { in.CompetencyIDs, e = optionalOf[[]uuid.UUID](p, "competency_ids"); return },
		func() (e error) { in.PrerequisiteIDs, e = optionalOf[[]uuid.UUID](p, "prerequisite_ids"); return },
	}
	for _, s := range steps {
		if err := s(); err != nil {
			fail(w, r, err)
			return
		}
	}
	o, err := h.Commands.UpdateObjective(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, o.RowVersion)
	writeJSON(w, http.StatusOK, dto.ObjectiveJSON(o))
}

func (h Handlers) deleteObjective(w http.ResponseWriter, r *http.Request) {
	h.deleteChild(w, r, "objective_id", func(in command.DeleteInput) error { return h.Commands.DeleteObjective(r.Context(), in) })
}

// ---------- competencies

func (h Handlers) listCmsCompetencies(w http.ResponseWriter, r *http.Request) {
	vid, err := queryUUID(r, "curriculum_version_id")
	if err == nil && vid == nil {
		err = problem.Invalid("curriculum_version_id", problem.SubRequired, "is required")
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListCompetencies(r.Context(), *vid, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.CompetencyJSON)
}

func (h Handlers) createCompetency(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CurriculumVersionID uuid.UUID         `json:"curriculum_version_id"`
		Code                string            `json:"code"`
		Name                map[string]string `json:"name"`
		Kind                string            `json:"kind"`
		SubjectID           *uuid.UUID        `json:"subject_id"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	c, err := h.Commands.CreateCompetency(r.Context(), command.CompetencyInput{VersionID: b.CurriculumVersionID, Code: b.Code, Name: b.Name,
		Kind: b.Kind, SubjectID: b.SubjectID})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, c.RowVersion)
	writeJSON(w, http.StatusCreated, dto.CompetencyJSON(c))
}

func (h Handlers) updateCompetency(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "competency_id")
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
		Name map[string]string `json:"name"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	c, err := h.Commands.UpdateCompetency(r.Context(), command.UpdateCompetencyInput{ID: id, IfMatch: v, Name: b.Name})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, c.RowVersion)
	writeJSON(w, http.StatusOK, dto.CompetencyJSON(c))
}

// ---------- textbooks

func (h Handlers) listTextbookSeries(w http.ResponseWriter, r *http.Request) {
	p, err := h.Queries.ListSeries(r.Context(), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.SeriesJSON)
}

func (h Handlers) createTextbookSeries(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Code      string  `json:"code"`
		Name      string  `json:"name"`
		Publisher *string `json:"publisher"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.CreateSeries(r.Context(), command.SeriesInput{Code: b.Code, Name: b.Name, Publisher: b.Publisher})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.SeriesJSON(s))
}

func (h Handlers) listTextbooks(w http.ResponseWriter, r *http.Request) {
	f := read.TextbookFilter{GradeLevel: queryInt(r, "grade_level")}
	var err error
	if f.SeriesID, err = queryUUID(r, "series_id"); err == nil {
		f.SubjectID, err = queryUUID(r, "subject_id")
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListTextbooks(r.Context(), f, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.TextbookJSON)
}

func (h Handlers) createTextbook(w http.ResponseWriter, r *http.Request) {
	var b struct {
		SeriesID   uuid.UUID `json:"series_id"`
		SubjectID  uuid.UUID `json:"subject_id"`
		GradeLevel int       `json:"grade_level"`
		Volume     int       `json:"volume"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	t, err := h.Commands.CreateTextbook(r.Context(), command.TextbookInput{SeriesID: b.SeriesID, SubjectID: b.SubjectID, GradeLevel: b.GradeLevel,
		Volume: b.Volume})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.TextbookJSON(t))
}

func (h Handlers) listTextbookSections(w http.ResponseWriter, r *http.Request) {
	tid, err := queryUUID(r, "textbook_id")
	if err == nil && tid == nil {
		err = problem.Invalid("textbook_id", problem.SubRequired, "is required")
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListSections(r.Context(), *tid, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.SectionJSON)
}

func (h Handlers) createTextbookSection(w http.ResponseWriter, r *http.Request) {
	var b struct {
		TextbookID uuid.UUID `json:"textbook_id"`
		Chapter    *string   `json:"chapter"`
		LessonNo   int       `json:"lesson_no"`
		Title      string    `json:"title"`
		PageRange  *string   `json:"page_range"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.CreateSection(r.Context(), command.SectionInput{TextbookID: b.TextbookID, Chapter: b.Chapter, LessonNo: b.LessonNo,
		Title: b.Title, PageRange: b.PageRange})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.RowVersion)
	writeJSON(w, http.StatusCreated, dto.SectionJSON(s))
}

func (h Handlers) updateTextbookSection(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "section_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.UpdateSectionInput{ID: id, IfMatch: v}
	if in.Chapter, err = nullable[string](p, "chapter"); err == nil {
		if in.PageRange, err = nullable[string](p, "page_range"); err == nil {
			if in.LessonNo, err = optionalOf[int](p, "lesson_no"); err == nil {
				in.Title, err = optionalOf[string](p, "title")
			}
		}
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.UpdateSection(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.RowVersion)
	writeJSON(w, http.StatusOK, dto.SectionJSON(s))
}

func (h Handlers) putTextbookSectionObjectives(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "section_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		ObjectiveIDs []uuid.UUID `json:"objective_ids"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.PutSectionObjectives(r.Context(), command.SectionObjectivesInput{SectionID: id, ObjectiveIDs: b.ObjectiveIDs})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.SectionJSON(s))
}
