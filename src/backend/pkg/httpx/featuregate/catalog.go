package featuregate

import (
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// Action is one action of the portal menu catalog (specs/portal/menu-catalog.yaml, ADR-0090).
type Action struct {
	Key        string
	Menu       string // parent menu key
	CanDisable bool   // false = protected item, never blocked (ADR-0090 §2)
}

// Catalog indexes the actions of the menu catalog by key.
type Catalog struct {
	actions map[string]Action
}

type catalogFile struct {
	Workspaces []struct {
		Key   string `yaml:"key"`
		Menus []struct {
			Key        string `yaml:"key"`
			CanDisable *bool  `yaml:"can_disable"`
			Actions    []struct {
				Key        string `yaml:"key"`
				CanDisable *bool  `yaml:"can_disable"`
			} `yaml:"actions"`
		} `yaml:"menus"`
	} `yaml:"workspaces"`
}

// LoadCatalog reads the menu catalog file (path from configuration of the composition root).
func LoadCatalog(path string) (*Catalog, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path comes from the binary's configuration, not from a request
	if err != nil {
		return nil, fmt.Errorf("featuregate: read catalog: %w", err)
	}
	return ParseCatalog(b)
}

// ParseCatalog parses the menu catalog. It keeps what the gate needs (action → parent menu, can_disable)
// and refuses a catalog the gate could misread: duplicate keys, missing can_disable, or an action whose
// can_disable differs from its menu (the full contract check is scripts/specs/check-portal-catalog.mjs).
func ParseCatalog(data []byte) (*Catalog, error) {
	var f catalogFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("featuregate: parse catalog: %w", err)
	}
	c := &Catalog{actions: map[string]Action{}}
	menus := map[string]bool{}
	for _, ws := range f.Workspaces {
		for _, m := range ws.Menus {
			if m.Key == "" || m.CanDisable == nil {
				return nil, fmt.Errorf("featuregate: catalog menu %q needs key and can_disable", m.Key)
			}
			if menus[m.Key] {
				return nil, fmt.Errorf("featuregate: duplicate catalog menu %s", m.Key)
			}
			menus[m.Key] = true
			for _, a := range m.Actions {
				if a.Key == "" || a.CanDisable == nil {
					return nil, fmt.Errorf("featuregate: catalog action %q needs key and can_disable", a.Key)
				}
				if _, dup := c.actions[a.Key]; dup {
					return nil, fmt.Errorf("featuregate: duplicate catalog action %s", a.Key)
				}
				if *a.CanDisable != *m.CanDisable {
					return nil, fmt.Errorf("featuregate: action %s can_disable differs from menu %s", a.Key, m.Key)
				}
				c.actions[a.Key] = Action{Key: a.Key, Menu: m.Key, CanDisable: *a.CanDisable}
			}
		}
	}
	if len(c.actions) == 0 {
		return nil, errors.New("featuregate: catalog has no action")
	}
	return c, nil
}

// Action returns the catalog entry of an action key.
func (c *Catalog) Action(key string) (Action, bool) {
	a, ok := c.actions[key]
	return a, ok
}
