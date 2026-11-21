package http

import (
	"errors"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// PortalAuthConfigFrom reads the configuration of the portal login use cases for the composition root
// (LOCAL_DEVELOPMENT §4): EW_OIDC_ISSUERS, EW_OIDC_PLATFORM_MAX_AUTH_AGE and
// EW_PORTAL_BASE_URL. portal_setup_ticket_ttl, portal_password_reset_ttl and portal_login_rate keep the
// CONFIG_DEFAULTS values until they are read from platform settings.
func PortalAuthConfigFrom(c *config.Config) (command.PortalAuthConfig, error) {
	if err := c.Require("EW_OIDC_ISSUERS", "EW_PORTAL_BASE_URL"); err != nil {
		return command.PortalAuthConfig{}, err
	}
	base := strings.TrimRight(c.String("EW_PORTAL_BASE_URL"), "/")
	if !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://localhost") {
		return command.PortalAuthConfig{}, errors.New("identity: EW_PORTAL_BASE_URL must be https (http only for localhost)")
	}
	return command.PortalAuthConfig{
		Issuers:    c.List("EW_OIDC_ISSUERS"),
		MaxAuthAge: c.Duration("EW_OIDC_PLATFORM_MAX_AUTH_AGE", command.DefaultPlatformMaxAuthAge), PortalBaseURL: base,
		SetupTicketTTL: command.DefaultPortalSetupTicketTTL, ResetTTL: command.DefaultPortalPasswordResetTTL,
		Limits: command.DefaultPortalRateLimits(),
	}, nil
}
