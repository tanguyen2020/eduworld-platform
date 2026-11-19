// Package sisinternal serves the machine-to-machine SIS webhook POST /internal/sis/webhooks/{connection_id}
// (receiveSisChanges of the EduWorld SIS Contract, F017 US5). It is not an /api/v1 operation: no Bearer
// token, the HMAC of the connection's webhook secret authenticates the SIS, and the route exists only on the
// internal/partner gateway with an IP allowlist (deploy/apisix routes, F017-AC-20). Bodies are never logged.
package sisinternal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Path is the route of the webhook (mounted at the root of the internal listener).
const Path = "/internal/sis/webhooks/{connection_id}"

// maxBody bounds a ChangeBatch (500 changes).
const maxBody = 8 << 20

// Receiver is command.ReceiveSisWebhook.
type Receiver interface {
	Handle(ctx context.Context, connectionID uuid.UUID, signature, timestamp string, body []byte) (uuid.UUID, error)
}

// Webhook is the HTTP handler of the SIS webhook.
type Webhook struct{ Receive Receiver }

// Register mounts the route on r (the composition root mounts it on the internal listener only).
func Register(r chi.Router, h Webhook) {
	r.Post(Path, h.ServeHTTP)
}

// ServeHTTP answers 202 after the JetStream ack, 401 without a reason, 404, 409 (replay), 400 or 503.
func (h Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "connection_id"))
	if err != nil {
		httpx.WriteError(w, r, problem.New(problem.NotFound, ""))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		httpx.WriteError(w, r, problem.New(problem.BadRequest, "body too large"))
		return
	}
	delivery, err := h.Receive.Handle(r.Context(), id, r.Header.Get("X-EduWorld-Signature"), r.Header.Get("X-EduWorld-Timestamp"), body)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"delivery_id": delivery.String()})
}
