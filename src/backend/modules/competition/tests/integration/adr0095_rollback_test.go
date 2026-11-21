//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// ADR-0095 mục 8 (WP-R): the rollback script recreates ref.enrollments, and the contract migration drops it again (backup
// plan step 4). On a fresh database no party row carries an old CHECK value, so the CHECKs are narrowed.
func TestADR0095RollbackRoundTrip(t *testing.T) {
	ctx := context.Background()
	testkit.RollbackRoundTrip(ctx, t, "competition",
		"src/backend/modules/competition/migrations/20261231000004_adr0095_drop_enrollments.sql",
		"scripts/db/rollback/adr-0095-competition.sql", []string{"ref.enrollments"}, nil,
		map[string][]string{"parties_type_chk": {"study_group"}, "parties_visibility_chk": {"class"}, "party_policies_scope_chk": {"class"}})
}
