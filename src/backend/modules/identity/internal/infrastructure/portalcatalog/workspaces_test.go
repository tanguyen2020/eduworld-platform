package portalcatalog

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// platformWorkspaces are the workspaces of platform staff only (ADMIN_PORTAL §1, ADR-0027 §3, ADR-0090, AGENTS R20).
var platformWorkspaces = []string{"game_ops", "live_ops", "platform_admin"}

func catalogFile(t *testing.T) []byte {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "..", "..", "..", "..", "..", "specs", "portal", "menu-catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// E2E Portal v2 finding 3: a menu of a platform workspace requires platform-only permissions, so no tenant role (and
// no permission a tenant role may hold) ever makes a platform workspace or menu visible (getMyPermissions).
func TestPlatformWorkspacesNeedPlatformPermissions(t *testing.T) {
	b := catalogFile(t)
	var f struct {
		Workspaces []struct {
			Key   string `yaml:"key"`
			Menus []struct {
				Key                 string   `yaml:"key"`
				RequiredPermissions []string `yaml:"required_permissions"`
			} `yaml:"menus"`
		} `yaml:"workspaces"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	for _, ws := range f.Workspaces {
		if !slices.Contains(platformWorkspaces, ws.Key) {
			continue
		}
		for _, m := range ws.Menus {
			for _, p := range m.RequiredPermissions {
				if !authz.Permission(p).PlatformOnly() {
					t.Errorf("%s: %s is not a platform-only permission (a tenant role may hold it)", m.Key, p)
				}
			}
		}
	}
	w, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range authz.SystemRoles() {
		if r.ScopeKind != authz.RoleScopeTenant {
			continue
		}
		perms := make([]string, len(r.Permissions))
		for i, p := range r.Permissions {
			perms[i] = string(p)
		}
		for _, ws := range w.Workspaces(perms) {
			if slices.Contains(platformWorkspaces, ws) {
				t.Errorf("tenant role %s sees platform workspace %s", r.Name, ws)
			}
		}
	}
}
