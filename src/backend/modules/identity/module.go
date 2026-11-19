// Package identity is the composition entry of module identity (ADR-0074 §1).
// cmd/<binary> may only call Register (and later Facade/Wire); everything else lives in internal/.
package identity

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/portalcatalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

// Name is the module name used for its database (eduworld_identity), env prefix and checks.
const Name = "identity"

// Module is the registered module. Facade() and Wire(Imports) are added when the module exposes or
// consumes facade operations (specs/openapi/modules/*.internal.v1.yaml).
type Module struct{}

// Register wires the module into the running binary. F001: health checks only; routes, consumers,
// jobs and facades are added by the features that own them.
func Register(r *module.Registry) (*Module, error) {
	db, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("identity: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(db))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(db, migrations.Latest()))
	return &Module{}, nil
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
