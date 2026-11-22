package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisConnectionsHandler is the connection use cases (command.SisConnections).
type SisConnectionsHandler interface {
	Create(ctx context.Context, tenantID uuid.UUID, providerKind, reason string) (write.SisConnection, error)
	// CreateContract is the contract_v1 branch of createSisConnection (F017-AC-01).
	CreateContract(ctx context.Context, tenantID uuid.UUID, in command.ContractCreateInput) (dto.SisConnection, error)
	SetSchools(ctx context.Context, id uuid.UUID, schoolIDs []uuid.UUID, reason string) ([]write.SisConnectionSchool, error)
	Schools(ctx context.Context, id uuid.UUID, limit int, cursor string) (command.SisConnectionSchoolPage, error)
}

// F011 platform operations on connections and runs (gateway nội bộ).
var sisAdminOperations = map[string]func(Handlers) http.HandlerFunc{
	"listSisConnections":       func(h Handlers) http.HandlerFunc { return h.listSisConnections },
	"createSisConnection":      func(h Handlers) http.HandlerFunc { return h.createSisConnection },
	"listSisConnectionSchools": func(h Handlers) http.HandlerFunc { return h.listSisConnectionSchools },
	"setSisConnectionSchools":  func(h Handlers) http.HandlerFunc { return h.setSisConnectionSchools },
	"listSisSyncRuns":          func(h Handlers) http.HandlerFunc { return h.listSisSyncRuns },
	"getSisSyncRun":            func(h Handlers) http.HandlerFunc { return h.getSisSyncRun },
	"listSisSyncRunErrors":     func(h Handlers) http.HandlerFunc { return h.listSisSyncRunErrors },
}

type connectionSchoolView struct {
	ConnectionID uuid.UUID `json:"sis_connection_id"`
	SchoolID     uuid.UUID `json:"school_id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	Active       bool      `json:"active"`
	AssignedAt   time.Time `json:"assigned_at"`
}

func schoolsView(rows []write.SisConnectionSchool) map[string][]connectionSchoolView {
	out := make([]connectionSchoolView, 0, len(rows))
	for _, s := range rows {
		out = append(out, connectionSchoolView{ConnectionID: s.ConnectionID, SchoolID: s.SchoolID, TenantID: s.TenantID, Active: s.Active, AssignedAt: s.AssignedAt})
	}
	return map[string][]connectionSchoolView{"data": out}
}

func (h Handlers) listSisConnections(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.SisAdmin.ListConnections(r.Context(), q.Get("provider_kind"), q.Get("status"), list(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) createSisConnection(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ProviderKind     string              `json:"provider_kind"`
		Reason           string              `json:"reason"`
		ContractVersion  string              `json:"contract_version"`
		Config           *sis.ContractConfig `json:"config"`
		SecretRef        string              `json:"secret_ref"`
		WebhookSecretRef string              `json:"webhook_secret_ref"`
		Schedule         string              `json:"schedule"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	p, ok := authz.PrincipalFrom(r.Context())
	if !ok || p.TenantID == nil {
		fail(w, r, problem.New(problem.TenantHeaderRequired, ""))
		return
	}
	if b.ProviderKind == sis.ProviderContractV1 { // F017
		in := command.ContractCreateInput{ContractVersion: b.ContractVersion, SecretRef: b.SecretRef, WebhookSecretRef: b.WebhookSecretRef,
			Schedule: b.Schedule, Reason: b.Reason}
		if b.Config != nil {
			in.Config = *b.Config
		}
		out, err := h.SisConnections.CreateContract(r.Context(), *p.TenantID, in)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
		return
	}
	c, err := h.SisConnections.Create(r.Context(), *p.TenantID, b.ProviderKind, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	updated := c.UpdatedAt
	writeJSON(w, http.StatusCreated, dto.SisConnection{ConnectionID: c.ID, TenantID: c.TenantID, ProviderKind: c.ProviderKind, Status: c.Status,
		CreatedAt: c.CreatedAt, UpdatedAt: &updated})
}

func (h Handlers) listSisConnectionSchools(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "connection_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.SisConnections.Schools(r.Context(), id, queryInt(r, "limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": schoolsView(res.Data)["data"], "page": res.Page})
}

func (h Handlers) setSisConnectionSchools(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "connection_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		SchoolIDs []uuid.UUID `json:"school_ids"`
		Reason    string      `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	rows, err := h.SisConnections.SetSchools(r.Context(), id, b.SchoolIDs, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, schoolsView(rows))
}

func (h Handlers) listSisSyncRuns(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "connection_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.SisAdmin.ListRuns(r.Context(), id, r.URL.Query().Get("status"), list(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getSisSyncRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "run_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.SisAdmin.GetRun(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listSisSyncRunErrors(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "run_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.SisAdmin.ListRunErrors(r.Context(), id, list(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
