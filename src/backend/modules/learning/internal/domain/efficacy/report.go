package efficacy

import (
	"slices"
	"sort"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy/stats"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/examquality"
)

// Answer is one graded item of an efficacy test (latest graded attempt of a student in an active session of a wave).
type Answer struct {
	StudentID         uuid.UUID
	WaveKind          string
	Form              string
	QuestionVersionID uuid.UUID
	ObjectiveIDs      []uuid.UUID
	Correct           bool
	Score, Max        float64
}

// Participant is a participant as the report sees it.
type Participant struct {
	StudentID uuid.UUID
	ClassID   uuid.UUID
	Arm       string
	Status    string
}

// Snapshot is M_eff and the level of an intervention participant on an objective at the post wave.
type Snapshot struct {
	StudentID, ObjectiveID uuid.UUID
	MEff                   float64
	Level                  string
}

// Usage is the use of the product by a participant between the pre and the post waves.
type Usage struct {
	StudentID        uuid.UUID
	ActiveSeconds    float64
	Days             int
	Answers          int
	RapidGuesses     int
	PracticeAttempts int
}

// Recommendation is a recommendation shown to a participant in the intervention window.
type Recommendation struct {
	StudentID    uuid.UUID
	ObjectiveIDs []uuid.UUID
	Followed     bool
}

// ReportInput is everything the report reads (F035-AC-29…AC-31).
type ReportInput struct {
	Objectives      []uuid.UUID
	Classes         map[string]int // enrolled + withdrawn classes by arm
	Participants    []Participant
	Answers         []Answer
	Excluded        []uuid.UUID
	Snapshots       []Snapshot
	Usage           []Usage
	Recommendations []Recommendation
}

// ReportResult is the aggregated, small-cell-suppressed results and the descriptive_only label.
type ReportResult struct {
	Results         map[string]any
	DescriptiveOnly bool
}

// cell returns n, or nil when 0 < n < min (hidden small cell).
func cell(n, minCell int) any {
	if n > 0 && n < minCell {
		return nil
	}
	return n
}

func round(v float64) float64 { return stats.Round(v, 4) }

// proportions computes the proportion correct of each student on a wave kind, overall and by objective.
func proportions(answers []Answer, kind string, excluded []uuid.UUID) (map[uuid.UUID]float64, map[uuid.UUID]map[uuid.UUID]float64) {
	type acc struct{ right, all float64 }
	total := map[uuid.UUID]*acc{}
	byLO := map[uuid.UUID]map[uuid.UUID]*acc{}
	for _, a := range answers {
		if a.WaveKind != kind || slices.Contains(excluded, a.QuestionVersionID) {
			continue
		}
		if total[a.StudentID] == nil {
			total[a.StudentID], byLO[a.StudentID] = &acc{}, map[uuid.UUID]*acc{}
		}
		r := 0.0
		if a.Correct {
			r = 1
		}
		total[a.StudentID].right += r
		total[a.StudentID].all++
		for _, o := range a.ObjectiveIDs {
			if byLO[a.StudentID][o] == nil {
				byLO[a.StudentID][o] = &acc{}
			}
			byLO[a.StudentID][o].right += r
			byLO[a.StudentID][o].all++
		}
	}
	out, outLO := map[uuid.UUID]float64{}, map[uuid.UUID]map[uuid.UUID]float64{}
	for s, a := range total {
		out[s] = a.right / a.all
		outLO[s] = map[uuid.UUID]float64{}
		for o, x := range byLO[s] {
			outLO[s][o] = x.right / x.all
		}
	}
	return out, outLO
}

// BuildReport computes the efficacy report (F035-AC-29…AC-31, LEARNING_EFFICACY_PLAN §4–§5): per arm the participants,
// attrition (missing pre/post, withdrawn), gain and normalized gain; Hedges' g of the gain difference with its 95 % CI
// corrected by the design effect of the classes; gain by objective; KR-20 per form × wave; usage; the predictive
// validity of M_eff (Spearman ρ by objective, calibration, AUC of "≥ 2/3 of the objective", recommendations). Cells
// under MinCellSize are hidden (null), aggregates only, no name, class nor ranking; descriptive_only under the
// recommended number of classes per arm. The output depends on the input only.
func BuildReport(in ReportInput, cfg Config) ReportResult {
	minCell := cfg.MinCellSize
	pre, preLO := proportions(in.Answers, WavePre, in.Excluded)
	post, postLO := proportions(in.Answers, WavePost, in.Excluded)
	parts := slices.Clone(in.Participants)
	sort.Slice(parts, func(i, j int) bool { return parts[i].StudentID.String() < parts[j].StudentID.String() })
	type armData struct {
		total, withdrawn, missingPre, missingPost int
		gains, normGains, pres, posts             []float64
		byClass                                   map[uuid.UUID][]float64
		students                                  []uuid.UUID
	}
	arms := map[string]*armData{ArmIntervention: {byClass: map[uuid.UUID][]float64{}}, ArmComparison: {byClass: map[uuid.UUID][]float64{}}}
	for _, p := range parts {
		a, ok := arms[p.Arm]
		if !ok {
			continue
		}
		a.total++
		if p.Status != ParticipantActive {
			a.withdrawn++
			continue
		}
		x, okPre := pre[p.StudentID]
		y, okPost := post[p.StudentID]
		if !okPre {
			a.missingPre++
		}
		if !okPost {
			a.missingPost++
		}
		if !okPre || !okPost {
			continue
		}
		g := stats.Gain(x, y)
		a.gains = append(a.gains, g)
		a.pres, a.posts = append(a.pres, x), append(a.posts, y)
		if ng, ok := stats.NormalizedGain(x, y); ok {
			a.normGains = append(a.normGains, ng)
		}
		a.byClass[p.ClassID] = append(a.byClass[p.ClassID], g)
		a.students = append(a.students, p.StudentID)
	}
	armOut := map[string]any{}
	suppressed := map[string]bool{}
	attritionWarn := false
	for _, name := range []string{ArmIntervention, ArmComparison} {
		a := arms[name]
		n := len(a.gains)
		dropped := a.total - n
		rate := 0.0
		if a.total > 0 {
			rate = float64(dropped) / float64(a.total)
		}
		attritionWarn = attritionWarn || rate >= cfg.AttritionWarn
		suppressed[name] = n < minCell
		m := map[string]any{"classes": in.Classes[name], "participants": cell(a.total, minCell), "analysed": cell(n, minCell),
			"withdrawn": cell(a.withdrawn, minCell), "missing_pre": cell(a.missingPre, minCell), "missing_post": cell(a.missingPost, minCell),
			"attrition_rate": round(rate), "suppressed": suppressed[name]}
		if !suppressed[name] {
			m["pre_mean"], m["post_mean"] = round(stats.Mean(a.pres)), round(stats.Mean(a.posts))
			m["gain_mean"], m["gain_sd"] = round(stats.Mean(a.gains)), round(sqrt(stats.Variance(a.gains)))
			m["normalized_gain_mean"] = round(stats.Mean(a.normGains))
		} else {
			m["attrition_rate"] = nil // with a hidden cell the rate would give it away
		}
		armOut[name] = m
	}
	res := map[string]any{"arms": armOut, "attrition_warning": attritionWarn, "excluded_question_version_ids": sortedIDs(in.Excluded)}
	// Effect size with the design effect of the classes (both arms pooled).
	if !suppressed[ArmIntervention] && !suppressed[ArmComparison] {
		var clusters [][]float64
		for _, name := range []string{ArmIntervention, ArmComparison} {
			for _, id := range sortedKeys(arms[name].byClass) {
				clusters = append(clusters, arms[name].byClass[id])
			}
		}
		icc, mbar := stats.ICC(clusters)
		deff := stats.DesignEffect(icc, mbar)
		e := stats.HedgesG(arms[ArmIntervention].gains, arms[ArmComparison].gains, deff)
		if e.OK {
			res["effect"] = map[string]any{"hedges_g": round(e.G), "ci_low": round(e.Low), "ci_high": round(e.High), "se": round(e.SE),
				"se_adjusted": round(e.SEAdj), "icc": round(icc), "design_effect": round(deff), "n_intervention": e.NI, "n_comparison": e.NC}
		}
	}
	// Gain by objective.
	var los []map[string]any
	for _, o := range sortedIDs(in.Objectives) {
		row := map[string]any{"objective_id": o}
		for _, name := range []string{ArmIntervention, ArmComparison} {
			var gs []float64
			for _, s := range arms[name].students {
				x, ok1 := preLO[s][o]
				y, ok2 := postLO[s][o]
				if ok1 && ok2 {
					gs = append(gs, y-x)
				}
			}
			if len(gs) < minCell {
				row[name+"_gain"], row[name+"_n"] = nil, cell(len(gs), minCell)
				continue
			}
			row[name+"_gain"], row[name+"_n"] = round(stats.Mean(gs)), len(gs)
		}
		los = append(los, row)
	}
	res["objectives"] = orEmpty(los)
	res["reliability"] = reliability(in.Answers, in.Excluded, cfg)
	res["usage"] = usage(in, arms[ArmIntervention].students, arms[ArmComparison].students, minCell)
	res["validity"] = validity(in, arms[ArmIntervention].students, pre, preLO, postLO, cfg)
	descriptive := in.Classes[ArmIntervention] < cfg.RecommendedClassesPerArm || in.Classes[ArmComparison] < cfg.RecommendedClassesPerArm
	res["descriptive_only"] = descriptive
	res["thresholds"] = map[string]any{"min_cell_size": minCell, "attrition_warn": cfg.AttritionWarn,
		"recommended_classes_per_arm": cfg.RecommendedClassesPerArm, "validity_rho": cfg.ValidityRho,
		"validity_calibration": cfg.ValidityCalibration, "validity_auc": cfg.ValidityAUC, "reliability_warn": cfg.ReliabilityWarn}
	return ReportResult{Results: res, DescriptiveOnly: descriptive}
}

func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for range 30 {
		x = (x + v/x) / 2
	}
	return x
}

func orEmpty(xs []map[string]any) []map[string]any {
	if xs == nil {
		return []map[string]any{}
	}
	return xs
}

func sortedIDs(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	if out == nil {
		out = []uuid.UUID{}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func sortedKeys[V any](m map[uuid.UUID]V) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return sortedIDs(out)
}

// reliability is KR-20 (or alpha) of each form in each wave (FR-EXM-010; warning under exam_reliability_warn).
func reliability(answers []Answer, excluded []uuid.UUID, cfg Config) []map[string]any {
	type key struct{ wave, form string }
	sheets := map[key]map[uuid.UUID]*examquality.Sheet{}
	for _, a := range answers {
		if slices.Contains(excluded, a.QuestionVersionID) {
			continue
		}
		k := key{a.WaveKind, a.Form}
		if sheets[k] == nil {
			sheets[k] = map[uuid.UUID]*examquality.Sheet{}
		}
		s := sheets[k][a.StudentID]
		if s == nil {
			s = &examquality.Sheet{StudentID: a.StudentID, VariantNo: 1}
			sheets[k][a.StudentID] = s
		}
		s.Items = append(s.Items, examquality.ItemScore{QuestionVersionID: a.QuestionVersionID, Score: a.Score, Max: a.Max, Correct: a.Correct})
		s.Total += a.Score
		s.Max += a.Max
	}
	keys := make([]key, 0, len(sheets))
	for k := range sheets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].wave != keys[j].wave {
			return keys[i].wave < keys[j].wave
		}
		return keys[i].form < keys[j].form
	})
	qc := examquality.DefaultConfig()
	qc.WarnBelow = cfg.ReliabilityWarn
	var out []map[string]any
	for _, k := range keys {
		var list []examquality.Sheet
		for _, id := range sortedKeys(sheets[k]) {
			s := *sheets[k][id]
			sort.Slice(s.Items, func(i, j int) bool {
				return s.Items[i].QuestionVersionID.String() < s.Items[j].QuestionVersionID.String()
			})
			list = append(list, s)
		}
		rep := examquality.Analyze(list, nil, qc)
		row := map[string]any{"wave": k.wave, "form": k.form, "n": cell(len(list), cfg.MinCellSize), "kr20": nil, "warning": false}
		if len(rep.Variants) > 0 && rep.Variants[0].Reliability != nil && len(list) >= cfg.MinCellSize {
			r := *rep.Variants[0].Reliability
			row["kr20"], row["warning"] = round(r), r < cfg.ReliabilityWarn
		}
		out = append(out, row)
	}
	return orEmpty(out)
}

// usage is the use of the product per arm and the share of comparison participants who practised (LEARNING_EFFICACY_PLAN §4).
func usage(in ReportInput, intervention, comparison []uuid.UUID, minCell int) map[string]any {
	by := map[uuid.UUID]Usage{}
	for _, u := range in.Usage {
		by[u.StudentID] = u
	}
	arm := func(students []uuid.UUID) (map[string]any, float64) {
		if len(students) < minCell {
			return map[string]any{"suppressed": true}, 0
		}
		var minutes, days []float64
		var answers, rapid, practised float64
		for _, s := range students {
			u := by[s]
			minutes, days = append(minutes, u.ActiveSeconds/60), append(days, float64(u.Days))
			answers += float64(u.Answers)
			rapid += float64(u.RapidGuesses)
			if u.PracticeAttempts > 0 {
				practised++
			}
		}
		rate := 0.0
		if answers > 0 {
			rate = rapid / answers
		}
		return map[string]any{"suppressed": false, "active_minutes_mean": round(stats.Mean(minutes)), "active_days_mean": round(stats.Mean(days)),
			"rapid_guess_rate": round(rate)}, practised / float64(len(students))
	}
	iv, _ := arm(intervention)
	cv, practised := arm(comparison)
	out := map[string]any{ArmIntervention: iv, ArmComparison: cv, "comparison_practice_rate": nil}
	if len(comparison) >= minCell {
		out["comparison_practice_rate"] = round(practised)
	}
	return out
}

// validity is the predictive validity of M_eff for the intervention participants (F035-AC-30, LEARNING_EFFICACY_PLAN §5).
func validity(in ReportInput, intervention []uuid.UUID, pre map[uuid.UUID]float64, preLO, postLO map[uuid.UUID]map[uuid.UUID]float64,
	cfg Config) map[string]any {
	snap := map[[2]uuid.UUID]Snapshot{}
	for _, s := range in.Snapshots {
		snap[[2]uuid.UUID{s.StudentID, s.ObjectiveID}] = s
	}
	var allM, allPost []float64
	var allLevels []string
	var positive []bool
	var los []map[string]any
	for _, o := range sortedIDs(in.Objectives) {
		var xs, ys []float64
		for _, st := range intervention {
			s, ok := snap[[2]uuid.UUID{st, o}]
			y, ok2 := postLO[st][o]
			if !ok || !ok2 {
				continue
			}
			xs, ys = append(xs, s.MEff), append(ys, y)
			allM, allPost, allLevels = append(allM, s.MEff), append(allPost, y), append(allLevels, s.Level)
			positive = append(positive, y >= 2.0/3-1e-9)
		}
		row := map[string]any{"objective_id": o, "n": cell(len(xs), cfg.MinCellSize), "rho": nil, "below_threshold": false}
		if len(xs) >= cfg.MinCellSize {
			if rho, ok := stats.Spearman(xs, ys); ok {
				row["rho"], row["below_threshold"] = round(rho), rho < cfg.ValidityRho
			}
		}
		los = append(los, row)
	}
	out := map[string]any{"objectives": orEmpty(los), "calibration": nil, "calibration_below_threshold": false, "auc": nil,
		"auc_below_threshold": false}
	if len(allM) >= cfg.MinCellSize {
		if c, ok := stats.Calibration(allLevels, allPost); ok {
			out["calibration"], out["calibration_below_threshold"] = round(c), c < cfg.ValidityCalibration
		}
		if a, ok := stats.AUC(allM, positive); ok {
			out["auc"], out["auc_below_threshold"] = round(a), a < cfg.ValidityAUC
		}
	}
	// Recommendations: share followed, and gain of the recommended objectives against the others.
	recLO := map[uuid.UUID]map[uuid.UUID]bool{}
	var shown, followed float64
	inArm := map[uuid.UUID]bool{}
	for _, s := range intervention {
		inArm[s] = true
	}
	for _, r := range in.Recommendations {
		if !inArm[r.StudentID] {
			continue
		}
		shown++
		if r.Followed {
			followed++
		}
		if recLO[r.StudentID] == nil {
			recLO[r.StudentID] = map[uuid.UUID]bool{}
		}
		for _, o := range r.ObjectiveIDs {
			recLO[r.StudentID][o] = true
		}
	}
	var recGain, otherGain []float64
	for _, st := range intervention {
		for _, o := range in.Objectives {
			x, ok1 := preLO[st][o]
			y, ok2 := postLO[st][o]
			if !ok1 || !ok2 {
				continue
			}
			if recLO[st][o] {
				recGain = append(recGain, y-x)
			} else {
				otherGain = append(otherGain, y-x)
			}
		}
	}
	_ = pre
	rec := map[string]any{"shown": cell(int(shown), cfg.MinCellSize), "follow_rate": nil, "recommended_lo_gain": nil, "other_lo_gain": nil}
	if shown >= float64(cfg.MinCellSize) {
		rec["follow_rate"] = round(followed / shown)
	}
	if len(recGain) >= cfg.MinCellSize {
		rec["recommended_lo_gain"] = round(stats.Mean(recGain))
	}
	if len(otherGain) >= cfg.MinCellSize {
		rec["other_lo_gain"] = round(stats.Mean(otherGain))
	}
	out["recommendations"] = rec
	return out
}
