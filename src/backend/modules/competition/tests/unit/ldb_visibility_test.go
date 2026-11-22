package unit

import (
	"testing"

	"github.com/google/uuid"

	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
)

func board(n int) []ldb.Ranked {
	out := make([]ldb.Ranked, n)
	for i := range out {
		out[i] = ldb.Ranked{Student: uuid.New(), Rank: i + 1, Value: float64(1000 - i)}
	}
	return out
}

// F106-AC-17 (SC-004): grades 1–3 never get a personal rank — every ranked board is closed to them, only the collective
// goal of their grade (grade_goal) is open; grades 4–9 open the boards of their grades.
func TestVisibilityGrades1to3(t *testing.T) {
	v := seedRules(t).Visibility
	gg := def("grade_goal", "grade", "weekly", 1, 2, 3, 4, 5, 6, 7, 8, 9)
	arena := def("competition_points", "grade", "weekly", 1, 2, 3, 4, 5, 6, 7, 8, 9)
	for g := 1; g <= 3; g++ {
		if !ldb.Sees(v, gg, g) || ldb.Sees(v, arena, g) {
			t.Fatalf("grade %d", g)
		}
		b := board(30)
		view := ldb.Apply(v, g, 30, b, &b[3], b)
		if view.Visibility != ldb.VisibilityGradeGoal || len(view.Entries) != 0 || view.Me != nil || len(view.Around) != 0 {
			t.Fatalf("grade %d sees a ranking: %+v", g, view)
		}
	}
	if !ldb.Sees(v, arena, 4) || ldb.Sees(v, def("competition_points", "grade", "weekly", 6), 4) {
		t.Fatal("grade 4")
	}
}

// F106-AC-17: grades 4–5 see the top 10 and their own place; outside the top 10 only "nhóm X%"; the last member is never
// shown to others.
func TestVisibilityGrades4to5(t *testing.T) {
	v := seedRules(t).Visibility
	b := board(40)
	view := ldb.Apply(v, 4, 40, b, &b[24], nil)
	if view.Visibility != ldb.VisibilityPercentile || len(view.Entries) != 10 || view.Me.Rank != nil || *view.Me.Percentile != 100 {
		t.Fatalf("rank 25/40: %+v", view)
	}
	if view := ldb.Apply(v, 5, 40, b, &b[3], nil); view.Me.Rank == nil || *view.Me.Rank != 4 || view.Me.Percentile != nil {
		t.Fatalf("rank 4: %+v", view.Me)
	}
	if ldb.Bucket(4, 40) != 10 || ldb.Bucket(9, 40) != 25 || ldb.Bucket(20, 40) != 50 || ldb.Bucket(21, 40) != 100 {
		t.Fatal("buckets")
	}
	small := board(6)
	view = ldb.Apply(v, 4, 6, small, &small[0], nil)
	for _, e := range view.Entries {
		if e.Rank == 6 {
			t.Fatal("the last member is shown")
		}
	}
	if len(view.Entries) != 5 {
		t.Fatalf("entries %d", len(view.Entries))
	}
	// the last member sees itself
	view = ldb.Apply(v, 4, 6, small, &small[5], nil)
	if len(view.Entries) != 6 {
		t.Fatalf("the last member does not see itself: %d", len(view.Entries))
	}
}

// F106-AC-17: grades 6–9 see the top 50 and ±3 around themselves.
func TestVisibilityGrades6to9(t *testing.T) {
	v := seedRules(t).Visibility
	b := board(120)
	view := ldb.Apply(v, 7, 120, b, &b[79], b[70:90])
	if view.Visibility != ldb.VisibilityRanked || len(view.Entries) != 50 || *view.Me.Rank != 80 || len(view.Around) != 7 ||
		view.Around[0].Rank != 77 || view.Around[6].Rank != 83 {
		t.Fatalf("grade 7: entries %d around %+v", len(view.Entries), view.Around)
	}
}

// F106-AC-16: the collective goal of a grade has no rank and no name; the contributors cell is hidden below
// analytics_min_cell_size (5) students.
func TestGradeGoalSmallCell(t *testing.T) {
	min := seedRules(t).MinCellSize
	mine := 12.0
	g := ldb.NewGradeGoal(3, 41, 300, &mine, 3, min)
	if g.Contributors != nil || g.Reached || g.Current != 41 || *g.Mine != 12 {
		t.Fatalf("3 contributors: %+v", g)
	}
	g = ldb.NewGradeGoal(3, 301, 300, nil, 5, min)
	if g.Contributors == nil || *g.Contributors != 5 || !g.Reached {
		t.Fatalf("5 contributors: %+v", g)
	}
}
