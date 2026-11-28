package query

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// FeatureFlagView is the FeatureFlag schema.
type FeatureFlagView struct {
	FlagKey          string     `json:"flag_key"`
	FlagKind         string     `json:"flag_kind"`
	ParentKey        *string    `json:"parent_key"`
	Description      string     `json:"description,omitempty"`
	DefaultEnabled   bool       `json:"default_enabled"`
	TenantOverride   *bool      `json:"tenant_override"`
	EffectiveEnabled bool       `json:"effective_enabled"`
	CanDisable       bool       `json:"can_disable"`
	Version          int64      `json:"version"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	UpdatedBy        *uuid.UUID `json:"updated_by"`
}

// FeatureFlagPage is FeatureFlagPage.
type FeatureFlagPage struct {
	Data []FeatureFlagView `json:"data"`
	Page page.Info         `json:"page"`
}

// CatalogAction is PortalCatalogAction.
type CatalogAction struct {
	Key                 string   `json:"key"`
	LabelKey            string   `json:"label_key"`
	Title               string   `json:"title,omitempty"`
	RequiredPermissions []string `json:"required_permissions"`
	OperationIDs        []string `json:"operation_ids"`
	CanDisable          bool     `json:"can_disable"`
	DefaultEnabled      bool     `json:"default_enabled"`
	TenantOverride      *bool    `json:"tenant_override"`
	EffectiveEnabled    bool     `json:"effective_enabled"`
}

// CatalogMenu is PortalCatalogMenu.
type CatalogMenu struct {
	Key                 string          `json:"key"`
	LabelKey            string          `json:"label_key"`
	Title               string          `json:"title,omitempty"`
	Path                string          `json:"path"`
	Order               int             `json:"order"`
	RequiredPermissions []string        `json:"required_permissions"`
	CanDisable          bool            `json:"can_disable"`
	DefaultEnabled      bool            `json:"default_enabled"`
	TenantOverride      *bool           `json:"tenant_override"`
	EffectiveEnabled    bool            `json:"effective_enabled"`
	Actions             []CatalogAction `json:"actions"`
}

// CatalogWorkspace is PortalCatalogWorkspace.
type CatalogWorkspace struct {
	Key      string        `json:"key"`
	LabelKey string        `json:"label_key"`
	Menus    []CatalogMenu `json:"menus"`
}

// PortalCatalog is PortalMenuCatalogResponse.
type PortalCatalog struct {
	CatalogVersion string             `json:"catalog_version"`
	TenantID       *uuid.UUID         `json:"tenant_id"`
	Workspaces     []CatalogWorkspace `json:"workspaces"`
}

// FeatureFlags implements listFeatureFlags and listPortalMenuCatalog (F015-AC-28).
type FeatureFlags struct {
	Reader  read.FeatureFlagReader
	Cursors *page.Codec
}

// admin checks platform.feature_flag.write (System Admin, platform role guard of /admin) and, for a tenant view,
// records cross_tenant.accessed (ADR-0027 §6).
func (q FeatureFlags) admin(ctx context.Context, tenant *uuid.UUID) (context.Context, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return ctx, err
	}
	if p.TenantID != nil {
		return ctx, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformFeatureFlagWrite, authz.Resource{}); err != nil {
		return ctx, err
	}
	if tenant != nil {
		return tenancy.Bypass(ctx, tenancy.ReasonSystemAdminCrossTenant, tenancy.WithTenantIDs(*tenant))
	}
	return ctx, nil
}

func (q FeatureFlags) load(ctx context.Context) (string, []settings.CatalogItem, map[string]settings.Flag, error) {
	version, items, err := q.Reader.Catalog(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	rows, err := q.Reader.Flags(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	flags := map[string]settings.Flag{}
	for _, f := range rows {
		flags[f.Key] = f
	}
	return version, items, flags, nil
}

// flagCursor is the position after a flag key. Flags are keyed by their key only; the page codec refuses a cursor
// without an id (keyset of other lists), so the id is derived from the key (stable, never read back) — without it the
// second page answered 400 "invalid cursor" (found by the E2E F056 quickstart 22, 2026-11-28).
func flagCursor(key string) page.Cursor {
	return page.Cursor{Key: key, ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("feature_flag:"+key))}
}

// List is listFeatureFlags: feature flags with a row plus every menu/action of the catalog (no row =
// enabled), filtered by kind, values for tenant_id when given, keyset paging by key.
func (q FeatureFlags) List(ctx context.Context, kind string, tenant *uuid.UUID, limit int, cursor string) (out FeatureFlagPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListFeatureFlags")
	defer func() { done(err) }()
	if ctx, err = q.admin(ctx, tenant); err != nil {
		return out, err
	}
	after, err := q.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	_, items, flags, err := q.load(ctx)
	if err != nil {
		return out, err
	}
	var all []FeatureFlagView
	seen := map[string]bool{}
	for _, it := range items {
		if it.Kind == "workspace" {
			continue
		}
		seen[it.Key] = true
		all = append(all, view(it.Key, it.Kind, it.Parent, it.CanDisable, flags, tenant, &it))
	}
	for k, f := range flags {
		if !seen[k] && f.Kind == "feature" {
			all = append(all, view(k, "feature", "", true, flags, tenant, nil))
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].FlagKey < all[j].FlagKey })
	limit = page.Limit(limit)
	out.Page.Limit = limit
	for _, v := range all {
		if (kind != "" && v.FlagKind != kind) || (after != nil && v.FlagKey <= after.Key) {
			continue
		}
		if len(out.Data) == limit {
			next := q.Cursors.Encode(flagCursor(out.Data[limit-1].FlagKey))
			out.Page.HasMore, out.Page.NextCursor = true, &next
			break
		}
		out.Data = append(out.Data, v)
	}
	if out.Data == nil {
		out.Data = []FeatureFlagView{}
	}
	return out, nil
}

func view(key, kind, parent string, canDisable bool, flags map[string]settings.Flag, tenant *uuid.UUID, item *settings.CatalogItem) FeatureFlagView {
	v := FeatureFlagView{FlagKey: key, FlagKind: kind, DefaultEnabled: true, CanDisable: canDisable, EffectiveEnabled: true}
	if kind == "action" {
		p := parent
		v.ParentKey = &p
	}
	if f, ok := flags[key]; ok {
		v.Description, v.DefaultEnabled, v.TenantOverride = f.Description, f.Default, f.Override(tenant)
		v.Version, v.UpdatedAt, v.UpdatedBy = f.Version, f.UpdatedAt, f.UpdatedBy
		v.EffectiveEnabled = f.ValueFor(tenant)
	}
	if item != nil {
		v.EffectiveEnabled = settings.Enabled(*item, flags, tenant)
	}
	return v
}

// Catalog is listPortalMenuCatalog: the workspace → menu → action tree with the flag values of the platform
// or of tenant_id.
func (q FeatureFlags) Catalog(ctx context.Context, tenant *uuid.UUID) (out PortalCatalog, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListPortalMenuCatalog")
	defer func() { done(err) }()
	if ctx, err = q.admin(ctx, tenant); err != nil {
		return out, err
	}
	version, items, flags, err := q.load(ctx)
	if err != nil {
		return out, err
	}
	out.CatalogVersion, out.TenantID = version, tenant
	ws := map[string]int{}
	menus := map[string][2]int{}
	nonNil := func(xs []string) []string {
		if xs == nil {
			return []string{}
		}
		return xs
	}
	for _, it := range items {
		def, override := true, (*bool)(nil)
		if f, ok := flags[it.Key]; ok {
			def, override = f.Default, f.Override(tenant)
		}
		eff := settings.Enabled(it, flags, tenant)
		switch it.Kind {
		case "workspace":
			ws[it.Key] = len(out.Workspaces)
			out.Workspaces = append(out.Workspaces, CatalogWorkspace{Key: it.Key, LabelKey: it.LabelKey, Menus: []CatalogMenu{}})
		case "menu":
			wi, ok := ws[it.Parent]
			if !ok {
				continue
			}
			menus[it.Key] = [2]int{wi, len(out.Workspaces[wi].Menus)}
			out.Workspaces[wi].Menus = append(out.Workspaces[wi].Menus, CatalogMenu{Key: it.Key, LabelKey: it.LabelKey, Title: it.Title,
				Path: it.Path, Order: it.Order, RequiredPermissions: nonNil(it.RequiredPermissions), CanDisable: it.CanDisable,
				DefaultEnabled: def, TenantOverride: override, EffectiveEnabled: eff, Actions: []CatalogAction{}})
		case "action":
			pos, ok := menus[it.Parent]
			if !ok {
				continue
			}
			m := &out.Workspaces[pos[0]].Menus[pos[1]]
			m.Actions = append(m.Actions, CatalogAction{Key: it.Key, LabelKey: it.LabelKey, Title: it.Title,
				RequiredPermissions: nonNil(it.RequiredPermissions), OperationIDs: nonNil(it.OperationIDs), CanDisable: it.CanDisable,
				DefaultEnabled: def, TenantOverride: override, EffectiveEnabled: eff})
		}
	}
	return out, nil
}
