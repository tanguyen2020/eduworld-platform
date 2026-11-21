package query

import (
	"context"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// IntegrationClient is one Keycloak client the platform uses: only its id and purpose (portal, game,
// admin_api), never a secret.
type IntegrationClient struct {
	ClientID string `json:"client_id"`
	Purpose  string `json:"purpose"`
}

// IntegrationConfig is the non-secret configuration shown by getIntegrationOverview, read once from the
// environment by the composition root (EW_OIDC_ISSUERS, EW_KC_*_CLIENT_ID, EW_OIDC_PLATFORM_MAX_AUTH_AGE,
// EW_GATEWAY_*_BASE_URL). It never carries a secret, a credential or a secret reference (AGENTS R3, R4).
type IntegrationConfig struct {
	IssuerURL          string
	Realm              string
	Clients            []IntegrationClient
	PlatformMaxAuthAge time.Duration
	PublicBaseURL      string // "" = not configured
	InternalBaseURL    string // "" = not configured
	// RouteGroups are the prefix → service groups of the gateway, generated at build time from the route-as-code
	// file (OQ F059-12).
	RouteGroups []RouteGroup
}

// RouteGroup is one IntegrationOverview.apisix.route_groups[] item.
type RouteGroup struct {
	Prefix  string `json:"prefix"`
	Gateway string `json:"gateway"` // public | internal
	Service string `json:"service"`
}

// IntegrationOverview is the IntegrationOverview schema.
type IntegrationOverview struct {
	Keycloak    KeycloakOverview `json:"keycloak"`
	Apisix      ApisixOverview   `json:"apisix"`
	Sis         SisOverview      `json:"sis"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// KeycloakOverview is IntegrationOverview.keycloak.
type KeycloakOverview struct {
	IssuerURL string              `json:"issuer_url"`
	Realm     string              `json:"realm"`
	Clients   []IntegrationClient `json:"clients"`
	// PlatformAuthMaxAgeSeconds is the maximum age of a platform staff portal session (ADR-0094 §2).
	PlatformAuthMaxAgeSeconds int64 `json:"platform_auth_max_age_seconds"`
}

// ApisixOverview is IntegrationOverview.apisix (null = not configured).
type ApisixOverview struct {
	PublicBaseURL   *string      `json:"public_base_url"`
	InternalBaseURL *string      `json:"internal_base_url"`
	RouteGroups     []RouteGroup `json:"route_groups"`
}

// SisOverview is IntegrationOverview.sis.
type SisOverview struct {
	ConnectionsTotal    int            `json:"connections_total"`
	ConnectionsByStatus map[string]int `json:"connections_by_status"`
}

// Integrations is getIntegrationOverview (F059-2, owner identity/F012): a read-only summary for System Admin.
type Integrations struct {
	Config IntegrationConfig
	Stats  read.SisConnectionStats
	Clock  func() time.Time
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Overview runs getIntegrationOverview: platform staff holding platform.sis_connection.manage or
// platform.tenant.manage (the any-of of action.platform_admin.integrations.view, ADR-0090). The SIS part
// counts the connections of every tenant (audited bypass in the read model); nothing else leaves a tenant.
func (q Integrations) Overview(ctx context.Context) (out IntegrationOverview, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetIntegrationOverview")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID != nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	perm := authz.PermPlatformSisConnectionManage
	if !p.Has(perm) && p.Has(authz.PermPlatformTenantManage) {
		perm = authz.PermPlatformTenantManage
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return out, err
	}
	counts, err := q.Stats.ConnectionStatusCounts(ctx)
	if err != nil {
		return out, err
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	if counts == nil {
		counts = map[string]int{}
	}
	clients := q.Config.Clients
	if clients == nil {
		clients = []IntegrationClient{}
	}
	groups := q.Config.RouteGroups
	if groups == nil {
		groups = []RouteGroup{}
	}
	now := time.Now
	if q.Clock != nil {
		now = q.Clock
	}
	return IntegrationOverview{
		Keycloak: KeycloakOverview{IssuerURL: q.Config.IssuerURL, Realm: q.Config.Realm, Clients: clients,
			PlatformAuthMaxAgeSeconds: int64(q.Config.PlatformMaxAuthAge / time.Second)},
		Apisix: ApisixOverview{PublicBaseURL: optional(q.Config.PublicBaseURL), InternalBaseURL: optional(q.Config.InternalBaseURL),
			RouteGroups: groups},
		Sis:         SisOverview{ConnectionsTotal: total, ConnectionsByStatus: counts},
		GeneratedAt: now().UTC(),
	}, nil
}
