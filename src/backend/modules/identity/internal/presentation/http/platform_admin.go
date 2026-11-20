package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PlatformAdminHandlers are the use cases of F045 (Platform Administration) served over HTTP; registered
// apart from Operations so the features of the module evolve in separate files.
type PlatformAdminHandlers struct {
	Reads    query.PlatformAdmin
	Assign   command.AssignToTenants
	Unassign command.UnassignFromTenants
	Restore  command.RestoreTrashItem
	Clone    command.CreateTenantClone
	Merge    command.MergeUsers
}

// PlatformAdminOperations are the F045 operations; the contract test checks them against the merged spec.
// /admin/* also gets platform_mfa from httpx (internal gateway, azp eduworld-portal, MFA, F045-AC-04).
var PlatformAdminOperations = map[string]httpx.Operation{
	"listTrash":              portalOp2("listTrash", http.MethodGet, "/admin/trash", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"restoreTrashItem":       portalOp2("restoreTrashItem", http.MethodPost, "/admin/trash/{resource_type}/{resource_id}:restore", httpx.TenantOptional, httpx.IdempotencyRequired, 204),
	"listTenantAssignments":  portalOp2("listTenantAssignments", http.MethodGet, "/admin/tenant-assignments", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"assignToTenants":        portalOp2("assignToTenants", http.MethodPost, "/admin/tenant-assignments", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"unassignFromTenants":    portalOp2("unassignFromTenants", http.MethodDelete, "/admin/tenant-assignments", httpx.TenantOptional, httpx.IdempotencyRequired, 204),
	"createTenantClone":      portalOp2("createTenantClone", http.MethodPost, "/admin/tenant-clones", httpx.TenantOptional, httpx.IdempotencyRequired, 202),
	"getTenantClone":         portalOp2("getTenantClone", http.MethodGet, "/admin/tenant-clones/{job_id}", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"mergeUsers":             portalOp2("mergeUsers", http.MethodPost, "/admin/users:merge", httpx.TenantRequired, httpx.IdempotencyRequired, 202),
	"getUserMerge":           portalOp2("getUserMerge", http.MethodGet, "/admin/user-merges/{merge_id}", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"listTenantTrash":        portalOp2("listTenantTrash", http.MethodGet, "/tenant/trash", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"restoreTenantTrashItem": portalOp2("restoreTenantTrashItem", http.MethodPost, "/tenant/trash/{resource_type}/{resource_id}:restore", httpx.TenantRequired, httpx.IdempotencyRequired, 204),
}

// RegisterPlatformAdmin mounts the F045 operations.
func RegisterPlatformAdmin(s *httpx.Server, h PlatformAdminHandlers) error {
	routes := map[string]http.HandlerFunc{
		"listTrash":              h.listTrash,
		"restoreTrashItem":       h.restoreTrashItem,
		"listTenantAssignments":  h.listTenantAssignments,
		"assignToTenants":        h.assignToTenants,
		"unassignFromTenants":    h.unassignFromTenants,
		"createTenantClone":      h.createTenantClone,
		"getTenantClone":         h.getTenantClone,
		"mergeUsers":             h.mergeUsers,
		"getUserMerge":           h.getUserMerge,
		"listTenantTrash":        h.listTenantTrash,
		"restoreTenantTrashItem": h.restoreTenantTrashItem,
	}
	for id, fn := range routes {
		o, ok := PlatformAdminOperations[id]
		if !ok {
			return errors.New("identity: no operation " + id)
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

// withIdemKey carries the Idempotency-Key of the request to the facade calls of the use case (R29).
func withIdemKey(r *http.Request) *http.Request {
	if k := r.Header.Get("Idempotency-Key"); k != "" {
		return r.WithContext(modcall.WithIdempotencyKey(r.Context(), k))
	}
	return r
}

func uuidsParam(r *http.Request, name string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, v := range r.URL.Query()[name] {
		id, err := uuid.Parse(v)
		if err != nil {
			return nil, problem.Invalid(name, problem.SubInvalid, "must be UUIDs")
		}
		out = append(out, id)
	}
	return out, nil
}

func trashInput(r *http.Request) query.TrashInput {
	q := r.URL.Query()
	return query.TrashInput{ResourceType: q.Get("resource_type"), Cursor: q.Get("cursor"), Limit: queryInt(r, "limit")}
}

func (h PlatformAdminHandlers) listTrash(w http.ResponseWriter, r *http.Request) {
	in := trashInput(r)
	var err error
	if in.TenantIDs, err = uuidsParam(r, "tenant_ids"); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Reads.ListTrash(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h PlatformAdminHandlers) listTenantTrash(w http.ResponseWriter, r *http.Request) {
	out, err := h.Reads.ListTenantTrash(r.Context(), trashInput(r)) // tenant_ids is ignored (F045-AC-02)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type restoreBody struct {
	Reason string `json:"reason"`
}

func restoreInput(r *http.Request) (command.RestoreInput, error) {
	var b restoreBody
	if err := decode(r, &b); err != nil {
		return command.RestoreInput{}, err
	}
	id, err := pathUUID(r, "resource_id")
	if err != nil {
		return command.RestoreInput{}, err
	}
	return command.RestoreInput{ResourceType: chi.URLParam(r, "resource_type"), ID: id, Reason: b.Reason}, nil
}

func (h PlatformAdminHandlers) restoreTrashItem(w http.ResponseWriter, r *http.Request) {
	r = withIdemKey(r)
	in, err := restoreInput(r)
	if err == nil {
		err = h.Restore.Admin(r.Context(), in)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h PlatformAdminHandlers) restoreTenantTrashItem(w http.ResponseWriter, r *http.Request) {
	r = withIdemKey(r)
	in, err := restoreInput(r)
	if err == nil {
		err = h.Restore.Tenant(r.Context(), in)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h PlatformAdminHandlers) listTenantAssignments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id, err := uuid.Parse(q.Get("resource_id"))
	if err != nil {
		fail(w, r, problem.Invalid("resource_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	out, err := h.Reads.ListTenantAssignments(r.Context(), query.AssignmentsInput{ResourceType: q.Get("resource_type"), ResourceID: id,
		Cursor: q.Get("cursor"), Limit: queryInt(r, "limit")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type assignBody struct {
	ResourceType      string      `json:"resource_type"`
	ResourceID        uuid.UUID   `json:"resource_id"`
	TenantIDs         []uuid.UUID `json:"tenant_ids"`
	ApplyToAllTenants bool        `json:"apply_to_all_tenants"`
	ApplyToNewTenants bool        `json:"apply_to_new_tenants"`
	Reason            string      `json:"reason"`
	RemoveDefault     bool        `json:"remove_default"` // unassignFromTenants only (OQ F045-9)
}

func (h PlatformAdminHandlers) assignToTenants(w http.ResponseWriter, r *http.Request) {
	r = withIdemKey(r)
	var b assignBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Assign.Handle(r.Context(), command.AssignInput{ResourceType: b.ResourceType, ResourceID: b.ResourceID, TenantIDs: b.TenantIDs,
		ApplyToAll: b.ApplyToAllTenants, ApplyToNew: b.ApplyToNewTenants})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h PlatformAdminHandlers) unassignFromTenants(w http.ResponseWriter, r *http.Request) {
	r = withIdemKey(r)
	var b assignBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Unassign.Handle(r.Context(), command.UnassignInput{ResourceType: b.ResourceType, ResourceID: b.ResourceID, TenantIDs: b.TenantIDs,
		Reason: b.Reason, RemoveDefault: b.RemoveDefault}); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type cloneBody struct {
	SourceTenantID uuid.UUID   `json:"source_tenant_id"`
	TargetTenantID uuid.UUID   `json:"target_tenant_id"`
	ResourceType   string      `json:"resource_type"`
	ResourceIDs    []uuid.UUID `json:"resource_ids"`
}

func (h PlatformAdminHandlers) createTenantClone(w http.ResponseWriter, r *http.Request) {
	var b cloneBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Clone.Handle(r.Context(), command.CloneInput{Source: b.SourceTenantID, Target: b.TargetTenantID, ResourceType: b.ResourceType,
		ResourceIDs: b.ResourceIDs, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h PlatformAdminHandlers) getTenantClone(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "job_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Reads.GetTenantClone(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type mergeBody struct {
	FromUserID            uuid.UUID `json:"from_user_id"`
	IntoUserID            uuid.UUID `json:"into_user_id"`
	ReconciliationIssueID uuid.UUID `json:"reconciliation_issue_id"`
	Reason                string    `json:"reason"`
}

func (h PlatformAdminHandlers) mergeUsers(w http.ResponseWriter, r *http.Request) {
	var b mergeBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Merge.Handle(r.Context(), command.MergeInput{From: b.FromUserID, Into: b.IntoUserID, IssueID: b.ReconciliationIssueID,
		Reason: b.Reason, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h PlatformAdminHandlers) getUserMerge(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "merge_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Reads.GetUserMerge(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
