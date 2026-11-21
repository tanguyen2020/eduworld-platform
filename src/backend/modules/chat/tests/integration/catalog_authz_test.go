//go:build integration

package integration

import (
	"bytes"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

const (
	phPlatform   = "zqplatformterm"
	phTenantOnly = "zqtenantonly"
	phPolitics   = "zqpolitic"
)

// syncBuffer collects the log lines of the test (background goroutines log too).
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// T021 / F116-AC-16, F116-AC-17, F116-AC-18, F116-AC-19 (TC-CHAT-015): the Safety Team adds an extra term to one target
// tenant (X-Tenant-Id) — in force only there; a platform term cannot be edited through the tenant routes; a category
// without allow_tenant_toggle is TENANT_TOGGLE_NOT_ALLOWED and a toggle only changes its tenant; every write endpoint is
// refused to tenant roles and game tokens; tenant roles see the categories and the switch state, never a term; no
// catalogue content reaches a game-facing response or a log line.
func TestCatalogAuthz(t *testing.T) {
	// One literal Cover call per operation (tools/routematrix reads the literals).
	authztest.Cover(t, "listSensitiveCategories", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createSensitiveCategory", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateSensitiveCategory", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteSensitiveCategory", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSensitiveTerms", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createSensitiveTerm", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateSensitiveTerm", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteSensitiveTerm", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "testSensitiveTerms", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSensitiveAllowlist", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createSensitiveAllowlistEntry", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteSensitiveAllowlistEntry", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "publishSensitiveCatalog", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSensitiveCatalogVersions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listTenantSensitiveTerms", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createTenantSensitiveTerm", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateTenantSensitiveTerm", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteTenantSensitiveTerm", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "publishTenantSensitiveCatalog", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listTenantSensitiveCategories", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "toggleTenantSensitiveCategory", authztest.CrossTenant, authztest.RoleDenied)

	logs := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	c := setupChat(t, chatOpts{real: true})
	sa := c.platformStaff("system_admin")
	smA, smB := c.inTenant("safety_moderator", c.tenantA), c.inTenant("safety_moderator", c.tenantB)
	hdrA, hdrB := []string{"X-Tenant-Id", c.tenantA.String()}, []string{"X-Tenant-Id", c.tenantB.String()}

	// Platform catalogue: an insult term and a politics term (a category tenants may have toggled), published.
	r := c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, phPlatform, "word"))
	platID := idOf(r, "term_id")
	if r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catPolitics, phPolitics, "word")); r.status != http.StatusCreated {
		t.Fatalf("politics term: %s", r.raw)
	}
	if r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-catalog:publish", map[string]any{"change_summary": "danh mục thử"}); r.status != http.StatusCreated {
		t.Fatalf("publish platform: %d %s", r.status, r.raw)
	}

	// Safety Team writes an extra term for tenant A (X-Tenant-Id), only into an existing category.
	r = c.admin(smA, http.MethodPost, "/admin/chat/tenant-terms", termInput(catInsult, phTenantOnly, "word"), hdrA...)
	if r.status != http.StatusCreated || r.body["scope"] != "tenant" || r.body["tenant_id"] != c.tenantA.String() || r.body["status"] != "draft" {
		t.Fatalf("tenant term: %d %s", r.status, r.raw)
	}
	tenantTerm := idOf(r, "term_id")
	if r = c.admin(smA, http.MethodPost, "/admin/chat/tenant-terms", termInput(uuid.New(), "zqnewcat", "word"), hdrA...); r.code() != "SENSITIVE_TERM_INVALID" {
		t.Fatalf("tenant term in an unknown category: %d %s", r.status, r.raw)
	}
	// A platform term through the tenant routes → SENSITIVE_TERM_INVALID (F116-AC-16).
	if r = c.admin(smA, http.MethodPatch, "/admin/chat/tenant-terms/"+platID, termInput(catInsult, phPlatform, "word"),
		append(hdrA, "If-Match", `W/"1"`)...); r.code() != "SENSITIVE_TERM_INVALID" {
		t.Fatalf("update platform term as tenant term: %d %s", r.status, r.raw)
	}
	if r = c.admin(smA, http.MethodDelete, "/admin/chat/tenant-terms/"+platID, nil, hdrA...); r.code() != "SENSITIVE_TERM_INVALID" {
		t.Fatalf("delete platform term as tenant term: %d %s", r.status, r.raw)
	}
	// Cross-tenant: the term of tenant A does not exist for tenant B (404), lists are per tenant.
	if r = c.admin(smB, http.MethodPatch, "/admin/chat/tenant-terms/"+tenantTerm, termInput(catInsult, phTenantOnly, "word"),
		append(hdrB, "If-Match", `W/"1"`)...); r.status != http.StatusNotFound {
		t.Fatalf("cross-tenant update: %d %s", r.status, r.raw)
	}
	if r = c.admin(smB, http.MethodDelete, "/admin/chat/tenant-terms/"+tenantTerm, nil, hdrB...); r.status != http.StatusNotFound {
		t.Fatalf("cross-tenant delete: %d %s", r.status, r.raw)
	}
	if r = c.admin(smA, http.MethodGet, "/admin/chat/tenant-terms", nil, hdrA...); len(r.data()) != 1 {
		t.Fatalf("tenant A terms: %s", r.raw)
	}
	if r = c.admin(smB, http.MethodGet, "/admin/chat/tenant-terms", nil, hdrB...); r.status != http.StatusOK || len(r.data()) != 0 {
		t.Fatalf("tenant B terms: %d %s", r.status, r.raw)
	}
	// The Safety Team has no platform write (chat.sensitive_terms.write); another platform role has no catalogue right.
	if r = c.admin(smA, http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, "zqsmplatform", "word")); r.code() != "PERMISSION_DENIED" {
		t.Fatalf("safety moderator platform write: %d %s", r.status, r.raw)
	}
	other := c.platformStaff("content_admin")
	for _, p := range []string{"/admin/chat/sensitive-categories", "/admin/chat/sensitive-terms", "/admin/chat/sensitive-allowlist",
		"/admin/chat/sensitive-catalog/versions"} {
		if r = c.admin(other, http.MethodGet, p, nil); r.code() != "PERMISSION_DENIED" {
			t.Fatalf("content_admin %s: %d %s", p, r.status, r.raw)
		}
	}
	// Publish the tenant extra terms: a tenant version, in force only in tenant A.
	r = c.admin(smA, http.MethodPost, "/admin/chat/tenant-terms:publish", map[string]any{"change_summary": "YC-2026-001 của trường"}, hdrA...)
	if r.status != http.StatusCreated || r.body["scope"] != "tenant" || r.body["catalog_version"] != float64(1) || r.body["tenant_id"] != c.tenantA.String() {
		t.Fatalf("publish tenant: %d %s", r.status, r.raw)
	}
	if r = c.admin(sa, http.MethodGet, "/admin/chat/sensitive-catalog/versions?tenant_ids="+c.tenantA.String(), nil); len(r.data()) != 1 {
		t.Fatalf("tenant versions: %s", r.raw)
	}
	c.reload()
	a1, a2 := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	b1, b2 := c.student(c.tenantB, c.schoolB), c.student(c.tenantB, c.schoolB)
	chA, chB := c.open(channel.Party, a1, a2), c.open(channel.Party, b1, b2)
	if reply := c.sendWS(a1, "conn-a", chA, "nói "+phTenantOnly); replyCode(reply) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("tenant term in tenant A: %+v", reply)
	}
	if reply := c.sendWS(b1, "conn-b", chB, "nói "+phTenantOnly); !reply.OK {
		t.Fatalf("tenant term leaked to tenant B: %+v", reply.Problem)
	}

	// Category toggles (F116-AC-17): not allowed → 403; reason required; the toggle changes tenant A only.
	if r = c.admin(smA, http.MethodPost, "/admin/chat/tenant-categories/insult:toggle", map[string]any{"enabled": false,
		"reason": "Trường đề nghị tắt nhóm này"}, hdrA...); r.status != http.StatusForbidden || r.code() != "TENANT_TOGGLE_NOT_ALLOWED" {
		t.Fatalf("toggle not allowed: %d %s", r.status, r.raw)
	}
	if r = c.admin(smA, http.MethodPost, "/admin/chat/tenant-categories/politics_religion:toggle", map[string]any{"enabled": false,
		"reason": "ngắn"}, hdrA...); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("toggle without reason: %d %s", r.status, r.raw)
	}
	if r = c.admin(smA, http.MethodPost, "/admin/chat/tenant-categories/unknown_code:toggle", map[string]any{"enabled": false,
		"reason": "Trường đề nghị tắt nhóm này"}, hdrA...); r.status != http.StatusNotFound {
		t.Fatalf("toggle unknown category: %d %s", r.status, r.raw)
	}
	r = c.admin(smA, http.MethodPost, "/admin/chat/tenant-categories/politics_religion:toggle", map[string]any{"enabled": false,
		"reason": "Trường đề nghị theo công văn 12/2026"}, hdrA...)
	if r.status != http.StatusOK || r.body["enabled_in_tenant"] != false || r.body["enabled_platform"] != true || r.body["code"] != "politics_religion" {
		t.Fatalf("toggle: %d %s", r.status, r.raw)
	}
	if seen := c.checkEvents("chat_policy.tenant_category_toggled"); seen["chat_policy.tenant_category_toggled"] != 1 {
		t.Fatalf("toggle event: %v", seen)
	}
	if !slices.Contains(c.audits(), "chat.tenant_category.toggled") {
		t.Fatal("toggle not audited")
	}
	c.reload()
	if reply := c.sendWS(a1, "conn-a", chA, "nói "+phPolitics); !reply.OK {
		t.Fatalf("toggled category still enforced in tenant A: %+v", reply.Problem)
	}
	if reply := c.sendWS(b1, "conn-b", chB, "nói "+phPolitics); replyCode(reply) != "CHAT_CONTENT_BLOCKED" {
		t.Fatalf("toggle leaked to tenant B: %+v", reply)
	}

	// Tenant roles read the categories and the switch state, never a term (F116-AC-18).
	r = c.do(http.MethodGet, "/tenant/chat/categories", c.staff(c.tenantA, "tenant_admin"), map[string]string{"X-Tenant-Id": c.tenantA.String()}, nil)
	if r.status != http.StatusOK || len(r.data()) != 10 || strings.Contains(r.raw, `"term"`) || strings.Contains(r.raw, "zq") {
		t.Fatalf("tenant categories: %d %s", r.status, r.raw)
	}
	for _, cat := range r.data() {
		if cat["code"] == "politics_religion" && cat["enabled_in_tenant"] != false {
			t.Fatalf("politics in tenant A: %v", cat)
		}
		if cat["code"] == "insult" && cat["tenant_term_count"] != float64(1) {
			t.Fatalf("tenant term count: %v", cat)
		}
	}
	sw, _ := r.body["moderation_switches"].(map[string]any)
	eff, _ := sw["effective"].(map[string]any)
	effSteps, _ := eff["steps"].(map[string]any)
	if _, hasPII := effSteps["pii"]; sw["scope"] != "tenant" || sw["configured"] != nil || eff["enabled_all"] != true || hasPII ||
		effSteps["spam"] != true || len(effSteps) != 4 { // pii and grooming have no switch (ADR-0078)
		t.Fatalf("switch state: %v", sw)
	}
	r = c.do(http.MethodGet, "/tenant/chat/categories", c.staff(c.tenantB, "tenant_admin"), map[string]string{"X-Tenant-Id": c.tenantB.String()}, nil)
	for _, cat := range r.data() {
		if cat["code"] == "politics_religion" && cat["enabled_in_tenant"] != true {
			t.Fatalf("politics in tenant B: %v", cat)
		}
	}
	if r = c.do(http.MethodGet, "/tenant/chat/categories", c.staff(c.tenantA, "school_admin"), map[string]string{"X-Tenant-Id": c.tenantA.String()},
		nil); r.code() != "PERMISSION_DENIED" {
		t.Fatalf("school_admin categories: %d %s", r.status, r.raw)
	}
	// Cross-tenant on the tenant route: a tenant admin of A asking for tenant B → TENANT_MISMATCH (403).
	if r = c.do(http.MethodGet, "/tenant/chat/categories", c.staff(c.tenantA, "tenant_admin"), map[string]string{"X-Tenant-Id": c.tenantB.String()},
		nil); r.status != http.StatusForbidden {
		t.Fatalf("cross-tenant categories: %d %s", r.status, r.raw)
	}

	// Every write endpoint × {tenant_admin, school_admin, academic_manager, game token} → 403 (F116-AC-18).
	writes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/admin/chat/sensitive-categories", map[string]any{"code": "zqx", "name": "x", "severity": "low", "default_action": "mask",
			"enabled": true, "allow_tenant_toggle": false}},
		{http.MethodPatch, "/admin/chat/sensitive-categories/" + catInsult.String(), map[string]any{}},
		{http.MethodDelete, "/admin/chat/sensitive-categories/" + catInsult.String(), nil},
		{http.MethodPost, "/admin/chat/sensitive-terms", termInput(catInsult, "zqdenied", "word")},
		{http.MethodPatch, "/admin/chat/sensitive-terms/" + platID, termInput(catInsult, "zqdenied", "word")},
		{http.MethodDelete, "/admin/chat/sensitive-terms/" + platID, nil},
		{http.MethodPost, "/admin/chat/sensitive-allowlist", map[string]any{"phrase": "zqdenied", "locale": "vi", "reason": "x"}},
		{http.MethodDelete, "/admin/chat/sensitive-allowlist/" + uuid.NewString(), nil},
		{http.MethodPost, "/admin/chat/sensitive-catalog:publish", map[string]any{"change_summary": "từ chối"}},
		{http.MethodPost, "/admin/chat/tenant-terms", termInput(catInsult, "zqdenied", "word")},
		{http.MethodPatch, "/admin/chat/tenant-terms/" + tenantTerm, termInput(catInsult, "zqdenied", "word")},
		{http.MethodDelete, "/admin/chat/tenant-terms/" + tenantTerm, nil},
		{http.MethodPost, "/admin/chat/tenant-terms:publish", map[string]any{"change_summary": "từ chối"}},
		{http.MethodPost, "/admin/chat/tenant-categories/politics_religion:toggle", map[string]any{"enabled": true, "reason": "Tự bật lại nhóm này"}},
	}
	tokens := map[string]string{"tenant_admin": c.staff(c.tenantA, "tenant_admin"), "school_admin": c.staff(c.tenantA, "school_admin"),
		"academic_manager": c.staff(c.tenantA, "academic_manager"), "game": a1.token}
	before := c.count(`SELECT count(*) FROM chat.sensitive_terms`) + c.count(`SELECT count(*) FROM chat.sensitive_catalog_versions`) +
		c.count(`SELECT count(*) FROM chat.tenant_category_settings WHERE enabled`)
	for who, tok := range tokens {
		for _, w := range writes {
			r = c.admin(tok, w.method, w.path, w.body, append(hdrA, "If-Match", `W/"1"`)...)
			want := "PERMISSION_DENIED"
			if who == "game" {
				want = "PORTAL_ACCESS_STAFF_ONLY"
			}
			if r.status != http.StatusForbidden || r.code() != want {
				t.Errorf("%s %s %s: %d %s", who, w.method, w.path, r.status, r.raw)
			}
		}
	}
	after := c.count(`SELECT count(*) FROM chat.sensitive_terms`) + c.count(`SELECT count(*) FROM chat.sensitive_catalog_versions`) +
		c.count(`SELECT count(*) FROM chat.tenant_category_settings WHERE enabled`)
	if after != before {
		t.Fatal("a refused write changed the catalogue")
	}

	// No catalogue for the game client (F116-AC-19): every catalogue read is refused to a game token without content.
	for _, p := range []string{"/admin/chat/sensitive-categories", "/admin/chat/sensitive-terms", "/admin/chat/sensitive-allowlist",
		"/admin/chat/sensitive-catalog/versions", "/admin/chat/tenant-terms", "/tenant/chat/categories"} {
		if r = c.call(a1, http.MethodGet, p, "", nil); r.status != http.StatusForbidden || strings.Contains(r.raw, "zq") {
			t.Errorf("game token %s: %d %s", p, r.status, r.raw)
		}
	}
	if r = c.call(a1, http.MethodPost, "/admin/chat/sensitive-terms:test", "", map[string]any{"text": phPlatform}); r.status != http.StatusForbidden ||
		strings.Contains(r.raw, "zq") {
		t.Errorf("game token test: %d %s", r.status, r.raw)
	}
	if r = c.call(a1, http.MethodGet, "/chat/quick-messages", "", nil); strings.Contains(r.raw, "zq") {
		t.Errorf("quick messages carry catalogue content: %s", r.raw)
	}
	c.outboxLeaks(phPlatform, phTenantOnly, phPolitics)
	if out := logs.String(); strings.Contains(strings.ToLower(out), "zq") {
		t.Fatalf("a log line carries catalogue or message content")
	}
}
