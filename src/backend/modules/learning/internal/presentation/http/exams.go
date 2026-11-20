package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ExamCommands are the write use cases of the exam engine (command.Exams, F042).
type ExamCommands interface {
	CreateExamSession(ctx context.Context, in command.ExamSessionInput) (dto.ExamSessionView, error)
	ReleaseExamResults(ctx context.Context, id uuid.UUID) (dto.ExamSessionView, error)
	StartExamAttempt(ctx context.Context, in command.StartExamInput) (dto.AttemptView, error)
	SaveExamAnswer(ctx context.Context, in command.SaveExamInput) error
	SubmitExamAttempt(ctx context.Context, in command.SubmitExamInput) (dto.ExamSubmitView, error)
}

// ExamQueries are the reads of the exam engine (query.Exams, F042).
type ExamQueries interface {
	Sessions(ctx context.Context, in query.ExamListInput, pg query.PageInput) (query.Page[dto.ExamSessionView], error)
	Library(ctx context.Context, in query.ExamListInput, pg query.PageInput) (query.Page[dto.ExamSessionView], error)
	Get(ctx context.Context, id uuid.UUID) (dto.ExamSessionView, error)
	Papers(ctx context.Context, id uuid.UUID) ([]dto.ExamPaperView, error)
	Result(ctx context.Context, id uuid.UUID) (dto.ReviewView, string, error)
	Analysis(ctx context.Context, id uuid.UUID) (dto.ExamAnalysisView, string, error)
}

// examDeviceHeader is ExamDeviceToken (F042-AC-14).
const examDeviceHeader = "X-Exam-Device-Token"

func init() {
	session := "/exams/sessions/{exam_session_id}"
	att := "/exams/attempts/{attempt_id}"
	for id, o := range map[string]httpx.Operation{
		"listExamSessions":       lmsOp("listExamSessions", http.MethodGet, "/exams/sessions", httpx.IdempotencyNone, 200),
		"createExamSession":      lmsOp("createExamSession", http.MethodPost, "/exams/sessions", httpx.IdempotencyRequired, 201),
		"getExamSession":         lmsOp("getExamSession", http.MethodGet, session, httpx.IdempotencyNone, 200),
		"releaseExamResults":     lmsOp("releaseExamResults", http.MethodPost, session+":release-results", httpx.IdempotencyRequired, 200),
		"listExamSessionPapers":  lmsOp("listExamSessionPapers", http.MethodGet, session+"/papers", httpx.IdempotencyNone, 200),
		"getExamResult":          lmsOp("getExamResult", http.MethodGet, att+"/result", httpx.IdempotencyNone, 200),
		"getExamAttemptAnalysis": lmsOp("getExamAttemptAnalysis", http.MethodGet, att+"/analysis", httpx.IdempotencyNone, 200),
		"startExamAttempt":       player("startExamAttempt", http.MethodPost, session+"/attempts", httpx.IdempotencyRequired, 201),
		"saveExamAnswer":         player("saveExamAnswer", http.MethodPut, att+"/answers/{position}", httpx.IdempotencyNone, 204),
		"submitExamAttempt":      player("submitExamAttempt", http.MethodPost, att+":submit", httpx.IdempotencyRequired, 200),
		"listExamLibrary":        player("listExamLibrary", http.MethodGet, "/exams/library", httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var examRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listExamSessions": Handlers.listExamSessions, "createExamSession": Handlers.createExamSession, "getExamSession": Handlers.getExamSession,
	"releaseExamResults": Handlers.releaseExamResults, "listExamSessionPapers": Handlers.listExamSessionPapers,
	"getExamResult": Handlers.getExamResult, "getExamAttemptAnalysis": Handlers.getExamAttemptAnalysis,
	"startExamAttempt": Handlers.startExamAttempt, "saveExamAnswer": Handlers.saveExamAnswer, "submitExamAttempt": Handlers.submitExamAttempt,
	"listExamLibrary": Handlers.listExamLibrary,
}

func deviceToken(r *http.Request) (*string, error) {
	v := r.Header.Get(examDeviceHeader)
	if v == "" {
		return nil, nil
	}
	if len(v) < 8 || len(v) > 128 {
		return nil, problem.Invalid("/headers/"+examDeviceHeader, problem.SubInvalid, "must be 8–128 characters")
	}
	for _, c := range v {
		if !tokenChar(c) {
			return nil, problem.Invalid("/headers/"+examDeviceHeader, problem.SubInvalid, "has a character outside A-Z a-z 0-9 . _ ~ -")
		}
	}
	return &v, nil
}

// tokenChar is a character of ExamDeviceToken (A-Z a-z 0-9 . _ ~ -).
func tokenChar(c rune) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '~' || c == '-'
}

func examListInput(r *http.Request) (query.ExamListInput, error) {
	var in query.ExamListInput
	var err error
	if in.SubjectID, err = queryUUID(r, "subject_id"); err != nil {
		return in, err
	}
	if g := r.URL.Query().Get("grade_level"); g != "" {
		n, err := strconv.Atoi(g)
		if err != nil || n < 1 || n > 12 {
			return in, problem.Invalid("/grade_level", problem.SubInvalid, "must be 1–12")
		}
		in.GradeLevel = &n
	}
	in.Type = r.URL.Query().Get("type")
	return in, nil
}

func (h Handlers) listExamSessions(w http.ResponseWriter, r *http.Request) {
	in, err := examListInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.ExamQueries.Sessions(r.Context(), in, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.ExamSessionView) dto.ExamSessionView { return v })
}

func (h Handlers) listExamLibrary(w http.ResponseWriter, r *http.Request) {
	in, err := examListInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.ExamQueries.Library(r.Context(), in, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.ExamSessionView) dto.ExamSessionView { return v })
}

func examSessionResult(w http.ResponseWriter, r *http.Request, status int, v dto.ExamSessionView, err error) {
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.Version)
	writeJSON(w, status, v)
}

func (h Handlers) createExamSession(w http.ResponseWriter, r *http.Request) {
	var in command.ExamSessionInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.ExamCommands.CreateExamSession(r.Context(), in)
	examSessionResult(w, r, http.StatusCreated, v, err)
}

func (h Handlers) getExamSession(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "exam_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.ExamQueries.Get(r.Context(), id)
	examSessionResult(w, r, http.StatusOK, v, err)
}

func (h Handlers) releaseExamResults(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "exam_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.ExamCommands.ReleaseExamResults(r.Context(), id)
	examSessionResult(w, r, http.StatusOK, v, err)
}

func (h Handlers) listExamSessionPapers(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "exam_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.ExamQueries.Papers(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": v})
}

func (h Handlers) startExamAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "exam_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.StartExamInput{SessionID: id, AcceptLanguage: r.Header.Get("Accept-Language")}
	if in.DeviceToken, err = deviceToken(r); err != nil {
		fail(w, r, err)
		return
	}
	if r.ContentLength != 0 {
		var body struct {
			TakeOver bool `json:"take_over"`
		}
		if err := decode(r, &body); err != nil {
			fail(w, r, err)
			return
		}
		in.TakeOver = body.TakeOver
	}
	v, err := h.ExamCommands.StartExamAttempt(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Language", v.DisplayLanguage)
	writeJSON(w, http.StatusCreated, v)
}

func (h Handlers) saveExamAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	pos, err := strconv.Atoi(r.PathValue("position"))
	if err != nil || pos < 1 {
		fail(w, r, problem.Invalid("/position", problem.SubInvalid, "must be a positive integer"))
		return
	}
	var body struct {
		Kind             string          `json:"kind"`
		Position         *int            `json:"position"`
		QuestionVersion  *uuid.UUID      `json:"question_version_id"`
		Response         json.RawMessage `json:"response"`
		ClientElapsedMS  *int64          `json:"client_elapsed_ms"`
		AnsweredAtClient *time.Time      `json:"answered_at_client"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if body.Kind != "" && body.Kind != "question" {
		fail(w, r, problem.Invalid("/kind", problem.SubInvalid, "an exam answer is a question response"))
		return
	}
	in := command.SaveExamInput{AttemptID: id, Position: pos, Response: body.Response, ClientElapsedMS: body.ClientElapsedMS,
		AnsweredAtClient: body.AnsweredAtClient}
	if in.DeviceToken, err = deviceToken(r); err != nil {
		fail(w, r, err)
		return
	}
	if err := h.ExamCommands.SaveExamAnswer(r.Context(), in); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) submitExamAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.SubmitExamInput{AttemptID: id}
	if in.DeviceToken, err = deviceToken(r); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.ExamCommands.SubmitExamAttempt(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getExamResult(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, lang, err := h.ExamQueries.Result(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Language", lang)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getExamAttemptAnalysis(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, lang, err := h.ExamQueries.Analysis(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Language", lang)
	writeJSON(w, http.StatusOK, v)
}
