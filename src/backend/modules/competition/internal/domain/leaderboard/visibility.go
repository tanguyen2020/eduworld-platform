package leaderboard

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// Visibilities of a page (LeaderboardPage.visibility).
const (
	VisibilityRanked     = "ranked"
	VisibilityPercentile = "top_n_percentile"
	VisibilityGradeGoal  = "grade_goal"
)

// Member is where a student stands: tenant, school and grade (ref.students, ADR-0095: no class).
type Member struct {
	Tenant uuid.UUID
	School uuid.UUID
	Grade  int
}

// ScopeKey is the key of the scope of a board that holds m (the sorted set and the snapshot rows): school
// "<school>", grade "<school>:g<N>", tenant "<tenant>:g<N>", global "g<N>", friends/team/tournament "all" (friends are
// read from the tenant-wide base set of the board). Boards across schools are split by grade (fairness §5).
func ScopeKey(d Definition, m Member) string {
	switch d.Scope {
	case ScopeSchool:
		return m.School.String()
	case ScopeGrade:
		return GradeKey(m.School, m.Grade)
	case ScopeTenant:
		return fmt.Sprintf("%s:g%d", m.Tenant, m.Grade)
	case ScopeGlobal:
		return fmt.Sprintf("g%d", m.Grade)
	}
	return "all"
}

// GradeKey is the scope key of one grade of one school.
func GradeKey(school uuid.UUID, grade int) string { return fmt.Sprintf("%s:g%d", school, grade) }

// Band is the leaderboard_visibility band of a grade.
func Band(v ranking.Visibility, grade int) (ranking.BandVisibility, string) {
	switch {
	case grade <= 3:
		return v.G13, "g1_3"
	case grade <= 5:
		return v.G45, "g4_5"
	}
	return v.G69, "g6_9"
}

// Sees reports whether a student of grade may open the board at all (F106-AC-17): grades 1–3 only see the collective
// goal of their grade (no personal rank anywhere, SC-004).
func Sees(v ranking.Visibility, d Definition, grade int) bool {
	if !d.ForGrade(grade) {
		return false
	}
	b, _ := Band(v, grade)
	return d.Metric == MetricGradeGoal || b.PersonalRank || grade > 3
}

// Ranked is one ranked member of a board.
type Ranked struct {
	Student uuid.UUID
	Rank    int
	Value   float64
}

// Me is the viewer's own position: the rank, or only "nhóm X%" outside the top of grades 4–5.
type Me struct {
	Rank       *int
	Value      *float64
	Percentile *int
}

// View is what a student sees of a board.
type View struct {
	Visibility string
	Entries    []Ranked
	Me         *Me
	Around     []Ranked
}

// Percentiles are the buckets of "Em đang ở nhóm X%".
var Percentiles = []int{10, 25, 50, 100}

// Bucket is the percentile bucket of rank among total.
func Bucket(rank, total int) int {
	if total <= 0 {
		return 100
	}
	p := float64(rank) * 100 / float64(total)
	for _, b := range Percentiles {
		if p <= float64(b) {
			return b
		}
	}
	return 100
}

// Apply applies leaderboard_visibility for a student of grade (F106-AC-17): top is the head of the board (at least the
// band's top), me the viewer (nil when not on the board), around the members within ±Around of the viewer, total the
// number of members. Grades 4–5: top N, the last member never shown to others, "nhóm X%" outside the top; grades 6–9:
// top N and ±Around.
func Apply(v ranking.Visibility, grade int, total int, top []Ranked, me *Ranked, around []Ranked) View {
	b, band := Band(v, grade)
	switch band {
	case "g1_3":
		return View{Visibility: VisibilityGradeGoal}
	case "g4_5":
		out := View{Visibility: VisibilityPercentile}
		for _, r := range top {
			if r.Rank > b.Top {
				break
			}
			if b.HideLast && total > 1 && r.Rank == total && (me == nil || me.Student != r.Student) {
				continue
			}
			out.Entries = append(out.Entries, r)
		}
		if me != nil {
			val := me.Value
			m := &Me{Value: &val}
			if me.Rank <= b.Top {
				rk := me.Rank
				m.Rank = &rk
			} else if b.Percentile {
				p := Bucket(me.Rank, total)
				m.Percentile = &p
			}
			out.Me = m
		}
		return out
	}
	out := View{Visibility: VisibilityRanked}
	for _, r := range top {
		if r.Rank > b.Top {
			break
		}
		out.Entries = append(out.Entries, r)
	}
	if me != nil {
		rk, val := me.Rank, me.Value
		out.Me = &Me{Rank: &rk, Value: &val}
		for _, r := range around {
			if r.Rank >= me.Rank-b.Around && r.Rank <= me.Rank+b.Around {
				out.Around = append(out.Around, r)
			}
		}
	}
	return out
}

// GradeGoal is the collective goal of one grade of one school for a week (ADR-0095 §5): the total effort of the grade,
// the target of the definition, the viewer's own share and the number of contributors — never a rank, a nickname or a
// name of another student.
type GradeGoal struct {
	Grade        int
	Current      float64
	Target       float64
	Reached      bool
	Mine         *float64
	Contributors *int
}

// NewGradeGoal builds the goal; the contributors cell is hidden (nil) below minCell students (analytics_min_cell_size,
// F106-AC-16).
func NewGradeGoal(grade int, current, target float64, mine *float64, contributors, minCell int) GradeGoal {
	g := GradeGoal{Grade: grade, Current: current, Target: target, Reached: target > 0 && current >= target, Mine: mine}
	if contributors >= minCell {
		n := contributors
		g.Contributors = &n
	}
	return g
}
