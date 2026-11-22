package unit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"

	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
)

func def(metric, scope, period string, grades ...int) ldb.Definition {
	return ldb.Definition{ID: uuid.New(), Code: "c-" + metric + "-" + scope, Title: "Bảng", Metric: metric, Scope: scope, Period: period,
		Grades: grades, Status: "active"}
}

// seedDefinitions decodes seed/leaderboard_definitions.json (the default catalog, COMPETITION_SYSTEM §9.2).
func seedDefinitions(t *testing.T) []ldb.Definition {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "seed", "leaderboard_definitions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Definitions []struct {
			ID      uuid.UUID   `json:"leaderboard_id"`
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
			Status string `json:"status"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	var out []ldb.Definition
	for _, d := range doc.Definitions {
		out = append(out, ldb.Definition{ID: d.ID, Code: d.Code, Title: d.Title, Metric: d.Metric, Scope: d.Scope, Period: d.Period,
			Filters: d.Filters, Grades: d.Grades, Status: d.Status, GradeGoalTarget: d.Display.Target})
	}
	return out
}

// F106-AC-12: the 12 board types of FR-LDB-001 are definitions (metric × scope × period × filter): Global, School,
// Grade, Subject, Topic, Game Mode, Friends, Team, Daily, Weekly, Monthly, Season — and the default catalog is valid.
// Scope class and metric class_goal are refused (ADR-0095).
func TestDefinitionsCoverTwelveTypes(t *testing.T) {
	subject, topic, mode := uuid.New(), uuid.New(), "battle_1v1"
	withF := func(d ldb.Definition, f ldb.Filters) ldb.Definition { d.Filters = f; return d }
	types := map[string]ldb.Definition{
		"global":    def("competition_points", "global", "season", 6, 7, 8, 9),
		"school":    def("competition_points", "school", "weekly", 4, 5, 6),
		"grade":     def("competition_points", "grade", "weekly", 4, 5),
		"subject":   withF(def("competition_points", "grade", "season", 6), ldb.Filters{SubjectID: &subject}),
		"topic":     withF(def("competition_points", "grade", "season", 6), ldb.Filters{SubjectID: &subject, TopicID: &topic}),
		"game_mode": withF(def("competition_points", "school", "season", 6), ldb.Filters{Mode: &mode}),
		"friends":   def("xp", "friends", "weekly", 4, 9),
		"team":      def("team_points", "school", "season", 6, 7),
		"daily":     def("competition_points", "grade", "daily", 7),
		"weekly":    def("effort_points", "friends", "weekly", 4),
		"monthly":   def("competition_points", "tenant", "monthly", 8),
		"season":    withF(def("rating", "grade", "season", 6, 7, 8, 9), ldb.Filters{SubjectID: &subject, Mode: &mode}),
	}
	if len(types) != 12 {
		t.Fatal("12 types")
	}
	for name, d := range types {
		if err := d.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, d := range seedDefinitions(t) {
		if err := d.Validate(); err != nil {
			t.Fatalf("seed %s: %v", d.Code, err)
		}
	}
	for _, d := range []ldb.Definition{def("competition_points", "class", "weekly", 3), def("class_goal", "grade", "weekly", 3)} {
		if err := d.Validate(); !errors.Is(err, ldb.ErrDefinition) {
			t.Fatalf("%s/%s accepted", d.Metric, d.Scope)
		}
	}
}

// F106-AC-16 (AC-LDB-007): no school/grade (or tenant/global) board ranks XP; grade_goal is one grade of a school,
// weekly, with a target; all_time only for xp/quest_completed; rating boards for grades 6–9 with mode and subject.
func TestDefinitionInvariants(t *testing.T) {
	gg := def("grade_goal", "grade", "weekly", 1, 2, 3)
	gg.GradeGoalTarget = 300
	if err := gg.Validate(); err != nil {
		t.Fatal(err)
	}
	noTarget := gg
	noTarget.GradeGoalTarget = 0
	badGG := gg
	badGG.Scope = "school"
	for name, d := range map[string]ldb.Definition{
		"xp school": def("xp", "school", "weekly", 6), "xp grade": def("xp", "grade", "weekly", 6),
		"xp tenant": def("xp", "tenant", "weekly", 6), "grade_goal no target": noTarget, "grade_goal school": badGG,
		"all_time points": def("competition_points", "grade", "all_time", 6), "rating g4": def("rating", "grade", "season", 4),
		"rating no filters": def("rating", "grade", "season", 7), "effort filter": func() ldb.Definition {
			d := def("effort_points", "friends", "weekly", 6)
			s := uuid.New()
			d.Filters.SubjectID = &s
			return d
		}(), "grade 0": def("competition_points", "grade", "weekly", 0), "no grades": def("competition_points", "grade", "weekly"),
	} {
		if err := d.Validate(); !errors.Is(err, ldb.ErrDefinition) {
			t.Fatalf("%s accepted", name)
		}
	}
}

// F106-AC-16: effort_points of the Ranking Rules — +1 per activity (10 a day), +2 for a review activity, +3 for an active
// day, at most 25 a day; a rapid-guess attempt earns nothing; right or wrong never matters.
func TestEffortPoints(t *testing.T) {
	tb := seedRules(t).Effort
	a := ldb.Effort(tb, ldb.Activity{}, ldb.EffortDay{})
	if a.Activity != 1 || a.ActiveDay != 3 || a.ActivitySource != ldb.SourceActivity {
		t.Fatalf("first activity: %+v", a)
	}
	if a := ldb.Effort(tb, ldb.Activity{Review: true}, ldb.EffortDay{Activities: 2, Total: 5, Active: true}); a.Activity != 2 ||
		a.ActiveDay != 0 || a.ActivitySource != ldb.SourceReview {
		t.Fatalf("review: %+v", a)
	}
	if a := ldb.Effort(tb, ldb.Activity{RapidGuess: true}, ldb.EffortDay{}); a.Activity != 0 || a.ActiveDay != 0 {
		t.Fatalf("rapid guess: %+v", a)
	}
	if a := ldb.Effort(tb, ldb.Activity{}, ldb.EffortDay{Activities: 10, Total: 13, Active: true}); a.Activity != 0 {
		t.Fatalf("11th activity: %+v", a)
	}
	if a := ldb.Effort(tb, ldb.Activity{Review: true}, ldb.EffortDay{Activities: 9, Total: 24, Active: true}); a.Activity != 1 {
		t.Fatalf("daily cap: %+v", a)
	}
	// 12 activities of which 2 rapid guesses, 1 review: 3 (active day) + 9·1 + 2 = 14 ≤ 25, the 2 rapid ones count 0
	day := ldb.EffortDay{}
	for i := 0; i < 12; i++ {
		act := ldb.Activity{RapidGuess: i == 4 || i == 7, Review: i == 10}
		a := ldb.Effort(tb, act, day)
		if act.RapidGuess {
			if a.Activity+a.ActiveDay != 0 {
				t.Fatal("a rapid guess earned effort")
			}
			continue
		}
		if a.Activity > 0 {
			day.Activities++
		}
		day.Total += a.Activity + a.ActiveDay
		day.Active = day.Active || a.ActiveDay > 0
	}
	if day.Total != 14 || day.Total > tb.DailyCap {
		t.Fatalf("day total %d", day.Total)
	}
}

// F106-AC-15: period keys in Asia/Ho_Chi_Minh — the week closes at 00:00 Monday ICT (Sunday 17:00 UTC).
func TestPeriodKeysICT(t *testing.T) {
	sun := time.Date(2026, 11, 22, 16, 59, 59, 0, time.UTC) // Sunday 23:59:59 ICT
	mon := sun.Add(time.Second)                             // Monday 00:00 ICT
	if ldb.Key("weekly", sun, nil) != "2026-W47" || ldb.Key("weekly", mon, nil) != "2026-W48" {
		t.Fatalf("weekly keys %s %s", ldb.Key("weekly", sun, nil), ldb.Key("weekly", mon, nil))
	}
	if ldb.Key("daily", mon, nil) != "2026-11-23" || ldb.Key("monthly", time.Date(2026, 11, 30, 17, 0, 0, 0, time.UTC), nil) != "2026-12" {
		t.Fatal("daily/monthly keys")
	}
	s, e, ok := ldb.Bounds("weekly", "2026-W48", nil)
	if !ok || !s.Equal(mon) || !e.Equal(mon.AddDate(0, 0, 7)) {
		t.Fatalf("bounds %v %v %v", s, e, ok)
	}
	if ldb.Previous("weekly", mon.Add(5*time.Minute)) != "2026-W47" || ldb.Previous("monthly", mon) != "2026-10" {
		t.Fatal("previous period")
	}
	if _, _, ok := ldb.Bounds("weekly", "2026-W99", nil); ok {
		t.Fatal("malformed week accepted")
	}
	// equal points: who got there first ranks above
	early := ldb.Score(30, ldb.Tick(s, s.Add(time.Hour)))
	late := ldb.Score(30, ldb.Tick(s, s.Add(48*time.Hour)))
	if early <= late || ldb.ValueOf(early) != 30 || ldb.ValueOf(late) != 30 || ldb.Score(31, ldb.Tick(s, e.Add(-time.Second))) <= early {
		t.Fatal("tie-break by time")
	}
}
