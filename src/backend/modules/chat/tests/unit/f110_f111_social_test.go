package unit

import (
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
)

// Delta F095 for the client F111 ("Vừa chơi cùng"): the pairs recorded for one shared activity — both directions of
// every two students of the event tenant; above 8 participants the same team only; never a participant of another
// tenant, never a student with itself; the context of a party activity (coop for quest, boss, coop_challenge).
func TestCoplayPairs(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	small := []social.Coplayer{{StudentID: a, TenantID: tenant, Team: -1}, {StudentID: b, TenantID: tenant, Team: -1},
		{StudentID: c, TenantID: other, Team: -1}, {StudentID: a, TenantID: tenant, Team: -1}}
	got := social.CoplayPairs(tenant, small)
	if len(got) != 2 || got[0] != (social.CoplayPair{Student: a, Other: b}) || got[1] != (social.CoplayPair{Student: b, Other: a}) {
		t.Fatalf("small: %+v", got)
	}
	var big []social.Coplayer
	for i := range 10 {
		big = append(big, social.Coplayer{StudentID: uuid.New(), TenantID: tenant, Team: i % 2})
	}
	if n := len(social.CoplayPairs(tenant, big)); n != 2*5*4 {
		t.Fatalf("big match: %d pairs", n)
	}
	noTeam := append([]social.Coplayer{}, big...)
	noTeam[0].Team = -1
	for _, p := range social.CoplayPairs(tenant, noTeam) {
		if p.Student == noTeam[0].StudentID || p.Other == noTeam[0].StudentID {
			t.Fatal("a participant without a team paired in a big match")
		}
	}
	if len(social.CoplayPairs(tenant, nil)) != 0 || len(social.CoplayPairs(tenant, small[:1])) != 0 {
		t.Fatal("pairs without two students")
	}
	for in, want := range map[string]string{"quest": social.CoplayCoop, "boss": social.CoplayCoop, "coop_challenge": social.CoplayCoop,
		"explore": social.CoplayParty, "learning_challenge": social.CoplayParty, "team_battle": social.CoplayParty} {
		if got := social.CoplayContext(in); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
}
