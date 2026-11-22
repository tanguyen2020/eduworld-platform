package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LifecycleHandlers are the account lifecycle use cases of F016 served over HTTP.
type LifecycleHandlers struct {
	Lifecycle     command.UserLifecycle
	ResetPassword command.ResetStudentPassword
	Cleanup       command.StudentCleanup
}

func lifecycleOp(id, method, pattern string, tenant httpx.TenantMode, status int) httpx.Operation {
	return portalOp2(id, method, pattern, tenant, httpx.IdempotencyRequired, status)
}

// LifecycleOperations are the F016 operations; the contract test checks them against the merged spec.
var LifecycleOperations = map[string]httpx.Operation{
	"lockUser":                       lifecycleOp("lockUser", http.MethodPost, "/admin/users/{user_id}:lock", httpx.TenantOptional, 204),
	"unlockUser":                     lifecycleOp("unlockUser", http.MethodPost, "/admin/users/{user_id}:unlock", httpx.TenantOptional, 204),
	"deactivateUser":                 lifecycleOp("deactivateUser", http.MethodPost, "/admin/users/{user_id}:deactivate", httpx.TenantOptional, 204),
	"lockTenantUser":                 lifecycleOp("lockTenantUser", http.MethodPost, "/tenant/users/{user_id}:lock", httpx.TenantRequired, 204),
	"unlockTenantUser":               lifecycleOp("unlockTenantUser", http.MethodPost, "/tenant/users/{user_id}:unlock", httpx.TenantRequired, 204),
	"resetStudentPassword":           lifecycleOp("resetStudentPassword", http.MethodPost, "/tenant/students/{student_id}/password:reset", httpx.TenantRequired, 200),
	"previewTenantStudentCleanup":    portalOp2("previewTenantStudentCleanup", http.MethodGet, "/tenant/student-cleanup/preview", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"createStudentCleanupExclusions": lifecycleOp("createStudentCleanupExclusions", http.MethodPost, "/tenant/student-cleanup/exclusions", httpx.TenantRequired, 204),
	"deleteStudentCleanupExclusions": portalOp2("deleteStudentCleanupExclusions", http.MethodDelete, "/tenant/student-cleanup/exclusions", httpx.TenantRequired, httpx.IdempotencyNone, 204),
	"listStudentCleanupRuns":         portalOp2("listStudentCleanupRuns", http.MethodGet, "/admin/student-cleanup/runs", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"approveStudentCleanupRun":       lifecycleOp("approveStudentCleanupRun", http.MethodPost, "/admin/student-cleanup/runs/{run_id}:approve", httpx.TenantOptional, 202),
	"cancelStudentCleanupRun":        lifecycleOp("cancelStudentCleanupRun", http.MethodPost, "/admin/student-cleanup/runs/{run_id}:cancel", httpx.TenantOptional, 204),
}

// RegisterLifecycle mounts the F016 operations.
func RegisterLifecycle(s *httpx.Server, h LifecycleHandlers) error {
	change := func(route, action string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { h.change(w, r, route, action) }
	}
	routes := map[string]http.HandlerFunc{
		"lockUser":                       change(command.LifecycleRoutePlatform, command.LifecycleLock),
		"unlockUser":                     change(command.LifecycleRoutePlatform, command.LifecycleUnlock),
		"deactivateUser":                 change(command.LifecycleRoutePlatform, command.LifecycleDeactivate),
		"lockTenantUser":                 change(command.LifecycleRouteTenant, command.LifecycleLock),
		"unlockTenantUser":               change(command.LifecycleRouteTenant, command.LifecycleUnlock),
		"resetStudentPassword":           h.resetStudentPassword,
		"previewTenantStudentCleanup":    h.previewTenantStudentCleanup,
		"createStudentCleanupExclusions": h.exclusions(false),
		"deleteStudentCleanupExclusions": h.exclusions(true),
		"listStudentCleanupRuns":         h.listStudentCleanupRuns,
		"approveStudentCleanupRun":       h.approveStudentCleanupRun,
		"cancelStudentCleanupRun":        h.cancelStudentCleanupRun,
	}
	for id, fn := range routes {
		o, ok := LifecycleOperations[id]
		if !ok {
			return errors.New("identity: no operation " + id)
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

func (h LifecycleHandlers) change(w http.ResponseWriter, r *http.Request, route, action string) {
	id, err := pathUUID(r, "user_id")
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
	if err := h.Lifecycle.Change(r.Context(), route, action, id, b.Reason); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h LifecycleHandlers) resetStudentPassword(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "student_id")
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
	out, err := h.ResetPassword.Handle(r.Context(), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store") // the password is shown once (F016-AC-25)
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, out)
}

func (h LifecycleHandlers) previewTenantStudentCleanup(w http.ResponseWriter, r *http.Request) {
	var cursor *page.Cursor
	if c := r.URL.Query().Get("cursor"); c != "" && h.Cleanup.Cursors != nil {
		v, err := h.Cleanup.Cursors.Decode(c)
		if err != nil {
			fail(w, r, problem.New(problem.BadRequest, "invalid cursor"))
			return
		}
		cursor = v
	}
	out, err := h.Cleanup.Preview(r.Context(), queryInt(r, "limit"), cursor)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h LifecycleHandlers) exclusions(remove bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			StudentIDs []uuid.UUID `json:"student_ids"`
			Until      *time.Time  `json:"until"`
			Reason     string      `json:"reason"`
		}
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
		if err := h.Cleanup.Exclude(r.Context(), b.StudentIDs, b.Until, b.Reason, remove); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type cleanupRunView struct {
	RunID         uuid.UUID  `json:"run_id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	Status        string     `json:"status"`
	Candidates    int        `json:"candidates"`
	Cleaned       int        `json:"cleaned"`
	Excluded      int        `json:"excluded"`
	SkippedActive int        `json:"skipped_active"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
}

func (h LifecycleHandlers) listStudentCleanupRuns(w http.ResponseWriter, r *http.Request) {
	var cursor *page.Cursor
	if c := r.URL.Query().Get("cursor"); c != "" && h.Cleanup.Cursors != nil {
		v, err := h.Cleanup.Cursors.Decode(c)
		if err != nil {
			fail(w, r, problem.New(problem.BadRequest, "invalid cursor"))
			return
		}
		cursor = v
	}
	res, err := h.Cleanup.ListRuns(r.Context(), queryInt(r, "limit"), cursor)
	if err != nil {
		fail(w, r, err)
		return
	}
	out := struct {
		Data []cleanupRunView `json:"data"`
		Page page.Info        `json:"page"`
	}{Data: []cleanupRunView{}, Page: res.Page}
	for _, x := range res.Runs {
		out.Data = append(out.Data, cleanupRunView{RunID: x.ID, TenantID: x.TenantID, Status: x.Status, Candidates: x.Candidates,
			Cleaned: x.Cleaned, Excluded: x.Excluded, SkippedActive: x.SkippedActive, StartedAt: x.StartedAt, FinishedAt: x.FinishedAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h LifecycleHandlers) approveStudentCleanupRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "run_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if _, err := h.Cleanup.Approve(r.Context(), id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h LifecycleHandlers) cancelStudentCleanupRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "run_id")
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
	if _, err := h.Cleanup.Cancel(r.Context(), id, b.Reason); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
