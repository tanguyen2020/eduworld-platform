package config

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// T001: the F070 parameters come from the configuration with the CONFIG_DEFAULTS values, never hard-coded in use cases.
func TestProfileFrom(t *testing.T) {
	c, err := ProfileFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	if c.EmoteWheelSlots != 6 || c.SavedOutfitsMax != 3 || c.Nickname.MonthlyLimit != 1 || c.SuggestionTTL != 10*time.Minute ||
		c.Nickname.Location == nil || len(c.SupportedLocales) != 2 {
		t.Fatalf("defaults: %+v", c)
	}
	t.Setenv("EW_GAME_NICKNAME_CHANGE_MONTHLY_LIMIT", "2")
	t.Setenv("EW_GAME_NICKNAME_SUGGESTION_TTL", "5m")
	if c, err = ProfileFrom(config.Load()); err != nil || c.Nickname.MonthlyLimit != 2 || c.SuggestionTTL != 5*time.Minute {
		t.Fatalf("overrides: %+v %v", c, err)
	}
	for k, v := range map[string]string{"EW_GAME_EMOTE_WHEEL_SLOTS": "7", "EW_GAME_SAVED_OUTFITS_MAX": "4",
		"EW_GAME_NICKNAME_CHANGE_MONTHLY_LIMIT": "-1", "EW_GAME_NICKNAME_SUGGESTION_TTL": "10s"} {
		t.Run(k, func(t *testing.T) {
			t.Setenv(k, v)
			if _, err := ProfileFrom(config.Load()); err == nil {
				t.Fatalf("%s=%s accepted", k, v)
			}
		})
	}
}
