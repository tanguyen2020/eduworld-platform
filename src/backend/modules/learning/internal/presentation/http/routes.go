// Package http is the public HTTP presentation of module learning: thin handlers that decode the request, call one
// use case and encode the response; authentication, X-Tenant-Id, principal, guards, OpenAPI validation and
// idempotency are the pkg/httpx chain declared per operation (Operations).
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CurriculumCommands are the write use cases of the curriculum (implemented by command.Curriculum).
type CurriculumCommands interface {
	CreateCurriculum(ctx context.Context, in command.CreateCurriculumInput) (dto.Curriculum, error)
	UpdateCurriculum(ctx context.Context, in command.UpdateCurriculumInput) (dto.Curriculum, error)
	DeleteCurriculum(ctx context.Context, in command.DeleteCurriculumInput) error
	CreateVersion(ctx context.Context, in command.NewVersionInput) (dto.CurriculumVersion, error)
	UpdateVersionDraft(ctx context.Context, in command.UpdateVersionDraftInput) (dto.CurriculumVersion, error)
	Transition(ctx context.Context, in command.TransitionInput) (dto.CurriculumVersion, error)
	Publish(ctx context.Context, in command.PublishInput) (dto.CurriculumVersion, error)
	PutLoMappings(ctx context.Context, in command.LoMappingsInput) (command.LoMappings, error)
	CreateSubject(ctx context.Context, in command.SubjectInput) (dto.Subject, error)
	UpdateSubject(ctx context.Context, in command.UpdateSubjectInput) (dto.Subject, error)
	CreateTopic(ctx context.Context, in command.TopicInput) (dto.Topic, error)
	UpdateTopic(ctx context.Context, in command.UpdateTopicInput) (dto.Topic, error)
	DeleteTopic(ctx context.Context, in command.DeleteInput) error
	CreateObjective(ctx context.Context, in command.ObjectiveInput) (dto.Objective, error)
	UpdateObjective(ctx context.Context, in command.UpdateObjectiveInput) (dto.Objective, error)
	DeleteObjective(ctx context.Context, in command.DeleteInput) error
	CreateCompetency(ctx context.Context, in command.CompetencyInput) (dto.Competency, error)
	UpdateCompetency(ctx context.Context, in command.UpdateCompetencyInput) (dto.Competency, error)
	CreateSeries(ctx context.Context, in command.SeriesInput) (dto.TextbookSeries, error)
	CreateTextbook(ctx context.Context, in command.TextbookInput) (dto.Textbook, error)
	CreateSection(ctx context.Context, in command.SectionInput) (dto.TextbookSection, error)
	UpdateSection(ctx context.Context, in command.UpdateSectionInput) (dto.TextbookSection, error)
	PutSectionObjectives(ctx context.Context, in command.SectionObjectivesInput) (dto.TextbookSection, error)
	CreateImport(ctx context.Context, in command.CreateImportInput) (dto.ImportJob, error)
}

// CurriculumQueries are the read use cases of the curriculum (implemented by query.Curriculum).
type CurriculumQueries interface {
	ListCurricula(ctx context.Context, ownerScope string, in query.PageInput) (query.Page[dto.Curriculum], error)
	GetCurriculum(ctx context.Context, id uuid.UUID) (dto.Curriculum, error)
	ListVersions(ctx context.Context, curriculumID uuid.UUID, in query.PageInput) (query.Page[dto.CurriculumVersion], error)
	GetVersion(ctx context.Context, curriculumID uuid.UUID, versionNo int) (dto.CurriculumVersion, error)
	ListSubjects(ctx context.Context, grade *int, in query.PageInput) (query.Page[dto.Subject], error)
	ListTopics(ctx context.Context, versionID uuid.UUID, subject *uuid.UUID, grade *int, in query.PageInput) (query.Page[dto.Topic], error)
	ListObjectives(ctx context.Context, in query.ObjectiveListInput) (query.Page[dto.Objective], error)
	ListCompetencies(ctx context.Context, versionID uuid.UUID, in query.PageInput) (query.Page[dto.Competency], error)
	LoMappings(ctx context.Context, curriculumID uuid.UUID, versionNo int) (dto.LoMappingListView, int64, error)
	ListSeries(ctx context.Context, in query.PageInput) (query.Page[dto.TextbookSeries], error)
	ListTextbooks(ctx context.Context, f read.TextbookFilter, in query.PageInput) (query.Page[dto.Textbook], error)
	ListSections(ctx context.Context, textbookID uuid.UUID, in query.PageInput) (query.Page[dto.TextbookSection], error)
	GetImport(ctx context.Context, id uuid.UUID) (dto.ImportJob, error)
	Export(ctx context.Context, curriculumID uuid.UUID, versionNo int, format string) ([]byte, error)
	PublicCurricula(ctx context.Context, subject *uuid.UUID, grade *int) ([]query.AppliedCurriculum, error)
	PublicSubjects(ctx context.Context, grade *int) ([]dto.Subject, error)
	PublicTopics(ctx context.Context, curriculumID, subject *uuid.UUID, grade *int) ([]dto.Topic, string, error)
	PublicObjectives(ctx context.Context, in query.PublicObjectiveInput) (query.Page[dto.Objective], string, error)
	PublicObjective(ctx context.Context, id uuid.UUID) (dto.Objective, string, error)
}

// Handlers are the use cases served over HTTP (F021 curriculum, F022 question bank, F027 media and preview).
type Handlers struct {
	Commands         CurriculumCommands
	Queries          CurriculumQueries
	QuestionCommands QuestionCommands    // F022
	QuestionQueries  QuestionQueries     // F022
	Media            MediaCommands       // F027
	MediaQueries     MediaQueries        // F027
	Preview          PreviewCommands     // F027
	Docs             DocCommands         // F024
	DocQueries       DocQueries          // F024
	Widgets          WidgetQueries       // F023
	Workflow         WorkflowCommands    // F025
	Reviews          ReviewQueries       // F025
	Translations     TranslationCommands // F025 US5
	TranslationJobs  TranslationQueries  // F025 US5
	Releases         ReleaseCommands     // F026
	ReleaseQueries   ReleaseQueries      // F026
	CmsActivities    CmsActivityQueries  // F030 US4
	ActivityTypes    ActivityTypeQueries // F030 US4
	Attempts         AttemptCommands     // F030 US1–US3, US6
	AttemptQueries   AttemptQueries      // F030 US1–US3
	Progress         ProgressQueries     // F031 US3
	ProfileAudit     ProfileAudit        // F031 US3 (learning.profile_viewed)
	Regrades         RegradeCommands     // F031 US4
	RegradeQueries   RegradeQueries      // F031 US4
}

func op(id, method, pattern string, tenant httpx.TenantMode, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return httpx.Operation{ID: id, Module: "learning", Method: method, Pattern: pattern, Tenant: tenant, Idempotency: idem, SuccessStatus: status}
}

const (
	cmsCurricula  = "/cms/learning/curricula"
	cmsCurriculum = cmsCurricula + "/{curriculum_id}"
	cmsVersions   = cmsCurriculum + "/versions"
	cmsVersion    = cmsVersions + "/{version}"
)

// Operations of F021 (method, path, X-Tenant-Id mode, Idempotency-Key and success status of the spec; the contract
// test checks them against specs/openapi/eduworld-api.v1.yaml). /cms/ routes get the portal_staff guard from httpx.
var Operations = map[string]httpx.Operation{
	"listCmsCurricula":               op("listCmsCurricula", http.MethodGet, cmsCurricula, httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createCurriculum":               op("createCurriculum", http.MethodPost, cmsCurricula, httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"getCmsCurriculum":               op("getCmsCurriculum", http.MethodGet, cmsCurriculum, httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"updateCurriculum":               op("updateCurriculum", http.MethodPatch, cmsCurriculum, httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"deleteCurriculum":               op("deleteCurriculum", http.MethodDelete, cmsCurriculum, httpx.TenantOptional, httpx.IdempotencyNone, 204),
	"listCurriculumVersions":         op("listCurriculumVersions", http.MethodGet, cmsVersions, httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createCurriculumVersion":        op("createCurriculumVersion", http.MethodPost, cmsVersions, httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"getCurriculumVersion":           op("getCurriculumVersion", http.MethodGet, cmsVersion, httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"updateCurriculumVersionDraft":   op("updateCurriculumVersionDraft", http.MethodPatch, cmsVersion, httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"transitionCurriculumVersion":    op("transitionCurriculumVersion", http.MethodPost, cmsVersion+":transition", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"publishCurriculumVersion":       op("publishCurriculumVersion", http.MethodPost, cmsVersion+":publish", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"exportCurriculumVersion":        op("exportCurriculumVersion", http.MethodGet, cmsVersion+":export", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"getCurriculumVersionLoMappings": op("getCurriculumVersionLoMappings", http.MethodGet, cmsVersion+"/lo-mappings", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"putCurriculumVersionLoMappings": op("putCurriculumVersionLoMappings", http.MethodPut, cmsVersion+"/lo-mappings", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"listCmsSubjects":                op("listCmsSubjects", http.MethodGet, "/cms/learning/subjects", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createSubject":                  op("createSubject", http.MethodPost, "/cms/learning/subjects", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"updateSubject":                  op("updateSubject", http.MethodPatch, "/cms/learning/subjects/{subject_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"listCmsTopics":                  op("listCmsTopics", http.MethodGet, "/cms/learning/topics", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createTopic":                    op("createTopic", http.MethodPost, "/cms/learning/topics", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"updateTopic":                    op("updateTopic", http.MethodPatch, "/cms/learning/topics/{topic_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"deleteTopic":                    op("deleteTopic", http.MethodDelete, "/cms/learning/topics/{topic_id}", httpx.TenantOptional, httpx.IdempotencyNone, 204),
	"listCmsObjectives":              op("listCmsObjectives", http.MethodGet, "/cms/learning/objectives", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createObjective":                op("createObjective", http.MethodPost, "/cms/learning/objectives", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"updateObjective":                op("updateObjective", http.MethodPatch, "/cms/learning/objectives/{objective_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"deleteObjective":                op("deleteObjective", http.MethodDelete, "/cms/learning/objectives/{objective_id}", httpx.TenantOptional, httpx.IdempotencyNone, 204),
	"listCmsCompetencies":            op("listCmsCompetencies", http.MethodGet, "/cms/learning/competencies", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createCompetency":               op("createCompetency", http.MethodPost, "/cms/learning/competencies", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"updateCompetency":               op("updateCompetency", http.MethodPatch, "/cms/learning/competencies/{competency_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"listTextbookSeries":             op("listTextbookSeries", http.MethodGet, "/cms/learning/textbook-series", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createTextbookSeries":           op("createTextbookSeries", http.MethodPost, "/cms/learning/textbook-series", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"listTextbooks":                  op("listTextbooks", http.MethodGet, "/cms/learning/textbooks", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createTextbook":                 op("createTextbook", http.MethodPost, "/cms/learning/textbooks", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"listTextbookSections":           op("listTextbookSections", http.MethodGet, "/cms/learning/textbook-sections", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createTextbookSection":          op("createTextbookSection", http.MethodPost, "/cms/learning/textbook-sections", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"updateTextbookSection":          op("updateTextbookSection", http.MethodPatch, "/cms/learning/textbook-sections/{section_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"putTextbookSectionObjectives":   op("putTextbookSectionObjectives", http.MethodPut, "/cms/learning/textbook-sections/{section_id}/objectives", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"createContentImport":            op("createContentImport", http.MethodPost, "/cms/learning/imports", httpx.TenantOptional, httpx.IdempotencyRequired, 202),
	"getContentImport":               op("getContentImport", http.MethodGet, "/cms/learning/imports/{import_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"listCurricula":                  op("listCurricula", http.MethodGet, "/learning/curricula", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listSubjects":                   op("listSubjects", http.MethodGet, "/learning/subjects", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listTopics":                     op("listTopics", http.MethodGet, "/learning/topics", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listLearningObjectives":         op("listLearningObjectives", http.MethodGet, "/learning/objectives", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"getLearningObjective":           op("getLearningObjective", http.MethodGet, "/learning/objectives/{objective_id}", httpx.TenantRequired, httpx.IdempotencyNone, 200),
}

// Register mounts every F021 operation on the server.
func Register(s *httpx.Server, h Handlers) error {
	routes := map[string]http.HandlerFunc{}
	for id, fn := range cmsCurriculumRoutes {
		routes[id] = func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) }
	}
	for id, fn := range cmsStructureRoutes {
		routes[id] = func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) }
	}
	for id, fn := range questionRoutes {
		routes[id] = func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) }
	}
	for id, fn := range mediaRoutes {
		routes[id] = func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) }
	}
	for id, fn := range docOps {
		d := routeOf(id)
		routes[id] = func(w http.ResponseWriter, r *http.Request) { fn(h, d, w, r) }
	}
	for id, fn := range workflowRoutes {
		routes[id] = func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) }
	}
	attemptsWired := h.Attempts != nil && h.AttemptQueries != nil
	for id, fn := range attemptRoutes {
		routes[id] = wiredOr(attemptsWired, func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) })
	}
	activitiesWired := h.CmsActivities != nil && h.ActivityTypes != nil
	for id, fn := range cmsActivityRoutes {
		routes[id] = wiredOr(activitiesWired, func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) })
	}
	progressWired := h.Progress != nil && h.ProfileAudit != nil
	for id, fn := range progressRoutes {
		routes[id] = wiredOr(progressWired, func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) })
	}
	regradeWired := h.Regrades != nil && h.RegradeQueries != nil
	for id, fn := range regradeRoutes {
		routes[id] = wiredOr(regradeWired, func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) })
	}
	for id, fn := range publicRoutes {
		routes[id] = func(w http.ResponseWriter, r *http.Request) { fn(h, w, r) }
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
	if len(routes) != len(Operations) {
		return problem.New(problem.Internal, "an operation of module learning has no handler")
	}
	return nil
}

// wiredOr serves fn when its use cases are wired into the binary, otherwise 503 (a route of the contract is never a
// nil-handler panic while the composition root has not wired a feature yet).
func wiredOr(wired bool, fn http.HandlerFunc) http.HandlerFunc {
	if wired {
		return fn
	}
	return func(w http.ResponseWriter, r *http.Request) {
		fail(w, r, problem.New(problem.Unavailable, "this operation is not wired in this binary yet"))
	}
}

// ---------- encoding helpers

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

func pathVersion(r *http.Request) (int, error) {
	n, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil || n < 1 {
		return 0, problem.Invalid("version", problem.SubInvalid, "must be a positive integer")
	}
	return n, nil
}

func queryUUID(r *http.Request, name string) (*uuid.UUID, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, problem.Invalid(name, problem.SubInvalid, "must be a UUID")
	}
	return &id, nil
}

func queryInt(r *http.Request, name string) *int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

func pageIn(r *http.Request) query.PageInput {
	limit := 0
	if n := queryInt(r, "limit"); n != nil {
		limit = *n
	}
	return query.PageInput{Cursor: r.URL.Query().Get("cursor"), Limit: limit}
}

// pageJSON is PageInfo.
type pageJSON struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Limit      int     `json:"limit"`
}

func list[T, V any](w http.ResponseWriter, p query.Page[T], view func(T) V) {
	data := make([]V, len(p.Data))
	for i, x := range p.Data {
		data[i] = view(x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": pageJSON{NextCursor: p.NextCursor, HasMore: p.HasMore, Limit: p.Limit}})
}

func fail(w http.ResponseWriter, r *http.Request, err error) { httpx.WriteError(w, r, err) }

// patch is a decoded merge-patch body: fields present (null included) by name.
type patch map[string]json.RawMessage

func decodePatch(r *http.Request) (patch, error) {
	var p patch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return p, nil
}

func (p patch) has(k string) bool { _, ok := p[k]; return ok }

func (p patch) isNull(k string) bool { return string(p[k]) == "null" }

// into decodes field k into v when present and not null; it reports whether it did.
func (p patch) into(k string, v any) (bool, error) {
	raw, ok := p[k]
	if !ok || string(raw) == "null" {
		return false, nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return false, problem.Invalid(k, problem.SubInvalid, "has the wrong type")
	}
	return true, nil
}

// nullable decodes a nullable field into a command.Patch.
func nullable[T any](p patch, k string) (command.Patch[T], error) {
	if !p.has(k) {
		return command.Patch[T]{}, nil
	}
	if p.isNull(k) {
		return command.Patch[T]{Set: true}, nil
	}
	var v T
	if _, err := p.into(k, &v); err != nil {
		return command.Patch[T]{}, err
	}
	return command.Patch[T]{Set: true, Value: &v}, nil
}

// optionalOf decodes a non-nullable optional field into a pointer.
func optionalOf[T any](p patch, k string) (*T, error) {
	var v T
	ok, err := p.into(k, &v)
	if err != nil || !ok {
		return nil, err
	}
	return &v, nil
}
