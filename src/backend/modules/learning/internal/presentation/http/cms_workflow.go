package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// WorkflowCommands are the review workflow use cases (command.Workflow, F025).
type WorkflowCommands interface {
	Transition(ctx context.Context, in command.TransitionContentInput) (dto.ContentVersionView, error)
}

// ReviewQueries are the reads of the review queue (query.ReviewTasks, F025).
type ReviewQueries interface {
	List(ctx context.Context, in query.ReviewTaskInput) (query.Page[dto.ReviewTaskView], error)
}

// WorkflowOperations of F025 (merged into Operations).
var WorkflowOperations = map[string]httpx.Operation{
	"transitionContentVersion": op("transitionContentVersion", http.MethodPost,
		"/cms/learning/content/{content_type}/{content_id}/versions/{version}:transition", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"listContentReviewTasks": op("listContentReviewTasks", http.MethodGet, "/cms/learning/review-tasks", httpx.TenantOptional, httpx.IdempotencyNone, 200),
}

func init() {
	for id, o := range WorkflowOperations {
		Operations[id] = o
	}
}

var workflowRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"transitionContentVersion": Handlers.transitionContentVersion,
	"listContentReviewTasks":   Handlers.listContentReviewTasks,
}

type transitionBody struct {
	Action                     string                 `json:"action"`
	Note                       string                 `json:"note"`
	AudioExemptionAcknowledged bool                   `json:"audio_exemption_acknowledged"`
	FieldComments              []command.FieldComment `json:"field_comments"`
}

func (h Handlers) transitionContentVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "content_id")
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
	var b transitionBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Workflow.Transition(r.Context(), command.TransitionContentInput{Type: chi.URLParam(r, "content_type"), ContentID: id, VersionNo: n,
		IfMatch: rv, Action: b.Action, Note: b.Note, AudioExemptionAcknowledged: b.AudioExemptionAcknowledged,
		FieldComments: b.FieldComments})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.RowVersion)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listContentReviewTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, err := h.Reviews.List(r.Context(), query.ReviewTaskInput{Lane: q.Get("lane"), Status: q.Get("status"), ContentType: q.Get("content_type"),
		AssignedToMe: q.Get("assigned_to_me") == "true", Page: pageIn(r)})
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.ReviewTaskView) dto.ReviewTaskView { return v })
}
