package params

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// F023-AC-14, AC-20: names, types, ranges, steps, constraints, limits.
func TestParse(t *testing.T) {
	ok := `{"a":{"type":"int","min":6,"max":9},"b":{"type":"int","min":2,"max":9},"x":{"type":"decimal","min":0.5,"max":2,"step":0.5},` +
		`"k":{"type":"choice","values":[10,100]},"constraints":["a != b"]}`
	if _, vs := Parse(json.RawMessage(ok)); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	bad := map[string]string{
		`{"A":{"type":"int","min":1,"max":2}}`:                              "/A",
		`{"a":{"type":"int","min":5,"max":2}}`:                              "/a/min",
		`{"a":{"type":"decimal","min":0,"max":1}}`:                          "/a/step",
		`{"a":{"type":"int","min":0,"max":1,"step":0}}`:                     "/a/step",
		`{"a":{"type":"text"}}`:                                             "/a/type",
		`{"a":{"type":"choice","values":["x"]}}`:                            "/a/values",
		`{"a":{"type":"int","min":0,"max":1},"constraints":["a == zz"]}`:    "/constraints/0",
		`{"a":{"type":"int","min":0,"max":1},"constraints":["abs(a) > 0"]}`: "/constraints/0",
	}
	for raw, want := range bad {
		_, vs := Parse(json.RawMessage(raw))
		found := false
		for _, v := range vs {
			if strings.HasSuffix(v.Field, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: %v, want %s", raw, vs, want)
		}
	}
	var many []string
	for i := range 11 {
		many = append(many, `"v`+string(rune('a'+i))+`":{"type":"int","min":0,"max":1}`)
	}
	if _, vs := Parse(json.RawMessage("{" + strings.Join(many, ",") + "}")); len(vs) == 0 {
		t.Fatal("11 variables accepted")
	}
}

// F023-AC-16, SC-003: 1,000 seeds all satisfy the constraints and the same seed gives the same values.
func TestSampleProperty(t *testing.T) {
	s, vs := Parse(json.RawMessage(`{"a":{"type":"int","min":6,"max":9},"b":{"type":"int","min":2,"max":9},"x":{"type":"decimal","min":0.5,"max":2,"step":0.5},` +
		`"constraints":["a != b","(a * b) / (a - b) != 0"]}`))
	if len(vs) != 0 {
		t.Fatal(vs)
	}
	ver := uuid.New()
	seen := map[string]bool{}
	for seed := range uint64(1000) {
		v, err := s.Sample(seed, ver)
		if err != nil {
			t.Fatal(err)
		}
		if v["a"].Cmp(v["b"]) == 0 {
			t.Fatalf("seed %d violates a != b", seed)
		}
		if x := v["x"]; x.Cmp(big.NewRat(1, 2)) < 0 || x.Cmp(big.NewRat(2, 1)) > 0 {
			t.Fatalf("x out of range %s", x)
		}
		again, _ := s.Sample(seed, ver)
		if again["a"].Cmp(v["a"]) != 0 || again["x"].Cmp(v["x"]) != 0 {
			t.Fatalf("seed %d not reproducible", seed)
		}
		seen[v["a"].String()+v["b"].String()] = true
	}
	if len(seen) < 10 {
		t.Fatalf("only %d distinct value sets", len(seen))
	}
	infeasible, _ := Parse(json.RawMessage(`{"a":{"type":"int","min":1,"max":3},"constraints":["a > 5"]}`))
	if _, err := infeasible.Sample(1, ver); err != ErrInfeasible { //nolint:errorlint // sentinel
		t.Fatalf("infeasible: %v", err)
	}
}

// F023-AC-16: placeholders replaced, decimal comma for vi, point for en; {{blank:…}} untouched.
func TestExpand(t *testing.T) {
	v := Values{"a": big.NewRat(7, 1), "x": big.NewRat(3, 2)}
	doc := json.RawMessage(`{"text":"{{a}} × {{ x }} = ? {{blank:b_1}}"}`)
	if got := string(Expand(doc, v, nil, "vi")); got != `{"text":"7 × 1,5 = ? {{blank:b_1}}"}` {
		t.Fatalf("vi %s", got)
	}
	if got := string(Expand(doc, v, nil, "en")); got != `{"text":"7 × 1.5 = ? {{blank:b_1}}"}` {
		t.Fatalf("en %s", got)
	}
	if p := Placeholders(doc); len(p) != 2 || Expression("{{a}} * {{x}}") != "a * x" {
		t.Fatalf("placeholders %v", p)
	}
	back, ok := FromStrings(v.Strings())
	if !ok || back["x"].Cmp(v["x"]) != 0 {
		t.Fatal("round trip")
	}
}
