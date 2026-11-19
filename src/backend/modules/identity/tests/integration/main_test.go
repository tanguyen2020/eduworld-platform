//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// One migrated PostgreSQL for the package; each test works on its own copy of the databases.
func TestMain(m *testing.M) { os.Exit(testkit.RunWithSharedCluster(m)) }
