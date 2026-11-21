package efficacy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// fixture builds n students of an arm in one cohort answering 3 items of the LO at the pre and post waves.
func fixture(in *ReportInput, arm string, n, preRight, postRight int, lo uuid.UUID) []uuid.UUID {
	cohort := uuid.New()
	var out []uuid.UUID
	for i := 0; i < n; i++ {
		s := uuid.New()
		out = append(out, s)
		in.Participants = append(in.Participants, Participant{StudentID: s, CohortID: cohort, Arm: arm, Status: ParticipantActive})
		for k := 0; k < 3; k++ {
			q := uuid.NewSHA1(uuid.Nil, []byte{byte(k)})
			in.Answers = append(in.Answers, Answer{StudentID: s, WaveKind: WavePre, Form: FormA, QuestionVersionID: q, ObjectiveIDs: []uuid.UUID{lo},
				Correct: k < preRight, Score: b2f(k < preRight), Max: 1})
			in.Answers = append(in.Answers, Answer{StudentID: s, WaveKind: WavePost, Form: FormB, QuestionVersionID: uuid.NewSHA1(uuid.Nil, []byte{byte(10 + k)}),
				ObjectiveIDs: []uuid.UUID{lo}, Correct: k < postRight+(i%2), Score: b2f(k < postRight+(i%2)), Max: 1})
		}
	}
	return out
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// F035-AC-29, AC-31: aggregates per arm, the effect with its interval, small cells hidden, no identity, descriptive_only.
func TestBuildReport(t *testing.T) {
	lo := uuid.New()
	in := ReportInput{Objectives: []uuid.UUID{lo}, Cohorts: map[string]int{ArmIntervention: 1, ArmComparison: 1}}
	iv := fixture(&in, ArmIntervention, 6, 1, 2, lo)
	cv := fixture(&in, ArmComparison, 6, 1, 1, lo)
	// A withdrawn participant counts as attrition and is analysed nowhere.
	gone := uuid.New()
	in.Participants = append(in.Participants, Participant{StudentID: gone, CohortID: uuid.New(), Arm: ArmComparison, Status: ParticipantWithdrawn})
	for _, s := range iv {
		in.Snapshots = append(in.Snapshots, Snapshot{StudentID: s, ObjectiveID: lo, MEff: 0.5, Level: "developing"})
	}
	cfg := DefaultConfig()
	res := BuildReport(in, cfg)
	if !res.DescriptiveOnly || res.Results["descriptive_only"] != true {
		t.Fatal("1 cohort per arm is descriptive only")
	}
	arms := res.Results["arms"].(map[string]any)
	ia, ca := arms[ArmIntervention].(map[string]any), arms[ArmComparison].(map[string]any)
	if ia["analysed"] != 6 || ca["analysed"] != 6 || ca["participants"] != 7 || ca["withdrawn"] != nil {
		t.Fatalf("counts %v / %v", ia, ca) // withdrawn = 1 < 5 is hidden
	}
	if ia["gain_mean"].(float64) <= ca["gain_mean"].(float64) {
		t.Fatalf("gains %v %v", ia["gain_mean"], ca["gain_mean"])
	}
	e, ok := res.Results["effect"].(map[string]any)
	if !ok || e["hedges_g"].(float64) <= 0 || e["ci_low"].(float64) >= e["ci_high"].(float64) {
		t.Fatalf("effect %v", res.Results["effect"])
	}
	raw, _ := json.Marshal(res.Results)
	for _, s := range append(append(iv, cv...), gone) {
		if strings.Contains(string(raw), s.String()) {
			t.Fatal("a student id is in the report")
		}
	}
	// ADR-0095: clusters (school, grade), the threshold efficacy_recommended_cohorts_per_arm = 4.
	if th := res.Results["thresholds"].(map[string]any); th["recommended_cohorts_per_arm"] != 4 || ia["cohorts"] != 1 {
		t.Fatalf("cohort threshold %v / %v", th, ia)
	}
	for _, f := range []string{"name", "class_name", "classes", "rank"} {
		if strings.Contains(string(raw), `"`+f+`"`) {
			t.Fatalf("field %s in the report", f)
		}
	}
	// Deterministic.
	again, _ := json.Marshal(BuildReport(in, cfg).Results)
	if string(again) != string(raw) {
		t.Fatal("same input, other output")
	}
	// A small arm (< analytics_min_cell_size) is hidden with the effect and the attrition rate.
	small := ReportInput{Objectives: []uuid.UUID{lo}, Cohorts: map[string]int{ArmIntervention: 4, ArmComparison: 4}}
	fixture(&small, ArmIntervention, 6, 1, 2, lo)
	fixture(&small, ArmComparison, 3, 1, 1, lo)
	sr := BuildReport(small, cfg)
	sa := sr.Results["arms"].(map[string]any)[ArmComparison].(map[string]any)
	if sa["suppressed"] != true || sa["gain_mean"] != nil || sa["analysed"] != nil || sa["participants"] != nil || sa["attrition_rate"] != nil {
		t.Fatalf("small arm %v", sa)
	}
	if _, ok := sr.Results["effect"]; ok || sr.DescriptiveOnly {
		t.Fatal("effect with a hidden arm, or descriptive with 4 cohorts per arm (efficacy_recommended_cohorts_per_arm)")
	}
}
