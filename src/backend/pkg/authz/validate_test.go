package authz_test

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// F002-AC-14: the system roles of ADMIN_PORTAL §2.3 pass validation against the catalog of §2.2.
func TestSystemRolesValid(t *testing.T) {
	if err := authz.ValidateCatalog(authz.SystemRoles()); err != nil {
		t.Fatal(err)
	}
	want := []string{"tenant_admin", "school_admin", "academic_manager", "tenant_content_editor", "tenant_content_reviewer",
		"system_admin", "content_admin", "content_reviewer", "game_designer", "liveops", "competition_admin", "safety_moderator"}
	got := map[string]authz.RoleDef{}
	for _, r := range authz.SystemRoles() {
		got[r.Name] = r
	}
	for _, n := range want {
		if _, ok := got[n]; !ok {
			t.Fatalf("role %s missing", n)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("roles = %d, want %d (no teacher/tenant_moderator, ADR-0072)", len(got), len(want))
	}
	if got["tenant_admin"].Has(authz.PermLmsChallengeWrite) || !got["school_admin"].Has(authz.PermLmsChallengeWrite) ||
		got["academic_manager"].Has(authz.PermLmsChallengeWrite) {
		t.Fatal("lms.challenge.write belongs to school_admin only among tenant roles")
	}
	if got["game_designer"].Has(authz.PermGameContentPublish) || got["game_designer"].Has(authz.PermGameEconomyAdjust) ||
		!got["game_designer"].Has(authz.PermGameQuestWrite) {
		t.Fatal("game_designer: game.* except content.publish and economy.adjust")
	}
	if !got["safety_moderator"].Has(authz.PermModerationCaseMessagesRead) || got["safety_moderator"].Has(authz.PermChatPolicyToggle) {
		t.Fatal("safety_moderator permissions")
	}
	if !got["system_admin"].Has(authz.PermPlatformCrossTenantRead) || !got["system_admin"].Has(authz.PermChatPolicyToggle) {
		t.Fatal("system_admin permissions")
	}
	if got["tenant_admin"].ScopeKind != authz.RoleScopeTenant || got["system_admin"].ScopeKind != authz.RoleScopePlatform {
		t.Fatal("scope kinds")
	}
}

// F002-AC-15 (TC-CMS-AUTHZ-04): a tenant role holding any platform-only permission is rejected.
func TestTenantRoleWithPlatformPermissionRejected(t *testing.T) {
	for _, p := range []authz.Permission{authz.PermGameQuestWrite, authz.PermCompetitionConfigSeasonWrite, authz.PermPlatformTenantManage,
		authz.PermModerationCaseRead, authz.PermModerationSanctionWrite, authz.PermModerationSchoolNotificationSend,
		authz.PermChatSensitiveTermsWrite, authz.PermChatPolicyToggle, authz.PermChatTenantTermsWrite, authz.PermChatTenantCategoryToggle} {
		roles := append(authz.SystemRoles(), authz.RoleDef{Name: "evil", ScopeKind: authz.RoleScopeTenant,
			Permissions: []authz.Permission{authz.PermCurriculumRead, p}})
		err := authz.ValidateCatalog(roles)
		var ce *authz.CatalogError
		if !errors.As(err, &ce) || len(ce.Violations) != 1 || ce.Violations[0].Role != "evil" || ce.Violations[0].Permission != p {
			t.Fatalf("%s accepted in tenant role: %v", p, err)
		}
		if !strings.Contains(err.Error(), "evil") || !strings.Contains(err.Error(), string(p)) {
			t.Fatalf("error must name role and permission: %v", err)
		}
	}
	// The same permission is fine in a platform role.
	roles := append(authz.SystemRoles(), authz.RoleDef{Name: "ok", ScopeKind: authz.RoleScopePlatform,
		Permissions: []authz.Permission{authz.PermGameQuestWrite}})
	if err := authz.ValidateCatalog(roles); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownPermissionAndDuplicateRoleRejected(t *testing.T) {
	bad := append(authz.SystemRoles(), authz.RoleDef{Name: "x", ScopeKind: authz.RoleScopeTenant, Permissions: []authz.Permission{"lms.nope"}})
	if err := authz.ValidateCatalog(bad); err == nil {
		t.Fatal("unknown permission accepted")
	}
	dup := append(authz.SystemRoles(), authz.SystemRoles()[0])
	if err := authz.ValidateCatalog(dup); err == nil {
		t.Fatal("duplicate role accepted")
	}
	if err := authz.ValidateCatalog([]authz.RoleDef{{Name: "y", ScopeKind: "class"}}); err == nil {
		t.Fatal("unknown scope kind accepted")
	}
}

func TestMustValidateExits(t *testing.T) {
	code := -1
	restore := authz.SetExitForTest(func(c int) { code = c })
	defer restore()
	var logged strings.Builder
	authz.MustValidate(&logged, []authz.RoleDef{{Name: "evil", ScopeKind: authz.RoleScopeTenant,
		Permissions: []authz.Permission{authz.PermPlatformTenantManage}}})
	if code != 1 || !strings.Contains(logged.String(), "evil") || !strings.Contains(logged.String(), "platform.tenant.manage") {
		t.Fatalf("exit=%d log=%q", code, logged.String())
	}
	code = -1
	authz.MustValidate(&logged, authz.SystemRoles())
	if code != -1 {
		t.Fatal("valid catalog exited")
	}
}

// The Go catalog mirrors ADMIN_PORTAL §2.2 (R33: the document owns the list).
func TestCatalogMatchesAdminPortalDoc(t *testing.T) {
	b, err := os.ReadFile("../../../../docs/04-admin-portal/ADMIN_PORTAL.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "### 2.2. Permission catalog")
	j := strings.Index(s[i:], "### 2.3")
	if i < 0 || j < 0 {
		t.Fatal("ADMIN_PORTAL §2.2 not found")
	}
	doc := map[string]bool{}
	group := regexp.MustCompile("`([a-z_]+(?:\\.[a-z_]+)*)\\.\\*`\\s*\\(([^)]*)\\)")
	item := regexp.MustCompile("`([a-z_]+(?:\\.[a-z_]+)*)`")
	full := regexp.MustCompile("`([a-z_]+(?:\\.[a-z_]+)+(?:/[a-z_]+)*)`")
	for _, l := range strings.Split(s[i:i+j], "\n") {
		if !strings.HasPrefix(l, "| `") {
			continue
		}
		c := strings.Split(l, "|")
		line := c[1] + "|" + c[2]
		for _, m := range group.FindAllStringSubmatch(line, -1) {
			for _, x := range item.FindAllStringSubmatch(m[2], -1) {
				doc[m[1]+"."+x[1]] = true
			}
		}
		for _, m := range full.FindAllStringSubmatch(line, -1) {
			parts := strings.Split(m[1], "/")
			doc[parts[0]] = true
			base := parts[0][:strings.LastIndex(parts[0], ".")]
			for _, a := range parts[1:] {
				doc[base+"."+a] = true
			}
		}
	}
	// Mentions in the description column that are not permissions (event name, shorthand).
	delete(doc, "attempt.re_evaluated")
	delete(doc, "messages.read")
	code := map[string]bool{}
	for _, p := range authz.Catalog() {
		code[string(p)] = true
		if !doc[string(p)] {
			t.Errorf("%s in catalog.go but not in ADMIN_PORTAL §2.2", p)
		}
	}
	for p := range doc {
		if !code[p] {
			t.Errorf("%s in ADMIN_PORTAL §2.2 but not in catalog.go", p)
		}
	}
}
