package blueprint

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

func def(t *testing.T, raw string) Definition {
	t.Helper()
	d, err := Parse(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// F024-AC-10 (TC-EXM-001): total points, feedback timing, fixed questions.
func TestValidate(t *testing.T) {
	lo := uuid.NewString()
	ok := `{"total_points":10,"sections":[{"id":"sec_a","rules":[{"objective_ids":["` + lo + `"],"count":8,"points_each":0.5},` +
		`{"objective_ids":["` + lo + `"],"count":3,"points_each":2}]}]}`
	if vs := def(t, ok).Validate("after_close"); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	bad := strings.Replace(ok, `"total_points":10`, `"total_points":9.5`, 1)
	if vs := def(t, bad).Validate("after_close"); len(vs) != 1 || vs[0].Field != "/definition/total_points" {
		t.Fatalf("total: %v", vs)
	}
	if vs := def(t, ok).Validate("immediate"); len(vs) != 1 || vs[0].Field != "/feedback_timing" {
		t.Fatalf("immediate: %v", vs)
	}
	q := uuid.NewString()
	fixed := `{"total_points":3,"sections":[{"id":"sec_a","rules":[{"fixed_question_version_ids":["` + q + `"],"count":2,"points_each":1},` +
		`{"fixed_question_version_ids":["` + q + `"],"count":1,"points_each":1}]}]}`
	if vs := def(t, fixed).Validate("after_close"); len(vs) != 2 {
		t.Fatalf("fixed: %v", vs)
	}
}

// F024-AC-12: the configured max_mean_p_diff fills a missing balance.
func TestWithDefaults(t *testing.T) {
	out, err := WithDefaults(json.RawMessage(`{"variants":{"strategy":"independent_draw","count":4}}`), DefaultMaxMeanPDiff)
	if err != nil || !strings.Contains(string(out), `"max_mean_p_diff":0.05`) {
		t.Fatalf("%s %v", out, err)
	}
	out, _ = WithDefaults(json.RawMessage(`{"variants":{"strategy":"fixed","balance":{"max_mean_p_diff":0.1}}}`), DefaultMaxMeanPDiff)
	if !strings.Contains(string(out), `"max_mean_p_diff":0.1`) {
		t.Fatalf("kept %s", out)
	}
}

// F024-AC-11: independent_draw needs count × variants.count.
func TestStats(t *testing.T) {
	lo := uuid.New()
	pool := make([]selection.Candidate, 10)
	for i := range pool {
		pool[i] = selection.Candidate{VersionID: uuid.New(), QuestionID: uuid.New(), ObjectiveIDs: []uuid.UUID{lo}, Difficulty: 2}
	}
	d := def(t, `{"total_points":3,"variants":{"strategy":"independent_draw","count":4},"sections":[{"id":"sec_a","rules":[{"objective_ids":["`+
		lo.String()+`"],"count":3,"points_each":1}]}]}`)
	st := d.Stats(pool)
	if len(st) != 1 || st[0].Required != 12 || st[0].Available != 10 || !st[0].Insufficient {
		t.Fatalf("%+v", st)
	}
	d.Variants.Strategy = StrategyFixed
	if st := d.Stats(pool); st[0].Required != 3 || st[0].Insufficient {
		t.Fatalf("fixed %+v", st)
	}
}
