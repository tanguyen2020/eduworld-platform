//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// T034 (F070-AC-22, TC-TEN-001, TC-TEN-011): every F070 route with the X-Tenant-Id of another tenant is 403
// TENANT_MISMATCH; the same nickname in another tenant is allowed; RLS hides the profiles and outfits of tenant A from
// a session of tenant B.
func TestProfileTenantIsolation(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	tok := e.gtoken(an)
	for _, id := range []string{"getGameProfile", "updateGameProfile", "suggestNicknames", "updateAvatar", "listAvatarPresets",
		"saveAvatarPreset", "getCharacterCatalog"} {
		o := gamehttp.Operations[id]
		path := o.Pattern
		if id == "saveAvatarPreset" {
			path = "/game/avatar-presets/1"
		}
		var body any
		h := map[string]string{"X-Tenant-Id": e.tenantB.String()}
		switch id {
		case "updateAvatar":
			body = avatar()
		case "saveAvatarPreset":
			body = map[string]any{"avatar_config": avatar()}
		case "updateGameProfile":
			body = map[string]any{"preferred_language": "vi-VN"}
			h["Content-Type"] = "application/merge-patch+json"
		}
		r := e.do(t, o.Method, path, tok, h, body)
		if r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
			t.Fatalf("%s with another tenant: %d %s", id, r.status, r.raw)
		}
	}
	authztest.Cover(t, "getGameProfile", authztest.CrossTenant)
	authztest.Cover(t, "updateGameProfile", authztest.CrossTenant)
	authztest.Cover(t, "suggestNicknames", authztest.CrossTenant)
	authztest.Cover(t, "updateAvatar", authztest.CrossTenant)
	authztest.Cover(t, "listAvatarPresets", authztest.CrossTenant)
	authztest.Cover(t, "saveAvatarPreset", authztest.CrossTenant)
	authztest.Cover(t, "getCharacterCatalog", authztest.CrossTenant)

	// The same nickname in two tenants: both allowed (unique per tenant only).
	e.ready(t, an, "Cú Mèo Chăm Chỉ 27")
	e.ready(t, dung, "Cú Mèo Chăm Chỉ 27")
	if r := e.do(t, http.MethodPut, "/game/avatar-presets/1", tok, th(an), map[string]any{"avatar_config": avatar()}); r.status != http.StatusOK {
		t.Fatalf("preset: %d %s", r.status, r.raw)
	}
	// RLS: game_app in a session of tenant B never sees the rows of tenant A, even without a WHERE; no DELETE, and the
	// nickname history is append-only.
	for _, table := range []string{"game.game_profiles", "game.avatar_presets", "game.nickname_changes", "game.inventory_items"} {
		tx, err := e.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantB.String()); err != nil {
			t.Fatal(err)
		}
		// tenancyguard:ok RLS test: counts the rows of tenant A visible to game_app in tenant B on purpose
		if n := count(ctx, t, tx, `SELECT count(*) FROM `+table+` WHERE tenant_id = $1`, e.tenantA); n != 0 {
			t.Errorf("%s: %d rows of tenant A visible in tenant B", table, n)
		}
		_ = tx.Rollback()
		if _, err := e.app.ExecContext(ctx, deleteAll(table)); err == nil {
			t.Errorf("game_app may delete from %s", table)
		}
	}
	// tenancyguard:ok append-only test: the UPDATE must be refused by the grants
	if _, err := e.app.ExecContext(ctx, `UPDATE game.nickname_changes SET new_nickname = 'x'`); err == nil {
		t.Error("nickname history is not append-only")
	}
	if r := e.profile(t, dung); r.body["nickname"] != "Cú Mèo Chăm Chỉ 27" || r.body["student_id"] != dung.id.String() {
		t.Fatalf("tenant B profile: %s", r.raw)
	}
}
