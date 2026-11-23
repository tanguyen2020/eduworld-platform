package season

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
)

func track() Track {
	free := false
	t := Track{SeasonTrackID: uuid.New(), Code: "season.s1", SeasonID: uuid.New(), Name: liveops.Localized{"vi": "Mùa 1", "en": "Season 1"},
		PaidTrack: &free, Points: Points{PerTier: 100, DailyCap: 100, Sources: []liveops.Source{{SourceCode: liveops.SrcDailyQuest, Points: 20}}}}
	t.EndOfSeason.UnclaimedRewards = "auto_grant"
	for i := 1; i <= 30; i++ {
		t.Tiers = append(t.Tiers, Tier{Tier: i, IsMilestone: i%10 == 0, Rewards: []liveops.Reward{{"type": "currency", "currency_code": "xu", "amount": 10.0}}})
	}
	return t
}

// F077-AC-18, AC-19, AC-20: tiers every per_tier points up to the last tier; unclaimed tiers; never a paid track, never XP
// or a collection item in a tier.
func TestTrack(t *testing.T) {
	tr := track()
	if err := tr.Validate(); err != nil {
		t.Fatal(err)
	}
	if tr.TierFor(0) != 0 || tr.TierFor(99) != 0 || tr.TierFor(250) != 2 || tr.TierFor(100000) != 30 {
		t.Fatal("tiers")
	}
	if got := Unclaimed(3, []int{1}, []int{3}); len(got) != 1 || got[0] != 2 {
		t.Fatalf("unclaimed: %v", got)
	}
	for name, edit := range map[string]func(t *Track){
		"paid":         func(t *Track) { p := true; t.PaidTrack = &p },
		"paid missing": func(t *Track) { t.PaidTrack = nil },
		"xp reward":    func(t *Track) { t.Tiers[0].Rewards = []liveops.Reward{{"type": "xp", "amount": 10.0}} },
		"collection item": func(t *Track) {
			t.Tiers[0].Rewards = []liveops.Reward{{"type": "collection_item", "collection_item_id": "card"}}
		},
		"real currency": func(t *Track) {
			t.Tiers[0].Rewards = []liveops.Reward{{"type": "currency", "currency_code": "vnd", "amount": 10.0}}
		},
		"unknown source": func(t *Track) { t.Points.Sources = []liveops.Source{{SourceCode: "login", Points: 5}} },
		"tier gap":       func(t *Track) { t.Tiers[1].Tier = 5 },
	} {
		c := track()
		edit(&c)
		if err := c.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
