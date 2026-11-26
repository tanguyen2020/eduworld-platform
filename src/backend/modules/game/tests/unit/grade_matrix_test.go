package unit

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	gamecfg "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Grade matrix 1–9 of module game (TEST_STRATEGY §6.10): every grade-dependent rule of the domain runs for all nine
// grades with the band boundaries 3→4, 4→5, 5→6 and 7→8 asserted exactly. Parameters come from the configuration of
// the module (infrastructure/config: CONFIG_DEFAULTS g1_3_challenge_rules, difficulty_levels, xp_below_grade_multiplier,
// learning_block_max_items, analytics_min_cell_size) — never from this file.

var grades = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}

var t0 = time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC)

func challengeRules(t *testing.T) challenge.Rules {
	t.Helper()
	c, err := gamecfg.ChallengeFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	return c.Rules
}

// fiveLevels is a challenge of every grade with the five levels of CHALLENGE_BOSS_SYSTEM §2.2 (expert for grades 4–9,
// legendary for 6–9 by profile grade_levels; the time and lives come from difficulty_levels).
func fiveLevels(typ string, timeLimit int) challenge.Version {
	return challenge.Version{ChallengeID: uuid.NewString(), Code: "chl.matrix", Type: typ, Name: challenge.Text{VI: "Ma trận", EN: "Matrix"},
		GradeLevels: grades, Mode: challenge.ModeSolo, TimeLimitSeconds: timeLimit,
		Difficulties: []challenge.Profile{{Level: challenge.Easy, Min: 1, Max: 2, RewardMultiplier: 1},
			{Level: challenge.Normal, Min: 2, Max: 3, RewardMultiplier: 1.3}, {Level: challenge.Hard, Min: 3, Max: 4, RewardMultiplier: 1.6},
			{Level: challenge.Expert, Min: 4, Max: 5, RewardMultiplier: 2, GradeLevels: []int{4, 5, 6, 7, 8, 9}},
			{Level: challenge.Legendary, Min: 5, Max: 5, RewardMultiplier: 2.5, GradeLevels: []int{6, 7, 8, 9}}}}
}

// TestGradeMatrixChallengeLevels (F076-AC-01, CHALLENGE_BOSS_SYSTEM §2.2, AC-UX-0601): per grade, the levels a student
// may choose (grades 1–3 easy–hard whatever the profile says), the time × 1.5 of grades 1–3 on the whole challenge and
// on one activity, and the delta difficulties[] of getGameChallenge: grades 1–3 never receive expert/legendary, a level
// locked for the grade carries grade_not_allowed, an available level carries no reason.
func TestGradeMatrixChallengeLevels(t *testing.T) {
	r := challengeRules(t)
	factor := r.G13TimeFactor
	if factor != 1.5 || r.G13MaxLevel != challenge.Hard {
		t.Fatalf("g1_3_challenge_rules from config: × %v up to %s", factor, r.G13MaxLevel)
	}
	open := challengeVersionForAll(fiveLevels(challenge.TypeTime, 300))
	survival := fiveLevels(challenge.TypeSurvival, 900)
	for _, g := range grades {
		want := []string{challenge.Easy, challenge.Normal, challenge.Hard}
		switch {
		case g >= 6:
			want = challenge.Levels
		case g >= 4:
			want = append(want, challenge.Expert)
		}
		if got := r.Difficulties(survival, g); !slices.Equal(got, want) {
			t.Errorf("grade %d levels %v, want %v", g, got, want)
		}
		// A profile open to every grade still stops at hard for grades 1–3.
		wantOpen := challenge.Levels
		if g <= 3 {
			wantOpen = []string{challenge.Easy, challenge.Normal, challenge.Hard}
		}
		if got := r.Difficulties(open, g); !slices.Equal(got, wantOpen) {
			t.Errorf("grade %d levels of an open challenge %v, want %v", g, got, wantOpen)
		}
		f := 1.0
		if g <= 3 {
			f = factor
		}
		d := r.Deadline(survival, g, t0)
		if d == nil || d.Sub(t0) != time.Duration(math.Floor(900*f))*time.Second {
			t.Errorf("grade %d deadline %v", g, d)
		}
		for _, l := range challenge.Levels {
			p, _ := survival.Profile(l)
			if got, wantS := r.ItemSeconds(p, g), int(math.Floor(float64(r.DefaultSeconds[l])*f)); got != wantS {
				t.Errorf("grade %d %s seconds per item %d, want %d", g, l, got, wantS)
			}
		}
		opts := r.Options(survival, g)
		var levels []string
		for _, o := range opts {
			levels = append(levels, o.Level)
			if o.Available != slices.Contains(want, o.Level) {
				t.Errorf("grade %d %s available %v", g, o.Level, o.Available)
			}
			if (o.LockedReason == challenge.LockedGrade) == o.Available || (o.Available && o.LockedReason != "") {
				t.Errorf("grade %d %s reason %q", g, o.Level, o.LockedReason)
			}
			p, _ := survival.Profile(o.Level)
			if o.TimeLimitSeconds == nil || *o.TimeLimitSeconds != int(math.Floor(900*f)) || o.ItemSeconds != max(r.ItemSeconds(p, g), challenge.MinItemSeconds) ||
				o.Lives == nil || *o.Lives != r.DefaultLives[o.Level] || o.RewardMultiplier != p.RewardMultiplier {
				t.Errorf("grade %d %s option %+v", g, o.Level, o)
			}
		}
		wantLevels := challenge.Levels
		if g <= 3 {
			wantLevels = []string{challenge.Easy, challenge.Normal, challenge.Hard} // AC-UX-0601: expert/legendary never shown
		}
		if !slices.Equal(levels, wantLevels) {
			t.Errorf("grade %d options %v, want %v", g, levels, wantLevels)
		}
	}
	// A time challenge without lives, a challenge without a time limit.
	for _, o := range r.Options(fiveLevels(challenge.TypeWeekly, 0), 7) {
		if o.Lives != nil || o.TimeLimitSeconds != nil {
			t.Errorf("weekly %s: lives %v time %v", o.Level, o.Lives, o.TimeLimitSeconds)
		}
	}
}

func challengeVersionForAll(v challenge.Version) challenge.Version {
	for i := range v.Difficulties {
		v.Difficulties[i].GradeLevels = nil
	}
	return v
}

// TestGradeMatrixChallengeVisibility (F076-AC-01/02): a challenge of grades [g] is visible to grade g only — never to a
// neighbour grade across a band boundary.
func TestGradeMatrixChallengeVisibility(t *testing.T) {
	for _, owner := range grades {
		v := fiveLevels(challenge.TypeTime, 300)
		v.GradeLevels = []int{owner}
		for _, g := range grades {
			ok, reason := v.Visible(challenge.Student{Grade: g}, t0)
			if ok != (g == owner) || (!ok && reason != challenge.ReasonGrade) {
				t.Errorf("challenge of grade %d seen by grade %d: %v %s", owner, g, ok, reason)
			}
		}
	}
}

// TestGradeMatrixXPBelowGrade (F074-AC-07, PROGRESSION_SYSTEM §3.2, xp_below_grade_multiplier): student grade × content
// grade — content two or more grades below gets × 0.25, never for a recommendation or an assignment; content of the
// grade, above or one grade below gets × 1; a rule listing grade_levels only rewards those grades.
func TestGradeMatrixXPBelowGrade(t *testing.T) {
	p, err := gamecfg.ProgressionFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	if p.BelowGradeMultiplier != 0.25 || p.BelowGradeGap != 2 {
		t.Fatalf("xp_below_grade_multiplier from config: × %v from %d grades", p.BelowGradeMultiplier, p.BelowGradeGap)
	}
	rule := reward.Rule{RuleID: uuid.New(), Code: "activity.completed", SourceCode: "activity.completed", EventType: "activity.completed",
		Grants: []reward.Grant{{Type: "xp", Amount: 100}}, Caps: reward.Caps{XPPerDay: 1000}}
	none := func(reward.Rule) reward.History { return reward.History{} }
	rec := uuid.New()
	for _, st := range grades {
		for _, content := range grades {
			for _, mode := range []string{"practice", "assignment", "recommended"} {
				tr := reward.Trigger{EventType: "activity.completed", SourceType: "activity", SourceCodes: []string{"activity.completed"},
					Mode: mode, ContentGrades: []int{content}}
				if mode == "recommended" {
					tr.Mode, tr.RecommendationID = "practice", &rec
				}
				out := p.Evaluate(tr, []reward.Rule{rule}, reward.Student{Grade: st}, none, map[string]int{})
				want := 100
				if st-content >= 2 && mode == "practice" {
					want = 25
				}
				if len(out) != 1 || out[0].Amount != want {
					t.Errorf("student %d content %d %s: %+v, want %d", st, content, mode, out, want)
				}
			}
		}
		// A rule restricted to grades 4–5 rewards those grades only.
		only := rule
		only.Conditions.GradeLevels = []int{4, 5}
		out := p.Evaluate(reward.Trigger{EventType: "activity.completed", SourceType: "activity", SourceCodes: []string{"activity.completed"},
			ContentGrades: []int{st}}, []reward.Rule{only}, reward.Student{Grade: st}, none, map[string]int{})
		if (len(out) == 1) != (st == 4 || st == 5) {
			t.Errorf("grade %d rule of grades 4–5: %d awards", st, len(out))
		}
	}
}

// questOf is a learning quest of grades with one activity step asking n questions (between two talk_npc beats).
func questOf(gs []int, n int) quest.Version {
	txt := quest.Text{VI: "Nhiệm vụ", EN: "Quest"}
	m := 0.7
	return quest.Version{SchemaVersion: 2, QuestID: uuid.NewString(), Code: "math.matrix.q", OwnerScope: "platform",
		QuestType: quest.TypeLearning, Name: txt, Description: txt, GradeLevels: gs, SubjectID: uuid.NewString(), SubjectCode: "math",
		ObjectiveIDs: []string{uuid.NewString()}, StartTrigger: quest.Trigger{Kind: quest.TriggerNPC, NPCID: "npc.mr_numbers"},
		PartyMode: "solo", CompletionRule: quest.CompletionRule{Type: quest.CompleteAllRequired},
		Steps: []quest.Step{
			{ID: "meet", Order: 1, Type: quest.StepTalkNPC, Title: txt, NPCID: "npc.mr_numbers", AreaID: "learning_zone.math"},
			{ID: "solve", Order: 2, Type: quest.StepActivity, Title: txt, ContentRef: &quest.ContentRef{Kind: "activity", ID: uuid.NewString()},
				PassRule: &quest.PassRule{MinCorrectRatio: &m, MinQuestions: &n}},
			{ID: "back", Order: 3, Type: quest.StepTalkNPC, Title: txt, NPCID: "npc.mr_numbers"},
		}}
}

// TestGradeMatrixQuests (F072-AC-01/05/27, F074-AC-25, QUEST_SYSTEM §4, LEARNING_EXPERIENCE §2.1): a quest of grade g is
// visible and accepted by grade g only; its learning block may ask at most learning_block_max_items of the band of its
// lowest grade (3 / 5 / 8); a learning quest expires on a move to a grade it does not hold, an exploration quest never.
func TestGradeMatrixQuests(t *testing.T) {
	qc, err := gamecfg.QuestFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	cfg := quest.ValidatorConfig{PassRule: qc.PassRule, BlockMaxByBand: qc.BlockMax}
	cat := quest.Catalog{Areas: map[string]quest.AreaRef{"learning_zone.math": {NPCs: []string{"npc.mr_numbers"}}},
		Chapters: map[string]bool{}, Quests: map[string]bool{}}
	wantBand := map[int]string{1: "g1_3", 2: "g1_3", 3: "g1_3", 4: "g4_5", 5: "g4_5", 6: "g6_9", 7: "g6_9", 8: "g6_9", 9: "g6_9"}
	wantBlock := map[string]int{"g1_3": 3, "g4_5": 5, "g6_9": 8}
	for _, g := range grades {
		band := quest.GradeBand(g)
		if band != wantBand[g] || qc.BlockMax[band] != wantBlock[band] {
			t.Fatalf("grade %d band %s block %d", g, band, qc.BlockMax[band])
		}
		limit := wantBlock[band]
		if err := quest.Validate(questOf([]int{g}, limit), cfg, cat); err != nil {
			t.Errorf("grade %d: %d questions in a row refused: %v", g, limit, err)
		}
		if err := quest.Validate(questOf([]int{g}, limit+1), cfg, cat); !errors.Is(err, quest.ErrInvalid) {
			t.Errorf("grade %d: %d questions in a row accepted: %v", g, limit+1, err)
		}
		v := questOf([]int{g}, 3)
		for _, h := range grades {
			d := quest.Decide(v, quest.Student{Grade: h}, quest.Facts{}, t0)
			if d.Visible != (h == g) {
				t.Errorf("quest of grade %d visible to grade %d: %v", g, h, d.Visible)
			}
			if v.ExpiresOnGradeChange(h) != (h != g) {
				t.Errorf("quest of grade %d on a move to grade %d: expires %v", g, h, v.ExpiresOnGradeChange(h))
			}
		}
		// Daily and weekly quests (granted by grade) expire like learning quests; exploration and social quests never.
		for typ, expires := range map[string]bool{quest.TypeDaily: true, quest.TypeWeekly: true, quest.TypeExploration: false,
			quest.TypeSocial: false} {
			other := v
			other.QuestType = typ
			for _, h := range grades {
				if other.ExpiresOnGradeChange(h) != (expires && h != g) {
					t.Errorf("%s quest of grade %d on a move to grade %d: expires %v", typ, g, h, other.ExpiresOnGradeChange(h))
				}
			}
		}
	}
	// A quest across a band boundary takes the stricter block of its lowest grade.
	for _, b := range []struct {
		gs    []int
		limit int
	}{{[]int{3, 4}, 3}, {[]int{5, 6}, 5}, {[]int{7, 8}, 8}} {
		if err := quest.Validate(questOf(b.gs, b.limit+1), cfg, cat); !errors.Is(err, quest.ErrInvalid) {
			t.Errorf("grades %v: block %d accepted", b.gs, b.limit+1)
		}
	}
}

// TestGradeMatrixWorldAndBands (F071, MULTIPLAYER_GAMEPLAY §6, GAME_ANALYTICS A-5): the grade bands of the world
// instance, the analytics dimension and the quest band agree on every grade; an area or dialogue of grades [g] admits
// grade g only; an area without grades admits every grade.
func TestGradeMatrixWorldAndBands(t *testing.T) {
	want4 := map[int]string{1: "g1_3", 2: "g1_3", 3: "g1_3", 4: "g4_5", 5: "g4_5", 6: "g6_7", 7: "g6_7", 8: "g8_9", 9: "g8_9"}
	for _, g := range grades {
		wb, ok := world.GradeBand(g)
		if !ok || wb != want4[g] || analytics.GradeBand(g) != want4[g] {
			t.Errorf("grade %d: world %s analytics %s, want %s", g, wb, analytics.GradeBand(g), want4[g])
		}
		area := world.Area{Access: world.AccessPolicy{GradeLevels: []int{g}}}
		dlg := world.Dialogue{GradeLevels: []int{g}}
		for _, h := range grades {
			if area.AdmitsGrade(h) != (h == g) || dlg.AdmitsGrade(h) != (h == g) {
				t.Errorf("area/dialogue of grade %d admits grade %d", g, h)
			}
		}
		if !(world.Area{}).AdmitsGrade(g) {
			t.Errorf("open area refuses grade %d", g)
		}
	}
	for _, g := range []int{0, 10} {
		if _, ok := world.GradeBand(g); ok || analytics.GradeBand(g) != analytics.Unknown {
			t.Errorf("grade %d has a band", g)
		}
	}
}

// TestGradeMatrixMiniGame (F073-AC-07/09, LEARNING_MECHANICS §2): a config of grades [g] is playable by grade g only;
// its display shows the speed bonus from grade 6 (grades 1–5 and an unknown grade never).
func TestGradeMatrixMiniGame(t *testing.T) {
	for _, g := range grades {
		c := minigame.Config{GradeLevels: []int{g}, Scoring: minigame.Scoring{PointsPerCorrect: 10, TimeBonus: &minigame.Bonus{Enabled: true}}}
		for _, h := range grades {
			if c.ForGrade(h) != (h == g) {
				t.Errorf("config of grade %d playable by grade %d", g, h)
			}
		}
		gg := g
		on := c.Display(&gg)["scoring"].(map[string]any)["time_bonus_enabled"].(bool)
		if on != (g >= 6) {
			t.Errorf("grade %d time bonus shown %v", g, on)
		}
	}
	c := minigame.Config{Scoring: minigame.Scoring{TimeBonus: &minigame.Bonus{Enabled: true}}}
	if c.Display(nil)["scoring"].(map[string]any)["time_bonus_enabled"].(bool) {
		t.Error("unknown grade sees the speed bonus")
	}
}

// TestGradeMatrixLiveOps (F077, F081-AC, EVENT_SEASON_SYSTEM §6, community_goal_grade_breakdown_grades,
// analytics_min_cell_size): an event audience of grades [g] includes grade g only; the by_grade breakdown of a school
// event shows grades 4–9 only and hides a grade below the minimum cell.
func TestGradeMatrixLiveOps(t *testing.T) {
	lo, err := gamecfg.LiveOpsFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	minCell := lo.Query.MinCell
	if minCell != 5 {
		t.Fatalf("analytics_min_cell_size from config: %d", minCell)
	}
	tenant, school := uuid.New(), uuid.New()
	for _, g := range grades {
		a := liveops.Audience{Scope: liveops.ScopeTenant, GradeLevels: []int{g}}
		if err := a.Validate(liveops.OwnerPlatform); err != nil {
			t.Fatal(err)
		}
		for _, h := range grades {
			if a.Includes(&tenant, tenant, school, h) != (h == g) {
				t.Errorf("audience of grade %d includes grade %d", g, h)
			}
		}
	}
	for _, bad := range []int{0, 10} {
		if (liveops.Audience{Scope: liveops.ScopeTenant, GradeLevels: []int{bad}}).Validate(liveops.OwnerPlatform) == nil {
			t.Errorf("audience grade %d accepted", bad)
		}
	}
	for _, n := range []int{minCell - 1, minCell} {
		var rows []liveops.GradeRow
		for _, g := range grades {
			rows = append(rows, liveops.GradeRow{Grade: g, Progress: 100, Contributors: n})
		}
		var shown []int
		for _, r := range liveops.VisibleGrades(rows, minCell) {
			shown = append(shown, r.Grade)
		}
		want := []int{}
		if n >= minCell {
			want = []int{4, 5, 6, 7, 8, 9}
		}
		if !slices.Equal(append([]int{}, shown...), want) {
			t.Errorf("%d contributors per grade: shown %v, want %v", n, shown, want)
		}
	}
}

// TestGradeMatrixShop (F075-AC-06): a listing of grades [g] is on sale for grade g only; without grades for every grade.
func TestGradeMatrixShop(t *testing.T) {
	for _, g := range grades {
		l := economy.Listing{Assigned: true, Currency: economy.Xu, Price: 10, Grades: []int{g}}
		for _, h := range grades {
			if l.Available(h, t0, false) != (h == g) {
				t.Errorf("listing of grade %d for grade %d", g, h)
			}
		}
		if !(economy.Listing{Assigned: true, Currency: economy.Xu, Price: 10}).Available(g, t0, false) {
			t.Errorf("open listing refused to grade %d", g)
		}
	}
}

// TestGradeMatrixAnalyticsSuppression (F080-AC-16, GAME_ANALYTICS A-5): the grade-band cells of a school — a band
// below analytics_min_cell_size is hidden, and with exactly one hidden band the smallest visible band is hidden too.
func TestGradeMatrixAnalyticsSuppression(t *testing.T) {
	a, err := gamecfg.AnalyticsFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	k := a.Config.MinCellSize
	if k != 5 {
		t.Fatalf("analytics_min_cell_size from config: %d", k)
	}
	bands := []string{"g1_3", "g4_5", "g6_7", "g8_9"}
	for i, small := range bands {
		cells := make([]analytics.Population, 0, len(bands))
		for j, b := range bands {
			n := int64(20 + j)
			if j == i {
				n = int64(k - 1)
			}
			cells = append(cells, analytics.Population{Key: b, Players: n})
		}
		hidden := analytics.Suppress(cells, k)
		smallestOther := bands[0]
		if i == 0 {
			smallestOther = bands[1]
		}
		if !hidden[small] || !hidden[smallestOther] || len(hidden) != 2 {
			t.Errorf("band %s below %d: hidden %v", small, k, hidden)
		}
	}
	full := []analytics.Population{{Key: "g1_3", Players: int64(k)}, {Key: "g4_5", Players: int64(k)}}
	if len(analytics.Suppress(full, k)) != 0 {
		t.Errorf("cells of exactly %d hidden", k)
	}
	if fmt.Sprint(analytics.Small(int64(k-1), k), analytics.Small(int64(k), k)) != "true false" {
		t.Error("Small")
	}
}
