package port

import (
	"context"

	"github.com/google/uuid"
)

// WorkspaceCatalog maps permissions to the Admin Portal workspaces a user sees (menu catalog
// specs/portal/menu-catalog.yaml: a workspace is visible when the user holds one required permission of one
// of its menus — ADR-0027 §7, ADR-0090). Only used to build the menu, never to authorize.
type WorkspaceCatalog interface {
	Workspaces(permissions []string) []string
}

// FeatureToggles returns the menus and actions disabled for a session scope (tenant, or platform when nil),
// actions of a disabled menu included (ADR-0090). Backed by featuregate.Source (implemented by F015).
type FeatureToggles interface {
	Disabled(ctx context.Context, tenantID *uuid.UUID) (menus, actions []string, err error)
}

// FeatureToggleCache drops the cached disabled menus/actions of a scope (nil tenant = every scope).
type FeatureToggleCache interface {
	Invalidate(ctx context.Context, tenantID *uuid.UUID) error
}
