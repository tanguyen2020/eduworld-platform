package settings

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CatalogItem is one workspace, menu or action of the Admin Portal menu catalog (ADR-0090).
type CatalogItem struct {
	Key                 string
	Kind                string // workspace | menu | action
	Workspace           string
	Parent              string // menu → workspace, action → menu
	Path                string
	LabelKey            string
	Title               string
	Order               int
	RequiredPermissions []string
	OperationIDs        []string
	CanDisable          bool
}

// Flag is one row of platform.feature_flags. No row means enabled.
type Flag struct {
	Key         string
	Kind        string // feature | menu | action
	Parent      string
	Description string
	Default     bool
	Overrides   map[string]bool // tenant id → enabled
	Version     int64
	UpdatedAt   *time.Time
	UpdatedBy   *uuid.UUID
}

// ValueFor returns the flag value for a tenant: its override when present, else the platform default.
// A nil tenant is the platform scope (System Admin without X-Tenant-Id).
func (f Flag) ValueFor(tenant *uuid.UUID) bool {
	if tenant != nil {
		if v, ok := f.Overrides[tenant.String()]; ok {
			return v
		}
	}
	return f.Default
}

// Override returns the override of a tenant (nil = inherits).
func (f Flag) Override(tenant *uuid.UUID) *bool {
	if tenant == nil {
		return nil
	}
	if v, ok := f.Overrides[tenant.String()]; ok {
		return &v
	}
	return nil
}

// Enabled returns the effective state of a catalog key for a tenant: a menu is its flag; an action is its
// flag AND the flag of its parent menu (F015-AC-27). Keys without a flag row are enabled.
func Enabled(item CatalogItem, flags map[string]Flag, tenant *uuid.UUID) bool {
	v := true
	if f, ok := flags[item.Key]; ok {
		v = f.ValueFor(tenant)
	}
	if item.Kind == "action" {
		if p, ok := flags[item.Parent]; ok {
			v = v && p.ValueFor(tenant)
		}
	}
	return v
}

// Disabled returns the disabled menus and actions of the catalog for a tenant (sorted), the content of
// featuregate.Set and of getMyPermissions.disabled_* (F015-AC-27, AC-29: feature flags are not included).
func Disabled(items []CatalogItem, flags map[string]Flag, tenant *uuid.UUID) (menus, actions []string) {
	for _, it := range items {
		if it.Kind == "workspace" || Enabled(it, flags, tenant) {
			continue
		}
		if it.Kind == "menu" {
			menus = append(menus, it.Key)
		} else {
			actions = append(actions, it.Key)
		}
	}
	sort.Strings(menus)
	sort.Strings(actions)
	return menus, actions
}

// KindOfKey derives the flag kind of a key from its form (menu./action. prefixes belong to the catalog).
func KindOfKey(key string) string {
	switch {
	case strings.HasPrefix(key, "menu."):
		return "menu"
	case strings.HasPrefix(key, "action."):
		return "action"
	}
	return "feature"
}
