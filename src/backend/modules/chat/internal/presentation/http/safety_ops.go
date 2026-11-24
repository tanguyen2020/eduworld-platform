package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// SafetyOpsCommands are the write use cases of the Safety Team operations (command.Safety, F124).
type SafetyOpsCommands interface {
	CreateOncallShift(ctx context.Context, in command.ShiftInput) (read.OncallShiftRow, error)
	DeleteOncallShift(ctx context.Context, id uuid.UUID) error
	RequestBreakGlass(ctx context.Context, in command.BreakGlassInput) (read.GrantRow, error)
	DecideBreakGlass(ctx context.Context, id uuid.UUID, version int64, approve bool, note *string) (read.GrantRow, error)
	RevokeBreakGlass(ctx context.Context, id uuid.UUID, reason string) (read.GrantRow, error)
	RecordBreakGlassView(ctx context.Context, g read.GrantRow, n int) error
}

// SafetyOpsQueries are the read use cases of the Safety Team operations (query.SafetyOps, F124).
type SafetyOpsQueries interface {
	ListOncallShifts(ctx context.Context, from, to *time.Time, role string, limit int, cursor string) (query.ShiftPage, error)
	ListBreakGlassGrants(ctx context.Context, tenants []uuid.UUID, status string, limit int, cursor string) (query.GrantPage, error)
	ListBreakGlassMessages(ctx context.Context, id uuid.UUID, limit int, cursor string) (query.BreakGlassMessagePage, error)
}

// SafetyOpsHandlers serve the on-call rotation and the break-glass operations (core-api): /admin/moderation/oncall-shifts*
// and /admin/break-glass/* (internal gateway: httpx adds GuardPortalStaff + GuardPlatformRole; the use cases require
// moderation.case.read/lead and platform.break_glass.request/approve in Go, R7, R20).
type SafetyOpsHandlers struct {
	Commands SafetyOpsCommands
	Queries  SafetyOpsQueries
}

// SafetyOpsOperations are the F124 operations (checked against the spec by the contract test).
var SafetyOpsOperations = []string{"listModerationOncallShifts", "createModerationOncallShift", "deleteModerationOncallShift",
	"listBreakGlassGrants", "createBreakGlassRequest", "approveBreakGlassRequest", "rejectBreakGlassRequest", "revokeBreakGlassGrant",
	"listBreakGlassMessages"}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "listModerationOncallShifts", Method: http.MethodGet, Pattern: "/admin/moderation/oncall-shifts", Tenant: httpx.TenantNone,
			SuccessStatus: http.StatusOK},
		{ID: "createModerationOncallShift", Method: http.MethodPost, Pattern: "/admin/moderation/oncall-shifts", Tenant: httpx.TenantNone,
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "deleteModerationOncallShift", Method: http.MethodDelete, Pattern: "/admin/moderation/oncall-shifts/{shift_id}",
			Tenant: httpx.TenantNone, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusNoContent},
		{ID: "listBreakGlassGrants", Method: http.MethodGet, Pattern: "/admin/break-glass/grants", Tenant: httpx.TenantNone,
			SuccessStatus: http.StatusOK},
		{ID: "createBreakGlassRequest", Method: http.MethodPost, Pattern: "/admin/break-glass/grants", Tenant: httpx.TenantRequired,
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "approveBreakGlassRequest", Method: http.MethodPost, Pattern: "/admin/break-glass/grants/{grant_id}:approve",
			Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "rejectBreakGlassRequest", Method: http.MethodPost, Pattern: "/admin/break-glass/grants/{grant_id}:reject",
			Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "revokeBreakGlassGrant", Method: http.MethodPost, Pattern: "/admin/break-glass/grants/{grant_id}:revoke",
			Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "listBreakGlassMessages", Method: http.MethodGet, Pattern: "/admin/break-glass/grants/{grant_id}/messages",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
	} {
		o.Module = "chat"
		Operations[o.ID] = o
	}
}

// registerSafetyOps mounts the F124 operations.
func registerSafetyOps(s *httpx.Server, h *SafetyOpsHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the safety operations handlers are required")
	}
	routes := map[string]http.HandlerFunc{
		"listModerationOncallShifts":  h.listShifts,
		"createModerationOncallShift": h.createShift,
		"deleteModerationOncallShift": h.deleteShift,
		"listBreakGlassGrants":        h.listGrants,
		"createBreakGlassRequest":     h.requestGlass,
		"approveBreakGlassRequest":    h.decide(true),
		"rejectBreakGlassRequest":     h.decide(false),
		"revokeBreakGlassGrant":       h.revokeGlass,
		"listBreakGlassMessages":      h.glassMessages,
	}
	for _, id := range SafetyOpsOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func (h SafetyOpsHandlers) listShifts(w http.ResponseWriter, r *http.Request) {
	from, err := optTime(r, "from")
	if err != nil {
		fail(w, r, err)
		return
	}
	to, err := optTime(r, "to")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListOncallShifts(r.Context(), from, to, r.URL.Query().Get("oncall_role"), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, s := range p.Data {
		data[i] = service.ShiftBody(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h SafetyOpsHandlers) createShift(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ModeratorUserID uuid.UUID `json:"moderator_user_id"`
		OncallRole      string    `json:"oncall_role"`
		StartsAt        time.Time `json:"starts_at"`
		EndsAt          time.Time `json:"ends_at"`
		Note            *string   `json:"note"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.CreateOncallShift(r.Context(), command.ShiftInput{Moderator: b.ModeratorUserID, Role: b.OncallRole,
		Start: b.StartsAt, End: b.EndsAt, Note: b.Note})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, service.ShiftBody(s))
}

func (h SafetyOpsHandlers) deleteShift(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "shift_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Commands.DeleteOncallShift(r.Context(), id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h SafetyOpsHandlers) listGrants(w http.ResponseWriter, r *http.Request) {
	tenants, err := tenantIDs(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListBreakGlassGrants(r.Context(), tenants, r.URL.Query().Get("status"), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, g := range p.Data {
		data[i] = service.GrantBody(g)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h SafetyOpsHandlers) requestGlass(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ScopeType       string    `json:"scope_type"`
		ScopeID         uuid.UUID `json:"scope_id"`
		WindowFrom      time.Time `json:"window_from"`
		WindowTo        time.Time `json:"window_to"`
		DurationMinutes int       `json:"duration_minutes"`
		Reason          string    `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	g, err := h.Commands.RequestBreakGlass(r.Context(), command.BreakGlassInput{ScopeType: b.ScopeType, ScopeID: b.ScopeID,
		WindowFrom: b.WindowFrom, WindowTo: b.WindowTo, DurationMinutes: b.DurationMinutes, Reason: b.Reason})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, g.Version)
	writeJSON(w, http.StatusCreated, service.GrantBody(g))
}

func (h SafetyOpsHandlers) decide(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "grant_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		version, err := ifMatch(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var b struct {
			DecisionNote *string `json:"decision_note"`
		}
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
		g, err := h.Commands.DecideBreakGlass(r.Context(), id, version, approve, b.DecisionNote)
		if err != nil {
			fail(w, r, err)
			return
		}
		etag(w, g.Version)
		writeJSON(w, http.StatusOK, service.GrantBody(g))
	}
}

func (h SafetyOpsHandlers) revokeGlass(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "grant_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	g, err := h.Commands.RevokeBreakGlass(r.Context(), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, g.Version)
	writeJSON(w, http.StatusOK, service.GrantBody(g))
}

// glassMessages reads one page of the messages of a grant, then records the view (count + audit) before answering: a
// read that cannot be audited is not served (F124-AC-19).
func (h SafetyOpsHandlers) glassMessages(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "grant_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListBreakGlassMessages(r.Context(), id, limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Commands.RecordBreakGlassView(r.Context(), p.Grant, len(p.Data)); err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, m := range p.Data {
		data[i] = service.BreakGlassMessageBody(m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}
