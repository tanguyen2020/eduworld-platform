package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// RegradeCommands create regrade requests (command.Regrades, F031 US4).
type RegradeCommands interface {
	Create(ctx context.Context, in command.CreateRegradeInput) (dto.RegradeRequestView, error)
}

// RegradeQueries read regrade requests (query.Regrades).
type RegradeQueries interface {
	Get(ctx context.Context, id uuid.UUID) (dto.RegradeRequestView, error)
}

func init() {
	Operations["createRegradeRequest"] = op("createRegradeRequest", http.MethodPost, "/cms/learning/regrade-requests", httpx.TenantOptional,
		httpx.IdempotencyRequired, 202)
	Operations["getRegradeRequest"] = op("getRegradeRequest", http.MethodGet, "/cms/learning/regrade-requests/{regrade_request_id}",
		httpx.TenantOptional, httpx.IdempotencyNone, 200)
}

var regradeRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"createRegradeRequest": Handlers.createRegradeRequest,
	"getRegradeRequest":    Handlers.getRegradeRequest,
}

func (h Handlers) createRegradeRequest(w http.ResponseWriter, r *http.Request) {
	var b command.CreateRegradeInput
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Regrades.Create(r.Context(), b)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, v)
}

func (h Handlers) getRegradeRequest(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "regrade_request_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.RegradeQueries.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
