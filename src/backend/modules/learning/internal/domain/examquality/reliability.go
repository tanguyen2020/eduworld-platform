package examquality

import (
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// Methods of the reliability (EXAM_ENGINE §8.1).
const (
	MethodKR20  = "kr20"
	MethodAlpha = "alpha"
)

// SuggestDropItem is the suggestion of a problem item (re-evaluation drop_item through the regrade process).
const SuggestDropItem = "drop_item"

// Config of the quality report (EXAM_ENGINE §8.1, CONFIG_DEFAULTS exam_reliability_warn: < 0.7 with ≥ 30 students;
// problem item: discrimination < 0.1 or p_value outside [0.1, 0.95] with ≥ item_analysis_min_responses answers; variant
// gap ≥ 1 point on the 10-point scale). PassScore10 is the pass mark of the pass rate [Proposed: 5/10].
type Config struct {
	MinStudents      int
	WarnBelow        float64
	GapPoints        float64
	MinItemResponses int
	PLow, PHigh      float64
	DiscLow          float64
	PassScore10      float64
}

// DefaultConfig returns the defaults of EXAM_ENGINE §8.1.
func DefaultConfig() Config {
	return Config{MinStudents: 30, WarnBelow: 0.7, GapPoints: 1, MinItemResponses: 30, PLow: 0.1, PHigh: 0.95, DiscLow: 0.1, PassScore10: 5}
}

// ItemScore is the graded answer of one item in one attempt.
type ItemScore struct {
	QuestionVersionID uuid.UUID
	Score, Max        float64
	Correct           bool
	RapidGuess        bool
	TimeMS            *int
	Misconceptions    []string // misconception tags of the wrong options chosen
}

// Sheet is the graded attempt of one student (score policy of the session already applied).
type Sheet struct {
	StudentID  uuid.UUID
	VariantNo  int
	Items      []ItemScore
	Total, Max float64
}

// VariantStats of one variant.
type VariantStats struct {
	VariantNo   int      `json:"variant_no"`
	N           int      `json:"n"`
	Mean10      float64  `json:"mean"`
	SD10        float64  `json:"sd"`
	Enough      bool     `json:"enough_data"`
	Reliability *float64 `json:"reliability"`
	Method      string   `json:"method,omitempty"`
	SEM         *float64 `json:"sem"`
	ExpectedP   *float64 `json:"expected_p"`
	Warning     bool     `json:"reliability_warning"`
}

// TagCount is a misconception tag and how many wrong answers carried it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ItemStats of one question version over the session (rapid guesses excluded).
type ItemStats struct {
	QuestionVersionID uuid.UUID  `json:"question_version_id"`
	N                 int        `json:"n"`
	PValue            *float64   `json:"p_value"`
	Discrimination    *float64   `json:"discrimination"`
	AvgTimeMS         *int       `json:"avg_time_ms"`
	RapidGuessRate    float64    `json:"rapid_guess_rate"`
	TopMisconceptions []TagCount `json:"top_misconceptions"`
	Problem           bool       `json:"problem"`
	Suggestion        string     `json:"suggestion,omitempty"`
}

// Report is the quality report of a session (assessment.exam_quality_reports).
type Report struct {
	Variants           []VariantStats
	VariantGap         bool
	ReliabilityWarning bool
	Distribution       []int // 21 bins of 0.5 point on the 10-point scale
	Mean, Median       float64
	PassRate           float64
	N                  int
	Items              []ItemStats
}

func score10(s Sheet) float64 {
	if s.Max <= 0 {
		return 0
	}
	return s.Total / s.Max * 10
}

func meanSD(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	m := sum / float64(len(xs))
	var ss float64
	for _, x := range xs {
		ss += (x - m) * (x - m)
	}
	return m, math.Sqrt(ss / float64(len(xs)))
}

// Analyze computes the report (F044-AC-23, F044-AC-24). Reliability per variant: KR-20 when every item is scored 0/max,
// else Cronbach alpha; not computed under MinStudents ("not enough data") or without variance; SEM = SD × √(1 − r) on the
// 10-point scale. The score is never changed here.
func Analyze(sheets []Sheet, expected map[int]*float64, cfg Config) Report {
	rep := Report{Distribution: make([]int, 21), N: len(sheets)}
	byVariant := map[int][]Sheet{}
	var all []float64
	for _, s := range sheets {
		byVariant[s.VariantNo] = append(byVariant[s.VariantNo], s)
		x := score10(s)
		all = append(all, x)
		rep.Distribution[min(int(math.Floor(x*2+1e-9)), 20)]++
		if x >= cfg.PassScore10 {
			rep.PassRate++
		}
	}
	if len(all) > 0 {
		rep.PassRate /= float64(len(all))
		rep.Mean, _ = meanSD(all)
		sorted := slices.Clone(all)
		slices.Sort(sorted)
		n := len(sorted)
		rep.Median = sorted[n/2]
		if n%2 == 0 {
			rep.Median = (sorted[n/2-1] + sorted[n/2]) / 2
		}
	}
	nos := make([]int, 0, len(byVariant))
	for v := range byVariant {
		nos = append(nos, v)
	}
	slices.Sort(nos)
	for _, v := range nos {
		vs := variantStats(byVariant[v], cfg)
		vs.VariantNo, vs.ExpectedP = v, expected[v]
		rep.ReliabilityWarning = rep.ReliabilityWarning || vs.Warning
		rep.Variants = append(rep.Variants, vs)
	}
	if len(rep.Variants) >= 2 {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, v := range rep.Variants {
			lo, hi = math.Min(lo, v.Mean10), math.Max(hi, v.Mean10)
		}
		rep.VariantGap = hi-lo >= cfg.GapPoints
	}
	rep.Items = itemStats(sheets, cfg)
	return rep
}

func variantStats(sheets []Sheet, cfg Config) VariantStats {
	scores := make([]float64, len(sheets))
	for i, s := range sheets {
		scores[i] = score10(s)
	}
	vs := VariantStats{N: len(sheets)}
	vs.Mean10, vs.SD10 = meanSD(scores)
	if len(sheets) < cfg.MinStudents {
		return vs
	}
	vs.Enough = true
	// Item columns: the union of the versions of the variant; a missing answer scores 0.
	var cols []uuid.UUID
	maxOf := map[uuid.UUID]float64{}
	for _, s := range sheets {
		for _, it := range s.Items {
			if _, ok := maxOf[it.QuestionVersionID]; !ok {
				cols = append(cols, it.QuestionVersionID)
			}
			maxOf[it.QuestionVersionID] = math.Max(maxOf[it.QuestionVersionID], it.Max)
		}
	}
	k := len(cols)
	if k < 2 {
		return vs
	}
	binary := true
	matrix := make([][]float64, len(sheets))
	totals := make([]float64, len(sheets))
	for i, s := range sheets {
		row := make([]float64, k)
		for _, it := range s.Items {
			j := slices.Index(cols, it.QuestionVersionID)
			row[j] = it.Score
			if it.Score != 0 && it.Score != maxOf[it.QuestionVersionID] {
				binary = false
			}
		}
		for _, x := range row {
			totals[i] += x
		}
		matrix[i] = row
	}
	_, sdT := meanSD(totals)
	varT := sdT * sdT
	if varT == 0 {
		return vs
	}
	var sumItemVar float64
	for j := 0; j < k; j++ {
		col := make([]float64, len(matrix))
		for i := range matrix {
			col[i] = matrix[i][j]
			if binary && maxOf[cols[j]] > 0 {
				col[i] /= maxOf[cols[j]]
			}
		}
		m, sd := meanSD(col)
		if binary {
			sumItemVar += m * (1 - m) // p·q
		} else {
			sumItemVar += sd * sd
		}
	}
	if binary {
		// KR-20 on 0/1 items: σ² of the totals counted in items answered right.
		normTotals := make([]float64, len(matrix))
		for i := range matrix {
			for j := range cols {
				if maxOf[cols[j]] > 0 {
					normTotals[i] += matrix[i][j] / maxOf[cols[j]]
				}
			}
		}
		_, sdN := meanSD(normTotals)
		varT = sdN * sdN
		if varT == 0 {
			return vs
		}
	}
	r := float64(k) / float64(k-1) * (1 - sumItemVar/varT)
	vs.Method = MethodAlpha
	if binary {
		vs.Method = MethodKR20
	}
	sem := vs.SD10 * math.Sqrt(math.Max(0, 1-r))
	vs.Reliability, vs.SEM = &r, &sem
	vs.Warning = r < cfg.WarnBelow
	return vs
}

func itemStats(sheets []Sheet, cfg Config) []ItemStats {
	type acc struct {
		ratio, total []float64
		rapid, all   int
		timeSum      int
		timeN        int
		tags         map[string]int
	}
	byItem := map[uuid.UUID]*acc{}
	var order []uuid.UUID
	for _, s := range sheets {
		tot := 0.0
		if s.Max > 0 {
			tot = s.Total / s.Max
		}
		for _, it := range s.Items {
			a, ok := byItem[it.QuestionVersionID]
			if !ok {
				a = &acc{tags: map[string]int{}}
				byItem[it.QuestionVersionID] = a
				order = append(order, it.QuestionVersionID)
			}
			a.all++
			if it.RapidGuess {
				a.rapid++
				continue
			}
			r := 0.0
			if it.Max > 0 {
				r = it.Score / it.Max
			}
			a.ratio, a.total = append(a.ratio, r), append(a.total, tot)
			if it.TimeMS != nil {
				a.timeSum += *it.TimeMS
				a.timeN++
			}
			if !it.Correct {
				for _, tag := range it.Misconceptions {
					a.tags[tag]++
				}
			}
		}
	}
	out := make([]ItemStats, 0, len(order))
	for _, id := range order {
		a := byItem[id]
		st := ItemStats{QuestionVersionID: id, N: len(a.ratio), TopMisconceptions: topTags(a.tags, 3)}
		if a.all > 0 {
			st.RapidGuessRate = float64(a.rapid) / float64(a.all)
		}
		if a.timeN > 0 {
			t := a.timeSum / a.timeN
			st.AvgTimeMS = &t
		}
		if len(a.ratio) > 0 {
			p, _ := meanSD(a.ratio)
			st.PValue = &p
			st.Discrimination = pearson(a.ratio, a.total)
			if len(a.ratio) >= cfg.MinItemResponses &&
				(p < cfg.PLow || p > cfg.PHigh || st.Discrimination == nil || *st.Discrimination < cfg.DiscLow) {
				st.Problem, st.Suggestion = true, SuggestDropItem
			}
		}
		out = append(out, st)
	}
	return out
}

// pearson is the correlation of the item ratio with the attempt total (the point-biserial for 0/1 items); nil without
// variance.
func pearson(x, y []float64) *float64 {
	mx, sx := meanSD(x)
	my, sy := meanSD(y)
	if sx == 0 || sy == 0 {
		return nil
	}
	var cov float64
	for i := range x {
		cov += (x[i] - mx) * (y[i] - my)
	}
	r := cov / float64(len(x)) / (sx * sy)
	return &r
}

func topTags(m map[string]int, n int) []TagCount {
	out := make([]TagCount, 0, len(m))
	for t, c := range m {
		out = append(out, TagCount{Tag: t, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return strings.Compare(out[i].Tag, out[j].Tag) < 0
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
