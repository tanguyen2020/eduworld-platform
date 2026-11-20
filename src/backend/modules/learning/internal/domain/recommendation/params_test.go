package recommendation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func pv[T any](v T) *T { return &v }

// F035-AC-24 (T051): the rule_params of a live recommendation_rule arm change the rules; zero params = the engine of
// F032 unchanged.
func TestArmParams(t *testing.T) {
	sub, t1, t2 := uuid.New(), uuid.New(), uuid.New()
	g := 3
	// review_threshold: a proficient objective under the arm threshold becomes weak (R2).
	p := lo(sub, t1, "proficient", 5)
	p.Value = 0.62
	s := Snapshot{Now: now, Grade: &g, Objectives: []Objective{p}}
	if got := (R2{}).Evaluate(s); len(got) != 0 {
		t.Fatalf("default R2: %+v", got)
	}
	s.Params.ReviewThreshold = pv(0.70)
	if got := (R2{}).Evaluate(s); len(got) != 1 || got[0].TargetID != p.ID {
		t.Fatalf("R2 with review_threshold: %+v", got)
	}
	// prerequisite_depth: a weak prerequisite two levels up is reviewed only with depth 2.
	root := lo(sub, t1, "beginning", 4)
	mid := lo(sub, t1, "proficient", 5, root.ID)
	w := lo(sub, t1, "beginning", 3, mid.ID)
	s = Snapshot{Now: now, Grade: &g, Objectives: []Objective{root, mid, w}}
	if got := (R3{}).Evaluate(s); len(got) != 0 {
		t.Fatalf("default R3 (depth 1): %+v", got)
	}
	s.Params.PrerequisiteDepth = pv(2)
	if got := (R3{}).Evaluate(s); len(got) != 1 || got[0].TargetID != root.ID {
		t.Fatalf("R3 depth 2: %+v", got)
	}
	// spaced_review_days: a decay review waits until the last evidence is old enough.
	d := lo(sub, t1, "developing", 6)
	d.StoredLevel, d.StoredValue, d.Value = "mastered", 0.85, 0.64
	s = Snapshot{Now: now, Objectives: []Objective{d}}
	if got := (R4{}).Evaluate(s); len(got) != 1 {
		t.Fatalf("default R4: %+v", got)
	}
	s.Params.SpacedReviewDays = pv(3)
	if got := (R4{}).Evaluate(s); len(got) != 0 {
		t.Fatalf("R4 within spacing: %+v", got)
	}
	old := now.Add(-4 * 24 * time.Hour)
	s.Objectives[0].LastEvidenceAt = &old
	if got := (R4{}).Evaluate(s); len(got) != 1 {
		t.Fatalf("R4 after spacing: %+v", got)
	}
	// diagnostic_threshold: objectives with fewer evidence count as not started (R7 topic warm-up).
	learned := lo(sub, t1, "proficient", 6)
	earlier := now.Add(-48 * time.Hour)
	learned.LastEvidenceAt = &earlier
	topic := Snapshot{Now: now, Grade: &g, Objectives: []Objective{learned, lo(sub, t2, "developing", 1),
		lo(sub, t2, "developing", 1), lo(sub, t2, "developing", 2)}}
	if got := (R7{}).Evaluate(topic); len(got) != 0 {
		t.Fatalf("default R7: %+v", got)
	}
	topic.Params.DiagnosticThreshold = pv(2)
	if got := (R7{}).Evaluate(topic); len(got) != 1 || got[0].TargetID != t2 {
		t.Fatalf("R7 diagnostic_threshold: %+v", got)
	}
	// max_recommendations and practice_difficulty.
	if (Params{}).Limit(20) != 20 || (Params{MaxRecommendations: pv(2)}).Limit(20) != 2 {
		t.Fatal("limit")
	}
	if (Params{}).DifficultyShift() != 0 || (Params{PracticeDifficulty: pv(-1)}).DifficultyShift() != -1 {
		t.Fatal("difficulty shift")
	}
}

func TestParseParams(t *testing.T) {
	if p, err := ParseParams(nil); err != nil || p.ReviewThreshold != nil {
		t.Fatalf("empty: %+v %v", p, err)
	}
	if _, err := ParseParams(json.RawMessage(`{"xp_x2":true}`)); err == nil {
		t.Fatal("unknown key accepted")
	}
	p, err := ParseParams(json.RawMessage(`{"max_recommendations":9,"review_threshold":0.05,"practice_difficulty":1}`))
	if err != nil {
		t.Fatal(err)
	}
	v := p.Violations()
	if len(v) != 2 || v["max_recommendations"] == "" || v["review_threshold"] == "" {
		t.Fatalf("violations: %v", v)
	}
	merged := (Params{PracticeDifficulty: pv(-1)}).Merge(p)
	if *merged.PracticeDifficulty != -1 || *merged.MaxRecommendations != 9 {
		t.Fatalf("merge: %+v", merged)
	}
}
