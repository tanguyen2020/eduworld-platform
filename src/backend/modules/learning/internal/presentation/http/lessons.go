package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// LessonCommands complete lessons (command.Lessons, F031 US6).
type LessonCommands interface {
	Complete(ctx context.Context, lessonID uuid.UUID) (dto.LessonCompletionView, error)
}

// LessonQueries read lessons for their student (query.Lessons).
type LessonQueries interface {
	Get(ctx context.Context, lessonID uuid.UUID) (dto.LessonView, error)
}

func init() {
	Operations["getLesson"] = player("getLesson", http.MethodGet, "/learning/lessons/{lesson_id}", httpx.IdempotencyNone, 200)
	Operations["completeLesson"] = player("completeLesson", http.MethodPost, "/learning/lessons/{lesson_id}:complete", httpx.IdempotencyRequired, 200)
}

var lessonRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"getLesson":      Handlers.getLesson,
	"completeLesson": Handlers.completeLesson,
}

func (h Handlers) getLesson(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "lesson_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.LessonQueries.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, "vi")
	writeJSON(w, http.StatusOK, v)
}

// completeLesson reads no body: results and durations sent by the client are ignored (ADR-0016).
func (h Handlers) completeLesson(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "lesson_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Lessons.Complete(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
