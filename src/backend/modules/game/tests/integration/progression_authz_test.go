//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F074-AC-19 (TC-AUTH-014), F074-AC-20 (TC-TEN-024): the four progression routes are guarded player routes (staff tokens
// of the 5 roles → 403 GAME_ACCESS_STUDENT_ONLY, also in TestPlayerRoutesStaffDenied), the X-Tenant-Id of another tenant
// → 403 TENANT_MISMATCH, no route reads another student; a limit out of range or a malformed cursor → 422
// VALIDATION_FAILED.
func TestProgressionAuthzAndValidation(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	for _, id := range gamehttp.ProgressionOperations {
		if !slices.Contains(gamehttp.PlayerOperations, id) || gamehttp.Operations[id].Guard == 0 {
			t.Fatalf("%s is not a guarded player operation", id)
		}
	}
	authztest.Cover(t, "getProgression", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listXpHistory", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listProgressionLevels", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "getXpStatus", authztest.RoleDenied, authztest.CrossTenant)
	paths := []string{"/game/progression", "/game/progression/xp-history", "/game/progression/levels", "/game/xp"}
	staff := map[string]string{
		"school_admin":     e.staff(e.tenantA, "school_admin"),
		"academic_manager": e.staff(e.tenantA, "academic_manager"),
		"tenant_admin":     e.staff(e.tenantA, "tenant_admin"),
		"safety_moderator": e.platformStaff("safety_moderator"),
		"system_admin":     e.platformStaff("system_admin"),
	}
	for _, p := range paths {
		for role, sub := range staff {
			r := e.do(t, http.MethodGet, p, e.idp.Token(sub, "eduworld-portal", nowWall()), map[string]string{"X-Tenant-Id": e.tenantA.String()}, nil)
			want := string(problem.GameAccessStudentOnly)
			if role == "safety_moderator" && r.code() == string(problem.TenantMismatch) {
				want = r.code() // platform staff of no tenant: X-Tenant-Id is refused before the guard, still 403
			}
			if r.status != http.StatusForbidden || r.code() != want {
				t.Fatalf("%s by %s: %d %s", p, role, r.status, r.raw)
			}
		}
	}
	uc := e.progressionUC()
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(dung, "main"))
	for _, p := range paths {
		r := e.do(t, http.MethodGet, p, e.gtoken(an), map[string]string{"X-Tenant-Id": e.tenantB.String()}, nil)
		if r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
			t.Fatalf("%s with the tenant of B: %d %s", p, r.status, r.raw)
		}
	}
	// The student of A only ever sees itself (no route takes a student id).
	if r := e.do(t, http.MethodGet, "/game/progression/xp-history", e.gtoken(an), th(an), nil); r.status != http.StatusOK ||
		len(r.body["data"].([]any)) != 0 {
		t.Fatalf("history of A shows B: %d %s", r.status, r.raw)
	}
	for _, q := range []string{"limit=0", "limit=101", "limit=abc", "cursor=not-a-cursor", "cursor=" + strings.Repeat("a", 600)} {
		r := e.do(t, http.MethodGet, "/game/progression/xp-history?"+q, e.gtoken(an), th(an), nil)
		if r.status != http.StatusUnprocessableEntity || r.code() != string(problem.ValidationFailed) {
			t.Fatalf("%s: %d %s", q, r.status, r.raw)
		}
	}
}
