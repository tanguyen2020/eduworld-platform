// Package runner executes the authz-matrix against a running environment (local stack or staging) and writes the
// report (F132 T026). It is a black-box client of the public contract: it never imports src/backend/modules.
package runner

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/cases"
)

// Fixtures describe the seed of the target environment: two tenants (A with schools A1, A2; B), one account per
// role (username only — passwords never live in the file) and path parameter values per scope.
// See fixtures.example.yaml. Synthetic data only (TEST_STRATEGY §4, AGENTS R16).
type Fixtures struct {
	Tenants struct {
		A string `yaml:"a"`
		B string `yaml:"b"`
	} `yaml:"tenants"`
	Accounts map[cases.Role]Account `yaml:"accounts"`
	// IDs maps a scope (a1 = caller's school, a2 = other school of tenant A, b = tenant B) to path parameter values by
	// parameter name. A missing value is a random UUID (the case is then marked synthetic_id in the report).
	IDs map[string]map[string]string `yaml:"ids"`
	// Bodies are request bodies by operationId for cases whose minimal body the validator rejects.
	Bodies map[string]any `yaml:"bodies"`
	// ClientVersion and Platform of the game login of the student account.
	ClientVersion string `yaml:"client_version"`
	Platform      string `yaml:"platform"`
}

// Account is the login of one role. Kind game logs in through /auth/game/login, portal through /auth/portal/login.
type Account struct {
	Username string `yaml:"username"`
	Kind     string `yaml:"kind"` // game | portal
}

// LoadFixtures reads and checks a fixtures file.
func LoadFixtures(path string) (Fixtures, error) {
	var f Fixtures
	b, err := os.ReadFile(path)
	if err != nil {
		return f, fmt.Errorf("runner: read fixtures: %w", err)
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return f, fmt.Errorf("runner: decode fixtures: %w", err)
	}
	if f.Tenants.A == "" || f.Tenants.B == "" || f.Tenants.A == f.Tenants.B {
		return f, fmt.Errorf("runner: fixtures need two distinct tenants a and b")
	}
	if f.ClientVersion == "" {
		f.ClientVersion = "1.0.0"
	}
	if f.Platform == "" {
		f.Platform = "web"
	}
	return f, nil
}

// scopeOf maps a relation to the fixture scope of its path parameter values.
func scopeOf(rel cases.Relation) string {
	switch rel {
	case cases.OtherSchoolSameTenant:
		return "a2"
	case cases.OtherTenant:
		return "b"
	}
	return "a1"
}

// Credentials supplies the secret of a role: a pre-issued access token (EW_AUTHZ_TOKEN_<ROLE>) or a password
// (EW_AUTHZ_PASSWORD_<ROLE>, or the file named by EW_AUTHZ_PASSWORD_FILE_<ROLE> — Docker/GitHub secret files).
type Credentials func(role cases.Role) (token, password string)

// EnvCredentials reads Credentials from the process environment.
func EnvCredentials(getenv func(string) string) Credentials {
	return func(role cases.Role) (string, string) {
		k := strings.ToUpper(string(role))
		if t := getenv("EW_AUTHZ_TOKEN_" + k); t != "" {
			return t, ""
		}
		if p := getenv("EW_AUTHZ_PASSWORD_" + k); p != "" {
			return "", p
		}
		if f := getenv("EW_AUTHZ_PASSWORD_FILE_" + k); f != "" {
			if b, err := os.ReadFile(f); err == nil {
				return "", strings.TrimSpace(string(b))
			}
		}
		return "", ""
	}
}
