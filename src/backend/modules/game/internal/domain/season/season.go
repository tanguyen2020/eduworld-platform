// Package season is the domain of the free season track of module game (F077, EVENT_SEASON_SYSTEM §7): one track per
// season, never paid (ADR-0049), season points from the catalog of sources under a daily cap, a tier every per_tier
// points, the tiers a student claims and the tiers granted when the season ends. The season calendar belongs to module
// competition (projection ref.seasons). Pure: no I/O.
package season

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
)

// ErrInvalid is a track version that cannot be published (422 VALIDATION_FAILED naming the field).
var ErrInvalid = errors.New("season: invalid track")

// FieldError names the invalid field.
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// RewardTypes are the rewards of a tier (F077-AC-20: cosmetic items, Xu, badges, titles — no XP, no collection item).
var RewardTypes = []string{"currency", "item", "badge", "title"}

// CosmeticKinds are the item kinds a tier may give (cosmetic only: never a learning or competition advantage).
var CosmeticKinds = []string{"hair", "outfit", "accessory", "emote", "pet", "title", "frame", "badge", "decoration", "consumable_cosmetic"}

// Tier is one tier of the track.
type Tier struct {
	Tier        int              `json:"tier"`
	IsMilestone bool             `json:"is_milestone,omitempty"`
	Rewards     []liveops.Reward `json:"rewards"`
}

// Points are the season points of the track.
type Points struct {
	PerTier  int              `json:"per_tier"`
	DailyCap int              `json:"daily_cap"`
	Sources  []liveops.Source `json:"sources"`
}

// Track is one season track version (season-track.schema.json without the server fields).
type Track struct {
	SeasonTrackID uuid.UUID         `json:"season_track_id"`
	Code          string            `json:"code"`
	SeasonID      uuid.UUID         `json:"season_id"`
	Name          liveops.Localized `json:"name"`
	ThemeCode     string            `json:"theme_code,omitempty"`
	PaidTrack     *bool             `json:"paid_track"`
	Points        Points            `json:"points"`
	Tiers         []Tier            `json:"tiers"`
	EndOfSeason   struct {
		UnclaimedRewards string `json:"unclaimed_rewards"`
	} `json:"end_of_season"`
}

// Parse reads a track version payload.
func Parse(raw []byte) (Track, error) {
	var t Track
	if err := json.Unmarshal(raw, &t); err != nil {
		return t, invalid("payload", "%v", err)
	}
	return t, nil
}

// Validate checks a track (F077-AC-20): never paid, tiers 1..n in order, rewards of the tier types only, Xu only, known
// sources; unclaimed rewards are granted at the end of the season.
func (t Track) Validate() error {
	if t.PaidTrack == nil || *t.PaidTrack {
		return invalid("paid_track", "a season track is always free")
	}
	if strings.TrimSpace(t.Name["vi"]) == "" || strings.TrimSpace(t.Name["en"]) == "" {
		return invalid("name", "vi and en required")
	}
	if t.EndOfSeason.UnclaimedRewards != "auto_grant" {
		return invalid("end_of_season.unclaimed_rewards", "auto_grant only")
	}
	if t.Points.PerTier < 1 || t.Points.DailyCap < 1 {
		return invalid("points", "per_tier and daily_cap at least 1")
	}
	seen := map[string]bool{}
	for i, s := range t.Points.Sources {
		if !slices.Contains(liveops.SourceCodes, s.SourceCode) || seen[s.SourceCode] || s.Points < 1 {
			return invalid(fmt.Sprintf("points.sources/%d", i), "unknown or repeated source")
		}
		seen[s.SourceCode] = true
	}
	if len(t.Tiers) == 0 {
		return invalid("tiers", "at least one tier")
	}
	for i, tr := range t.Tiers {
		f := fmt.Sprintf("tiers/%d", i)
		if tr.Tier != i+1 {
			return invalid(f+"/tier", "tiers are 1..n in order")
		}
		if len(tr.Rewards) == 0 {
			return invalid(f+"/rewards", "at least one reward")
		}
		if err := liveops.ValidRewards(f+"/rewards", tr.Rewards, RewardTypes); err != nil {
			return &FieldError{Field: f + "/rewards", Reason: err.Error()}
		}
	}
	return nil
}

// MaxTier is the last tier.
func (t Track) MaxTier() int { return len(t.Tiers) }

// TierFor is the tier of points (one tier every per_tier points, at most the last tier).
func (t Track) TierFor(points int) int {
	if t.Points.PerTier < 1 || points <= 0 {
		return 0
	}
	return min(points/t.Points.PerTier, t.MaxTier())
}

// Rewards returns the rewards of tier (nil when unknown).
func (t Track) Rewards(tier int) []liveops.Reward {
	if tier < 1 || tier > len(t.Tiers) {
		return nil
	}
	return t.Tiers[tier-1].Rewards
}

// Unclaimed returns the tiers 1..reached neither claimed nor granted (granted when the season ends, F077-AC-20).
func Unclaimed(reached int, claimed, granted []int) []int {
	var out []int
	for tier := 1; tier <= reached; tier++ {
		if !slices.Contains(claimed, tier) && !slices.Contains(granted, tier) {
			out = append(out, tier)
		}
	}
	return out
}
