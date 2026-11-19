package http

import (
	"context"
	"net/http"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/widget"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// WidgetQueries lists the interactive widgets (query.QuestionWidgets, F023).
type WidgetQueries interface {
	List(ctx context.Context) ([]widget.Info, error)
}

func init() {
	Operations["listQuestionWidgets"] = op("listQuestionWidgets", http.MethodGet, "/cms/learning/question-widgets", httpx.TenantOptional,
		httpx.IdempotencyNone, 200)
	questionRoutes["listQuestionWidgets"] = Handlers.listQuestionWidgets
}

func (h Handlers) listQuestionWidgets(w http.ResponseWriter, r *http.Request) {
	out, err := h.Widgets.List(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}
