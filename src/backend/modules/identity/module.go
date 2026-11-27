// Package identity is the composition entry of module identity (ADR-0074 §1).
// cmd/<binary> may only call Register (and later Facade/Wire); everything else lives in internal/.
package identity

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/featuretoggle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/portalcatalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/sisinternal"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

// Name is the module name used for its database (eduworld_identity), env prefix and checks.
const Name = "identity"

// Module is the registered module.
type Module struct {
	r          *module.Registry
	facade     Facade
	toggles    featuretoggle.Source
	hasToggles bool
	webhook    sisinternal.Webhook
	wired      bool
}

// Facade returns the in-process facade of module identity (nil before Wire or in a binary without its use cases).
func (m *Module) Facade() Facade { return m.facade }

// Wire connects the facades of the modules identity calls and builds the use cases, routes, consumers and jobs
// of module identity (OQ F010-2: core-api routes, worker events consumers, worker jobs). The other modules
// are registered first because the sagas and platform operations call them in process (BACKEND_DESIGN §3.3).
func (m *Module) Wire(in Imports) error {
	if m.wired {
		return fmt.Errorf("identity: Wire called twice")
	}
	m.wired = true
	if !m.r.ServesHTTP() && !m.r.RunsEvents() && !m.r.RunsJobs() && !m.r.Reconciles() {
		return nil
	}
	return build(m.r, m, in)
}

// SisWebhook is the machine-to-machine SIS webhook handler (sisinternal.Path, F017 US5): the composition root
// mounts it at the root of the core-api listener, outside /api/v1; the internal gateway alone routes to it.
func (m *Module) SisWebhook() http.Handler {
	if m.webhook.Receive == nil {
		return nil
	}
	router := chi.NewRouter()
	sisinternal.Register(router, m.webhook)
	return router
}

// SisWebhookPath is the route pattern of the SIS webhook.
const SisWebhookPath = sisinternal.Path

// Register opens the module database and its health checks; Wire builds the rest.
func Register(r *module.Registry) (*Module, error) {
	db, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("identity: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(db))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(db, migrations.Latest()))
	return &Module{r: r}, nil
}

// SyncAuthzCatalog writes the permission catalog and the system roles of pkg/authz into eduworld_identity
// (F012 T007). cmd/migrate calls it after the Atlas migrations with the identity_migrator connection.
func SyncAuthzCatalog(ctx context.Context, db *sql.DB) (string, error) {
	st, err := seed.SyncAuthz(ctx, db, authz.Catalog(), authz.SystemRoles())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d permissions, %d system roles, %d grants, %d retired", st.Permissions, st.Roles, st.RolePermissions, st.Retired), nil
}

// SyncPortalCatalog writes specs/portal/menu-catalog.yaml (the file at path) into platform.portal_menu_catalog
// (F015 T040, ADR-0090); cmd/migrate calls it after SyncAuthzCatalog with the identity_migrator connection.
func SyncPortalCatalog(ctx context.Context, db *sql.DB, path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("portal catalog: %w", err)
	}
	version, items, err := portalcatalog.Items(b)
	if err != nil {
		return "", err
	}
	st, err := seed.SyncPortalCatalog(ctx, db, version, items)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("catalog %s: %d items, %d retired, %d flags retired", version, st.Items, st.Retired, st.FlagsRetired), nil
}
