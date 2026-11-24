package identity

import (
	"os"
	"path/filepath"
	"testing"
)

// F124: the links of the staff emails come from the portal catalog (routes) and EW_PORTAL_BASE_URL (origin), never from
// a hardcoded environment URL (R3).
func TestMailLinksFromCatalog(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "specs", "portal", "menu-catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := linksFrom(b, "https://portal.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if l.SchoolNotifications != "/school-lms/safety-notifications" || l.ModerationCases != "/live-ops/moderation-cases" ||
		l.AuditLogs != "/platform-admin/audit-logs" || l.Base != "https://portal.example.test" {
		t.Fatalf("links: %+v", l)
	}
	if _, err := linksFrom([]byte("version: x\nworkspaces: []\n"), "https://p"); err == nil {
		t.Fatal("a catalog without the menus must fail")
	}
}
