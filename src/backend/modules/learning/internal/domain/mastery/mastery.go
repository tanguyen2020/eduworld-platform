// Package mastery is the mastery model of module learning (ADR-0017, LEARNING_ANALYTICS §3): the state of a
// (student, learning objective) pair, the Weighted Evidence EWMA update and the decay applied when reading. The
// package is pure: parameters come from the versioned configuration (learning.mastery_model_configs) and are passed in.
package mastery

import (
	"errors"
	"math"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // day_location (Asia/Ho_Chi_Minh) in images without a zoneinfo database

	"github.com/google/uuid"
)

// Level is the MasteryLevel of the API (LEARNING_ANALYTICS §3.2).
type Level string

// Levels in ascending order.
const (
	NotStarted Level = "not_started"
	Beginning  Level = "beginning"
	Developing Level = "developing"
	Proficient Level = "proficient"
	Mastered   Level = "mastered"
)

// Prior sources of a state (F032 writes diagnostic).
const (
	PriorDefault    = "default"
	PriorDiagnostic = "diagnostic"
)

// State is the stored mastery of one (student, objective): Value is the raw value before decay.
type State struct {
	Value            float64
	EvidenceCount    int
	DistinctDays     int
	LastEvidenceAt   *time.Time
	LastEvidenceDate string // YYYY-MM-DD in the day location of the parameters
	FirstMasteredAt  *time.Time
	PriorSource      string
}

// Evidence is one scored answer bound to the objective (question.answered with is_scored and not rapid_guess).
type Evidence struct {
	EventID      uuid.UUID
	OccurredAt   time.Time
	Context      string  // AttemptMode of the attempt (practice, assignment, quest, …) or timed_challenge
	Difficulty   int     // 1..5; 0 = unknown (weighted as 3)
	Outcome      float64 // o = score / max_score ∈ [0, 1]
	QuestionType string
	OptionCount  int  // k: options shown for single_choice / image_based choice (0 = unknown)
	HintsUsed    int  // hint levels opened before the answer
	Repeat       bool // the same question was answered by the student in the previous 24 hours (w_rep)
}

// Params are the parameters of the Weighted Evidence model (LEARNING_ANALYTICS §3.1, CONFIG_DEFAULTS mastery_model).
type Params struct {
	M0               float64            `json:"m0"`
	AlphaBase        float64            `json:"alpha_base"`
	AlphaMax         float64            `json:"alpha_max"`
	WCtx             map[string]float64 `json:"w_ctx"`
	WCtxDefault      float64            `json:"w_ctx_default"`
	WRep             float64            `json:"w_rep"`
	WRepWindowHours  int                `json:"w_rep_window_hours"`
	WHint            []float64          `json:"w_hint"`
	GuessCoefficient map[string]float64 `json:"guess_coefficient"` // < 0 = 1/k (k options shown)
	DefaultOptions   int                `json:"default_option_count"`
	CorrectThreshold float64            `json:"correct_threshold"`
	GraceDays        float64            `json:"grace_days"`
	HalfLifeDays     float64            `json:"half_life_days"`
	NMin             int                `json:"n_min"`
	DMin             int                `json:"d_min"`
	Thresholds       Thresholds         `json:"level_thresholds"`
	DayLocation      string             `json:"day_location"` // IANA zone of the calendar days of d_min
}

// Thresholds of M_eff for the levels.
type Thresholds struct {
	Developing float64 `json:"developing"`
	Proficient float64 `json:"proficient"`
	Mastered   float64 `json:"mastered"`
}

// DefaultParams are the values of LEARNING_ANALYTICS §3.1 (model version we-1.0; the source of truth at run time is the
// active row of learning.mastery_model_configs).
func DefaultParams() Params {
	return Params{M0: 0.30, AlphaBase: 0.35, AlphaMax: 0.60,
		WCtx: map[string]float64{"exam": 1.0, "assignment": 0.9, "practice": 0.7, "retry_wrong": 0.7, "quest": 0.6, "challenge": 0.6,
			"minigame": 0.6, "timed_challenge": 0.4, "competition": 0.4, "diagnostic": 0.7},
		WCtxDefault: 0.7, WRep: 0.5, WRepWindowHours: 24, WHint: []float64{1.0, 0.6, 0.3},
		GuessCoefficient: map[string]float64{"true_false": 0.5, "single_choice": -1, "image_based": -1, "multiple_choice": 0.1,
			"matching": 0.1, "ordering": 0.1, "drag_drop": 0.1},
		DefaultOptions: 4, CorrectThreshold: 0.5, GraceDays: 14, HalfLifeDays: 90, NMin: 5, DMin: 2,
		Thresholds: Thresholds{Developing: 0.40, Proficient: 0.60, Mastered: 0.80}, DayLocation: "Asia/Ho_Chi_Minh"}
}

// ErrInvalidParams is returned by Validate.
var ErrInvalidParams = errors.New("mastery: invalid model parameters")

// Validate checks the ranges of the parameters.
func (p Params) Validate() error {
	in01 := func(v float64) bool { return v >= 0 && v <= 1 }
	switch {
	case !in01(p.M0), !in01(p.AlphaBase), !in01(p.AlphaMax), p.AlphaMax < p.AlphaBase, !in01(p.WCtxDefault), !in01(p.WRep),
		len(p.WHint) == 0, !in01(p.CorrectThreshold), p.GraceDays < 0, p.HalfLifeDays <= 0, p.NMin < 1, p.DMin < 1,
		!(p.Thresholds.Developing < p.Thresholds.Proficient && p.Thresholds.Proficient < p.Thresholds.Mastered && p.Thresholds.Mastered <= 1):
		return ErrInvalidParams
	}
	for _, v := range p.WCtx {
		if !in01(v) {
			return ErrInvalidParams
		}
	}
	if _, err := p.location(); err != nil {
		return ErrInvalidParams
	}
	return nil
}

func (p Params) location() (*time.Location, error) {
	if p.DayLocation == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(p.DayLocation)
}

// Model is the MasteryModel of LEARNING_ANALYTICS §3.4.
type Model interface {
	Version() string
	Apply(s State, ev Evidence) State
	Effective(s State, now time.Time) (float64, Level)
}

// WeightedEvidence is the EWMA with context, difficulty, repeat, hint and guess weights and a cold-start rate (ADR-0017).
type WeightedEvidence struct {
	ModelVersion string
	P            Params
}

var _ Model = WeightedEvidence{}

// Version implements Model.
func (m WeightedEvidence) Version() string { return m.ModelVersion }

// Weight is w = w_ctx × w_diff × w_rep × w_hint × w_guess (w_hint and w_guess are 1 when o < threshold).
func (m WeightedEvidence) Weight(ev Evidence) float64 {
	p := m.P
	wctx, ok := p.WCtx[ev.Context]
	if !ok {
		wctx = p.WCtxDefault
	}
	d := ev.Difficulty
	if d < 1 || d > 5 {
		d = 3
	}
	correct := ev.Outcome >= p.CorrectThreshold
	var wdiff float64
	if correct {
		wdiff = 0.8 + 0.1*float64(d-1)
	} else {
		wdiff = 1.2 - 0.1*float64(d-1)
	}
	w := wctx * wdiff
	if ev.Repeat {
		w *= p.WRep
	}
	if correct {
		h := ev.HintsUsed
		if h < 0 {
			h = 0
		}
		if h >= len(p.WHint) {
			h = len(p.WHint) - 1
		}
		w *= p.WHint[h]
		w *= 1 - m.guess(ev)
	}
	return w
}

// guess is c(type): a negative coefficient means 1/k with k the options shown.
func (m WeightedEvidence) guess(ev Evidence) float64 {
	c, ok := m.P.GuessCoefficient[ev.QuestionType]
	if !ok {
		return 0
	}
	if c >= 0 {
		return c
	}
	k := ev.OptionCount
	if k < 2 {
		k = m.P.DefaultOptions
	}
	if k < 2 {
		return 0
	}
	return 1 / float64(k)
}

// Alpha is the update rate max(min(α_max, α_base·w), 1/(n+3)) for the (n+1)-th evidence.
func (m WeightedEvidence) Alpha(n int, ev Evidence) float64 {
	aw := math.Min(m.P.AlphaMax, m.P.AlphaBase*m.Weight(ev))
	return math.Max(aw, 1/float64(n+3))
}

// Apply implements Model: M_new = M + α (o − M); evidence count, distinct days and last evidence follow. A state with
// no evidence starts from its value (the prior: M0, or a diagnostic prior written by F032).
func (m WeightedEvidence) Apply(s State, ev Evidence) State {
	o := math.Max(0, math.Min(1, ev.Outcome))
	if s.EvidenceCount == 0 && s.PriorSource != PriorDiagnostic {
		s.Value = m.P.M0
	}
	s.Value = clamp01(s.Value + m.Alpha(s.EvidenceCount, ev)*(o-s.Value))
	s.EvidenceCount++
	day := m.Day(ev.OccurredAt)
	if day != s.LastEvidenceDate {
		s.DistinctDays++
		s.LastEvidenceDate = day
	}
	if s.LastEvidenceAt == nil || ev.OccurredAt.After(*s.LastEvidenceAt) {
		t := ev.OccurredAt.UTC()
		s.LastEvidenceAt = &t
	}
	if s.PriorSource == "" {
		s.PriorSource = PriorDefault
	}
	return s
}

// Day is the calendar day of t in the day location (distinct days of d_min).
func (m WeightedEvidence) Day(t time.Time) string {
	loc, err := m.P.location()
	if err != nil {
		loc = time.UTC
	}
	return t.In(loc).Format(time.DateOnly)
}

// Effective implements Model: the decayed value M × (0.5 + 0.5 × 0.5^(max(0, Δ − g)/h)) and its level; the stored
// value is never changed.
func (m WeightedEvidence) Effective(s State, now time.Time) (float64, Level) {
	if s.EvidenceCount == 0 || s.LastEvidenceAt == nil {
		v := s.Value
		if s.EvidenceCount == 0 && s.PriorSource != PriorDiagnostic {
			v = m.P.M0
		}
		return v, NotStarted
	}
	days := now.Sub(*s.LastEvidenceAt).Hours() / 24
	f := 0.5 + 0.5*math.Pow(0.5, math.Max(0, days-m.P.GraceDays)/m.P.HalfLifeDays)
	v := s.Value * f
	return v, m.level(s, v)
}

func (m WeightedEvidence) level(s State, v float64) Level {
	t := m.P.Thresholds
	switch {
	case v < t.Developing:
		return Beginning
	case v < t.Proficient:
		return Developing
	case v < t.Mastered:
		return Proficient
	case s.EvidenceCount >= m.P.NMin && s.DistinctDays >= m.P.DMin:
		return Mastered
	default:
		return Proficient
	}
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// Change is the result of applying evidence: the state before and after and whether the objective was mastered for the
// first time (learning_objective.mastered is emitted once per (student, objective), LEARNING_ANALYTICS §3.4).
type Change struct {
	Before, After           State
	LevelBefore, LevelAfter Level
	FirstMastered           bool
}

// Step applies ev to s and reports the change; levels are taken at the time of the evidence (no decay).
func Step(m Model, s State, ev Evidence) Change {
	_, before := m.Effective(s, ev.OccurredAt)
	after := m.Apply(s, ev)
	_, lvl := m.Effective(after, ev.OccurredAt)
	c := Change{Before: s, After: after, LevelBefore: before, LevelAfter: lvl}
	if lvl == Mastered && after.FirstMasteredAt == nil {
		t := ev.OccurredAt.UTC()
		c.After.FirstMasteredAt = &t
		c.FirstMastered = true
	}
	return c
}

// Replay rebuilds a state from its prior and the evidence in occurred_at order (recompute after re-evaluation, late
// evidence or a new model version). FirstMasteredAt of the prior is kept: a replay never re-emits mastered for an
// objective that was mastered before.
func Replay(m Model, prior State, evidence []Evidence) State {
	s := State{PriorSource: prior.PriorSource, FirstMasteredAt: prior.FirstMasteredAt}
	if prior.PriorSource == PriorDiagnostic {
		s.Value = prior.Value
	}
	ordered := slices.Clone(evidence)
	slices.SortStableFunc(ordered, func(a, b Evidence) int {
		if c := a.OccurredAt.Compare(b.OccurredAt); c != 0 {
			return c
		}
		return strings.Compare(a.EventID.String(), b.EventID.String())
	})
	for _, ev := range ordered {
		s = Step(m, s, ev).After
	}
	return s
}
