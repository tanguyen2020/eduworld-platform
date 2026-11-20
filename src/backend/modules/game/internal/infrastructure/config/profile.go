package config

import (
	"errors"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the game profile (owner: CHARACTER_SYSTEM §5).
const (
	DefaultEmoteWheelSlots    = 6                // emote_wheel_slots
	DefaultSavedOutfitsMax    = 3                // saved_outfits_max (DB CHECK slot BETWEEN 1 AND 3)
	DefaultNicknameMonthly    = 1                // nickname_change_monthly_limit (a school may only lower it)
	DefaultNicknameSuggestTTL = 10 * time.Minute // nickname_suggestion_ttl
)

// ProfileFrom reads the F070 configuration (T001): EW_GAME_EMOTE_WHEEL_SLOTS (emote_wheel_slots, 6, 1..6),
// EW_GAME_SAVED_OUTFITS_MAX (saved_outfits_max, 3, 1..3), EW_GAME_NICKNAME_CHANGE_MONTHLY_LIMIT
// (nickname_change_monthly_limit, 1 per calendar month in Asia/Ho_Chi_Minh; ref.schools settings may only lower it),
// EW_GAME_NICKNAME_SUGGESTION_TTL (nickname_suggestion_ttl, 10m, 1m..1h), EW_SUPPORTED_LOCALES (preferred_language).
func ProfileFrom(c *config.Config) (command.ProfileConfig, error) {
	out := command.ProfileConfig{
		EmoteWheelSlots: c.Int("EW_GAME_EMOTE_WHEEL_SLOTS", DefaultEmoteWheelSlots),
		SavedOutfitsMax: c.Int("EW_GAME_SAVED_OUTFITS_MAX", DefaultSavedOutfitsMax),
		Nickname: profile.NicknamePolicy{MonthlyLimit: c.Int("EW_GAME_NICKNAME_CHANGE_MONTHLY_LIMIT", DefaultNicknameMonthly),
			Location: Location()},
		SuggestionTTL:    c.Duration("EW_GAME_NICKNAME_SUGGESTION_TTL", DefaultNicknameSuggestTTL),
		SupportedLocales: c.List("EW_SUPPORTED_LOCALES"),
	}
	if len(out.SupportedLocales) == 0 {
		out.SupportedLocales = strings.Split(DefaultLocales, ",")
	}
	switch {
	case out.EmoteWheelSlots < 1 || out.EmoteWheelSlots > 6:
		return out, errors.New("game config: EW_GAME_EMOTE_WHEEL_SLOTS must be 1..6")
	case out.SavedOutfitsMax < 1 || out.SavedOutfitsMax > 3:
		return out, errors.New("game config: EW_GAME_SAVED_OUTFITS_MAX must be 1..3")
	case out.Nickname.MonthlyLimit < 0 || out.Nickname.MonthlyLimit > 31:
		return out, errors.New("game config: EW_GAME_NICKNAME_CHANGE_MONTHLY_LIMIT must be 0..31")
	case out.SuggestionTTL < time.Minute || out.SuggestionTTL > time.Hour:
		return out, errors.New("game config: EW_GAME_NICKNAME_SUGGESTION_TTL must be 1m..1h")
	}
	return out, nil
}
