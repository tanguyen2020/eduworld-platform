package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// QuestionCommands are the write use cases of the question bank (command.Questions).
type QuestionCommands interface {
	CreateQuestion(ctx context.Context, f command.QuestionFields) (command.QuestionResult, error)
	UpdateQuestionDraft(ctx context.Context, in command.UpdateQuestionDraftInput) (command.QuestionResult, error)
	CreateQuestionVersion(ctx context.Context, in command.NewQuestionVersionInput) (command.QuestionResult, error)
	DeleteQuestion(ctx context.Context, in command.DeleteQuestionInput) error
}

// QuestionQueries are the CMS reads of the question bank (query.Questions).
type QuestionQueries interface {
	GetQuestion(ctx context.Context, id uuid.UUID) (query.QuestionView, error)
	GetQuestionVersion(ctx context.Context, id uuid.UUID, versionNo int) (query.QuestionView, error)
	Search(ctx context.Context, f read.QuestionFilter, in query.PageInput) (query.Page[query.QuestionView], error)
}

const cmsQuestions = "/cms/learning/questions"

// QuestionOperations of F022 (merged into Operations).
var QuestionOperations = map[string]httpx.Operation{
	"searchQuestions":       op("searchQuestions", http.MethodGet, cmsQuestions, httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createQuestion":        op("createQuestion", http.MethodPost, cmsQuestions, httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"getCmsQuestion":        op("getCmsQuestion", http.MethodGet, cmsQuestions+"/{question_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"deleteQuestion":        op("deleteQuestion", http.MethodDelete, cmsQuestions+"/{question_id}", httpx.TenantOptional, httpx.IdempotencyNone, 204),
	"getCmsQuestionVersion": op("getCmsQuestionVersion", http.MethodGet, cmsQuestions+"/{question_id}/versions/{version}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"updateQuestionDraft":   op("updateQuestionDraft", http.MethodPatch, cmsQuestions+"/{question_id}/versions/{version}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createQuestionVersion": op("createQuestionVersion", http.MethodPost, cmsQuestions+"/{question_id}/versions", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
}

func init() {
	for id, o := range QuestionOperations {
		Operations[id] = o
	}
}

var questionRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"searchQuestions":       Handlers.searchQuestions,
	"createQuestion":        Handlers.createQuestion,
	"getCmsQuestion":        Handlers.getCmsQuestion,
	"deleteQuestion":        Handlers.deleteQuestion,
	"getCmsQuestionVersion": Handlers.getCmsQuestionVersion,
	"updateQuestionDraft":   Handlers.updateQuestionDraft,
	"createQuestionVersion": Handlers.createQuestionVersion,
}

// questionBody is QuestionAuthoringInput.
type questionBody struct {
	TenantID            *uuid.UUID      `json:"tenant_id"`
	OwnerScope          string          `json:"owner_scope"`
	Type                string          `json:"type"`
	Stem                json.RawMessage `json:"stem"`
	RenderPayload       json.RawMessage `json:"render_payload"`
	AnswerKey           json.RawMessage `json:"answer_key"`
	ScoringRule         json.RawMessage `json:"scoring_rule"`
	Explanation         json.RawMessage `json:"explanation"`
	Hints               json.RawMessage `json:"hints"`
	WorkedExampleRef    json.RawMessage `json:"worked_example_ref"`
	MathAssets          json.RawMessage `json:"math_assets"`
	MinResponseMS       *int            `json:"min_response_ms"`
	MaxScore            *float64        `json:"max_score"`
	Tags                []string        `json:"tags"`
	Difficulty          *int            `json:"difficulty"`
	GradeLevels         []int           `json:"grade_levels"`
	SubjectID           *uuid.UUID      `json:"subject_id"`
	TopicID             *uuid.UUID      `json:"topic_id"`
	CurriculumVersionID *uuid.UUID      `json:"curriculum_version_id"`
	ObjectiveIDs        []uuid.UUID     `json:"objective_ids"`
	MediaRefs           []uuid.UUID     `json:"media_refs"`
	ContentLanguage     string          `json:"content_language"`
	ChangeNote          *string         `json:"change_note"`
	ExamOnly            *bool           `json:"exam_only"`
	EfficacyInstrument  *bool           `json:"efficacy_instrument"`
}

func readQuestion(r *http.Request) (command.QuestionFields, error) {
	var keys map[string]json.RawMessage
	if err := decode(r, &keys); err != nil {
		return command.QuestionFields{}, err
	}
	b, _ := json.Marshal(keys)
	var q questionBody
	if err := json.Unmarshal(b, &q); err != nil {
		return command.QuestionFields{}, problem.New(problem.BadRequest, "invalid JSON body")
	}
	present := make(map[string]bool, len(keys))
	for k := range keys {
		present[k] = true
	}
	return command.QuestionFields{Present: present, TenantID: q.TenantID, OwnerScope: q.OwnerScope, Type: q.Type, Stem: q.Stem,
		RenderPayload: q.RenderPayload, AnswerKey: q.AnswerKey, ScoringRule: q.ScoringRule, Explanation: q.Explanation, Hints: q.Hints,
		WorkedExampleRef: q.WorkedExampleRef, MathAssets: q.MathAssets, MinResponseMS: q.MinResponseMS, MaxScore: q.MaxScore, Tags: q.Tags,
		Difficulty: q.Difficulty, GradeLevels: q.GradeLevels, SubjectID: q.SubjectID, TopicID: q.TopicID, CurriculumVersionID: q.CurriculumVersionID,
		ObjectiveIDs: q.ObjectiveIDs, MediaRefs: q.MediaRefs, ContentLanguage: q.ContentLanguage, ChangeNote: q.ChangeNote, ExamOnly: q.ExamOnly,
		EfficacyInstrument: q.EfficacyInstrument}, nil
}

func sessionTenantOf(r *http.Request) *uuid.UUID {
	if p, ok := authz.PrincipalFrom(r.Context()); ok {
		return p.TenantID
	}
	return nil
}

func (h Handlers) searchQuestions(w http.ResponseWriter, r *http.Request) {
	f := read.QuestionFilter{GradeLevel: queryInt(r, "grade_level"), Status: r.URL.Query().Get("status"), Type: r.URL.Query().Get("type"),
		Source: r.URL.Query().Get("source")}
	var err error
	for _, x := range []struct {
		name string
		dst  **uuid.UUID
	}{{"subject_id", &f.SubjectID}, {"topic_id", &f.TopicID}, {"objective_id", &f.ObjectiveID}} {
		if *x.dst, err = queryUUID(r, x.name); err != nil {
			fail(w, r, err)
			return
		}
	}
	for _, d := range r.URL.Query()["difficulty"] {
		n, err := strconv.Atoi(d)
		if err != nil {
			fail(w, r, problem.Invalid("difficulty", problem.SubInvalid, "must be 1–5"))
			return
		}
		f.Difficulties = append(f.Difficulties, n)
	}
	if e := r.URL.Query().Get("efficacy_instrument"); e != "" {
		b := e == "true"
		f.Efficacy = &b
	}
	p, err := h.QuestionQueries.Search(r.Context(), f, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v query.QuestionView) any { return v.View })
}

func (h Handlers) createQuestion(w http.ResponseWriter, r *http.Request) {
	f, err := readQuestion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.QuestionCommands.CreateQuestion(r.Context(), f)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusCreated, res.View(sessionTenantOf(r)))
}

func (h Handlers) getCmsQuestion(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "question_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.QuestionQueries.GetQuestion(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.Question.RowVersion) // If-Match of deleteQuestion; the version ETag comes with getCmsQuestionVersion
	writeJSON(w, http.StatusOK, v.View)
}

func (h Handlers) getCmsQuestionVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "question_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := pathVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.QuestionQueries.GetQuestionVersion(r.Context(), id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.Version.RowVersion)
	writeJSON(w, http.StatusOK, v.View)
}

func (h Handlers) updateQuestionDraft(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "question_id")
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
	f, err := readQuestion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.QuestionCommands.UpdateQuestionDraft(r.Context(), command.UpdateQuestionDraftInput{QuestionID: id, VersionNo: n, IfMatch: rv, Fields: f})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusOK, res.View(sessionTenantOf(r)))
}

func (h Handlers) createQuestionVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "question_id")
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
	res, err := h.QuestionCommands.CreateQuestionVersion(r.Context(), command.NewQuestionVersionInput{QuestionID: id, FromVersion: b.FromVersion, Note: b.Note})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusCreated, res.View(sessionTenantOf(r)))
}

func (h Handlers) deleteQuestion(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "question_id")
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
	if err := h.QuestionCommands.DeleteQuestion(r.Context(), command.DeleteQuestionInput{ID: id, IfMatch: rv, Reason: reason}); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
