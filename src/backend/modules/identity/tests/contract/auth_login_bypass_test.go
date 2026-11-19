package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F013-AC-30 (architecture part): the bypass reason auth_login and the function identity.memberships_for_user
// are used only by the login membership reader of module identity (routes /auth/game/*, /auth/session,
// login-credentials:verify go through it). Any other Go file of the backend that names them fails here.
func TestAuthLoginBypassOnlyOnLoginPath(t *testing.T) {
	backend := filepath.Join("..", "..", "..", "..")
	allowed := map[string]bool{
		filepath.ToSlash(filepath.Join("modules", "identity", "internal", "infrastructure", "persistence", "read", "login_membership_reader.go")): true,
		"pkg/tenancy/bypass.go": true, // the closed list itself
		"pkg/entx/uow.go":       true, // pool choice: auth_login runs as <module>_app
	}
	var offenders []string
	err := filepath.WalkDir(backend, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == "tests" || n == "testdata" || n == "node_modules" || n == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(backend, path)
		rel = filepath.ToSlash(rel)
		if allowed[rel] {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue // documentation may name them
			}
			if strings.Contains(line, "ReasonAuthLogin") || strings.Contains(line, `"auth_login"`) || strings.Contains(line, "memberships_for_user") {
				offenders = append(offenders, rel)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("auth_login / memberships_for_user outside the login path: %v", offenders)
	}
}
