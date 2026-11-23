//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// noRole registers portal staff of tenant without any permission and returns its token.
func (e *env) noRole(tenant uuid.UUID) string {
	sub := "kc-norole-" + uuid.NewString()
	e.res.Set(sub, authztest.NoRole(tenant).PrincipalData)
	return e.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

// F122 T005 — F122-AC-13, F122-AC-14 (TC-CHAT-016, TC-TEN-001): getTenantModerationStatus gives the staff of a tenant
// (tenant.settings.read or school.settings.read) the steps not in force for their tenant and the earliest time they come
// back — never the reason or who changed it; another tenant sees nothing of it; staff without either permission and
// another tenant's X-Tenant-Id are refused; a lapsed expires_at counts as on.
func TestTenantModerationStatus(t *testing.T) {
	authztest.Cover(t, "getTenantModerationStatus", authztest.CrossTenant, authztest.RoleDenied)

	c := setupChat(t, chatOpts{real: true})
	const secret = "integration test switch" // the reason stored by tenantSwitch
	status := func(token string, tenant uuid.UUID) resp {
		t.Helper()
		return c.do(http.MethodGet, "/tenant/chat/moderation-status", token, map[string]string{"X-Tenant-Id": tenant.String()}, nil)
	}
	// Nothing off: every step runs.
	r := status(c.staff(c.tenantA, "tenant_admin"), c.tenantA)
	if r.status != http.StatusOK || r.body["enabled_all"] != true || len(disabledSteps(r)) != 0 || r.body["expires_at"] != nil ||
		r.body["source_scope"] != nil || r.body["tenant_id"] != c.tenantA.String() {
		t.Fatalf("all on: %d %s", r.status, r.raw)
	}
	// Tenant A turns spam off for an hour (approved switch stored directly; the US3 commands have their own tests).
	c.tenantSwitch(c.tenantA, true, map[string]bool{"spam": false})
	exp := c.clock.Now().Add(time.Hour).UTC()
	for _, role := range []string{"tenant_admin", "school_admin"} {
		r = status(c.staff(c.tenantA, role), c.tenantA)
		got, _ := time.Parse(time.RFC3339Nano, str(r.body, "expires_at"))
		if r.status != http.StatusOK || r.body["enabled_all"] != true || strings.Join(disabledSteps(r), ",") != "spam" ||
			r.body["source_scope"] != "tenant" || !got.Equal(exp.Truncate(time.Microsecond)) {
			t.Fatalf("%s status: %d %s", role, r.status, r.raw)
		}
		for _, leak := range []string{secret, "reason", "updated_by", "actor", "pending"} {
			if strings.Contains(r.raw, leak) {
				t.Fatalf("%s status leaks %q: %s", role, leak, r.raw)
			}
		}
	}
	// Tenant B is not concerned (F122-AC-14).
	if r = status(c.staff(c.tenantB, "tenant_admin"), c.tenantB); r.status != http.StatusOK || len(disabledSteps(r)) != 0 ||
		r.body["source_scope"] != nil {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	// Cross-tenant: tenant A staff naming tenant B → refused (TENANT_MISMATCH / NOT_FOUND), nothing of B returned.
	if r = status(c.staff(c.tenantA, "tenant_admin"), c.tenantB); r.status != http.StatusForbidden && r.status != http.StatusNotFound {
		t.Fatalf("cross-tenant: %d %s", r.status, r.raw)
	}
	// No permission → 403 PERMISSION_DENIED; a game token never reaches the portal route.
	if r = status(c.noRole(c.tenantA), c.tenantA); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("no role: %d %s", r.status, r.raw)
	}
	an := c.student(c.tenantA, c.schoolA)
	if r = c.call(an, http.MethodGet, "/tenant/chat/moderation-status", "", nil); r.status != http.StatusForbidden && r.status != http.StatusUnauthorized {
		t.Fatalf("student: %d %s", r.status, r.raw)
	}
	// After expires_at the step counts as on again, before the expirer job runs.
	c.clock.Add(2 * time.Hour)
	if r = status(c.staff(c.tenantA, "tenant_admin"), c.tenantA); r.status != http.StatusOK || len(disabledSteps(r)) != 0 || r.body["expires_at"] != nil {
		t.Fatalf("lapsed: %d %s", r.status, r.raw)
	}
}
