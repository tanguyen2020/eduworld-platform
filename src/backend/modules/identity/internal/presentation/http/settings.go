package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SettingsHandlers are the use cases of F015 served over HTTP: audit log queries, platform/tenant/school
// settings, feature flags and the portal menu catalog. They are registered apart from Operations so the
// features of the module evolve in separate files.
type SettingsHandlers struct {
	AuditLogs query.AuditLogs
}

// SettingsOperations are the F015 operations; the contract test checks them against the merged spec.
var SettingsOperations = map[string]httpx.Operation{
	"listTenantAuditLogs":   portalOp2("listTenantAuditLogs", http.MethodGet, "/tenant/audit-logs", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listPlatformAuditLogs": portalOp2("listPlatformAuditLogs", http.MethodGet, "/admin/audit-logs", httpx.TenantNone, httpx.IdempotencyNone, 200),
}

// RegisterSettings mounts the F015 operations.
func RegisterSettings(s *httpx.Server, h SettingsHandlers) error {
	routes := map[string]http.HandlerFunc{
		"listTenantAuditLogs":   h.listTenantAuditLogs,
		"listPlatformAuditLogs": h.listPlatformAuditLogs,
	}
	for id, fn := range routes {
		o, ok := SettingsOperations[id]
		if !ok {
			return errors.New("identity: no operation " + id)
		}
		if err := s.Handle(o, fn); err != nil {
			return err
		}
	}
	return nil
}

func queryTime(r *http.Request, name string) (time.Time, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, problem.Invalid(name, problem.SubInvalid, "must be a date-time")
	}
	return t, nil
}

func auditInput(r *http.Request) (query.AuditLogsInput, error) {
	var in query.AuditLogsInput
	var err error
	if in.SchoolID, err = optUUID(r, "school_id"); err != nil {
		return in, err
	}
	if in.ActorID, err = optUUID(r, "actor_id"); err != nil {
		return in, err
	}
	if in.From, err = queryTime(r, "from"); err != nil {
		return in, err
	}
	if in.To, err = queryTime(r, "to"); err != nil {
		return in, err
	}
	q := r.URL.Query()
	in.Action, in.ResourceType, in.ResourceID = q.Get("action"), q.Get("resource_type"), q.Get("resource_id")
	in.Limit, in.Cursor = queryInt(r, "limit"), q.Get("cursor")
	return in, nil
}

func (h SettingsHandlers) listTenantAuditLogs(w http.ResponseWriter, r *http.Request) {
	in, err := auditInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.AuditLogs.ListTenant(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h SettingsHandlers) listPlatformAuditLogs(w http.ResponseWriter, r *http.Request) {
	in, err := auditInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.AuditLogs.ListPlatform(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// portalOp2 is an operation of the Admin Portal: portal_staff guard (F012-AC-05); /admin/* routes also get
// platform_mfa from httpx.
func portalOp2(id, method, pattern string, tenant httpx.TenantMode, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return withGuard(op(id, method, pattern, tenant, idem, status), httpx.GuardPortalStaff)
}
