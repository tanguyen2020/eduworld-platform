package questionset

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

func fields(t *testing.T, raw string) []string {
	t.Helper()
	d, err := Parse(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, v := range d.Validate() {
		out = append(out, v.Field)
	}
	return out
}

// F024-AC-01, AC-03: kinds need their parts; a rule's distribution adds up to its count, difficulty is ordered, ≤ 20 rules.
func TestRules(t *testing.T) {
	q := uuid.NewString()
	if got := fields(t, `{"kind":"static","purpose":"exercise","items":[{"question_version_id":"`+q+`"}]}`); len(got) != 0 {
		t.Fatalf("valid static: %v", got)
	}
	if got := fields(t, `{"kind":"static","purpose":"exercise","items":[{"question_version_id":"`+q+`"},{"question_version_id":"`+q+`"}]}`); len(got) != 1 {
		t.Fatalf("duplicate: %v", got)
	}
	if got := fields(t, `{"kind":"rule_based","purpose":"exercise"}`); len(got) != 1 || got[0] != "/definition/selection_rules" {
		t.Fatalf("rule_based without rules: %v", got)
	}
	if got := fields(t, `{"kind":"mixed","purpose":"exercise","selection_rules":[{"filters":{},"count":1}]}`); len(got) != 1 {
		t.Fatalf("mixed without items: %v", got)
	}
	bad := `{"kind":"rule_based","purpose":"exercise","selection_rules":[{"filters":{"difficulty":{"min":4,"max":2}},"count":10,
		"distribution":[{"difficulty":2,"count":4},{"difficulty":3,"count":5}]}]}`
	got := fields(t, bad)
	if strings.Join(got, ",") != "/definition/selection_rules/0/distribution,/definition/selection_rules/0/filters/difficulty" {
		t.Fatalf("rule errors: %v", got)
	}
	rules := make([]string, 21)
	for i := range rules {
		rules[i] = `{"filters":{},"count":1}`
	}
	if got := fields(t, `{"kind":"rule_based","purpose":"exercise","selection_rules":[`+strings.Join(rules, ",")+`]}`); len(got) != 1 {
		t.Fatalf("21 rules: %v", got)
	}
}

// F024-AC-04: available, insufficient (< count) and warning (< 3 × count) per rule.
func TestStats(t *testing.T) {
	sub := uuid.New()
	pool := make([]selection.Candidate, 12)
	for i := range pool {
		pool[i] = selection.Candidate{VersionID: uuid.New(), QuestionID: uuid.New(), SubjectID: sub, Difficulty: 2, Type: "single_choice"}
	}
	d, _ := Parse(json.RawMessage(`{"kind":"rule_based","purpose":"exercise","selection_rules":[{"filters":{"subject_id":"` + sub.String() +
		`"},"count":10},{"filters":{"difficulty":{"min":4}},"count":1}]}`))
	st := d.Stats(pool)
	if st[0].Available != 12 || st[0].Insufficient || !st[0].Warning || st[1].Available != 0 || !st[1].Insufficient {
		t.Fatalf("stats %+v", st)
	}
}
