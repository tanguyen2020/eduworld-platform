//go:build integration

package integration

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/settingscfg"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// settingsHandlers are the F015 use cases of the HTTP harness realAuthHTTP.
func settingsHandlers(e *env, codec *page.Codec) identityhttp.SettingsHandlers {
	defaults, err := settingscfg.DefaultsFrom(config.Load())
	if err != nil {
		panic(err)
	}
	return identityhttp.SettingsHandlers{AuditLogs: query.AuditLogs{Model: read.AuditModel{Session: e.read}, Cursors: codec},
		Settings:    command.Settings{UoW: e.uow, Repo: write.SettingsRepo{}, Events: e.deps.Events, Audit: e.deps.Audit, Defaults: defaults},
		SettingsGet: query.Settings{Reader: read.SettingsModel{Session: e.read}, Defaults: defaults}}
}

// portalUser creates a staff user linked to the test IDP with one role.
func portalUser(ctx context.Context, t *testing.T, e *env, issuer, username, role string, tenant *string, scope string, school *string) {
	t.Helper()
	id := staffUser(ctx, t, e, username, tenant)
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
		VALUES ($1, now(), now(), $2, $3, $4)`, uuid.New(), issuer, "sub-"+username, id); err != nil {
		t.Fatal(err)
	}
	grantRole(ctx, t, e, id, role, tenant, scope, school, "NULL")
}

func auditRow(ctx context.Context, t *testing.T, e *env, tenant, school *string, action string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO audit.audit_logs (audit_id, occurred_at, actor_type, tenant_id, school_id, action, target_type,
		target_id, result, metadata, producer) VALUES ($1, now() - interval '1 hour', 'system', $2::uuid, $3::uuid, $4, 'user', 'x', 'success',
		'{"role_code":"school_admin"}', 'identity/core')`, id, tenant, school, action); err != nil {
		t.Fatal(err)
	}
	return id
}

// F015 US2: F015-AC-07..AC-10 through the httpx chain with the real Principal resolution.
func TestAuditLogQueries(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-aq-a")
	b := seedTenant(ctx, t, e.platform, "pgd-aq-b")
	auditRow(ctx, t, e, &a.tenant, &a.school, "user_role.granted")
	auditRow(ctx, t, e, &a.tenant, &a.otherSchool, "user_role.granted")
	auditRow(ctx, t, e, &a.tenant, nil, "moderation.case_actioned")
	auditRow(ctx, t, e, &b.tenant, &b.school, "user_role.granted")
	auditRow(ctx, t, e, nil, nil, "tenant.created")

	iss := idp.Issuer()
	portalUser(ctx, t, e, iss, "ad.donvi.aq", "tenant_admin", &a.tenant, "tenant", nil)
	portalUser(ctx, t, e, iss, "ad.truong.aq", "school_admin", &a.tenant, "school", &a.school)
	portalUser(ctx, t, e, iss, "sa.aq", "system_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ca.aq", "content_admin", nil, "platform", nil)
	now := time.Now()
	window := "from=" + url.QueryEscape(now.Add(-24*time.Hour).Format(time.RFC3339)) + "&to=" + url.QueryEscape(now.Add(time.Minute).Format(time.RFC3339))
	tok := func(user string, mfa bool) string { return idp.Token("sub-"+user, httpxtest.PortalClient, mfa, now) }
	inA := map[string]string{"X-Tenant-Id": a.tenant}
	count := func(r httpResp) int {
		data, _ := r.body["data"].([]any)
		return len(data)
	}

	// AC-07: tenant admin sees the tenant, without the moderation journal.
	r := h.do(t, "GET", "/tenant/audit-logs?"+window, tok("ad.donvi.aq", false), inA, nil)
	if r.status != 200 || count(r) != 2 {
		t.Fatalf("tenant admin: %d %v", r.status, r.body)
	}
	// School Admin: only the school of the grant; another school → 403.
	r = h.do(t, "GET", "/tenant/audit-logs?"+window, tok("ad.truong.aq", false), inA, nil)
	if r.status != 200 || count(r) != 1 || r.body["data"].([]any)[0].(map[string]any)["school_id"] != a.school {
		t.Fatalf("school admin: %d %v", r.status, r.body)
	}
	if r = h.do(t, "GET", "/tenant/audit-logs?"+window+"&school_id="+a.otherSchool, tok("ad.truong.aq", false), inA, nil); r.status != 403 {
		t.Fatalf("other school: %d", r.status)
	}
	// Paging: limit 1 then the cursor.
	r = h.do(t, "GET", "/tenant/audit-logs?"+window+"&limit=1", tok("ad.donvi.aq", false), inA, nil)
	next, _ := r.body["page"].(map[string]any)["next_cursor"].(string)
	if r.status != 200 || count(r) != 1 || next == "" {
		t.Fatalf("page 1: %v", r.body)
	}
	if r = h.do(t, "GET", "/tenant/audit-logs?"+window+"&limit=1&cursor="+url.QueryEscape(next), tok("ad.donvi.aq", false), inA, nil); r.status != 200 || count(r) != 1 {
		t.Fatalf("page 2: %v", r.body)
	}
	// AC-10: missing or too wide window → 422.
	if r = h.do(t, "GET", "/tenant/audit-logs", tok("ad.donvi.aq", false), inA, nil); r.status != 422 {
		t.Fatalf("no window: %d", r.status)
	}
	wide := "from=" + url.QueryEscape(now.AddDate(0, 0, -100).Format(time.RFC3339)) + "&to=" + url.QueryEscape(now.Format(time.RFC3339))
	if r = h.do(t, "GET", "/tenant/audit-logs?"+wide, tok("ad.donvi.aq", false), inA, nil); r.status != 422 {
		t.Fatalf("wide window: %d", r.status)
	}

	// AC-08: System Admin with tenant_ids → rows of both tenants with tenant_id, audited.
	before := count0(ctx, t, e)
	r = h.do(t, "GET", "/admin/audit-logs?"+window+"&tenant_ids="+a.tenant+","+b.tenant, tok("sa.aq", true), nil, nil)
	if r.status != 200 || count(r) != 4 || r.body["data"].([]any)[0].(map[string]any)["tenant_id"] == nil {
		t.Fatalf("system admin: %d %v", r.status, r.body)
	}
	if count0(ctx, t, e) <= before {
		t.Fatal("cross_tenant.accessed not audited")
	}
	// Without tenant_ids: the platform entries only.
	if r = h.do(t, "GET", "/admin/audit-logs?"+window, tok("sa.aq", true), nil, nil); r.status != 200 || count(r) != 1 {
		t.Fatalf("platform rows: %d %v", r.status, r.body)
	}
	// Other roles → 403 (content admin; tenant admin on /admin; tenant admin of A asking for B is impossible — scope is the session).
	if r = h.do(t, "GET", "/admin/audit-logs?"+window, tok("ca.aq", true), nil, nil); r.status != 403 {
		t.Fatalf("content admin: %d", r.status)
	}
	if r = h.do(t, "GET", "/admin/audit-logs?"+window+"&tenant_ids="+b.tenant, tok("ad.donvi.aq", false), nil, nil); r.status != 403 {
		t.Fatalf("tenant admin on /admin: %d", r.status)
	}
}

// count0 counts the cross_tenant.accessed audits waiting in the outbox.
func count0(ctx context.Context, t *testing.T, e *env) int {
	return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND envelope::text LIKE '%cross_tenant.accessed%'`)
}
