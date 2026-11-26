package unit

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// Grade matrix 1–9 of module competition (TEST_STRATEGY §6.10): every grade-dependent rule runs for all nine grades,
// with the band boundaries 3→4, 4→5, 5→6 and 7→8 asserted exactly. Values come from the seeds (CONFIG_DEFAULTS
// grade_window_g6_9, speed_bonus_min_grade, leaderboard_visibility, battle_1v1_format, team_battle_format) — never
// from this file.

var gmGrades = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}

// gmBand is the band of a grade as the owning docs write it (COMPETITION_SYSTEM §9.4, MATCHMAKING §2.1).
func gmBand(g int) string {
	switch {
	case g <= 3:
		return "g1_3"
	case g <= 5:
		return "g4_5"
	case g <= 7:
		return "g6_7"
	}
	return "g8_9"
}

// gmMatchable is H1 of MATCHMAKING §2.1 written independently of the code: grades 1–5 play their own grade only;
// grades 6–9 their own grade, or ±1 inside g6_7 / g8_9 once both tickets waited grade_window_g6_9.after_s.
func gmMatchable(a, b int, waited, after time.Duration) bool {
	if a == b {
		return true
	}
	if a <= 5 || b <= 5 || gmBand(a) != gmBand(b) {
		return false
	}
	return (a-b == 1 || b-a == 1) && waited >= after
}

// TestGradeMatrixMatchmakingH1 (F104-AC-02, MATCHMAKING §2.1 H1): every pair of grades × waits 0 s, after_s − 1, after_s,
// 5 min, through GradeCompatible and through Pair of a real queue; grade matchmaking stays exact at any wait.
func TestGradeMatrixMatchmakingH1(t *testing.T) {
	c := seedConfig(t)
	after := time.Duration(c.GradeWindow.AfterS) * time.Second
	if c.GradeWindow.AfterS != gameconfig.GradeWindowMinWaitS || c.GradeWindow.Levels != gameconfig.GradeWindowMaxLevels {
		t.Fatalf("seed grade window %+v, want ±%d after %d s", c.GradeWindow, gameconfig.GradeWindowMaxLevels, gameconfig.GradeWindowMinWaitS)
	}
	now := time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC)
	for _, a := range gmGrades {
		for _, b := range gmGrades {
			for _, w := range []time.Duration{0, after - time.Second, after, 5 * time.Minute} {
				want := gmMatchable(a, b, w, after)
				if got := c.GradeCompatible(mm.TypeSkill, a, b, w, w); got != want {
					t.Errorf("skill grade %d vs %d after %s: %v, want %v", a, b, w, got, want)
				}
				if got := c.GradeCompatible(mm.TypeGrade, a, b, w, w); got != (a == b) {
					t.Errorf("grade matchmaking %d vs %d after %s: %v", a, b, w, got)
				}
				// Two tickets meet only in one queue: same grade key (H1 partition), then the pair check.
				k := queue(mm.ModeBattle1v1, mm.TypeSkill, a)
				if mm.GradeKey(mm.TypeSkill, a) != mm.GradeKey(mm.TypeSkill, b) {
					if want {
						t.Errorf("grades %d and %d match but sit in different queues", a, b)
					}
					continue
				}
				x, y := cand(a, 1500, w, now), cand(b, 1500, w, now)
				if got := len(c.Pair(k, []mm.Candidate{x, y}, mm.Env{Now: now})) == 1; got != want {
					t.Errorf("Pair grade %d vs %d after %s: %v, want %v", a, b, w, got, want)
				}
			}
		}
	}
	// Exact boundaries: 5→6 and 7→8 never meet, even after any wait; 6↔7 and 8↔9 only after the window.
	for _, p := range [][2]int{{5, 6}, {7, 8}, {3, 4}, {4, 5}} {
		if c.GradeCompatible(mm.TypeSkill, p[0], p[1], time.Hour, time.Hour) {
			t.Errorf("boundary %d→%d widened", p[0], p[1])
		}
	}
	for _, p := range [][2]int{{6, 7}, {8, 9}} {
		if c.GradeCompatible(mm.TypeSkill, p[0], p[1], after-time.Second, time.Hour) || !c.GradeCompatible(mm.TypeSkill, p[0], p[1], after, after) {
			t.Errorf("±1 inside the band %d↔%d not exactly at %s", p[0], p[1], after)
		}
	}
}

// TestGradeMatrixMatchmakingModes (F104, COMPETITION_SYSTEM §3.2): battle_1v1 opens for every grade, team_battle from
// grade 4 (the seed min_grade equals the platform floor); a party ticket of mixed grades is refused at every grade.
func TestGradeMatrixMatchmakingModes(t *testing.T) {
	c := seedConfig(t)
	team, _ := c.Mode(mm.ModeTeamBattle)
	if team.MinGrade != gameconfig.TeamBattleMinGrade {
		t.Fatalf("seed team_battle min_grade %d, platform floor %d", team.MinGrade, gameconfig.TeamBattleMinGrade)
	}
	ruleOf := func(err error) string {
		var pe mm.PolicyError
		if errors.As(err, &pe) {
			return pe.Rule
		}
		if err != nil {
			return err.Error()
		}
		return ""
	}
	player := func(g int) mm.Player {
		return mm.Player{ID: uuid.New(), Grade: g, Active: true, Rating: mm.Rating{Mu: 1500, RD: 350}, PartyType: "competition_team"}
	}
	party := uuid.New()
	for _, g := range gmGrades {
		solo := mm.Request{Mode: mm.ModeBattle1v1, Type: mm.TypeSkill, SubjectID: uuid.New()}
		if got := ruleOf(c.CheckPlayers(solo, []mm.Player{player(g)})); got != "" {
			t.Errorf("grade %d battle_1v1: %s", g, got)
		}
		tm := mm.Request{Mode: mm.ModeTeamBattle, Type: mm.TypeTeam, SubjectID: uuid.New(), PartyID: &party, AllowFill: true}
		want := ""
		if g < 4 {
			want = mm.RuleModeGrade
		}
		if got := ruleOf(c.CheckPlayers(tm, []mm.Player{player(g), player(g)})); got != want {
			t.Errorf("grade %d team_battle: %q, want %q", g, got, want)
		}
		if g >= 4 && g < 9 {
			if got := ruleOf(c.CheckPlayers(tm, []mm.Player{player(g), player(g + 1)})); got != mm.RulePartyGrade {
				t.Errorf("party of grades %d+%d: %q", g, g+1, got)
			}
		}
	}
	// Grade outside 1–9 (not projected) never queues.
	for _, g := range []int{0, 10} {
		if got := ruleOf(mm.Eligible(player(g))); got != mm.RuleGrade {
			t.Errorf("grade %d: %q", g, got)
		}
	}
}

// TestGradeMatrixBands (MATCHMAKING §3 friend duel, MULTIPLAYER_GAMEPLAY §6 hub partition, PARTY_SYSTEM §4): the same
// four bands everywhere — g1_3, g4_5, g6_7, g8_9 — and the queue key g<n> for 1–5, b6_7 / b8_9 for 6–9.
func TestGradeMatrixBands(t *testing.T) {
	for _, g := range gmGrades {
		hb, ok := hub.BandOf(g)
		if !ok || hb != gmBand(g) || mm.BandOf(g) != gmBand(g) || party.BandOf(g) != gmBand(g) {
			t.Errorf("grade %d: hub %q duel %q party %q, want %q", g, hb, mm.BandOf(g), party.BandOf(g), gmBand(g))
		}
		want := fmt.Sprintf("g%d", g)
		if g >= 6 {
			want = "b" + gmBand(g)[1:]
		}
		if got := mm.GradeKey(mm.TypeSkill, g); got != want {
			t.Errorf("grade %d queue key %q, want %q", g, got, want)
		}
	}
	for _, g := range []int{0, 10} {
		if _, ok := hub.BandOf(g); ok || mm.BandOf(g) != "" {
			t.Errorf("grade %d has a band", g)
		}
	}
}

// TestGradeMatrixPartyPolicy (PARTY_SYSTEM §4, §8 R7, ADR-0095 D2/D7): party types by grade and the pairing rules of
// an invitation — grades 1–3 friend/quest and the same grade only; competition_team from grade 4; study_party/custom
// from grade 6; never across a grade band.
func TestGradeMatrixPartyPolicy(t *testing.T) {
	school := uuid.New()
	now := time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC)
	for _, g := range gmGrades {
		r := party.PlatformRules(party.BandOf(g), party.MaxMembers)
		allowed := map[string]bool{party.TypeFriend: true, party.TypeQuest: true, party.TypeCompetitionTeam: g >= 4,
			party.TypeStudyParty: g >= 6, party.TypeCustom: g >= 6}
		for typ, want := range allowed {
			if got := party.CheckCreate(r, subj(school, g), typ, 4) == nil; got != want {
				t.Errorf("grade %d create %s: %v, want %v", g, typ, got, want)
			}
		}
		if r.SameGradeOnly != (g <= 3) || r.CrossGradeBand {
			t.Errorf("grade %d: same_grade_only %v cross_grade_band %v", g, r.SameGradeOnly, r.CrossGradeBand)
		}
		leader := subj(school, g)
		p, err := party.New(uuid.New(), uuid.New(), school, leader.ID, party.TypeQuest, party.BandOf(g), "Nhóm", party.Settings{}, 4, now)
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range gmGrades {
			want := ""
			switch {
			case party.BandOf(h) != party.BandOf(g):
				want = party.RuleGradeBand
			case g <= 3 && h != g:
				want = party.RuleGrade
			}
			chk := party.InviteCheck{Party: p, Inviter: leader, Invitee: subj(school, h), Friends: true, Rules: r}
			if got := rule(chk.Evaluate()); got != want {
				t.Errorf("grade %d invites grade %d: %q, want %q", g, h, got, want)
			}
		}
	}
}

// TestGradeMatrixBattleRules (F105, COMPETITION_SYSTEM §3.1–3.3, CONFIG_DEFAULTS battle_1v1_format, team_battle_format,
// speed_bonus_min_grade, cheat_fast_answer_flag): per grade, the question time (grades 1–3 × 1.5), the speed bonus
// (none below grade 6, also in the points of a fast correct answer), team battle from grade 4, the timed-quiz rank
// from grade 4 (grades 1–3 get the group goal), the fast-answer threshold and the bot profile.
func TestGradeMatrixBattleRules(t *testing.T) {
	c, err := battle.ParseConfig(gmSeedBytes(t, "matchmaking_config.json"))
	if err != nil {
		t.Fatal(err)
	}
	d := battle.DefaultConfig()
	for _, g := range gmGrades {
		r, err := c.RulesFor(battle.Mode1v1, g, false)
		if err != nil {
			t.Fatal(err)
		}
		wantTime := time.Duration(d.Battle1v1.TimeS) * time.Second
		if g <= 3 {
			wantTime = time.Duration(d.Battle1v1.TimeG13S) * time.Second
		}
		if r.TimeLimit != wantTime {
			t.Errorf("grade %d 1v1 time %s, want %s", g, r.TimeLimit, wantTime)
		}
		if r.SpeedBonus != (g >= 6) {
			t.Errorf("grade %d speed bonus %v", g, r.SpeedBonus)
		}
		pts, bonus := battle.Points(r, true, 2*time.Second, false, false, false)
		if (bonus > 0) != (g >= 6) || pts != r.CorrectPoints+bonus {
			t.Errorf("grade %d answer in 2 s: points %d bonus %d", g, pts, bonus)
		}
		wantFast := time.Duration(d.FastAnswer.G69MS) * time.Millisecond
		if g <= 5 {
			wantFast = time.Duration(d.FastAnswer.G15MS) * time.Millisecond
		}
		if r.FastAnswer != wantFast {
			t.Errorf("grade %d fast answer %s, want %s", g, r.FastAnswer, wantFast)
		}
		tr, err := c.RulesFor(battle.ModeTeam, g, false)
		if (err == nil) != (g >= 4) {
			t.Errorf("grade %d team battle: %v", g, err)
		}
		if err != nil && !errors.Is(err, battle.ErrModeGrade) {
			t.Errorf("grade %d team battle error %v", g, err)
		}
		if err == nil && tr.SpeedBonus != (g >= 6) {
			t.Errorf("grade %d team speed bonus %v", g, tr.SpeedBonus)
		}
		q, err := c.RulesFor(battle.ModeTimedQuiz, g, false)
		if err != nil {
			t.Fatal(err)
		}
		if q.Ranking != (g >= 4) || q.GroupGoal != (g <= 3) || q.SpeedBonus != (g >= 6) {
			t.Errorf("grade %d timed quiz: ranking %v group goal %v speed bonus %v", g, q.Ranking, q.GroupGoal, q.SpeedBonus)
		}
		b := c.BotFor(g)
		if g < b.MinGrade || g > b.MaxGrade {
			t.Errorf("grade %d bot profile %+v", g, b)
		}
	}
}

// TestGradeMatrixSpeedBonusFloorHolds (F108-AC-04, CONFIG_DEFAULTS speed_bonus_min_grade): every speed_bonus_min_grade a
// published matchmaking configuration may carry (1…10) is either refused by the invariants or leaves grades 1–5
// without a speed bonus; the same for team_battle min_grade (grades 1–3 never play a team battle) and the timed-quiz
// rank (grades 1–3 never ranked).
func TestGradeMatrixSpeedBonusFloorHolds(t *testing.T) {
	for v := 1; v <= 10; v++ {
		for _, key := range []string{"speed_bonus_min_grade", "team_battle", "timed_quiz"} {
			d := seedDoc(t, "matchmaking_config.json")
			switch key {
			case "speed_bonus_min_grade":
				d["battle"] = map[string]any{"speed_bonus_min_grade": v}
			case "team_battle":
				d["battle"] = map[string]any{"team_battle": map[string]any{"min_grade": v}}
			default:
				d["battle"] = map[string]any{"timed_quiz": map[string]any{"rank_min_grade": v}}
			}
			if len(check(t, gameconfig.TypeMatchmaking, d)) > 0 {
				continue // refused: the published configuration keeps the floor
			}
			b, _ := json.Marshal(d)
			c, err := battle.ParseConfig(b)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range gmGrades {
				r, _ := c.RulesFor(battle.Mode1v1, g, false)
				tm, terr := c.RulesFor(battle.ModeTeam, g, false)
				q, _ := c.RulesFor(battle.ModeTimedQuiz, g, false)
				switch {
				case g <= 5 && r.SpeedBonus:
					t.Errorf("%s=%d accepted and grade %d gets a speed bonus", key, v, g)
				case g <= 3 && terr == nil:
					t.Errorf("%s=%d accepted and grade %d plays a team battle (%v)", key, v, g, tm.Mode)
				case g <= 3 && q.Ranking:
					t.Errorf("%s=%d accepted and grade %d is ranked in a timed quiz", key, v, g)
				}
			}
		}
	}
}

// TestGradeMatrixLeaderboardVisibility (F106-AC-16/17, COMPETITION_SYSTEM §9.4): per grade on a board of 120 members
// and on a small board of 6 — grades 1–3 the grade goal only (no entry, no own rank, no neighbours) and no ranked
// board at all; grades 4–5 the top 10, own rank in the top else "nhóm X%", never the last member; grades 6–9 the
// top 50 and ±3 around themselves.
func TestGradeMatrixLeaderboardVisibility(t *testing.T) {
	v := seedRules(t).Visibility
	all := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	goal := def(ldb.MetricGradeGoal, ldb.ScopeGrade, "weekly", all...)
	arena := def(ldb.MetricCompetitionPoints, ldb.ScopeGrade, "weekly", all...)
	for _, g := range gmGrades {
		_, band := ldb.Band(v, g)
		want := gmBand(g)
		if g >= 6 {
			want = "g6_9" // one visibility band for grades 6–9
		}
		if band != want {
			t.Errorf("grade %d band %s", g, band)
		}
		if !ldb.Sees(v, goal, g) || ldb.Sees(v, arena, g) != (g >= 4) {
			t.Errorf("grade %d: sees goal %v, sees ranked %v", g, ldb.Sees(v, goal, g), ldb.Sees(v, arena, g))
		}
		b := board(120)
		mine := &b[79] // rank 80
		view := ldb.Apply(v, g, 120, b, mine, b[70:90])
		switch {
		case g <= 3:
			if view.Visibility != ldb.VisibilityGradeGoal || len(view.Entries) != 0 || view.Me != nil || len(view.Around) != 0 {
				t.Errorf("grade %d sees a ranking: %+v", g, view)
			}
		case g <= 5:
			if view.Visibility != ldb.VisibilityPercentile || len(view.Entries) != 10 || len(view.Around) != 0 ||
				view.Me == nil || view.Me.Rank != nil || view.Me.Percentile == nil || *view.Me.Percentile != 100 {
				t.Errorf("grade %d rank 80/120: %+v", g, view)
			}
			small := board(6)
			for _, e := range ldb.Apply(v, g, 6, small, &small[0], nil).Entries {
				if e.Rank == 6 {
					t.Errorf("grade %d: the last member is shown to others", g)
				}
			}
			if top := ldb.Apply(v, g, 120, b, &b[9], nil); top.Me.Rank == nil || *top.Me.Rank != 10 {
				t.Errorf("grade %d rank 10: own rank hidden", g)
			}
		default:
			if view.Visibility != ldb.VisibilityRanked || len(view.Entries) != 50 || view.Me == nil || *view.Me.Rank != 80 ||
				len(view.Around) != 7 || view.Around[0].Rank != 77 || view.Around[6].Rank != 83 {
				t.Errorf("grade %d: entries %d around %d", g, len(view.Entries), len(view.Around))
			}
		}
	}
}

// TestGradeMatrixGradeGoalMinCell (F106-AC-16, analytics_min_cell_size 5): for every grade the contributors of the
// grade goal are hidden below 5 students and shown from 5 — the goal never carries a rank or a name.
func TestGradeMatrixGradeGoalMinCell(t *testing.T) {
	minCell := seedRules(t).MinCellSize
	if minCell != gameconfig.MinCellSize {
		t.Fatalf("seed analytics_min_cell_size %d", minCell)
	}
	for _, g := range gmGrades {
		for n := 0; n <= minCell+1; n++ {
			gg := ldb.NewGradeGoal(g, 40, 100, nil, n, minCell)
			if (gg.Contributors != nil) != (n >= minCell) || gg.Grade != g {
				t.Errorf("grade %d, %d contributors: %+v", g, n, gg.Contributors)
			}
		}
	}
}

// TestGradeMatrixVisibilityConfigNeverLoosens (F108-AC-04, COMPETITION_SYSTEM §9.4, §13): 1,500 random
// leaderboard_visibility payloads; each one the invariants accept, applied to every grade, stays inside the floor of
// that grade (1–3 no ranking, 4–5 ≤ 10 and never the last, 6–9 ≤ 50 and ≤ ±3).
func TestGradeMatrixVisibilityConfigNeverLoosens(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 9))
	band := func() map[string]any {
		return map[string]any{"personal_rank": rng.IntN(2) == 0, "top": rng.IntN(70), "percentile": rng.IntN(2) == 0,
			"hide_last": rng.IntN(2) == 0, "around": rng.IntN(6)}
	}
	accepted := 0
	for i := 0; i < 1500; i++ {
		d := seedDoc(t, "ranking_rules.json")
		vis := map[string]any{"g1_3": band(), "g4_5": band(), "g6_9": band()}
		if i%3 == 0 { // keep a share of payloads valid for the bands not under test
			vis["g1_3"] = map[string]any{"personal_rank": false, "top": 0, "percentile": false, "hide_last": true, "around": 0}
		}
		d["leaderboard_visibility"] = vis
		if len(check(t, gameconfig.TypeRanking, d)) > 0 {
			continue
		}
		accepted++
		b, _ := json.Marshal(d)
		r, err := ranking.ParseRules(b)
		if err != nil {
			t.Fatal(err)
		}
		arena := def(ldb.MetricCompetitionPoints, ldb.ScopeGrade, "weekly", gmGrades...)
		for _, g := range gmGrades {
			bd := board(120)
			for _, me := range []int{0, 9, 59, 119} {
				view := ldb.Apply(r.Visibility, g, 120, bd, &bd[me], bd)
				switch {
				case g <= 3 && (len(view.Entries) > 0 || view.Me != nil || len(view.Around) > 0 || ldb.Sees(r.Visibility, arena, g)):
					t.Fatalf("accepted %v: grade %d sees a ranking", vis, g)
				case g >= 4 && g <= 5 && (len(view.Entries) > gameconfig.VisibilityG45MaxTop || len(view.Around) > 0):
					t.Fatalf("accepted %v: grade %d sees %d entries, %d around", vis, g, len(view.Entries), len(view.Around))
				case g >= 6 && (len(view.Entries) > gameconfig.VisibilityG69MaxTop || len(view.Around) > 2*gameconfig.VisibilityG69MaxAround+1):
					t.Fatalf("accepted %v: grade %d sees %d entries, %d around", vis, g, len(view.Entries), len(view.Around))
				}
				if g >= 4 && g <= 5 && me != 119 && slices.ContainsFunc(view.Entries, func(e ldb.Ranked) bool { return e.Rank == 120 }) {
					t.Fatalf("accepted %v: grade %d sees the last member", vis, g)
				}
			}
			small := board(6)
			if g >= 4 && g <= 5 && slices.ContainsFunc(ldb.Apply(r.Visibility, g, 6, small, &small[0], nil).Entries,
				func(e ldb.Ranked) bool { return e.Rank == 6 }) {
				t.Fatalf("accepted %v: grade %d sees the last member of a small board", vis, g)
			}
		}
	}
	if accepted == 0 {
		t.Fatal("no random payload was accepted: the matrix proves nothing")
	}
}

// TestGradeMatrixLeaderboardGradesConfig (F108-AC-04, COMPETITION_SYSTEM §9.4): an active ranked board that lists any
// grade 1–3 is refused; listing only grades 4–9 is accepted; a grade goal may list every grade.
func TestGradeMatrixLeaderboardGradesConfig(t *testing.T) {
	for _, g := range gmGrades {
		for _, metric := range []string{ldb.MetricCompetitionPoints, ldb.MetricGradeGoal} {
			d := seedDoc(t, "leaderboard_definitions.json")
			defs := d["definitions"].([]any)
			var x map[string]any
			idx := -1
			for i, it := range defs {
				if m := it.(map[string]any); m["metric"] == metric && m["status"] == "active" {
					x, idx = m, i
					break
				}
			}
			if x == nil {
				t.Fatalf("no active %s board in the seed", metric)
			}
			x["grade_levels"] = []int{g}
			vs := check(t, gameconfig.TypeLeaderboards, d)
			refused := slices.ContainsFunc(vs, func(v gameconfig.Violation) bool {
				return v.Field == fmt.Sprintf("payload/definitions/%d/grade_levels", idx)
			})
			if want := metric != ldb.MetricGradeGoal && g <= 3; refused != want {
				t.Errorf("%s board for grade %d: refused %v (%+v)", metric, g, refused, vs)
			}
		}
	}
}

func gmSeedBytes(t *testing.T, file string) []byte {
	t.Helper()
	b, err := json.Marshal(seedDoc(t, file))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
