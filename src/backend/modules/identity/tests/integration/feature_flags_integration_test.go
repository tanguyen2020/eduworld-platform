//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	identity "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/featuretoggle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F015 US5: F015-AC-20, AC-23..AC-29 (catalog sync, putFeatureFlag, listings, featuregate.Source with the
// Valkey cache, getMyPermissions, bootstrap feature flags).
func TestFeatureFlagsAndMenuCatalog(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-ff-a")
	b := seedTenant(ctx, t, e.platform, "pgd-ff-b")
	iss := idp.Issuer()
	portalUser(ctx, t, e, iss, "sa.ff", "system_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ad.donvi.ff", "tenant_admin", &a.tenant, "tenant", nil)
	now := time.Now()
	sa := idp.Token("sub-sa.ff", httpxtest.PortalClient, now)
	ta := idp.Token("sub-ad.donvi.ff", httpxtest.PortalClient, now)

	// AC-23: the catalog is synced (setup ran cmd/migrate's sync); a second sync is idempotent.
	var menu, action, protected string
	if err := e.platform.QueryRowContext(ctx, `SELECT m.item_key, a.item_key FROM platform.portal_menu_catalog m
		JOIN platform.portal_menu_catalog a ON a.parent_key = m.item_key AND a.deleted_at IS NULL
		WHERE m.item_kind = 'menu' AND m.can_disable AND m.deleted_at IS NULL ORDER BY m.item_key LIMIT 1`).Scan(&menu, &action); err != nil {
		t.Fatal(err)
	}
	if err := e.platform.QueryRowContext(ctx, `SELECT item_key FROM platform.portal_menu_catalog WHERE item_kind = 'menu' AND NOT can_disable
		AND deleted_at IS NULL ORDER BY item_key LIMIT 1`).Scan(&protected); err != nil {
		t.Fatal(err)
	}
	catalog := filepath.Join(testkit.RepoRoot(t), "specs", "portal", "menu-catalog.yaml")
	migrator := open(t, e.cl.ModuleDSN("identity", "migrator"))
	if s, err := identity.SyncPortalCatalog(ctx, migrator, catalog); err != nil || !contains(s, ", 0 retired, 0 flags retired") {
		t.Fatalf("second sync: %s %v", s, err)
	}

	tenantA := map[string]string{"X-Tenant-Id": a.tenant, "If-Match": `W/"0"`}
	put := func(key string, hdr map[string]string, body map[string]any, tok string) httpResp {
		return h.do(t, "PUT", "/admin/feature-flags/"+key, tok, hdr, body)
	}
	// AC-25: menu disabled for tenant A only; audit + platform_settings.changed scope feature_flag.
	r := put(menu, tenantA, map[string]any{"scope": "tenant", "state": "disabled", "reason": "Chưa dùng"}, sa)
	if r.status != 200 || r.body["effective_enabled"] != false || r.body["flag_kind"] != "menu" || r.header.Get("ETag") != `W/"1"` {
		t.Fatalf("put menu: %d %v", r.status, r.body)
	}
	if count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'platform_settings.changed' AND envelope::text LIKE '%feature_flag%'`) != 1 ||
		count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND envelope::text LIKE '%feature_flag.updated%'`) != 1 {
		t.Fatal("event/audit of the flag")
	}
	// AC-26: protected → 422; unknown catalog key → 404; inherit at platform → 422; no If-Match → 428; stale → 412; tenant admin → 403.
	if r = put(protected, tenantA, map[string]any{"scope": "tenant", "state": "disabled", "reason": "Thử tắt"}, sa); r.status != 422 || r.body["code"] != "FEATURE_NOT_DISABLEABLE" {
		t.Fatalf("protected: %d %v", r.status, r.body)
	}
	if r = put("menu.school_lms.khong_co", tenantA, map[string]any{"scope": "tenant", "state": "disabled", "reason": "Thử"}, sa); r.status != 404 {
		t.Fatalf("unknown: %d", r.status)
	}
	if r = put(menu, map[string]string{"If-Match": `W/"1"`}, map[string]any{"scope": "platform", "state": "inherit", "reason": "Sai"}, sa); r.status != 422 {
		t.Fatalf("inherit at platform: %d", r.status)
	}
	if r = put(menu, map[string]string{"X-Tenant-Id": a.tenant}, map[string]any{"scope": "tenant", "state": "enabled", "reason": "Lại"}, sa); r.status != 428 {
		t.Fatalf("no If-Match: %d", r.status)
	}
	if r = put(menu, tenantA, map[string]any{"scope": "tenant", "state": "enabled", "reason": "Lại"}, sa); r.status != 412 {
		t.Fatalf("stale: %d", r.status)
	}
	if r = put(menu, tenantA, map[string]any{"scope": "tenant", "state": "enabled", "reason": "Lại"}, ta); r.status != 403 {
		t.Fatalf("tenant admin: %d", r.status)
	}

	// AC-28: catalog per tenant (audited cross_tenant.accessed) and the flag list.
	before := count0(ctx, t, e)
	r = h.do(t, "GET", "/admin/portal-menu-catalog?tenant_id="+a.tenant, sa, nil, nil)
	if r.status != 200 || menuEnabled(r.body, menu) != false || count0(ctx, t, e) <= before {
		t.Fatalf("catalog A: %d", r.status)
	}
	if r = h.do(t, "GET", "/admin/portal-menu-catalog?tenant_id="+b.tenant, sa, nil, nil); r.status != 200 || menuEnabled(r.body, menu) != true {
		t.Fatalf("catalog B: %d", r.status)
	}
	r = h.do(t, "GET", "/admin/feature-flags?flag_kind=menu&limit=100&tenant_id="+a.tenant, sa, nil, nil)
	if r.status != 200 || !listed(r.body, menu, false) {
		t.Fatalf("flag list: %d %v", r.status, r.body["page"])
	}

	// AC-27: featuregate.Source and getMyPermissions; the action of the disabled menu is disabled too.
	_, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	src := featuretoggle.Source{Reader: read.FeatureFlagModel{Session: e.read}, C: vk, TTL: time.Minute}
	tA, tB := uuid.MustParse(a.tenant), uuid.MustParse(b.tenant)
	set, err := src.DisabledFor(ctx, &tA)
	if err != nil || !set.MenuDisabled(menu) || !set.ActionDisabled(action) {
		t.Fatalf("source A: %v %v", set.Menus(), err)
	}
	if set, _ := src.DisabledFor(ctx, &tB); set.MenuDisabled(menu) {
		t.Fatal("tenant B affected")
	}
	r = h.do(t, "GET", "/me/permissions", ta, map[string]string{"X-Tenant-Id": a.tenant}, nil)
	if r.status != 200 || !slices.Contains(anyStrings(r.body["disabled_menus"]), menu) {
		t.Fatalf("getMyPermissions: %d %v", r.status, r.body["disabled_menus"])
	}
	// Cache: a change through the command with the cache invalidates the tenant entry at once.
	flags := command.FeatureFlags{UoW: e.uow, Repo: write.FeatureFlagRepo{}, Events: e.deps.Events, Audit: e.deps.Audit, Cache: src}
	saCtx := authz.WithPrincipal(ctx, authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, IsSystemAdmin: true,
		Grants: []authz.Grant{{Role: "system_admin", ScopeKind: authz.RoleScopePlatform, Permissions: []authz.Permission{authz.PermPlatformFeatureFlagWrite}}}}))
	if _, _, err := flags.Put(saCtx, menu, command.FeatureFlagWrite{Scope: "tenant", State: "inherit", Reason: "Bật lại", TenantID: &tA}, 1); err != nil {
		t.Fatal(err)
	}
	if set, _ := src.DisabledFor(ctx, &tA); set.MenuDisabled(menu) {
		t.Fatal("cache not invalidated")
	}

	// AC-20, AC-29: a feature flag (platform off, tenant A on) reaches the bootstrap; menu flags never do.
	if _, _, err := flags.Put(saCtx, "preview_game_frame", command.FeatureFlagWrite{Scope: "platform", State: "disabled", Reason: "Mặc định tắt"}, 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := flags.Put(saCtx, "preview_game_frame", command.FeatureFlagWrite{Scope: "tenant", State: "enabled", Reason: "Thử ở A", TenantID: &tA}, 1); err != nil {
		t.Fatal(err)
	}
	st := enrolled(ctx, t, e.platform, a, "hs.ff")
	stB := enrolled(ctx, t, e.platform, b, "hs.ff.b")
	boot := func(tenant uuid.UUID, user uuid.UUID) map[string]bool {
		sc, _ := authz.EventScope(tenant)
		bs, err := read.BootstrapReader{Session: e.read}.Bootstrap(tenancy.WithScope(ctx, sc), user)
		if err != nil {
			t.Fatal(err)
		}
		return bs.FeatureFlags
	}
	// F095-AC-23: social.friends_enabled is on outside a common tenant (the common tenant case is in game_login).
	if fa, fb := boot(tA, st), boot(tB, stB); fa["preview_game_frame"] != true || fb["preview_game_frame"] != false || len(fa) != 2 ||
		fa[read.FriendsEnabledFlag] != true || fb[read.FriendsEnabledFlag] != true {
		t.Fatalf("bootstrap flags A=%v B=%v", fa, fb)
	}

	// AC-23: an item removed from the catalog file is soft-deleted together with its flags.
	raw, err := os.ReadFile(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO platform.feature_flags (flag_key, flag_kind, default_enabled) VALUES ('menu.live_ops.gone_menu', 'menu', false)`); err == nil {
		t.Fatal("flag outside the catalog accepted")
	}
	trimmed := filepath.Join(t.TempDir(), "catalog.yaml")
	if err := os.WriteFile(trimmed, removeAction(raw, action), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := flags.Put(saCtx, action, command.FeatureFlagWrite{Scope: "platform", State: "disabled", Reason: "Tắt action"}, 0); err != nil {
		t.Fatal(err)
	}
	if s, err := identity.SyncPortalCatalog(ctx, migrator, trimmed); err != nil || !contains(s, ", 1 retired, 1 flags retired") {
		t.Fatalf("retire: %s %v", s, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM platform.portal_menu_catalog WHERE item_key = $1 AND deleted_at IS NOT NULL`, action); n != 1 {
		t.Fatal("removed item not soft-deleted")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func menuEnabled(body map[string]any, key string) any {
	for _, w := range body["workspaces"].([]any) {
		for _, m := range w.(map[string]any)["menus"].([]any) {
			if mm := m.(map[string]any); mm["key"] == key {
				return mm["effective_enabled"]
			}
		}
	}
	return nil
}

func listed(body map[string]any, key string, effective bool) bool {
	for _, x := range body["data"].([]any) {
		if f := x.(map[string]any); f["flag_key"] == key {
			return f["effective_enabled"] == effective
		}
	}
	return false
}

// removeAction drops the YAML block of one action key (the "- key: <action>" item and its indented lines).
func removeAction(raw []byte, key string) []byte {
	lines := splitLines(string(raw))
	var out []string
	skip := -1
	for _, l := range lines {
		indent := len(l) - len(trimLeft(l))
		if skip >= 0 {
			if trimLeft(l) != "" && indent <= skip {
				skip = -1
			} else {
				continue
			}
		}
		if trimLeft(l) == "- key: "+key {
			skip = indent
			continue
		}
		out = append(out, l)
	}
	s := ""
	for _, l := range out {
		s += l + "\n"
	}
	return []byte(s)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, trimCR(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

func trimLeft(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[i:]
}
