package profile_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
)

type palette struct{}

func (palette) HasBodyType(c string) bool  { return c == "kid_a" || c == "kid_b" }
func (palette) HasSkinTone(c string) bool  { return c == "tone_01" || c == "tone_02" }
func (palette) HasHairColor(c string) bool { return c == "hair_col_01" || c == "hair_col_03" }

var items = map[string]profile.Item{
	"hair_short_01": {Kind: "hair", EquipSlot: "hair"}, "hair_gold_01": {Kind: "hair", EquipSlot: "hair"},
	"outfit_explorer_01": {Kind: "outfit", EquipSlot: "outfit"}, "outfit_knight_01": {Kind: "outfit", EquipSlot: "outfit"},
	"acc_cap_star": {Kind: "accessory", EquipSlot: "head"}, "acc_backpack_owl": {Kind: "accessory", EquipSlot: "back"},
	"acc_glasses_round": {Kind: "accessory", EquipSlot: "face"},
	"emote_wave":        {Kind: "emote", EquipSlot: "emote"}, "emote_clap": {Kind: "emote", EquipSlot: "emote"},
	"emote_dance": {Kind: "emote", EquipSlot: "emote"}, "emote_celebrate": {Kind: "emote", EquipSlot: "emote"},
	"emote_thank": {Kind: "emote", EquipSlot: "emote"}, "emote_apologize": {Kind: "emote", EquipSlot: "emote"},
	"emote_bow":  {Kind: "emote", EquipSlot: "emote"},
	"pet_cat_01": {Kind: "pet", EquipSlot: "pet"}, "title_star": {Kind: "title", EquipSlot: "title"},
	"frame_gold": {Kind: "frame", EquipSlot: "frame"}, "badge_math": {Kind: "badge", EquipSlot: "badge"},
	"badge_read": {Kind: "badge", EquipSlot: "badge"},
}

// owned: every item except outfit_knight_01 and hair_gold_01 (not in the inventory, not free).
func owned(id string) bool { return id != "outfit_knight_01" && id != "hair_gold_01" }

var rules = profile.Rules{EmoteWheelSlots: 6}

func base() profile.AvatarConfig {
	return profile.AvatarConfig{SchemaVersion: 1, BodyType: "kid_a", SkinTone: "tone_01",
		Hair: profile.Hair{ItemID: "hair_short_01", Color: "hair_col_01"}, Outfit: profile.ItemRef{ItemID: "outfit_explorer_01"}}
}

// T012 (F070-AC-04, AC-13, AC-14): every valid combination is accepted; each invalid one is refused with its error.
func TestAvatarConfigValidate(t *testing.T) {
	full := base()
	full.Accessories = []profile.Accessory{{Slot: "back", ItemID: "acc_backpack_owl"}, {Slot: "head", ItemID: "acc_cap_star"},
		{Slot: "face", ItemID: "acc_glasses_round"}}
	full.EmoteSlots = []string{"emote_wave", "emote_clap", "emote_dance", "emote_celebrate", "emote_thank", "emote_apologize"}
	full.Pet, full.Title, full.Frame = "pet_cat_01", "title_star", "frame_gold"
	full.BadgeShowcase = []string{"badge_math", "badge_read"}
	for name, c := range map[string]profile.AvatarConfig{"minimal": base(), "full": full} {
		if err := c.Validate(rules, palette{}, items, owned); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	var invalid *profile.InvalidError
	var notOwned *profile.NotOwnedError
	var slot *profile.SlotError
	cases := []struct {
		name   string
		mutate func(c *profile.AvatarConfig)
		want   any
	}{
		{"schema version", func(c *profile.AvatarConfig) { c.SchemaVersion = 2 }, &invalid},
		{"body type outside the catalog", func(c *profile.AvatarConfig) { c.BodyType = "kid_z" }, &invalid},
		{"skin tone outside the catalog", func(c *profile.AvatarConfig) { c.SkinTone = "tone_99" }, &invalid},
		{"hair colour outside the catalog", func(c *profile.AvatarConfig) { c.Hair.Color = "hair_col_99" }, &invalid},
		{"malformed code", func(c *profile.AvatarConfig) { c.Hair.ItemID = "Hair Short" }, &invalid},
		{"outfit not owned", func(c *profile.AvatarConfig) { c.Outfit.ItemID = "outfit_knight_01" }, &notOwned},
		{"hair not owned", func(c *profile.AvatarConfig) { c.Hair.ItemID = "hair_gold_01" }, &notOwned},
		{"unknown item", func(c *profile.AvatarConfig) { c.Pet = "pet_unknown" }, &notOwned},
		{"accessory in another slot", func(c *profile.AvatarConfig) {
			c.Accessories = []profile.Accessory{{Slot: "back", ItemID: "acc_cap_star"}}
		}, &slot},
		{"two items in one slot", func(c *profile.AvatarConfig) {
			c.Accessories = []profile.Accessory{{Slot: "head", ItemID: "acc_cap_star"}, {Slot: "head", ItemID: "acc_cap_star"}}
		}, &slot},
		{"unknown accessory slot", func(c *profile.AvatarConfig) {
			c.Accessories = []profile.Accessory{{Slot: "tail", ItemID: "acc_cap_star"}}
		}, &slot},
		{"outfit as hair", func(c *profile.AvatarConfig) { c.Hair.ItemID = "outfit_explorer_01" }, &slot},
		{"emote as pet", func(c *profile.AvatarConfig) { c.Pet = "emote_wave" }, &slot},
		{"more emotes than the wheel", func(c *profile.AvatarConfig) {
			c.EmoteSlots = []string{"emote_wave", "emote_clap", "emote_dance", "emote_celebrate", "emote_thank", "emote_apologize", "emote_bow"}
		}, &slot},
		{"same emote twice", func(c *profile.AvatarConfig) { c.EmoteSlots = []string{"emote_wave", "emote_wave"} }, &slot},
		{"item as emote", func(c *profile.AvatarConfig) { c.EmoteSlots = []string{"acc_cap_star"} }, &slot},
		{"four badges", func(c *profile.AvatarConfig) {
			c.BadgeShowcase = []string{"badge_math", "badge_read", "badge_math", "badge_read"}
		}, &slot},
		{"same badge twice", func(c *profile.AvatarConfig) { c.BadgeShowcase = []string{"badge_math", "badge_math"} }, &slot},
		{"title in the frame slot", func(c *profile.AvatarConfig) { c.Frame = "title_star" }, &slot},
	}
	for _, tc := range cases {
		c := base()
		tc.mutate(&c)
		err := c.Validate(rules, palette{}, items, owned)
		if err == nil || !errors.As(err, tc.want) {
			t.Errorf("%s: got %v", tc.name, err)
		}
	}
}

// A wheel smaller than 6 (emote_wheel_slots from the configuration) is enforced by the domain.
func TestEmoteWheelSlotsFromConfig(t *testing.T) {
	c := base()
	c.EmoteSlots = []string{"emote_wave", "emote_clap", "emote_dance"}
	var slot *profile.SlotError
	if err := c.Validate(profile.Rules{EmoteWheelSlots: 2}, palette{}, items, owned); !errors.As(err, &slot) {
		t.Fatalf("got %v", err)
	}
}

// Normalization fills absent emotes with the defaults (F070-AC-19), keeps an explicit empty wheel, orders the
// accessories, and gives the same hash to the same appearance.
func TestNormalizedAndHash(t *testing.T) {
	defaults := []string{"emote_wave", "emote_clap"}
	a := base()
	a.Accessories = []profile.Accessory{{Slot: "back", ItemID: "acc_backpack_owl"}, {Slot: "head", ItemID: "acc_cap_star"}}
	n := a.Normalized(defaults)
	if len(n.EmoteSlots) != 2 || n.EmoteSlots[0] != "emote_wave" || n.Accessories[0].Slot != "head" {
		t.Fatalf("normalized %+v", n)
	}
	empty := base()
	empty.EmoteSlots = []string{}
	if e := empty.Normalized(defaults); len(e.EmoteSlots) != 0 {
		t.Fatalf("explicit empty wheel replaced: %v", e.EmoteSlots)
	}
	b := base()
	b.Accessories = []profile.Accessory{{Slot: "head", ItemID: "acc_cap_star"}, {Slot: "back", ItemID: "acc_backpack_owl"}}
	h1, _ := n.Hash()
	h2, _ := b.Normalized(defaults).Hash()
	if h1 != h2 || len(h1) != 64 {
		t.Fatalf("hash %s vs %s", h1, h2)
	}
	b.SkinTone = "tone_02"
	if h3, _ := b.Normalized(defaults).Hash(); h3 == h1 {
		t.Fatal("different appearance, same hash")
	}
}

// avatar_created_at is set once (F070-AC-03); the same config is no change; another config is an update.
func TestSetAvatarOnce(t *testing.T) {
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	p := profile.New(uuid.New(), uuid.New(), uuid.New(), now)
	if !p.NeedsSetup() {
		t.Fatal("new profile must need setup")
	}
	c := base().Normalized(nil)
	h, _ := c.Hash()
	if p.SetAvatar(c, h, now) != profile.AvatarCreated {
		t.Fatal("first avatar")
	}
	created := *p.AvatarCreatedAt
	if p.SetAvatar(c, h, now.Add(time.Hour)) != profile.AvatarUnchanged || !p.AvatarCreatedAt.Equal(created) {
		t.Fatal("same avatar")
	}
	c2 := c
	c2.SkinTone = "tone_02"
	h2, _ := c2.Hash()
	if p.SetAvatar(c2, h2, now.Add(2*time.Hour)) != profile.AvatarUpdated || !p.AvatarCreatedAt.Equal(created) {
		t.Fatal("update")
	}
	if !p.NeedsSetup() {
		t.Fatal("no nickname yet: setup still required")
	}
	if ch := p.SetNickname("Cú Mèo Chăm Chỉ 27"); !ch.Changed || ch.Reason != profile.ReasonInitial {
		t.Fatalf("initial %+v", ch)
	}
	if p.NeedsSetup() {
		t.Fatal("avatar and nickname: ready")
	}
	if ch := p.SetNickname("Cú Mèo Chăm Chỉ 27"); ch.Changed {
		t.Fatal("same nickname is no change")
	}
	if ch := p.SetNickname("Sóc Nâu Vui Vẻ 12"); ch.Reason != profile.ReasonPlayerChange || *ch.Old != "Cú Mèo Chăm Chỉ 27" {
		t.Fatalf("player change %+v", ch)
	}
}

// nickname_change_monthly_limit per calendar month in ICT; a school may only lower it (F070-AC-11).
func TestNicknamePolicy(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	p := profile.NicknamePolicy{MonthlyLimit: 1, Location: ict}
	now := time.Date(2026, 11, 30, 18, 0, 0, 0, time.UTC) // 1 Dec 01:00 ICT
	if got := p.MonthStart(now); !got.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, ict)) {
		t.Fatalf("month start %v", got)
	}
	if got := p.NextMonth(now); !got.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, ict)) {
		t.Fatalf("next month %v", got)
	}
	if !p.Allows(0) || p.Allows(1) {
		t.Fatal("limit 1")
	}
	zero, three := 0, 3
	if p.Effective(&zero).Allows(0) || !p.Effective(&three).Allows(0) || p.Effective(&three).MonthlyLimit != 1 {
		t.Fatal("a school only tightens")
	}
}
