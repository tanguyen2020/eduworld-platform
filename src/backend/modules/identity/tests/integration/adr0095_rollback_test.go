//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// ADR-0095 mục 8 (WP-R): the rollback script recreates org.classes and org.enrollments, and the contract migration
// drops them again (backup plan step 4).
func TestADR0095RollbackRoundTrip(t *testing.T) {
	testkit.RollbackRoundTrip(context.Background(), t, "identity",
		"src/backend/modules/identity/migrations/20261201000001_adr0095_drop_class_enrollment.sql",
		"scripts/db/rollback/adr-0095-identity.sql",
		[]string{"org.classes", "org.enrollments", "org.classes_live", "org.enrollments_live"}, nil)
}
