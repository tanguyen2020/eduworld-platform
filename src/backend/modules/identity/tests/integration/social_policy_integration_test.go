//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/gameauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
)

func socialOf(t *testing.T, r gameResp) map[string]any {
	t.Helper()
	b, ok := r.body["bootstrap"].(map[string]any)
	if !ok {
		t.Fatalf("no bootstrap: %d %v", r.code, r.body)
	}
	pol, _ := b["policies"].(map[string]any)
	s, _ := pol["social"].(map[string]any)
	return s
}

// Delta F013/F015 for the client F111 (owner chốt 2026-11-28 [Confirmed]): PlayerBootstrap.policies.social comes from the
// settings of module identity only (R30: no other module on the login path) — the boolean school setting
// student_club_creation (true → requires_approval, else disabled), club_creation_eligible only for grades 4–9 with a
// policy other than disabled, cross_school_enabled from cross_school_social_enabled of a unit tenant. Omitted when the
// source is not wired; the settings of another tenant never leak into the bootstrap.
func TestSocialPolicyBootstrap(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-soc-a")
	b := seedTenant(ctx, t, g.platform, "pgd-soc-b")
	student := g.student(ctx, t, a, "hs.lap.club")
	dev := uuid.New()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := g.platform.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	login := func() map[string]any {
		t.Helper()
		r := g.login(t, "hs.lap.club", dev)
		if r.code != 200 {
			t.Fatalf("login: %d %v", r.code, r.body)
		}
		return socialOf(t, r)
	}
	want := func(got map[string]any, creation string, eligible, cross bool) {
		t.Helper()
		if got["student_club_creation"] != creation || got["club_creation_eligible"] != eligible || got["cross_school_enabled"] != cross ||
			len(got) != 3 {
			t.Fatalf("policies.social %v, want %s/%v/%v", got, creation, eligible, cross)
		}
	}
	if got := login(); got != nil {
		t.Fatalf("not wired: %v", got)
	}
	g.auth.Social = gameauth.SettingsPolicy{Reader: read.SettingsModel{Session: g.read}}
	want(login(), "disabled", false, false)
	// The other tenant turns everything on: nothing changes for tenant A.
	exec(`UPDATE org.schools SET settings = '{"student_club_creation": true}' WHERE tenant_id = $1`, b.tenant)
	exec(`UPDATE platform.tenants SET settings = '{"cross_school_social_enabled": true}' WHERE tenant_id = $1`, b.tenant)
	want(login(), "disabled", false, false)
	// The school allows clubs (requires approval): a grade 3 student still may not create one (SOCIAL_SYSTEM §6.1).
	exec(`UPDATE org.schools SET settings = '{"student_club_creation": true}' WHERE school_id = $1`, a.school)
	want(login(), "requires_approval", false, false)
	exec(`UPDATE org.students SET grade_level = 5 WHERE student_id = $1`, student)
	want(login(), "requires_approval", true, false)
	exec(`UPDATE platform.tenants SET settings = '{"cross_school_social_enabled": true}' WHERE tenant_id = $1`, a.tenant)
	want(login(), "requires_approval", true, true)
	exec(`UPDATE org.schools SET settings = '{"student_club_creation": false}' WHERE school_id = $1`, a.school)
	want(login(), "disabled", false, true)
}
