package http

import (
	"errors"
	"net/http"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// LifecycleHandlers are the account lifecycle use cases of F016 served over HTTP.
type LifecycleHandlers struct {
	Lifecycle     command.UserLifecycle
	ResetPassword command.ResetStudentPassword
}

func lifecycleOp(id, method, pattern string, tenant httpx.TenantMode, status int) httpx.Operation {
	return portalOp2(id, method, pattern, tenant, httpx.IdempotencyRequired, status)
}

// LifecycleOperations are the F016 operations; the contract test checks them against the merged spec.
var LifecycleOperations = map[string]httpx.Operation{
	"lockUser":             lifecycleOp("lockUser", http.MethodPost, "/admin/users/{user_id}:lock", httpx.TenantOptional, 204),
	"unlockUser":           lifecycleOp("unlockUser", http.MethodPost, "/admin/users/{user_id}:unlock", httpx.TenantOptional, 204),
	"deactivateUser":       lifecycleOp("deactivateUser", http.MethodPost, "/admin/users/{user_id}:deactivate", httpx.TenantOptional, 204),
	"lockTenantUser":       lifecycleOp("lockTenantUser", http.MethodPost, "/tenant/users/{user_id}:lock", httpx.TenantRequired, 204),
	"unlockTenantUser":     lifecycleOp("unlockTenantUser", http.MethodPost, "/tenant/users/{user_id}:unlock", httpx.TenantRequired, 204),
	"resetStudentPassword": lifecycleOp("resetStudentPassword", http.MethodPost, "/tenant/students/{student_id}/password:reset", httpx.TenantRequired, 200),
}

// RegisterLifecycle mounts the F016 operations.
func RegisterLifecycle(s *httpx.Server, h LifecycleHandlers) error {
	change := func(route, action string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { h.change(w, r, route, action) }
	}
	routes := map[string]http.HandlerFunc{
		"lockUser":             change(command.LifecycleRoutePlatform, command.LifecycleLock),
		"unlockUser":           change(command.LifecycleRoutePlatform, command.LifecycleUnlock),
		"deactivateUser":       change(command.LifecycleRoutePlatform, command.LifecycleDeactivate),
		"lockTenantUser":       change(command.LifecycleRouteTenant, command.LifecycleLock),
		"unlockTenantUser":     change(command.LifecycleRouteTenant, command.LifecycleUnlock),
		"resetStudentPassword": h.resetStudentPassword,
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
