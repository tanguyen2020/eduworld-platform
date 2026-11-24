package static

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// F124 T003, T044 / F124-AC-19, AGENTS R20, R27, ADR-0089 §3: tenancy.Bypass(break_glass) lifts the tenant filter only in
// the break-glass reads of the System Admin (query.SafetyOps); no other file of module chat uses that reason.
func TestBreakGlassBypassOnlyInBreakGlassReads(t *testing.T) {
	root := moduleDir(t)
	allowed := []string{"internal/application/query/safety_ops.go"}
	found := false
	for path, src := range goFiles(t, root) {
		if !strings.Contains(src, "ReasonBreakGlass") {
			continue
		}
		rel, _ := filepath.Rel(root, path)
		if !slices.Contains(allowed, filepath.ToSlash(rel)) {
			t.Errorf("%s uses tenancy.ReasonBreakGlass outside the break-glass reads", rel)
		}
		found = true
	}
	if !found {
		t.Fatal("the break-glass reads must lift the filter through tenancy.ReasonBreakGlass (audit + metric)")
	}
}

// F124 T037 / F124-AC-10, AC-15 (promtool is not available locally: the rule text is checked here): ModerationOncallGap on
// the coverage gauge, EmailDeliveryFailing and SchoolSafetyNotificationNoRecipient on the identity counters, and the
// route team=safety → safety-oncall of Alertmanager (ModerationP1Unclaimed carries team: safety).
func TestSafetyOperationsAlerts(t *testing.T) {
	deploy := filepath.Join(moduleDir(t), "..", "..", "..", "..", "deploy", "observability")
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(deploy, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	chat, identity, am := read("prometheus/alerts/chat.yml"), read("prometheus/alerts/identity.yml"), read("alertmanager/alertmanager.yml")
	for _, want := range []string{"alert: ModerationOncallGap", "eduworld_moderation_oncall_coverage) == 0", "team: safety"} {
		if !strings.Contains(chat, want) {
			t.Errorf("chat.yml lacks %q", want)
		}
	}
	for _, want := range []string{"alert: EmailDeliveryFailing", `eduworld_email_deliveries_total{status="failed"}`,
		"alert: SchoolSafetyNotificationNoRecipient", "eduworld_school_safety_notification_no_recipient_total"} {
		if !strings.Contains(identity, want) {
			t.Errorf("identity.yml lacks %q", want)
		}
	}
	if !strings.Contains(am, `matchers: ['team="safety"']`) || !strings.Contains(am, "receiver: safety-oncall") || strings.Contains(am, "http") {
		t.Errorf("alertmanager route team=safety → safety-oncall (no URL in the file): %s", am)
	}
	i := strings.Index(chat, "alert: ModerationP1Unclaimed")
	if i < 0 || !strings.Contains(chat[i:i+300], "team: safety") {
		t.Error("ModerationP1Unclaimed must route to safety-oncall (team: safety)")
	}
}
