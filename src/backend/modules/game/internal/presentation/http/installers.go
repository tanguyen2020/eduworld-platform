package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SchoolInstallersQuery is listSchoolClientInstallers (query.SchoolInstallers, F134 US3).
type SchoolInstallersQuery interface {
	List(ctx context.Context, school uuid.UUID) (query.SchoolInstallerView, error)
}

func init() {
	o := op("listSchoolClientInstallers", http.MethodGet, "/tenant/client-installers", httpx.TenantRequired, httpx.IdempotencyNone,
		http.StatusOK, httpx.GuardPortalStaff)
	Operations[o.ID] = o
}

func (h Handlers) installerRoutes() map[string]http.HandlerFunc {
	if h.Installers == nil {
		return nil
	}
	return map[string]http.HandlerFunc{"listSchoolClientInstallers": func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.URL.Query().Get("school_id"))
		if err != nil {
			fail(w, r, problem.Invalid("school_id", problem.SubRequired, "a school uuid is required"))
			return
		}
		out, err := h.Installers.List(r.Context(), id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}}
}
