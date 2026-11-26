package gameconfig

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// Violation is one broken rule of a payload, named by its field path ("payload/…", RFC 6901 tokens joined by "/").
// Code is the field error code of the Problem (required, invalid, out_of_range…).
type Violation struct {
	Field   string
	Code    string
	Message string
}

// Field error codes (ValidationProblem errors[].code: INVALID, REQUIRED like problem.Sub*).
const (
	CodeInvalid    = "INVALID"
	CodeRequired   = "REQUIRED"
	CodeOutOfRange = "OUT_OF_RANGE"
	CodeDuplicate  = "DUPLICATE"
)

// Child-safety and fairness floors of the competition rules (COMPETITION_SYSTEM §3, §5, §9.4 [Confirmed]): a published
// configuration may tighten them, never loosen them (F108-AC-04).
const (
	// GradeWindowMinWaitS: grades 6–9 may widen to ±1 grade only after 30 s of waiting (§5.2, grade_window_g6_9).
	GradeWindowMinWaitS = 30
	// GradeWindowMaxLevels: at most ±1 grade, only for grades 6–9 (§5.2); grades 1–5 always play their own grade.
	GradeWindowMaxLevels = 1
	// TeamBattleMinGrade: team battle opens from grade 4 (§3.2).
	TeamBattleMinGrade = 4
	// RankedMinGrade: grades 1–3 never get a personal rank (§9.4) — no ranked board, no timed-quiz rank below 4.
	RankedMinGrade = 4
	// Visibility ceilings by band (§9.4): grades 4–5 top 10, never the last; grades 6–9 top 50 ± 3.
	VisibilityG45MaxTop    = 10
	VisibilityG69MaxTop    = 50
	VisibilityG69MaxAround = 3
	// MinCellSize: the small-cell threshold of collective figures (FR-ANL-016, analytics_min_cell_size ≥ 5).
	MinCellSize = 5
)

// SpeedBonusMinGrade is the platform floor of the speed bonus (CONFIG_DEFAULTS speed_bonus_min_grade, COMPETITION_SYSTEM
// §3.1): grades below it never get a speed bonus, whatever a configuration says.
func SpeedBonusMinGrade() int { return battle.DefaultConfig().SpeedBonusMinGrade }

// RewardRefKeys are the only reward references a competition reward may carry: cosmetics, badges and titles of module
// game (AGENTS R19: no currency, no paid item, nothing that touches learning scores).
var RewardRefKeys = []string{"badge_code", "title_code", "cosmetic_code"}

// Check runs the business invariants of a payload of t (F108-AC-04) after its JSON Schema: nil when the payload may be
// saved/published. raw is the JSON of doc.
func Check(t Type, raw []byte) []Violation {
	var v []Violation
	switch t.Name {
	case TypeMatchmaking:
		v = checkMatchmaking(raw)
	case TypeRanking:
		v = checkRanking(raw)
	case TypeSeasons:
		v = checkSeasonPayload(raw)
	case TypeTournamentTp, TypeChallengeTp:
		v = checkTemplate(t, raw)
	case TypeLeaderboards:
		v = checkLeaderboards(raw)
	case TypeRewards:
		v = checkRewards(raw)
	}
	return v
}

func viol(field, code, format string, args ...any) Violation {
	return Violation{Field: "payload" + field, Code: code, Message: fmt.Sprintf(format, args...)}
}

// ---------- matchmaking-configs ----------

type mmDoc struct {
	GradeWindow struct {
		AfterS int `json:"after_s"`
		Levels int `json:"levels"`
	} `json:"grade_window_g6_9"`
	Modes []struct {
		Mode     string `json:"mode"`
		MinGrade int    `json:"min_grade"`
	} `json:"modes"`
	Battle *struct {
		SpeedBonusMinGrade *int `json:"speed_bonus_min_grade"`
		Team               *struct {
			MinGrade *int `json:"min_grade"`
		} `json:"team_battle"`
		TimedQuiz *struct {
			RankMinGrade *int `json:"rank_min_grade"`
		} `json:"timed_quiz"`
	} `json:"battle"`
	Bots []struct {
		MinGrade int `json:"min_grade"`
		MaxGrade int `json:"max_grade"`
		MinMS    int `json:"min_ms"`
		MaxMS    int `json:"max_ms"`
	} `json:"bot_profiles"`
}

func checkMatchmaking(raw []byte) []Violation {
	var d mmDoc
	if err := json.Unmarshal(raw, &d); err != nil {
		return []Violation{viol("", CodeInvalid, "not a matchmaking configuration")}
	}
	var out []Violation
	if d.GradeWindow.Levels > GradeWindowMaxLevels {
		out = append(out, viol("/grade_window_g6_9/levels", CodeOutOfRange, "grades 6–9 widen by at most ±%d grade (grades 1–5 never)", GradeWindowMaxLevels))
	}
	if d.GradeWindow.Levels > 0 && d.GradeWindow.AfterS < GradeWindowMinWaitS {
		out = append(out, viol("/grade_window_g6_9/after_s", CodeOutOfRange, "the grade window widens only after %d s of waiting", GradeWindowMinWaitS))
	}
	for i, m := range d.Modes {
		if m.Mode == mm.ModeTeamBattle && m.MinGrade < TeamBattleMinGrade {
			out = append(out, viol(fmt.Sprintf("/modes/%d/min_grade", i), CodeOutOfRange, "team battle opens from grade %d", TeamBattleMinGrade))
		}
	}
	if b := d.Battle; b != nil {
		if b.SpeedBonusMinGrade != nil && *b.SpeedBonusMinGrade < SpeedBonusMinGrade() {
			out = append(out, viol("/battle/speed_bonus_min_grade", CodeOutOfRange, "no speed bonus below grade %d", SpeedBonusMinGrade()))
		}
		if b.Team != nil && b.Team.MinGrade != nil && *b.Team.MinGrade < TeamBattleMinGrade {
			out = append(out, viol("/battle/team_battle/min_grade", CodeOutOfRange, "team battle opens from grade %d", TeamBattleMinGrade))
		}
		if b.TimedQuiz != nil && b.TimedQuiz.RankMinGrade != nil && *b.TimedQuiz.RankMinGrade < RankedMinGrade {
			out = append(out, viol("/battle/timed_quiz/rank_min_grade", CodeOutOfRange, "grades 1–3 never get a personal rank"))
		}
	}
	for i, b := range d.Bots {
		if (b.MinGrade != 0 || b.MaxGrade != 0) && b.MinGrade > b.MaxGrade {
			out = append(out, viol(fmt.Sprintf("/bot_profiles/%d/max_grade", i), CodeInvalid, "max_grade is below min_grade"))
		}
		if b.MinMS > b.MaxMS {
			out = append(out, viol(fmt.Sprintf("/bot_profiles/%d/max_ms", i), CodeInvalid, "max_ms is below min_ms"))
		}
	}
	if len(out) == 0 {
		if _, err := mm.Parse(raw); err != nil {
			out = append(out, viol("", CodeInvalid, "%s", strings.TrimPrefix(err.Error(), "matchmaking: invalid configuration: ")))
		}
	}
	return out
}

// ---------- ranking-rules ----------

func checkRanking(raw []byte) []Violation {
	var r ranking.Rules
	if err := json.Unmarshal(raw, &r); err != nil {
		return []Violation{viol("", CodeInvalid, "not ranking rules")}
	}
	var out []Violation
	var prev *float64
	for i, l := range r.Tiers.Levels {
		f := fmt.Sprintf("/tiers/levels/%d/min_rating", i)
		switch {
		case i == 0 && l.MinRating != nil:
			out = append(out, viol(f, CodeInvalid, "the first tier has no lower bound (null)"))
		case i > 0 && l.MinRating == nil:
			out = append(out, viol(f, CodeRequired, "every tier after the first has a threshold"))
		case i > 0 && prev != nil && *l.MinRating <= *prev:
			out = append(out, viol(f, CodeInvalid, "tier thresholds must grow"))
		}
		if l.MinRating != nil {
			prev = l.MinRating
		}
	}
	g := r.Glicko2
	if g.RDMin > g.RDMax {
		out = append(out, viol("/glicko2_params/rd_min", CodeInvalid, "rd_min is above rd_max"))
	}
	v := r.Visibility
	if v.G13.PersonalRank || v.G13.Top > 0 || v.G13.Around > 0 || v.G13.Percentile {
		out = append(out, viol("/leaderboard_visibility/g1_3", CodeInvalid, "grades 1–3 never get a personal rank, a top list or a percentile"))
	}
	if v.G45.Top > VisibilityG45MaxTop {
		out = append(out, viol("/leaderboard_visibility/g4_5/top", CodeOutOfRange, "grades 4–5 see at most the top %d", VisibilityG45MaxTop))
	}
	if !v.G45.HideLast {
		out = append(out, viol("/leaderboard_visibility/g4_5/hide_last", CodeInvalid, "grades 4–5 never show the last student"))
	}
	if v.G45.Around > 0 {
		out = append(out, viol("/leaderboard_visibility/g4_5/around", CodeOutOfRange, "grades 4–5 have no around-me list"))
	}
	if v.G69.Top > VisibilityG69MaxTop {
		out = append(out, viol("/leaderboard_visibility/g6_9/top", CodeOutOfRange, "grades 6–9 see at most the top %d", VisibilityG69MaxTop))
	}
	if v.G69.Around > VisibilityG69MaxAround {
		out = append(out, viol("/leaderboard_visibility/g6_9/around", CodeOutOfRange, "grades 6–9 see at most ±%d around them", VisibilityG69MaxAround))
	}
	if r.MinCellSize < MinCellSize {
		out = append(out, viol("/analytics_min_cell_size", CodeOutOfRange, "small cells are hidden below %d students", MinCellSize))
	}
	if r.Points.Abandon > 0 {
		out = append(out, viol("/competition_points/abandon", CodeOutOfRange, "abandoning never earns points"))
	}
	if len(out) == 0 {
		if _, err := ranking.ParseRules(raw); err != nil {
			out = append(out, viol("", CodeInvalid, "%s", err.Error()))
		}
	}
	return out
}

// ---------- seasons ----------

// SeasonItem is one season of a seasons payload.
type SeasonItem struct {
	ID       string    `json:"season_id"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Theme    *string   `json:"theme_code"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

// ParseSeasons decodes the seasons of a seasons payload.
func ParseSeasons(raw []byte) ([]SeasonItem, error) {
	var d struct {
		Seasons []SeasonItem `json:"seasons"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("seasons payload: %w", err)
	}
	return d.Seasons, nil
}

func checkSeasonPayload(raw []byte) []Violation {
	ss, err := ParseSeasons(raw)
	if err != nil {
		return []Violation{viol("", CodeInvalid, "not a season catalog")}
	}
	var out []Violation
	codes, ids := map[string]bool{}, map[string]bool{}
	for i, s := range ss {
		if codes[s.Code] {
			out = append(out, viol(fmt.Sprintf("/seasons/%d/code", i), CodeDuplicate, "season codes are unique"))
		}
		if ids[s.ID] {
			out = append(out, viol(fmt.Sprintf("/seasons/%d/season_id", i), CodeDuplicate, "season ids are unique"))
		}
		codes[s.Code], ids[s.ID] = true, true
		if !s.EndsAt.After(s.StartsAt) {
			out = append(out, viol(fmt.Sprintf("/seasons/%d/ends_at", i), CodeInvalid, "a season ends after it starts"))
		}
	}
	for i := range ss {
		for j := i + 1; j < len(ss); j++ {
			a, b := ss[i], ss[j]
			if a.StartsAt.Before(b.EndsAt) && b.StartsAt.Before(a.EndsAt) {
				out = append(out, viol(fmt.Sprintf("/seasons/%d/starts_at", j), CodeInvalid, "season %s overlaps season %s", b.Code, a.Code))
			}
		}
	}
	return out
}

// SeasonState is a season already in competition.seasons (code, status, period).
type SeasonState struct {
	Code     string
	Status   string // scheduled, active, ended
	StartsAt time.Time
	EndsAt   time.Time
}

// CheckSeasonChanges checks a seasons payload against the seasons already in place (F108 edge case "mùa chồng mùa đang
// active", ADR-0043 no retroactive change): an ended season keeps its period; an active season keeps its start and does
// not end in the past; a new season starts at effective time or later. Overlaps between the payload seasons are
// Check's (checkSeasonPayload).
func CheckSeasonChanges(raw []byte, existing []SeasonState, effective time.Time) []Violation {
	ss, err := ParseSeasons(raw)
	if err != nil {
		return []Violation{viol("", CodeInvalid, "not a season catalog")}
	}
	by := map[string]SeasonState{}
	for _, e := range existing {
		by[e.Code] = e
	}
	var out []Violation
	for i, s := range ss {
		e, ok := by[s.Code]
		f := fmt.Sprintf("/seasons/%d", i)
		switch {
		case !ok:
			if s.StartsAt.Before(effective) {
				out = append(out, viol(f+"/starts_at", CodeOutOfRange, "a new season cannot start before the configuration takes effect"))
			}
		case e.Status == "ended":
			if !s.StartsAt.Equal(e.StartsAt) || !s.EndsAt.Equal(e.EndsAt) {
				out = append(out, viol(f+"/ends_at", CodeInvalid, "an ended season keeps its period"))
			}
		case e.Status == "active":
			if !s.StartsAt.Equal(e.StartsAt) {
				out = append(out, viol(f+"/starts_at", CodeInvalid, "the active season keeps its start"))
			}
			if s.EndsAt.Before(effective) {
				out = append(out, viol(f+"/ends_at", CodeOutOfRange, "the active season cannot end in the past"))
			}
		default: // scheduled
			if s.StartsAt.Before(effective) && !s.StartsAt.Equal(e.StartsAt) {
				out = append(out, viol(f+"/starts_at", CodeOutOfRange, "a scheduled season cannot move into the past"))
			}
		}
	}
	return out
}

// ---------- templates ----------

func checkTemplate(t Type, raw []byte) []Violation {
	var d struct {
		Kind       string   `json:"kind"`
		Params     []string `json:"allowed_params"`
		Tournament *struct {
			Seeding string `json:"seeding_policy"`
		} `json:"tournament"`
		Rules []struct {
			RankMin   *int           `json:"rank_min"`
			RankMax   *int           `json:"rank_max"`
			RewardRef map[string]any `json:"reward_ref"`
		} `json:"reward_rule_refs"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return []Violation{viol("", CodeInvalid, "not a template")}
	}
	var out []Violation
	want := challenge.KindSchoolTournament
	if t.Name == TypeChallengeTp {
		want = challenge.KindGradeChallenge
	}
	if d.Kind != want {
		out = append(out, viol("/kind", CodeInvalid, "a %s has kind %s (no class challenge or class tournament — ADR-0095)", t.Name, want))
	}
	for i, p := range d.Params {
		if challenge.Forbidden(p) || !slices.Contains(challenge.Params, p) {
			out = append(out, viol(fmt.Sprintf("/allowed_params/%d", i), CodeInvalid, "%q is not a parameter a school may fill (rules, scoring, rewards and chat stay in the template)", p))
		}
	}
	if d.Tournament != nil && d.Tournament.Seeding == "separate_class" {
		out = append(out, viol("/tournament/seeding_policy", CodeInvalid, "no class seeding (ADR-0095)"))
	}
	for i, r := range d.Rules {
		out = append(out, checkRewardRef(fmt.Sprintf("/reward_rule_refs/%d/reward_ref", i), r.RewardRef)...)
		if r.RankMin != nil && r.RankMax != nil && *r.RankMin > *r.RankMax {
			out = append(out, viol(fmt.Sprintf("/reward_rule_refs/%d/rank_max", i), CodeInvalid, "rank_max is below rank_min"))
		}
	}
	if len(out) == 0 {
		if _, err := challenge.ParseTemplate(raw); err != nil {
			out = append(out, viol("", CodeInvalid, "%s", strings.TrimPrefix(err.Error(), "challenge: invalid template payload: ")))
		}
	}
	return out
}

func checkRewardRef(field string, ref map[string]any) []Violation {
	var out []Violation
	if len(ref) == 0 {
		return []Violation{viol(field, CodeRequired, "a reward is a cosmetic, a badge or a title")}
	}
	for k := range ref {
		if !slices.Contains(RewardRefKeys, k) {
			out = append(out, viol(field+"/"+escape(k), CodeInvalid, "only cosmetics, badges and titles (no currency or paid item — R19)"))
		}
	}
	return out
}

// ---------- leaderboard-definitions ----------

type defDoc struct {
	ID      string      `json:"leaderboard_id"`
	Code    string      `json:"code"`
	Title   string      `json:"title"`
	Metric  string      `json:"metric"`
	Scope   string      `json:"scope_type"`
	Period  string      `json:"period"`
	Filters ldb.Filters `json:"filters"`
	Grades  []int       `json:"grade_levels"`
	Display struct {
		Target float64 `json:"grade_goal_target"`
	} `json:"display"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status"`
}

func checkLeaderboards(raw []byte) []Violation {
	var d struct {
		Definitions []defDoc `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return []Violation{viol("", CodeInvalid, "not a leaderboard catalog")}
	}
	var out []Violation
	codes, ids := map[string]bool{}, map[string]bool{}
	for i, x := range d.Definitions {
		f := fmt.Sprintf("/definitions/%d", i)
		if codes[x.Code] {
			out = append(out, viol(f+"/code", CodeDuplicate, "definition codes are unique"))
		}
		if ids[x.ID] {
			out = append(out, viol(f+"/leaderboard_id", CodeDuplicate, "definition ids are unique"))
		}
		codes[x.Code], ids[x.ID] = true, true
		switch {
		case x.Scope == "class":
			out = append(out, viol(f+"/scope_type", CodeInvalid, "no class scope (ADR-0095)"))
			continue
		case x.Metric == "class_goal":
			out = append(out, viol(f+"/metric", CodeInvalid, "no class goal: the collective goal is grade_goal (ADR-0095)"))
			continue
		case x.Metric == ldb.MetricXP && x.Scope != ldb.ScopeFriends:
			out = append(out, viol(f+"/metric", CodeInvalid, "xp is ranked only among friends (never inside a school or grade)"))
			continue
		case x.Metric == ldb.MetricGradeGoal && x.Display.Target <= 0:
			out = append(out, viol(f+"/display/grade_goal_target", CodeRequired, "a grade goal has a weekly target"))
			continue
		}
		if x.Metric != ldb.MetricGradeGoal && x.Status == "active" {
			for _, g := range x.Grades {
				if g < RankedMinGrade {
					out = append(out, viol(f+"/grade_levels", CodeInvalid, "grades 1–3 never get a personal rank: only the grade goal is for them"))
					break
				}
			}
		}
		def := ldb.Definition{Code: x.Code, Title: x.Title, Metric: x.Metric, Scope: x.Scope, Period: x.Period, Filters: x.Filters, Grades: x.Grades,
			GradeGoalTarget: x.Display.Target, SortOrder: x.SortOrder, Status: x.Status}
		if err := def.Validate(); err != nil {
			out = append(out, viol(f, CodeInvalid, "%s", strings.TrimPrefix(err.Error(), "leaderboard: invalid definition: ")))
		}
	}
	return out
}

// ---------- competition-rewards ----------

func checkRewards(raw []byte) []Violation {
	var d struct {
		AppliesTo string `json:"applies_to"`
		Rules     []struct {
			RuleID     string         `json:"rule_id"`
			Tier       *string        `json:"tier_code"`
			PointsMin  *int           `json:"points_min"`
			RankMin    *int           `json:"rank_min"`
			RankMax    *int           `json:"rank_max"`
			MinMatches *int           `json:"min_matches"`
			RewardRef  map[string]any `json:"reward_ref"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return []Violation{viol("", CodeInvalid, "not a competition reward")}
	}
	var out []Violation
	seen := map[string]bool{}
	for i, r := range d.Rules {
		f := fmt.Sprintf("/rules/%d", i)
		if seen[r.RuleID] {
			out = append(out, viol(f+"/rule_id", CodeDuplicate, "rule ids are unique"))
		}
		seen[r.RuleID] = true
		out = append(out, checkRewardRef(f+"/reward_ref", r.RewardRef)...)
		ranked := r.RankMin != nil || r.RankMax != nil
		switch d.AppliesTo {
		case "season":
			// End-of-season rewards follow the highest tier reached, never an absolute rank (COMPETITION_SYSTEM §8.1).
			if r.Tier == nil {
				out = append(out, viol(f+"/tier_code", CodeRequired, "a season reward follows the highest tier reached"))
			}
			if ranked || r.PointsMin != nil || r.MinMatches != nil {
				out = append(out, viol(f, CodeInvalid, "a season reward is by tier only (no rank, no threshold)"))
			}
		case "leaderboard":
			// Period rewards follow a threshold, not only the top (§9.5).
			if r.PointsMin == nil {
				out = append(out, viol(f+"/points_min", CodeRequired, "a leaderboard reward follows a points threshold"))
			}
			if ranked || r.Tier != nil || r.MinMatches != nil {
				out = append(out, viol(f, CodeInvalid, "a leaderboard reward is by threshold only (no rank)"))
			}
		default: // tournament, challenge
			if !ranked && r.MinMatches == nil {
				out = append(out, viol(f, CodeRequired, "a tournament reward has a rank range or a minimum of matches"))
			}
			if r.Tier != nil || r.PointsMin != nil {
				out = append(out, viol(f, CodeInvalid, "a tournament reward is by rank or participation"))
			}
			if r.RankMin != nil && r.RankMax != nil && *r.RankMin > *r.RankMax {
				out = append(out, viol(f+"/rank_max", CodeInvalid, "rank_max is below rank_min"))
			}
		}
	}
	return out
}
