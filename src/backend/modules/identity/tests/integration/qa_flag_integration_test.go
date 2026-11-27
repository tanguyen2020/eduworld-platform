//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F134-AC-07 (TC-CMS-WS-02): System Admin flags a tenant QA through updateTenant (If-Match, Idempotency-Key): the flag
// is stored, audit tenant.qa_flag_changed carries before/after, tenant.updated carries is_qa and changed_fields; the
// replay of the same key changes nothing; the common tenant → 422; a tenant role never reaches the route (403); listTenants
// shows is_qa.
func TestTenantQAFlag(t *testing.T) {
	ctx := context.Background()
	h := newHTTPEnv(ctx, t, newKCUsers())
	now := time.Now()
	sa := authztest.Platform("system_admin").PrincipalData
	sa.UserID = uuid.New()
	h.res.Set("kc-sa-qa", sa)
	saTok := h.idp.Token("kc-sa-qa", httpxtest.PortalClient, now)
	created := h.do(t, "POST", "/admin/tenants", saTok, map[string]string{"Idempotency-Key": ids.New().String()},
		map[string]any{"code": "qa-probe", "name": "Tenant QA", "tenant_type": "school", "keycloak_org_alias": "qa-probe"})
	if created.status != 201 {
		t.Fatalf("create: %d %s", created.status, created.raw)
	}
	tid := created.body["tenant_id"].(string)
	key := ids.New().String()
	hdr := map[string]string{"Idempotency-Key": key, "If-Match": `W/"1"`}
	patch := map[string]any{"is_qa": true, "reason": "synthetic monitoring probe"}
	r := h.do(t, "PATCH", "/admin/tenants/"+tid, saTok, hdr, patch)
	if r.status != 200 || r.body["is_qa"] != true || r.header.Get("ETag") != `W/"2"` {
		t.Fatalf("flag: %d %s", r.status, r.raw)
	}
	if rp := h.do(t, "PATCH", "/admin/tenants/"+tid, saTok, hdr, patch); rp.status != 200 || rp.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %v %s", rp.status, rp.header, rp.raw)
	}
	var qa bool
	if err := h.platform.QueryRowContext(ctx, `SELECT is_qa FROM platform.tenants WHERE tenant_id = $1`, tid).Scan(&qa); err != nil || !qa {
		t.Fatalf("stored is_qa = %v (%v)", qa, err)
	}
	var payload json.RawMessage
	if err := h.relay.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'tenant.updated'
		AND tenant_id = $1 ORDER BY created_at DESC LIMIT 1`, tid).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var ev struct {
		IsQA    *bool    `json:"is_qa"`
		Changed []string `json:"changed_fields"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil || ev.IsQA == nil || !*ev.IsQA || strings.Join(ev.Changed, ",") != "is_qa" {
		t.Fatalf("tenant.updated payload %s", payload)
	}
	var audits int
	var meta string
	if err := h.relay.QueryRowContext(ctx, `SELECT count(*), max(envelope->'payload'->>'metadata') FROM infra.outbox_events
		WHERE event_type = 'audit.recorded' AND envelope->'payload'->>'action' = 'tenant.qa_flag_changed'`).Scan(&audits, &meta); err != nil {
		t.Fatal(err)
	}
	if audits != 1 || !strings.Contains(meta, `"is_qa_after": "true"`) || !strings.Contains(meta, `"is_qa_before": "false"`) {
		t.Fatalf("qa audits = %d, metadata %s", audits, meta)
	}
	if l := h.do(t, "GET", "/admin/tenants?limit=100", saTok, nil, nil); l.status != 200 || !strings.Contains(l.raw, `"is_qa":true`) {
		t.Fatalf("listTenants: %d %s", l.status, l.raw)
	}
	// The common tenant can never be a QA tenant.
	common := tenant.CommonTenantID.String()
	cur := h.do(t, "GET", "/admin/tenants?limit=100", saTok, nil, nil)
	version := 1.0
	for _, x := range cur.body["data"].([]any) {
		if m := x.(map[string]any); m["tenant_id"] == common {
			version = m["version"].(float64)
		}
	}
	c := h.do(t, "PATCH", "/admin/tenants/"+common, saTok, map[string]string{"Idempotency-Key": ids.New().String(),
		"If-Match": fmt.Sprintf(`W/"%d"`, int(version))},
		map[string]any{"is_qa": true, "reason": "must be refused"})
	if c.status != 422 || c.body["code"] != "VALIDATION_FAILED" {
		t.Fatalf("common tenant: %d %s", c.status, c.raw)
	}
	// A tenant role never reaches /admin/tenants (R20).
	tenantAdmin := authztest.Staff(uuid.MustParse(tid), "tenant_admin").PrincipalData
	h.res.Set("kc-ta-qa", tenantAdmin)
	taTok := h.idp.Token("kc-ta-qa", httpxtest.PortalClient, now)
	if r := h.do(t, "PATCH", "/admin/tenants/"+tid, taTok, map[string]string{"Idempotency-Key": ids.New().String(), "If-Match": `W/"2"`},
		map[string]any{"is_qa": false, "reason": "tenant role"}); r.status != 403 {
		t.Fatalf("tenant role: %d %s", r.status, r.raw)
	}
}
