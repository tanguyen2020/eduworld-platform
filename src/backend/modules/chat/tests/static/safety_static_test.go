package static

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// T002 / F121-AC-06, AGENTS R27, ADR-0072: tenancy.Bypass(moderation_case) is used only by the moderation use cases of
// module chat (the Safety Team queue, case reads and the tenant check of a write); no other file lifts the tenant filter
// for that reason.
func TestModerationBypassOnlyInModeration(t *testing.T) {
	root := moduleDir(t)
	allowed := []string{"internal/application/query/safety.go", "internal/application/command/safety.go"}
	for path, src := range goFiles(t, root) {
		if !strings.Contains(src, "ReasonModerationCase") {
			continue
		}
		rel, _ := filepath.Rel(root, path)
		if !slices.Contains(allowed, filepath.ToSlash(rel)) {
			t.Errorf("%s uses tenancy.ReasonModerationCase outside the moderation use cases", rel)
		}
	}
}

// T038 / F121-AC-22: the alert ModerationP1Unclaimed fires when the oldest unassigned p1 case is older than
// p1_unclaimed_alert_minutes (15 minutes) — rule of deploy/observability/prometheus/alerts/chat.yml on the gauge the
// job moderation-sla-escalator publishes (promtool is not available locally: the rule text is checked here).
func TestModerationAlertRules(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(moduleDir(t), "..", "..", "..", "..", "deploy", "observability", "prometheus", "alerts", "chat.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Groups []struct {
			Name  string `yaml:"name"`
			Rules []struct {
				Alert       string            `yaml:"alert"`
				Expr        string            `yaml:"expr"`
				Labels      map[string]string `yaml:"labels"`
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, g := range doc.Groups {
		for _, r := range g.Rules {
			found[r.Alert] = r.Expr
			if strings.HasPrefix(r.Alert, "Moderation") && r.Annotations["runbook_url"] == "" && r.Alert != "ModerationSchoolNotificationsOverdue" {
				t.Errorf("%s: runbook_url", r.Alert)
			}
		}
	}
	if e := found["ModerationP1Unclaimed"]; !strings.Contains(e, `eduworld_moderation_case_unclaimed_age_seconds{severity="p1"}`) || !strings.Contains(e, "> 900") {
		t.Errorf("ModerationP1Unclaimed: %q", e)
	}
	if found["ModerationSlaBreached"] == "" {
		t.Error("ModerationSlaBreached missing")
	}
}

// F121-AC-09 (AC-MOD-006, AGENTS R20): the role safety_moderator holds only the moderation permissions and the tenant
// term/category operations of the Safety Team — nothing of LMS, content, student profiles, login events or chat search,
// so every other module refuses it with 403 (each use case requires its own permission, R7).
func TestSafetyModeratorPermissions(t *testing.T) {
	allowed := func(p string) bool {
		return strings.HasPrefix(p, "moderation.") || slices.Contains([]string{"chat.sensitive_terms.read", "chat.tenant_terms.write",
			"chat.tenant_category.toggle"}, p)
	}
	for _, r := range authz.SystemRoles() {
		if r.Name != "safety_moderator" {
			continue
		}
		if r.ScopeKind != authz.RoleScopePlatform {
			t.Fatal("safety_moderator is a platform role (MFA)")
		}
		for _, p := range r.Permissions {
			if !allowed(string(p)) {
				t.Errorf("safety_moderator holds %s outside moderation", p)
			}
			if p == authz.PermPlatformCrossTenantRead || p == authz.PermModerationCaseLead {
				t.Errorf("safety_moderator holds %s by default", p)
			}
		}
		return
	}
	t.Fatal("no role safety_moderator")
}
