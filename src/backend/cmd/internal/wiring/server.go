package wiring

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/featuregate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Defaults of the public server (CONFIG_DEFAULTS: principal_cache 60 s + L1 15 s, feature_toggle_cache_ttl 60 s;
// JWKS refresh [Proposed]). Each one can be overridden by the variable named next to it.
const (
	DefaultPrincipalCacheTTL   = 60 * time.Second // EW_PRINCIPAL_CACHE_TTL
	DefaultPrincipalL1TTL      = 15 * time.Second // EW_PRINCIPAL_CACHE_L1_TTL
	DefaultPrincipalL1Size     = 10000            // EW_PRINCIPAL_CACHE_L1_SIZE
	DefaultFeatureToggleTTL    = 60 * time.Second // EW_FEATURE_TOGGLE_CACHE_TTL
	DefaultJWKSRefresh         = 10 * time.Minute // EW_OIDC_JWKS_REFRESH
	DefaultJWKSUnknownKidRetry = time.Minute      // EW_OIDC_JWKS_MIN_UNKNOWN_KID_INTERVAL
	jwksRetry                  = 5 * time.Second
)

// PublicOptions are the module-owned parts of the public server.
type PublicOptions struct {
	// Identity resolves the Principal of every request (identity facade, resolvePrincipal).
	Identity IdentityPrincipals
	// Toggles is the source of disabled Admin Portal menus/actions (featuregate, ADR-0090 §3).
	Toggles featuregate.Source
	// Extra are handlers served at the root of the listener outside /api/v1 (the SIS webhook of the
	// internal/partner gateway, F017 US5): pattern → handler.
	Extra map[string]http.Handler
}

// ConfigureHooks installs the composition-root hooks of pkg/tenancy and pkg/authz: cross-tenant bypasses and
// authorization denials are audited through the outbox of the module of the request (module.Registry
// AuditRecorder) and counted on the admin registry.
func ConfigureHooks(r *module.Registry) {
	rec := r.AuditRecorder()
	tenancy.Configure(tenancy.Hooks{Recorder: rec, Registerer: r.Metrics()})
	authz.Configure(authz.Hooks{Recorder: rec})
}

// NewPublicServer builds the public API of core-api (OQ F010-2): one pkg/httpx server with JWT verification over the
// JWKS of EW_OIDC_ISSUERS, the identity Principal resolver behind the L1 + Valkey cache (invalidated by the identity
// events), OpenAPI validation against EW_OPENAPI_SPEC, the idempotency store of each module database, the portal
// feature gate, and every operation declared by the modules (Registry.HTTP). It installs the handler on r.
func NewPublicServer(ctx context.Context, r *module.Registry, o PublicOptions) (*httpx.Server, error) {
	if o.Identity == nil {
		return nil, fmt.Errorf("wiring: the identity facade is required by the principal resolver")
	}
	cfg := r.Config()
	auth, err := httpx.AuthConfigFrom(cfg)
	if err != nil {
		return nil, err
	}
	if err := cfg.Require("EW_OPENAPI_SPEC", "EW_PORTAL_MENU_CATALOG"); err != nil {
		return nil, err
	}
	reg := r.Metrics()
	jwks, err := httpx.NewJWKS(httpx.JWKSOptions{Issuers: auth.Issuers, Registerer: reg,
		RefreshInterval:       cfg.Duration("EW_OIDC_JWKS_REFRESH", DefaultJWKSRefresh),
		MinUnknownKidInterval: cfg.Duration("EW_OIDC_JWKS_MIN_UNKNOWN_KID_INTERVAL", DefaultJWKSUnknownKidRetry)})
	if err != nil {
		return nil, err
	}
	r.Health().Register("jwks", health.Readiness, jwks.Check)
	r.Go("jwks", func(ctx context.Context) error {
		for jwks.Load(ctx) != nil { // readiness stays failed until every issuer answered
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(jwksRetry):
			}
		}
		jwks.Run(ctx)
		return nil
	})
	spec, err := httpx.LoadSpec(ctx, cfg.String("EW_OPENAPI_SPEC"))
	if err != nil {
		return nil, fmt.Errorf("openapi spec: %w", err)
	}
	vk, err := r.Valkey()
	if err != nil {
		return nil, err
	}
	principalTTL := cfg.Duration("EW_PRINCIPAL_CACHE_TTL", DefaultPrincipalCacheTTL)
	resolver, err := authz.NewCachedResolver(PrincipalResolver{Identity: o.Identity}, authz.CacheOptions{Module: "identity",
		TTL: principalTTL, L1TTL: cfg.Duration("EW_PRINCIPAL_CACHE_L1_TTL", DefaultPrincipalL1TTL),
		L1Size: cfg.Int("EW_PRINCIPAL_CACHE_L1_SIZE", DefaultPrincipalL1Size), Valkey: vk, Registerer: reg})
	if err != nil {
		return nil, err
	}
	r.Go("principal-invalidator", func(ctx context.Context) error {
		js, err := r.JetStream()
		if err != nil {
			return err
		}
		return (&natsx.PrincipalInvalidator{JS: js, Cache: resolver, Logger: slog.Default()}).Run(ctx)
	})
	deps := httpx.Deps{Auth: auth, Keys: jwks, Resolver: resolver, Registerer: reg, Spec: spec, IdempotencyStores: r.IdempotencyStores()}
	if o.Toggles != nil {
		catalog, err := featuregate.LoadCatalog(cfg.String("EW_PORTAL_MENU_CATALOG"))
		if err != nil {
			return nil, err
		}
		gate, err := featuregate.New(featuregate.Options{Source: o.Toggles, Catalog: catalog, Registerer: reg,
			TTL: cfg.Duration("EW_FEATURE_TOGGLE_CACHE_TTL", DefaultFeatureToggleTTL), PrincipalTTL: principalTTL})
		if err != nil {
			return nil, err
		}
		deps.FeatureGate = gate
	}
	s, err := httpx.NewServer(deps)
	if err != nil {
		return nil, err
	}
	if err := r.MountHTTP(s); err != nil {
		return nil, err
	}
	if len(o.Extra) == 0 {
		r.SetPublic(s)
		return s, nil
	}
	root := chi.NewRouter()
	for pattern, h := range o.Extra {
		root.Handle(pattern, h)
	}
	root.Handle("/*", s)
	r.SetPublic(root)
	return s, nil
}
