package portalcatalog

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
)

type fullFile struct {
	Version    string `yaml:"catalog_version"`
	Workspaces []struct {
		Key      string `yaml:"key"`
		LabelKey string `yaml:"label_key"`
		Title    string `yaml:"title"`
		Order    int    `yaml:"order"`
		Menus    []struct {
			Key                 string   `yaml:"key"`
			LabelKey            string   `yaml:"label_key"`
			Title               string   `yaml:"title"`
			Path                string   `yaml:"path"`
			Order               int      `yaml:"order"`
			RequiredPermissions []string `yaml:"required_permissions"`
			CanDisable          *bool    `yaml:"can_disable"`
			Actions             []struct {
				Key                 string   `yaml:"key"`
				LabelKey            string   `yaml:"label_key"`
				Title               string   `yaml:"title"`
				RequiredPermissions []string `yaml:"required_permissions"`
				OperationIDs        []string `yaml:"operation_ids"`
				CanDisable          *bool    `yaml:"can_disable"`
			} `yaml:"actions"`
		} `yaml:"menus"`
	} `yaml:"workspaces"`
}

// Items parses every workspace, menu and action of the catalog (F015 T040, ADR-0090) in parent-first order,
// with the catalog_version of the file. A missing can_disable means true; an action of a protected menu is
// protected too.
func Items(b []byte) (version string, items []settings.CatalogItem, err error) {
	var f fullFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return "", nil, fmt.Errorf("portalcatalog: %w", err)
	}
	if f.Version == "" {
		return "", nil, errors.New("portalcatalog: catalog_version missing")
	}
	can := func(p *bool) bool { return p == nil || *p }
	for _, ws := range f.Workspaces {
		items = append(items, settings.CatalogItem{Key: ws.Key, Kind: "workspace", Workspace: ws.Key, LabelKey: ws.LabelKey, Title: ws.Title,
			Order: ws.Order, CanDisable: true})
		for _, m := range ws.Menus {
			mc := can(m.CanDisable)
			items = append(items, settings.CatalogItem{Key: m.Key, Kind: "menu", Workspace: ws.Key, Parent: ws.Key, Path: m.Path, LabelKey: m.LabelKey,
				Title: m.Title, Order: m.Order, RequiredPermissions: m.RequiredPermissions, CanDisable: mc})
			for i, a := range m.Actions {
				items = append(items, settings.CatalogItem{Key: a.Key, Kind: "action", Workspace: ws.Key, Parent: m.Key, LabelKey: a.LabelKey,
					Title: a.Title, Order: i, RequiredPermissions: a.RequiredPermissions, OperationIDs: a.OperationIDs, CanDisable: mc && can(a.CanDisable)})
			}
		}
	}
	return f.Version, items, nil
}
