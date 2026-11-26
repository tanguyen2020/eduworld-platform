package unit

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// Delta F102: PartyMember.level comes from the public card (ref.player_profiles); a member whose card is not projected
// yet has no level (never a default 1 presented as real, never a failed read) and no member carries a real name,
// school or grade (R16).
func TestPartyBodyLevel(t *testing.T) {
	known, unknown := uuid.New(), uuid.New()
	now := time.Now()
	nick := "Meo"
	p := party.Party{ID: uuid.New(), Name: "Nhóm", LeaderID: known, Status: party.StatusOpen, CreatedAt: now, UpdatedAt: now,
		Members: []party.Member{{StudentID: known, Role: "leader", JoinedAt: now}, {StudentID: unknown, Role: "member", JoinedAt: now}}}
	body := view.NewParty(p, map[uuid.UUID]port.Card{known: {UserID: known, Nickname: &nick, Level: 12}}, nil).Body()
	members := body["members"].([]map[string]any)
	if members[0]["level"] != 12 {
		t.Fatalf("known card: %v", members[0])
	}
	if _, ok := members[1]["level"]; ok {
		t.Fatalf("level without card: %v", members[1])
	}
	for _, m := range members {
		for _, k := range []string{"real_name", "full_name", "school_id", "grade_level", "grade"} {
			if _, ok := m[k]; ok {
				t.Fatalf("member exposes %s (R16): %v", k, m)
			}
		}
	}
}
