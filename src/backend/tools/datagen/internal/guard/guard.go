// Package guard is the refusal of datagen (F132-AC-07, TEST_STRATEGY §4, §5): it runs only against a test
// environment (src/backend/tools/envguard) and only creates tenants whose code starts with "perf", so generated
// accounts can never be mistaken for, or mixed with, real schools.
package guard

import (
	"fmt"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
)

// TenantPrefix is the mandatory prefix of every generated tenant code (perf-<n>).
const TenantPrefix = "perf"

// Check validates the target environment and the tenant prefix.
func Check(env, prefix string) error {
	if err := envguard.Check(env); err != nil {
		return err
	}
	if !strings.HasPrefix(prefix, TenantPrefix) {
		return fmt.Errorf("%w: tenant prefix %q must start with %q", envguard.ErrRefused, prefix, TenantPrefix)
	}
	return nil
}
