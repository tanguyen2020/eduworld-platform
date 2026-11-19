package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisContractConnections is the contract_v1 connection use cases (command.SisContract).
type SisContractConnections interface {
	Update(ctx context.Context, id uuid.UUID, version int64, in command.ContractPatch) (dto.SisConnection, error)
	CheckHealth(ctx context.Context, id uuid.UUID) (dto.SisConnectionHealth, error)
}

// SisSyncTrigger is triggerSisSync (command.SisSync).
type SisSyncTrigger interface {
	Trigger(ctx context.Context, id uuid.UUID, in command.TriggerInput) (dto.SyncRun, error)
}

// SisContractHandlers serve the F017 operations of SIS connections (internal gateway only, x-internal).
type SisContractHandlers struct {
	Connections SisContractConnections
	Sync        SisSyncTrigger
}

func sisContractOp(id, method, pattern string, tenant httpx.TenantMode, idem httpx.IdempotencyMode, status int) httpx.Operation {
	o := op(id, method, pattern, tenant, idem, status)
	o.Guard |= httpx.GuardPortalStaff // Admin Portal operations (x-portal-action, F012-AC-05)
	return o
}

// SisContractOperations are the F017 operations (method, path, X-Tenant-Id mode, Idempotency-Key, success
// status of the spec; tests/contract/sis_contract_routes_test.go checks them against the merged spec).
var SisContractOperations = map[string]httpx.Operation{
	"triggerSisSync": sisContractOp("triggerSisSync", http.MethodPost, "/sis/connections/{connection_id}/sync-runs", httpx.TenantRequired,
		httpx.IdempotencyRequired, http.StatusAccepted),
	"updateSisConnection": sisContractOp("updateSisConnection", http.MethodPatch, "/sis/connections/{connection_id}", httpx.TenantOptional,
		httpx.IdempotencyNone, http.StatusOK),
	"checkSisConnectionHealth": sisContractOp("checkSisConnectionHealth", http.MethodPost, "/sis/connections/{connection_id}:health-check",
		httpx.TenantOptional, httpx.IdempotencyNone, http.StatusOK),
}

// RegisterSisContract mounts the F017 operations.
func RegisterSisContract(s *httpx.Server, h SisContractHandlers) error {
	if h.Connections == nil || h.Sync == nil {
		return errors.New("identity: SIS contract_v1 operations need the connection and sync use cases")
	}
	routes := map[string]http.HandlerFunc{
		"triggerSisSync":           h.triggerSisSync,
		"updateSisConnection":      h.updateSisConnection,
		"checkSisConnectionHealth": h.checkSisConnectionHealth,
	}
	for id, fn := range routes {
		o, ok := SisContractOperations[id]
		if !ok {
			return problem.New(problem.Internal, "no operation "+id)
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

func (h SisContractHandlers) triggerSisSync(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "connection_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Kind     string     `json:"kind"`
		SchoolID *uuid.UUID `json:"school_id"`
		DryRun   bool       `json:"dry_run"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Sync.Trigger(r.Context(), id, command.TriggerInput{Kind: b.Kind, SchoolID: b.SchoolID, DryRun: b.DryRun})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h SisContractHandlers) updateSisConnection(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "connection_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	version, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	// JSON Merge Patch: an absent key is unchanged, null clears webhook_secret_ref/schedule.
	var raw map[string]any
	if err := decode(r, &raw); err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Config           *sis.ContractConfig `json:"config"`
		SecretRef        *string             `json:"secret_ref"`
		WebhookSecretRef *string             `json:"webhook_secret_ref"`
		Schedule         *string             `json:"schedule"`
		Status           *string             `json:"status"`
		Reason           string              `json:"reason"`
	}
	if err := remarshal(raw, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := command.ContractPatch{Config: b.Config, SecretRef: b.SecretRef, WebhookSecretRef: b.WebhookSecretRef, Schedule: b.Schedule,
		Status: b.Status, Reason: b.Reason}
	empty := ""
	if v, ok := raw["webhook_secret_ref"]; ok && v == nil {
		in.WebhookSecretRef = &empty
	}
	if v, ok := raw["schedule"]; ok && v == nil {
		in.Schedule = &empty
	}
	out, err := h.Connections.Update(r.Context(), id, version, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, out)
}

func (h SisContractHandlers) checkSisConnectionHealth(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "connection_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Connections.CheckHealth(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// remarshal converts a decoded merge patch into its typed form.
func remarshal(raw map[string]any, v any) error {
	b, err := json.Marshal(raw)
	if err != nil || json.Unmarshal(b, v) != nil {
		return problem.New(problem.BadRequest, "invalid JSON body")
	}
	return nil
}
