package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SettingsHandlers are the use cases of F015 served over HTTP: audit log queries, platform/tenant/school
// settings, feature flags and the portal menu catalog. They are registered apart from Operations so the
// features of the module evolve in separate files.
type SettingsHandlers struct {
	AuditLogs   query.AuditLogs
	Settings    command.Settings
	SettingsGet query.Settings
}

// SettingsOperations are the F015 operations; the contract test checks them against the merged spec.
var SettingsOperations = map[string]httpx.Operation{
	"listTenantAuditLogs":       portalOp2("listTenantAuditLogs", http.MethodGet, "/tenant/audit-logs", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"listPlatformAuditLogs":     portalOp2("listPlatformAuditLogs", http.MethodGet, "/admin/audit-logs", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"getStudentCleanupSettings": portalOp2("getStudentCleanupSettings", http.MethodGet, "/admin/settings/student-cleanup", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"putStudentCleanupSettings": portalOp2("putStudentCleanupSettings", http.MethodPut, "/admin/settings/student-cleanup", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"getChatRetentionSettings":  portalOp2("getChatRetentionSettings", http.MethodGet, "/admin/settings/chat-retention", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"putChatRetentionSettings":  portalOp2("putChatRetentionSettings", http.MethodPut, "/admin/settings/chat-retention", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"getPlatformCurfewSettings": portalOp2("getPlatformCurfewSettings", http.MethodGet, "/admin/settings/curfew", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"putPlatformCurfewSettings": portalOp2("putPlatformCurfewSettings", http.MethodPut, "/admin/settings/curfew", httpx.TenantNone, httpx.IdempotencyNone, 200),
	"getTenantSettings":         portalOp2("getTenantSettings", http.MethodGet, "/tenant/settings", httpx.TenantRequired, httpx.IdempotencyNone, 200),
	"updateTenantSettings":      portalOp2("updateTenantSettings", http.MethodPatch, "/tenant/settings", httpx.TenantRequired, httpx.IdempotencyNone, 200),
}

// RegisterSettings mounts the F015 operations.
func RegisterSettings(s *httpx.Server, h SettingsHandlers) error {
	routes := map[string]http.HandlerFunc{
		"listTenantAuditLogs":       h.listTenantAuditLogs,
		"listPlatformAuditLogs":     h.listPlatformAuditLogs,
		"getStudentCleanupSettings": h.getStudentCleanupSettings,
		"putStudentCleanupSettings": h.putStudentCleanupSettings,
		"getChatRetentionSettings":  h.getChatRetentionSettings,
		"putChatRetentionSettings":  h.putChatRetentionSettings,
		"getPlatformCurfewSettings": h.getPlatformCurfewSettings,
		"putPlatformCurfewSettings": h.putPlatformCurfewSettings,
		"getTenantSettings":         h.getTenantSettings,
		"updateTenantSettings":      h.updateTenantSettings,
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

// settingsVersion parses If-Match for settings: version 0 (W/"0") is the ETag of a key not configured yet;
// missing → 428 PRECONDITION_REQUIRED, malformed → 412 VERSION_MISMATCH.
func settingsVersion(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return 0, problem.New(problem.PreconditionRequired, "")
	}
	n, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(raw, "W/"), `"`), 10, 64)
	if err != nil || n < 0 {
		return 0, problem.New(problem.VersionMismatch, "")
	}
	return n, nil
}

func writeVersioned(w http.ResponseWriter, version int64, v any) {
	etag(w, version)
	writeJSON(w, http.StatusOK, v)
}

func (h SettingsHandlers) getStudentCleanupSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.SettingsGet.StudentCleanup(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersioned(w, out.Version, out)
}

func (h SettingsHandlers) putStudentCleanupSettings(w http.ResponseWriter, r *http.Request) {
	v, err := settingsVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		settings.StudentCleanup
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Settings.PutStudentCleanup(r.Context(), b.StudentCleanup, b.Reason, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersioned(w, out.Version, out)
}

func (h SettingsHandlers) getChatRetentionSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.SettingsGet.ChatRetention(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersioned(w, out.Version, out)
}

func (h SettingsHandlers) putChatRetentionSettings(w http.ResponseWriter, r *http.Request) {
	v, err := settingsVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		settings.ChatRetention
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Settings.PutChatRetention(r.Context(), b.ChatRetention, b.Reason, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersioned(w, out.Version, out)
}

func (h SettingsHandlers) getPlatformCurfewSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.SettingsGet.PlatformCurfew(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersioned(w, out.Version, out)
}

func (h SettingsHandlers) putPlatformCurfewSettings(w http.ResponseWriter, r *http.Request) {
	v, err := settingsVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		settings.Curfew
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Settings.PutPlatformCurfew(r.Context(), b.Curfew, b.Reason, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersioned(w, out.Version, out)
}

func (h SettingsHandlers) getTenantSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.SettingsGet.TenantSettings(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersioned(w, out.Version, out)
}

func (h SettingsHandlers) updateTenantSettings(w http.ResponseWriter, r *http.Request) {
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var patch map[string]json.RawMessage
	if err := decode(r, &patch); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Settings.UpdateTenantSettings(r.Context(), patch, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeVersioned(w, out.Version, out)
}
