package identity

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/integration"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// integrationConfig reads the non-secret configuration of getIntegrationOverview (F059-2): the realm issuer
// (first of EW_OIDC_ISSUERS) and its realm name, the client ids with their purpose, the step-up window of
// platform routes and the optional gateway URLs EW_GATEWAY_PUBLIC_BASE_URL / EW_GATEWAY_INTERNAL_BASE_URL.
// Secrets (EW_KC_*_CLIENT_SECRET, …) are never read here. A URL carrying credentials, a query or a fragment
// is refused at startup so nothing secret can reach the response (AGENTS R3, R4).
func integrationConfig(cfg *config.Config, issuer string) (query.IntegrationConfig, error) {
	out := query.IntegrationConfig{IssuerURL: issuer,
		PlatformMaxAuthAge: cfg.Duration("EW_OIDC_PLATFORM_MAX_AUTH_AGE", httpx.DefaultPlatformMaxAuthAge)}
	if err := plainURL("EW_OIDC_ISSUERS", issuer); err != nil {
		return out, err
	}
	if i := strings.LastIndex(issuer, "/realms/"); i >= 0 {
		out.Realm = strings.Trim(issuer[i+len("/realms/"):], "/")
	}
	for _, c := range []struct{ key, purpose string }{
		{"EW_KC_PORTAL_CLIENT_ID", "portal"}, {"EW_KC_GAME_CLIENT_ID", "game"}, {"EW_KC_ADMIN_CLIENT_ID", "admin_api"},
	} {
		if id := cfg.String(c.key); id != "" {
			out.Clients = append(out.Clients, query.IntegrationClient{ClientID: id, Purpose: c.purpose})
		}
	}
	groups, err := integration.RouteGroups() // generated from deploy/apisix/routes/routes.yaml (OQ F059-12)
	if err != nil {
		return out, err
	}
	out.RouteGroups = groups
	out.PublicBaseURL, out.InternalBaseURL = cfg.String("EW_GATEWAY_PUBLIC_BASE_URL"), cfg.String("EW_GATEWAY_INTERNAL_BASE_URL")
	for key, v := range map[string]string{"EW_GATEWAY_PUBLIC_BASE_URL": out.PublicBaseURL, "EW_GATEWAY_INTERNAL_BASE_URL": out.InternalBaseURL} {
		if v == "" {
			continue
		}
		if err := plainURL(key, v); err != nil {
			return out, err
		}
	}
	return out, nil
}

// plainURL accepts an absolute http(s) URL of at most 2048 bytes without user info, query or fragment.
func plainURL(key, v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		len(v) > 2048 {
		return fmt.Errorf("identity: %s must be an absolute http(s) URL without credentials, query or fragment", key)
	}
	return nil
}
