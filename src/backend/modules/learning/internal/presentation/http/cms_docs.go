package http

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/questionset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DocCommands are the write use cases of the composed content types (command.Docs, F024).
type DocCommands interface {
	Create(ctx context.Context, t contentmodel.ContentType, f command.DocFields) (command.DocResult, error)
	UpdateDraft(ctx context.Context, in command.UpdateDocDraftInput) (command.DocResult, error)
	NewVersion(ctx context.Context, in command.NewDocVersionInput) (command.DocResult, error)
	Delete(ctx context.Context, in command.DeleteDocInput) error
}

// DocQueries are the reads of the composed content types (query.Docs).
type DocQueries interface {
	Get(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (query.DocView, error)
	List(ctx context.Context, t contentmodel.ContentType, f read.DocFilter, in query.PageInput) (query.Page[query.DocView], error)
	PoolStats(ctx context.Context, t contentmodel.ContentType, id uuid.UUID, versionNo int) ([]questionset.PoolStat, error)
}

// docRoute describes the paths of one type.
type docRoute struct {
	t      contentmodel.ContentType
	base   string // /cms/learning/<plural>
	param  string // path parameter of the item
	suffix string // operationId suffix
}

var docRoutes = []docRoute{
	{contentmodel.TypeQuestionSet, "/cms/learning/question-sets", "question_set_id", "QuestionSet"},
	{contentmodel.TypeLesson, "/cms/learning/lessons", "lesson_id", "Lesson"},
	{contentmodel.TypeLearningMaterial, "/cms/learning/learning-materials", "learning_material_id", "LearningMaterial"},
	{contentmodel.TypeAssessmentBlueprint, "/cms/learning/assessment-blueprints", "assessment_blueprint_id", "AssessmentBlueprint"},
}

var docOps = map[string]func(Handlers, docRoute, http.ResponseWriter, *http.Request){}

func plural(s string) string {
	if s == "QuestionSet" {
		return "QuestionSets"
	}
	return s + "s"
}

func init() {
	for _, d := range docRoutes {
		item := d.base + "/{" + d.param + "}"
		ops := map[string]httpx.Operation{
			"listCms" + plural(d.suffix):       op("listCms"+plural(d.suffix), http.MethodGet, d.base, httpx.TenantOptional, httpx.IdempotencyNone, 200),
			"createCms" + d.suffix:             op("createCms"+d.suffix, http.MethodPost, d.base, httpx.TenantOptional, httpx.IdempotencyRequired, 201),
			"getCms" + d.suffix:                op("getCms"+d.suffix, http.MethodGet, item, httpx.TenantOptional, httpx.IdempotencyNone, 200),
			"deleteCms" + d.suffix:             op("deleteCms"+d.suffix, http.MethodDelete, item, httpx.TenantOptional, httpx.IdempotencyNone, 204),
			"createCms" + d.suffix + "Version": op("createCms"+d.suffix+"Version", http.MethodPost, item+"/versions", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
			"updateCms" + d.suffix + "Draft":   op("updateCms"+d.suffix+"Draft", http.MethodPatch, item+"/versions/{version}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
		}
		for id, o := range ops {
			Operations[id] = o
		}
		docOps["listCms"+plural(d.suffix)] = Handlers.listDocs
		docOps["createCms"+d.suffix] = Handlers.createDoc
		docOps["getCms"+d.suffix] = Handlers.getDoc
		docOps["deleteCms"+d.suffix] = Handlers.deleteDoc
		docOps["createCms"+d.suffix+"Version"] = Handlers.newDocVersion
		docOps["updateCms"+d.suffix+"Draft"] = Handlers.updateDocDraft
	}
	Operations["getQuestionSetPoolStats"] = op("getQuestionSetPoolStats", http.MethodGet,
		"/cms/learning/question-sets/{question_set_id}/versions/{version}:pool-stats", httpx.TenantOptional, httpx.IdempotencyNone, 200)
	Operations["getAssessmentBlueprintPoolStats"] = op("getAssessmentBlueprintPoolStats", http.MethodGet,
		"/cms/learning/assessment-blueprints/{assessment_blueprint_id}/versions/{version}:pool-stats", httpx.TenantOptional, httpx.IdempotencyNone, 200)
	docOps["getQuestionSetPoolStats"] = Handlers.poolStats
	docOps["getAssessmentBlueprintPoolStats"] = Handlers.poolStats
}

// routeOf returns the docRoute of an operation id.
func routeOf(id string) docRoute {
	for _, d := range docRoutes {
		if slices.Contains([]string{"listCms" + plural(d.suffix), "createCms" + d.suffix, "getCms" + d.suffix, "deleteCms" + d.suffix,
			"createCms" + d.suffix + "Version", "updateCms" + d.suffix + "Draft", "get" + d.suffix + "PoolStats"}, id) {
			return d
		}
	}
	return docRoute{}
}

// readDoc decodes a create or merge-patch body of type t.
func readDoc(r *http.Request, t contentmodel.ContentType) (command.DocFields, error) {
	var p patch
	if err := decode(r, &p); err != nil {
		return command.DocFields{}, err
	}
	f := command.DocFields{Present: map[string]bool{}, Body: map[string]json.RawMessage{}}
	set := func(k string) { f.Present[k] = true }
	var err error
	get := func(k string, v any) {
		if err != nil {
			return
		}
		if _, e := p.into(k, v); e != nil {
			err = e
		}
	}
	if p.has("tenant_id") {
		set("tenant_id")
		get("tenant_id", &f.TenantID)
	}
	get("owner_scope", &f.OwnerScope)
	if p.has("title") {
		set("title")
		f.Title = p["title"]
	}
	summaryKey := "summary"
	if t == contentmodel.TypeQuestionSet {
		summaryKey = "description"
	}
	if p.has(summaryKey) {
		set("summary")
		f.Summary = p[summaryKey]
	}
	for _, k := range []string{"subject_id", "curriculum_version_id"} {
		if p.has(k) {
			set(k)
		}
	}
	get("subject_id", &f.SubjectID)
	get("curriculum_version_id", &f.CurriculumVersionID)
	if p.has("grade_levels") {
		set("grade_levels")
		get("grade_levels", &f.GradeLevels)
	}
	if p.has("grade_level") {
		set("grade_levels")
		var g int
		get("grade_level", &g)
		if !p.isNull("grade_level") {
			f.GradeLevels = []int{g}
		}
	}
	if p.has("topic_ids") {
		set("topic_ids")
		get("topic_ids", &f.TopicIDs)
	}
	if p.has("topic_id") {
		set("topic_ids")
		var id uuid.UUID
		get("topic_id", &id)
		if !p.isNull("topic_id") {
			f.TopicIDs = []uuid.UUID{id}
		}
	}
	if p.has("objective_ids") {
		set("objective_ids")
		get("objective_ids", &f.ObjectiveIDs)
	}
	if p.has("content_language") {
		set("content_language")
		get("content_language", &f.ContentLanguage)
	}
	if p.has("change_note") {
		set("change_note")
		get("change_note", &f.ChangeNote)
	}
	for _, k := range command.BodyKeys[t] {
		if p.has(k) {
			f.Body[k] = p[k]
		}
	}
	return f, err
}

func docView(v query.DocView, r *http.Request) map[string]any {
	return command.DocJSON(command.DocResult{Doc: v.Doc, Version: v.Version}, sessionTenantOf(r))
}

func (h Handlers) createDoc(d docRoute, w http.ResponseWriter, r *http.Request) {
	f, err := readDoc(r, d.t)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Docs.Create(r.Context(), d.t, f)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusCreated, command.DocJSON(res, sessionTenantOf(r)))
}

func (h Handlers) getDoc(d docRoute, w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, d.param)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.DocQueries.Get(r.Context(), d.t, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.Version.RowVersion)
	writeJSON(w, http.StatusOK, docView(v, r))
}

func (h Handlers) listDocs(d docRoute, w http.ResponseWriter, r *http.Request) {
	f := read.DocFilter{Q: r.URL.Query().Get("q"), Status: r.URL.Query().Get("status"), GradeLevel: queryInt(r, "grade_level")}
	var err error
	if f.SubjectID, err = queryUUID(r, "subject_id"); err == nil {
		if f.TopicID, err = queryUUID(r, "topic_id"); err == nil {
			f.ObjectiveID, err = queryUUID(r, "objective_id")
		}
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.DocQueries.List(r.Context(), d.t, f, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v query.DocView) map[string]any { return docView(v, r) })
}

func (h Handlers) updateDocDraft(d docRoute, w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, d.param)
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := pathVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	f, err := readDoc(r, d.t)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Docs.UpdateDraft(r.Context(), command.UpdateDocDraftInput{Type: d.t, ID: id, VersionNo: n, IfMatch: rv, Fields: f})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusOK, command.DocJSON(res, sessionTenantOf(r)))
}

func (h Handlers) newDocVersion(d docRoute, w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, d.param)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		FromVersion int    `json:"from_version"`
		Note        string `json:"note"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Docs.NewVersion(r.Context(), command.NewDocVersionInput{Type: d.t, ID: id, FromVersion: b.FromVersion, Note: b.Note})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusCreated, command.DocJSON(res, sessionTenantOf(r)))
}

func (h Handlers) deleteDoc(d docRoute, w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, d.param)
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	reason, err := readReason(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Docs.Delete(r.Context(), command.DeleteDocInput{Type: d.t, ID: id, IfMatch: rv, Reason: reason}); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) poolStats(d docRoute, w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, d.param)
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := pathVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if d.t != contentmodel.TypeQuestionSet && d.t != contentmodel.TypeAssessmentBlueprint {
		fail(w, r, problem.New(problem.NotFound, ""))
		return
	}
	st, err := h.DocQueries.PoolStats(r.Context(), d.t, id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": st})
}
