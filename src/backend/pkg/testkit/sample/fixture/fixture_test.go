package fixture_test

import (
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/fixture"
)

// The static policies of the fixture are exactly what tenancy.RLSPolicies generates.
func TestFixtureRLSMatchesHelper(t *testing.T) {
	for table, kind := range map[string]tenancy.TableKind{"sample.widgets": tenancy.KindT, "sample.widget_logs": tenancy.KindTA,
		"sample.templates": tenancy.KindTStar, "sample.ref_widgets": tenancy.KindT} {
		ddl, err := tenancy.RLSPolicies(table, kind)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(fixture.SQL, ddl) {
			t.Fatalf("fixture RLS of %s differs from tenancy.RLSPolicies:\n%s", table, ddl)
		}
	}
}
