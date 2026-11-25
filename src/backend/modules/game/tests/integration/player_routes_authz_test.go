//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F014-AC-18, F070-AC-21 (TC-AUTH-014, AGENTS R23): every player route of F014 and F070 × {school_admin, academic_manager, tenant_admin,
// safety_moderator, system_admin} with portal tokens → 403 GAME_ACCESS_STUDENT_ONLY; nothing is written.
func TestPlayerRoutesStaffDenied(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	subs := map[string]string{
		"school_admin":     e.staff(e.tenantA, "school_admin"),
		"academic_manager": e.staff(e.tenantA, "academic_manager"),
		"tenant_admin":     e.staff(e.tenantA, "tenant_admin"),
		"safety_moderator": e.platformStaff("safety_moderator"),
		"system_admin":     e.platformStaff("system_admin"),
	}
	session := uuid.NewString()
	for _, id := range gamehttp.PlayerOperations {
		o := gamehttp.Operations[id]
		path := strings.NewReplacer("{game_session_id}", session, "{slot}", "1", "{area_id}", "main_hub", "{level_id}", session,
			"{level_session_id}", session, "{npc_id}", "npc.cloudy", "{interactable_id}", "discovery.hub.spark_tree", "{quest_id}", session,
			"{step_id}", "meet", "{shop_item_id}", session, "{collection_id}", session, "{event_id}", session, "{milestone_id}", "m1",
			"{goal_id}", session, "{tier}", "1", "{minigame_config_id}", session, "{challenge_id}", session, "{challenge_attempt_id}", session,
			"{boss_id}", session, "{encounter_id}", session, "{role}", "measurer").Replace(o.Pattern)
		for role, sub := range subs {
			tok := e.idp.Token(sub, "eduworld-portal", nowWall())
			h := map[string]string{"X-Tenant-Id": e.tenantA.String(), "X-Device-Id": uuid.NewString(), "Idempotency-Key": newKey()}
			var body any
			if o.Method == http.MethodPost && id == "createGameSession" {
				body = map[string]any{"client_platform": "windows", "client_version": "1.2.3"}
			}
			if id == "purchaseShopItem" {
				body = map[string]any{"expected_price": 10}
			}
			if id == "startGameChallenge" {
				body = map[string]any{"difficulty": "easy"}
			}
			if id == "createBossEncounter" {
				body = map[string]any{"mode": "solo"}
			}
			if id == "assignQuestStepRoles" {
				body = map[string]any{"assignments": []any{map[string]any{"role": "measurer", "student_id": session}}}
			}
			r := e.do(t, o.Method, path, tok, h, body)
			want := string(problem.GameAccessStudentOnly)
			if role == "safety_moderator" && r.code() == string(problem.TenantMismatch) {
				want = r.code() // platform staff of no tenant: X-Tenant-Id is refused before the guard (OQ F014-4), still 403
			}
			if r.status != http.StatusForbidden || r.code() != want {
				t.Fatalf("%s by %s: %d %s", id, role, r.status, r.raw)
			}
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions`) + count(ctx, t, e.platform, `SELECT count(*) FROM game.login_events`) +
		count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles`) + count(ctx, t, e.platform, `SELECT count(*) FROM game.avatar_presets`); n != 0 {
		t.Fatalf("rows written: %d", n)
	}
	authztest.Cover(t, "getContentManifest", authztest.RoleDenied)
	for _, id := range []string{"getGameProfile", "updateGameProfile", "suggestNicknames", "updateAvatar", "listAvatarPresets",
		"saveAvatarPreset", "getCharacterCatalog"} {
		if !slices.Contains(gamehttp.PlayerOperations, id) {
			t.Fatalf("%s is not a player operation", id)
		}
	}
	authztest.Cover(t, "getGameProfile", authztest.RoleDenied)
	authztest.Cover(t, "updateGameProfile", authztest.RoleDenied)
	authztest.Cover(t, "suggestNicknames", authztest.RoleDenied)
	authztest.Cover(t, "updateAvatar", authztest.RoleDenied)
	authztest.Cover(t, "listAvatarPresets", authztest.RoleDenied)
	authztest.Cover(t, "saveAvatarPreset", authztest.RoleDenied)
	authztest.Cover(t, "getCharacterCatalog", authztest.RoleDenied)
}
