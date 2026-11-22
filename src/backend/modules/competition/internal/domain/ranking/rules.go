// Package ranking is the pure domain of Game Rating (F106, COMPETITION_SYSTEM §7–§8, ADR-0018): the Glicko-2 update
// (Illinois), the weekly "no games" RD step, provisional status, tiers with hysteresis, the Competition Points table
// and the season soft reset. Every number comes from the published Ranking Rules (config_type ranking-rules), never
// from code. No SQL, no Valkey.
package ranking

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// Modes that carry a Game Rating (COMPETITION_SYSTEM §7.1).
const (
	ModeBattle1v1  = "battle_1v1"
	ModeTeamBattle = "team_battle"
	ModeTimedQuiz  = "timed_quiz"
)

// RatedModes are the modes with a rating.
var RatedModes = []string{ModeBattle1v1, ModeTeamBattle, ModeTimedQuiz}

// ErrRules is a Ranking Rules payload out of range.
var ErrRules = errors.New("ranking: invalid ranking rules")

// Glicko2Params is glicko2_params (CONFIG_DEFAULTS): r0, RD0, σ0, τ, RD bounds, Illinois ε.
type Glicko2Params struct {
	R0      float64 `json:"r0"`
	RD0     float64 `json:"rd0"`
	Sigma0  float64 `json:"sigma0"`
	Tau     float64 `json:"tau"`
	RDMin   float64 `json:"rd_min"`
	RDMax   float64 `json:"rd_max"`
	Epsilon float64 `json:"epsilon"`
}

// Provisional is rating_provisional: fewer than MinGames games or RD above MaxRD.
type Provisional struct {
	MinGames int     `json:"min_games"`
	MaxRD    float64 `json:"max_rd"`
}

// TierLevel is one tier: from MinRating (nil = lowest tier) with its names per locale.
type TierLevel struct {
	Code      string            `json:"code"`
	MinRating *float64          `json:"min_rating"`
	Names     map[string]string `json:"names"`
}

// Tiers is tiers: ascending levels and the hysteresis of a demotion.
type Tiers struct {
	Hysteresis float64     `json:"hysteresis"`
	Levels     []TierLevel `json:"levels"`
}

// SoftReset is season_soft_reset: r_new = r0 + factor·(r_old − r0), RD_new = max(RD_old, rd_floor), σ kept.
type SoftReset struct {
	Factor  float64 `json:"factor"`
	RDFloor float64 `json:"rd_floor"`
}

// PointTable is the Competition Points table and competition_points_daily_cap (COMPETITION_SYSTEM §8.3).
type PointTable struct {
	Completed    int `json:"completed"`
	Win          int `json:"win"`
	Draw         int `json:"draw"`
	Correct      int `json:"correct"`
	CorrectMax   int `json:"correct_max"`
	Upset        int `json:"upset"`
	Abandon      int `json:"abandon"`
	DailyCap     int `json:"daily_cap"`
	DailyMatches int `json:"daily_matches"`
}

// EffortTable is effort_points (COMPETITION_SYSTEM §9.2).
type EffortTable struct {
	Activity         int `json:"activity"`
	ActivityDailyMax int `json:"activity_daily_max"`
	Review           int `json:"review"`
	ActiveDay        int `json:"active_day"`
	DailyCap         int `json:"daily_cap"`
}

// BandVisibility is the leaderboard_visibility of one grade band.
type BandVisibility struct {
	PersonalRank bool `json:"personal_rank"`
	Top          int  `json:"top"`
	Percentile   bool `json:"percentile"`
	HideLast     bool `json:"hide_last"`
	Around       int  `json:"around"`
}

// Visibility is leaderboard_visibility per grade band (COMPETITION_SYSTEM §9.4).
type Visibility struct {
	G13 BandVisibility `json:"g1_3"`
	G45 BandVisibility `json:"g4_5"`
	G69 BandVisibility `json:"g6_9"`
}

// Rules is the published Ranking Rules (competition.game_config_versions, config_type ranking-rules).
type Rules struct {
	SchemaVersion int           `json:"schema_version"`
	Glicko2       Glicko2Params `json:"glicko2_params"`
	Provisional   Provisional   `json:"rating_provisional"`
	Tiers         Tiers         `json:"tiers"`
	SoftReset     SoftReset     `json:"season_soft_reset"`
	Points        PointTable    `json:"competition_points"`
	Effort        EffortTable   `json:"effort_points"`
	Visibility    Visibility    `json:"leaderboard_visibility"`
	// MinCellSize is analytics_min_cell_size (CONFIG_DEFAULTS, owner GAME_ANALYTICS §2): grade_goal cells of fewer
	// students are hidden (F106-AC-16).
	MinCellSize int `json:"analytics_min_cell_size"`
	// FriendsMax bounds the friends read (edge case: the 200 most recent friends).
	FriendsMax int `json:"friends_max"`
}

// ParseRules decodes and validates a Ranking Rules payload.
func ParseRules(b []byte) (Rules, error) {
	var r Rules
	if err := json.Unmarshal(b, &r); err != nil {
		return Rules{}, errors.Join(ErrRules, err)
	}
	return r, r.Validate()
}

// Validate checks the ranges of the rules.
func (r Rules) Validate() error {
	bad := func(what string) error { return fmt.Errorf("%w: %s", ErrRules, what) }
	g := r.Glicko2
	switch {
	case r.SchemaVersion != 1:
		return bad("schema_version")
	case g.R0 <= 0, g.RD0 <= 0, g.Sigma0 <= 0, g.Tau <= 0, g.RDMin <= 0, g.RDMax < g.RDMin, g.RD0 > g.RDMax, g.Epsilon <= 0:
		return bad("glicko2_params")
	case r.Provisional.MinGames < 1, r.Provisional.MaxRD <= 0:
		return bad("rating_provisional")
	case r.Tiers.Hysteresis < 0, len(r.Tiers.Levels) == 0, r.Tiers.Levels[0].MinRating != nil:
		return bad("tiers: the first level has no lower bound")
	case r.SoftReset.Factor < 0, r.SoftReset.Factor > 1, r.SoftReset.RDFloor < 0, r.SoftReset.RDFloor > g.RDMax:
		return bad("season_soft_reset")
	case r.Points.DailyCap < 0, r.Points.DailyMatches < 0, r.Points.CorrectMax < 0, r.Points.Abandon > 0:
		return bad("competition_points")
	case r.Effort.DailyCap < 0, r.Effort.ActivityDailyMax < 0:
		return bad("effort_points")
	case r.Visibility.G13.PersonalRank:
		return bad("leaderboard_visibility: grades 1–3 never get a personal rank")
	case r.Visibility.G45.Top < 1, r.Visibility.G69.Top < 1, r.Visibility.G69.Around < 0:
		return bad("leaderboard_visibility")
	case r.MinCellSize < 1, r.FriendsMax < 1:
		return bad("analytics_min_cell_size/friends_max")
	}
	codes := map[string]bool{}
	prev := 0.0
	for i, l := range r.Tiers.Levels {
		if l.Code == "" || len(l.Code) > 64 || codes[l.Code] || len(l.Names) == 0 {
			return bad("tier code/names")
		}
		codes[l.Code] = true
		for _, n := range l.Names {
			if n == "" || len([]rune(n)) > 128 {
				return bad("tier name length")
			}
		}
		if i > 0 {
			if l.MinRating == nil || (i > 1 && *l.MinRating <= prev) {
				return bad("tier thresholds must grow")
			}
			prev = *l.MinRating
		}
	}
	return nil
}

// Rated reports whether mode carries a rating.
func Rated(mode string) bool { return slices.Contains(RatedModes, mode) }
