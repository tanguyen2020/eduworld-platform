package identity

import (
	"fmt"
	"os"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/portalcatalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Portal menus the staff emails link to (specs/portal/menu-catalog.yaml, ADR-0090): the route comes from the catalog,
// the origin from EW_PORTAL_BASE_URL — no environment URL in code (AGENTS R3).
const (
	menuSchoolSafetyNotifications = "menu.school_lms.safety_notifications"
	menuModerationCases           = "menu.live_ops.moderation_cases"
	menuPlatformAuditLogs         = "menu.platform_admin.audit_logs"
)

// mailLinks builds the links of the staff emails (F124) from the portal catalog (EW_PORTAL_MENU_CATALOG) and base.
func mailLinks(cfg *config.Config, base string) (command.MailLinks, error) {
	b, err := os.ReadFile(cfg.String("EW_PORTAL_MENU_CATALOG"))
	if err != nil {
		return command.MailLinks{}, fmt.Errorf("identity: portal menu catalog: %w", err)
	}
	return linksFrom(b, base)
}

func linksFrom(catalog []byte, base string) (command.MailLinks, error) {
	_, items, err := portalcatalog.Items(catalog)
	if err != nil {
		return command.MailLinks{}, err
	}
	paths := map[string]string{}
	for _, it := range items {
		paths[it.Key] = it.Path
	}
	out := command.MailLinks{Base: base, SchoolNotifications: paths[menuSchoolSafetyNotifications],
		ModerationCases: paths[menuModerationCases], AuditLogs: paths[menuPlatformAuditLogs]}
	if out.SchoolNotifications == "" || out.ModerationCases == "" || out.AuditLogs == "" {
		return out, fmt.Errorf("identity: the portal catalog lacks a menu of the staff emails")
	}
	return out, nil
}
