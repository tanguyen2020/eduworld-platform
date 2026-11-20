package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// HintCommands opens hint levels (command.Attempts, F032 US2).
type HintCommands interface {
	RevealHint(ctx context.Context, in command.RevealHintInput) (dto.HintView, error)
}

// SelfStudyQueries are the reads of the review list and the recommendations (query.SelfStudy, F032 US4, US6).
type SelfStudyQueries interface {
	WrongQuestions(ctx context.Context, subjectID *uuid.UUID, in query.PageInput) (query.Page[dto.WrongQuestionView], error)
	Recommendations(ctx context.Context, subjectID *uuid.UUID, lang string) (dto.RecommendationListView, error)
}

// MasteryQueries is getMyMastery (query.Progress, F032 US7).
type MasteryQueries interface {
	MyMastery(ctx context.Context, subjectID *uuid.UUID, grade *int) (dto.MyMasteryView, error)
}

func init() {
	for id, o := range map[string]httpx.Operation{
		"revealNextHint": player("revealNextHint", http.MethodPost, attemptsPath+"/{attempt_id}/items/{position}/hints:next",
			httpx.IdempotencyRequired, 200),
		"listMyWrongQuestions": player("listMyWrongQuestions", http.MethodGet, "/learning/wrong-questions", httpx.IdempotencyNone, 200),
		"getMyRecommendations": player("getMyRecommendations", http.MethodGet, "/learning/recommendations/me", httpx.IdempotencyNone, 200),
		"getMyMastery":         player("getMyMastery", http.MethodGet, "/learning/mastery/me", httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var selfStudyRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"revealNextHint":       Handlers.revealNextHint,
	"listMyWrongQuestions": Handlers.listMyWrongQuestions,
	"getMyRecommendations": Handlers.getMyRecommendations,
	"getMyMastery":         Handlers.getMyMastery,
}

func (h Handlers) revealNextHint(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "attempt_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	pos, err := strconv.Atoi(chi.URLParam(r, "position"))
	if err != nil {
		fail(w, r, problem.Invalid("/position", problem.SubInvalid, "is not an integer"))
		return
	}
	v, err := h.Hints.RevealHint(r.Context(), command.RevealHintInput{AttemptID: id, Position: pos})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) listMyWrongQuestions(w http.ResponseWriter, r *http.Request) {
	subject, err := queryUUID(r, "subject_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.SelfStudy.WrongQuestions(r.Context(), subject, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, "vi")
	list(w, p, func(v dto.WrongQuestionView) dto.WrongQuestionView { return v })
}

func (h Handlers) getMyRecommendations(w http.ResponseWriter, r *http.Request) {
	subject, err := queryUUID(r, "subject_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	lang := i18ntext.Locale(r.Header.Get("Accept-Language"))
	v, err := h.SelfStudy.Recommendations(r.Context(), subject, lang)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getMyMastery(w http.ResponseWriter, r *http.Request) {
	subject, err := queryUUID(r, "subject_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Mastery.MyMastery(r.Context(), subject, queryInt(r, "grade_level"))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, "vi")
	writeJSON(w, http.StatusOK, v)
}
