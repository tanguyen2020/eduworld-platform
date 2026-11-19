// Package portalcatalog reads the Admin Portal menu catalog (specs/portal/menu-catalog.yaml, ADR-0090) for
// module identity: the workspaces a set of permissions makes visible (getMyPermissions).
package portalcatalog

import (
	"fmt"
	"os"
	"slices"

	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

type file struct {
	Workspaces []struct {
		Key   string `yaml:"key"`
		Menus []struct {
			RequiredPermissions []string `yaml:"required_permissions"`
		} `yaml:"menus"`
	} `yaml:"workspaces"`
}

// Workspaces implements port.WorkspaceCatalog.
type Workspaces struct {
	order []string
	perms map[string]map[string]bool // workspace → any-of permissions of its menus
}

var _ port.WorkspaceCatalog = (*Workspaces)(nil)

// Load reads the catalog file (path from configuration, like the OpenAPI spec path).
func Load(path string) (*Workspaces, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("portalcatalog: %w", err)
	}
	return Parse(b)
}

// Parse builds the catalog from YAML.
func Parse(b []byte) (*Workspaces, error) {
	var f file
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("portalcatalog: %w", err)
	}
	w := &Workspaces{perms: map[string]map[string]bool{}}
	for _, ws := range f.Workspaces {
		if ws.Key == "" {
			return nil, fmt.Errorf("portalcatalog: workspace without key")
		}
		w.order = append(w.order, ws.Key)
		set := map[string]bool{}
		for _, m := range ws.Menus {
			for _, p := range m.RequiredPermissions {
				set[p] = true
			}
		}
		w.perms[ws.Key] = set
	}
	return w, nil
}

// Workspaces returns the visible workspaces in catalog order.
func (w *Workspaces) Workspaces(permissions []string) []string {
	out := []string{}
	for _, k := range w.order {
		if slices.ContainsFunc(permissions, func(p string) bool { return w.perms[k][p] }) {
			out = append(out, k)
		}
	}
	return out
}
