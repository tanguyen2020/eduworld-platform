//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// ADR-0095 mục 8 (WP-R): the rollback script widens the chat CHECKs again and the contract migration narrows them once
// more on a fresh database, where no channel or policy row carries class/study_group (backup plan step 4).
func TestADR0095RollbackRoundTrip(t *testing.T) {
	testkit.RollbackRoundTrip(context.Background(), t, "chat",
		"src/backend/modules/chat/migrations/20261231000003_adr0095_check_values.sql", "scripts/db/rollback/adr-0095-chat.sql",
		nil, nil, map[string][]string{"channels_type_chk": {"class", "study_group"}, "chat_policies_scope_chk": {"class"}})
}
