//go:build integration

package integration

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F017-AC-01, AC-04, AC-28…AC-30 through the httpx chain (auth, X-Tenant-Id, Idempotency-Key, OpenAPI
// validation, portal guard): role × operation matrix (sa.admin, ad.truong.a, ad.truong.b, qlcm.truong.a),
// another tenant → 404, missing key → 428, bounds → 422, no secret reference in any response.
func TestSisContractHTTP(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	s := newSisAPI(ctx, t, e)
	idp := httpxtest.NewIDP(t)
	res := &httpxtest.Resolver{}
	spec, err := httpx.LoadSpec(ctx, filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: res, Spec: spec,
		Idempotency: idempotency.NewStore(e.app, 0, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if err := identityhttp.Register(srv, identityhttp.Handlers{SisConnections: s.conns}); err != nil {
		t.Fatal(err)
	}
	if err := identityhttp.RegisterSisContract(srv, identityhttp.SisContractHandlers{Connections: s.contract, Sync: s.sync}); err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	h := &httpEnv{env: e, srv: hs, idp: idp, res: res}
	now := time.Now()
	tb := uuid.MustParse(s.b.tenant)
	res.Set("kc-sa", authztest.Platform("system_admin").PrincipalData)
	sa := idp.Token("kc-sa", httpxtest.PortalClient, true, now)
	res.Set("kc-adm-a", authztest.Staff(s.ta, "school_admin", authztest.AtSchool(s.school)).PrincipalData)
	admA := idp.Token("kc-adm-a", httpxtest.PortalClient, true, now)
	res.Set("kc-adm-b", authztest.Staff(tb, "school_admin", authztest.AtSchool(uuid.MustParse(s.b.school))).PrincipalData)
	admB := idp.Token("kc-adm-b", httpxtest.PortalClient, true, now)
	res.Set("kc-qlcm", authztest.Staff(s.ta, "academic_manager").PrincipalData)
	qlcm := idp.Token("kc-qlcm", httpxtest.PortalClient, true, now)
	hdr := func(tenant string, key bool) map[string]string {
		m := map[string]string{}
		if tenant != "" {
			m["X-Tenant-Id"] = tenant
		}
		if key {
			m["Idempotency-Key"] = ids.New().String()
		}
		return m
	}
	body := map[string]any{"provider_kind": "contract_v1", "contract_version": "1.2.0", "secret_ref": "sis/th-demo-a", "webhook_secret_ref": "sis/th-demo-a-webhook",
		"reason": "pilot", "config": map[string]any{"base_url": s.fake.URL + "/eduworld/v1", "auth_type": "api_key",
			"schools": []map[string]any{{"school_external_id": schoolExt, "school_id": s.a.school}}}}
	noSecret := func(r httpResp) {
		t.Helper()
		if strings.Contains(r.raw, "sis/th-demo-a") || strings.Contains(r.raw, "secret_ref") {
			t.Fatalf("secret reference in a response (AC-04): %s", r.raw)
		}
	}

	// AC-29: Idempotency-Key required; AC-28: only platform.sis_connection.manage creates.
	if r := h.do(t, "POST", "/sis/connections", sa, hdr(s.a.tenant, false), body); r.status != 428 {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/sis/connections", qlcm, hdr(s.a.tenant, true), body); r.status != 403 {
		t.Fatalf("qlcm.truong.a creates: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/sis/connections", admA, hdr(s.a.tenant, true), body); r.status != 403 {
		t.Fatalf("ad.truong.a creates: %d %s", r.status, r.raw)
	}
	// AC-30: bounds and https → 422 VALIDATION_FAILED.
	long := map[string]any{"provider_kind": "contract_v1", "contract_version": "1.2.0", "secret_ref": "sis/x",
		"config": map[string]any{"base_url": "https://x.example/" + strings.Repeat("a", 3000), "auth_type": "api_key", "schools": []any{}}}
	if r := h.do(t, "POST", "/sis/connections", sa, hdr(s.a.tenant, true), long); r.status != 422 || !strings.Contains(r.raw, "VALIDATION_FAILED") {
		t.Fatalf("base_url 3000: %d %s", r.status, r.raw)
	}
	ftp := map[string]any{"provider_kind": "contract_v1", "contract_version": "1.2.0", "secret_ref": "sis/x",
		"config": map[string]any{"base_url": "ftp://sis.example", "auth_type": "api_key", "schools": []any{}}}
	if r := h.do(t, "POST", "/sis/connections", sa, hdr(s.a.tenant, true), ftp); r.status != 422 {
		t.Fatalf("ftp: %d %s", r.status, r.raw)
	}
	key := hdr(s.a.tenant, true)
	r := h.do(t, "POST", "/sis/connections", sa, key, body)
	if r.status != 201 || r.body["has_secret"] != true || r.body["has_webhook_secret"] != true || r.body["provider_kind"] != "contract_v1" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	noSecret(r)
	conn := r.body["sis_connection_id"].(string)
	if again := h.do(t, "POST", "/sis/connections", sa, key, body); again.status != 201 || again.body["sis_connection_id"] != conn {
		t.Fatalf("idempotent replay: %d %s", again.status, again.raw)
	}
	if _, err := s.conns.SetSchools(s.sa, uuid.MustParse(conn), []uuid.UUID{s.school}, "pilot"); err != nil {
		t.Fatal(err)
	}

	// checkSisConnectionHealth / updateSisConnection: platform only; If-Match.
	hc := h.do(t, "POST", "/sis/connections/"+conn+":health-check", sa, nil, nil)
	if hc.status != 200 || hc.body["compatible"] != true {
		t.Fatalf("health: %d %s", hc.status, hc.raw)
	}
	noSecret(hc)
	if r := h.do(t, "POST", "/sis/connections/"+conn+":health-check", admA, hdr(s.a.tenant, false), nil); r.status != 403 {
		t.Fatalf("ad.truong.a health: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "PATCH", "/sis/connections/"+conn, sa, map[string]string{"Content-Type": "application/merge-patch+json"}, map[string]any{"schedule": "0 2 * * *"}); r.status != 428 {
		t.Fatalf("no If-Match: %d %s", r.status, r.raw)
	}
	pr := h.do(t, "PATCH", "/sis/connections/"+conn, sa, map[string]string{"Content-Type": "application/merge-patch+json", "If-Match": `W/"` + strconv.FormatInt(s.version(ctx, t, uuid.MustParse(conn)), 10) + `"`},
		map[string]any{"schedule": "0 2 * * *"})
	if pr.status != 200 || pr.body["schedule"] != "0 2 * * *" || pr.header.Get("ETag") == "" {
		t.Fatalf("patch: %d %s", pr.status, pr.raw)
	}
	noSecret(pr)

	// triggerSisSync: platform (any school), school admin of the school (school_id required), other tenant 404.
	if r := h.do(t, "POST", "/sis/connections/"+conn+"/sync-runs", sa, hdr(s.a.tenant, false), map[string]any{"kind": "full"}); r.status != 428 {
		t.Fatalf("trigger without key: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/sis/connections/"+conn+"/sync-runs", admB, hdr(s.b.tenant, true), map[string]any{"kind": "full",
		"school_id": s.b.school}); r.status != 404 {
		t.Fatalf("ad.truong.b on the connection of TH Demo A: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/sis/connections/"+conn+"/sync-runs", qlcm, hdr(s.a.tenant, true), map[string]any{"kind": "full",
		"school_id": s.a.school}); r.status != 403 {
		t.Fatalf("qlcm.truong.a trigger: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/sis/connections/"+conn+"/sync-runs", admA, hdr(s.a.tenant, true), map[string]any{"kind": "full",
		"school_id": s.a.otherSchool}); r.status != 403 && r.status != 404 {
		t.Fatalf("ad.truong.a on another school: %d %s", r.status, r.raw)
	}
	tr := h.do(t, "POST", "/sis/connections/"+conn+"/sync-runs", admA, hdr(s.a.tenant, true), map[string]any{"kind": "full", "school_id": s.a.school,
		"dry_run": true})
	if tr.status != 202 || tr.body["status"] != "queued" || tr.body["dry_run"] != true || tr.body["kind"] != "full" {
		t.Fatalf("ad.truong.a trigger: %d %s", tr.status, tr.raw)
	}
	if r := h.do(t, "POST", "/sis/connections/"+conn+"/sync-runs", sa, hdr(s.a.tenant, true), map[string]any{"kind": "full"}); r.status != 409 ||
		!strings.Contains(r.raw, "SIS_SYNC_ALREADY_RUNNING") {
		t.Fatalf("second trigger: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/sis/connections/"+conn+"/sync-runs", sa, hdr(s.a.tenant, true), map[string]any{"kind": "full", "extra": 1}); r.status != 422 &&
		r.status != 400 {
		t.Fatalf("additionalProperties: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/sis/connections/"+conn+"/sync-runs", sa, hdr(s.b.tenant, true), map[string]any{"kind": "full"}); r.status != 404 {
		t.Fatalf("X-Tenant-Id of another tenant: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "triggerSisSync", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateSisConnection", authztest.RoleDenied)
	authztest.Cover(t, "checkSisConnectionHealth", authztest.RoleDenied)
}
