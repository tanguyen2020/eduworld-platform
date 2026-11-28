package devseed

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// GoldenValues are the values of the School Learning Dashboard (F043, F056) for one grade of a school after the attempt
// script on a stack where no other student of that grade did anything: the school dashboard filtered on the grade
// (getSchoolDashboard) and the mastery heatmap rows of the grade (getGradeDashboard objective_distribution, read model
// refreshed every school_dashboard_refresh).
type GoldenValues struct {
	ActiveStudents  *int                       `json:"active_students,omitempty"`
	Accuracy        *float64                   `json:"accuracy,omitempty"`
	RapidGuessRatio *float64                   `json:"rapid_guess_ratio,omitempty"`
	Objectives      map[string]GoldenObjective `json:"objectives,omitempty"` // by objective code
	// Assignments are the progress of the DEV assignments of the grade by title (widgets.assignments).
	Assignments map[string]GoldenAssignment `json:"assignments,omitempty"`
}

// GoldenAssignment is the progress of one assignment for the grade.
type GoldenAssignment struct {
	Submitted int `json:"submitted"`
	Total     int `json:"total"`
}

// GoldenObjective is one heatmap row: students with evidence and their count per mastery level.
type GoldenObjective struct {
	StudentsWithEvidence int            `json:"students_with_evidence"`
	LevelCounts          map[string]int `json:"level_counts"`
}

// ict is the time zone of the dashboard days of the demo tenants (Asia/Ho_Chi_Minh, no DST).
var ict = time.FixedZone("ICT", 7*3600)

// DefaultRange is the default range of the dashboard of the portal (the last 7 days ending today, ICT — F056
// defaultRange): the range the golden values are read on.
func DefaultRange(now time.Time) (from, to string) {
	t := now.In(ict)
	return t.AddDate(0, 0, -6).Format(time.DateOnly), t.Format(time.DateOnly)
}

// Snapshot reads the dashboard values of (school, grade) on [from, to] as a staff member with analytics.school.read;
// codes are the heatmap rows to read.
func Snapshot(ctx context.Context, hc *http.Client, base, token, tenantID, schoolID string, grade int, from, to string, codes []string) (GoldenValues, error) {
	var out GoldenValues
	h := map[string]string{"X-Tenant-Id": tenantID}
	var sd struct {
		Widgets struct {
			ActiveStudents  *int     `json:"active_students"`
			Accuracy        *float64 `json:"accuracy"`
			RapidGuessRatio *float64 `json:"rapid_guess_ratio"`
			Assignments     []struct {
				Title     string `json:"title"`
				Submitted int    `json:"submitted"`
				Total     int    `json:"total"`
			} `json:"assignments"`
		} `json:"widgets"`
	}
	q := url.Values{"school_id": {schoolID}, "grade_level": {fmt.Sprint(grade)}, "from": {from}, "to": {to}}
	if _, err := Do(ctx, hc, base, token, http.MethodGet, "/lms/dashboards/school?"+q.Encode(), h, nil, &sd); err != nil {
		return out, err
	}
	out.ActiveStudents, out.Accuracy, out.RapidGuessRatio = sd.Widgets.ActiveStudents, sd.Widgets.Accuracy, sd.Widgets.RapidGuessRatio
	out.Assignments = map[string]GoldenAssignment{}
	for _, a := range sd.Widgets.Assignments {
		out.Assignments[a.Title] = GoldenAssignment{Submitted: a.Submitted, Total: a.Total}
	}
	var gd struct {
		Objectives []struct {
			Code                 string         `json:"code"`
			StudentsWithEvidence int            `json:"students_with_evidence"`
			LevelCounts          map[string]int `json:"level_counts"`
		} `json:"objective_distribution"`
	}
	gq := url.Values{"from": {from}, "to": {to}}
	if _, err := Do(ctx, hc, base, token, http.MethodGet, fmt.Sprintf("/lms/schools/%s/grades/%d/dashboard?%s", schoolID, grade, gq.Encode()), h, nil, &gd); err != nil {
		return out, err
	}
	out.Objectives = map[string]GoldenObjective{}
	for _, o := range gd.Objectives {
		if slices.Contains(codes, o.Code) {
			out.Objectives[o.Code] = GoldenObjective{StudentsWithEvidence: o.StudentsWithEvidence, LevelCounts: nonZero(o.LevelCounts)}
		}
	}
	return out, nil
}

func nonZero(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		if v != 0 {
			out[k] = v
		}
	}
	return out
}

// Diff lists the differences between the expected golden values and the observed ones (empty = match); a field that is
// absent from want is not checked.
// Ratios are compared to ±0.0005: the API rounds them to 3 decimals.
func (want GoldenValues) Diff(got GoldenValues) []string {
	var d []string
	intEq := func(name string, w, g *int) {
		if w != nil && (g == nil || *w != *g) {
			d = append(d, fmt.Sprintf("%s: want %d, got %s", name, *w, fmtPtr(g)))
		}
	}
	floatEq := func(name string, w, g *float64) {
		if w != nil && (g == nil || math.Abs(*w-*g) > 5e-4) {
			d = append(d, fmt.Sprintf("%s: want %.4f, got %s", name, *w, fmtPtr(g)))
		}
	}
	intEq("active_students", want.ActiveStudents, got.ActiveStudents)
	floatEq("accuracy", want.Accuracy, got.Accuracy)
	floatEq("rapid_guess_ratio", want.RapidGuessRatio, got.RapidGuessRatio)
	for _, title := range sortedAssignmentTitles(want.Assignments) {
		if g, ok := got.Assignments[title]; !ok || g != want.Assignments[title] {
			d = append(d, fmt.Sprintf("assignment %q: want %+v, got %+v (listed %v)", title, want.Assignments[title], g, ok))
		}
	}
	for _, code := range sortedObjectiveCodes(want.Objectives) {
		w := want.Objectives[code]
		g, ok := got.Objectives[code]
		if !ok {
			d = append(d, fmt.Sprintf("objective %s: missing from the heatmap", code))
			continue
		}
		if w.StudentsWithEvidence != g.StudentsWithEvidence || !mapsEqual(nonZero(w.LevelCounts), g.LevelCounts) {
			d = append(d, fmt.Sprintf("objective %s: want %d students %v, got %d students %v", code, w.StudentsWithEvidence,
				nonZero(w.LevelCounts), g.StudentsWithEvidence, g.LevelCounts))
		}
	}
	return d
}

// Codes returns the objective codes of the golden values.
func (want GoldenValues) Codes() []string { return sortedObjectiveCodes(want.Objectives) }

func sortedObjectiveCodes(m map[string]GoldenObjective) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func sortedAssignmentTitles(m map[string]GoldenAssignment) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func fmtPtr[T int | float64](p *T) string {
	if p == nil {
		return "null"
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", float64(*p)), "0"), ".")
}
