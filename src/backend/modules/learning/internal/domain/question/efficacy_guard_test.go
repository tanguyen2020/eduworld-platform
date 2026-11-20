package question

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// F035-AC-01: the flag on a question needs platform, exam_only and no params.
func TestEfficacyQuestion(t *testing.T) {
	plain := json.RawMessage(`{"options":[]}`)
	param := json.RawMessage(`{"params":{"a":{"min":1,"max":9}}}`)
	cases := []struct {
		name               string
		platform, examOnly bool
		render             json.RawMessage
		ok                 bool
	}{
		{"platform exam_only", true, true, plain, true},
		{"tenant", false, true, plain, false},
		{"not exam_only", true, false, plain, false},
		{"parameterized", true, true, param, false},
	}
	for _, c := range cases {
		vs := EfficacyQuestion(c.platform, c.examOnly, c.render)
		if (len(vs) == 0) != c.ok {
			t.Errorf("%s: violations %v", c.name, vs)
		}
		if !c.ok && vs[0].Field != "/efficacy_instrument" {
			t.Errorf("%s: field %s", c.name, vs[0].Field)
		}
	}
}

// F035-AC-01: the flag on a blueprint needs platform, fixed variants and fixed lists in every rule.
func TestEfficacyBlueprint(t *testing.T) {
	fixed := []EfficacyRule{{Pointer: "/definition/sections/0/rules/0", Fixed: []uuid.UUID{uuid.New()}}}
	if vs := EfficacyBlueprint(true, "fixed", fixed); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	if vs := EfficacyBlueprint(false, "fixed", fixed); len(vs) != 1 {
		t.Fatalf("tenant: %v", vs)
	}
	if vs := EfficacyBlueprint(true, "independent_draw", fixed); len(vs) != 1 {
		t.Fatalf("not fixed: %v", vs)
	}
	random := append(fixed, EfficacyRule{Pointer: "/definition/sections/0/rules/1"})
	vs := EfficacyBlueprint(true, "fixed", random)
	if len(vs) != 1 || vs[0].Field != "/definition/sections/0/rules/1/fixed_question_version_ids" {
		t.Fatalf("random rule: %v", vs)
	}
}

// F035-AC-01, AC-03: efficacy questions are referenced by efficacy blueprints only, which reference nothing else.
func TestEfficacyReference(t *testing.T) {
	if EfficacyReference(true, true) != "" || EfficacyReference(false, false) != "" {
		t.Fatal("allowed references refused")
	}
	if EfficacyReference(false, true) == "" || EfficacyReference(true, false) == "" {
		t.Fatal("mixed references allowed")
	}
	if !IsParameterized(json.RawMessage(`{"params":{"a":{}}}`)) || IsParameterized(json.RawMessage(`{"params":null}`)) {
		t.Fatal("IsParameterized")
	}
}
