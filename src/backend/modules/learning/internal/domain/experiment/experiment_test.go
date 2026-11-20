package experiment

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func def() Definition {
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return Definition{Code: "mastery-we-1-1", Kind: KindMastery, Unit: UnitClass, TenantIDs: []uuid.UUID{uuid.New()}, StartAt: now,
		EndAt: now.Add(30 * 24 * time.Hour), ActiveVer: "we-1.0", KnownVers: map[string]bool{"we-1.0": true, "we-1.1": true},
		Arms: []Arm{{Code: "control", AllocationPct: 50, IsControl: true, ModelVersion: "we-1.0"}, {Code: "we11", AllocationPct: 50, ModelVersion: "we-1.1"}}}
}

func fieldsOf(vs []Violation) string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Field)
	}
	return strings.Join(out, ",")
}

// F035-AC-21: arms, allocation, control model, rule parameters, consent, comparison classes, window.
func TestValidate(t *testing.T) {
	if vs := def().Validate(); len(vs) != 0 {
		t.Fatalf("valid: %v", vs)
	}
	cases := map[string]func(*Definition){
		"/arms":                  func(d *Definition) { d.Arms = d.Arms[:1] },
		"/arms/1/model_version":  func(d *Definition) { d.Arms[1].ModelVersion = "we-9" },
		"/arms/0/model_version":  func(d *Definition) { d.Arms[0].ModelVersion = "we-1.1" },
		"/arms/1/allocation_pct": func(d *Definition) { d.Arms[1].AllocationPct = 0 },
		"/code":                  func(d *Definition) { d.Code = "Bad Code" },
		"/tenant_ids":            func(d *Definition) { d.NoConsent = []uuid.UUID{d.TenantIDs[0]} },
		"/study_id":              func(d *Definition) { d.Comparison = true },
		"/end_at":                func(d *Definition) { d.EndAt = d.StartAt },
		"/arms/1/rule_params/xp_x2": func(d *Definition) {
			d.Kind = KindRecommendation
			d.Arms[1].RuleParams = json.RawMessage(`{"xp_x2":true}`)
		},
		"/arms/1/rule_params/max_recommendations": func(d *Definition) {
			d.Kind = KindRecommendation
			d.Arms[1].RuleParams = json.RawMessage(`{"max_recommendations":5}`)
		},
		"/arms/1/rule_params/practice_difficulty": func(d *Definition) {
			d.Kind = KindRecommendation
			d.Arms[1].RuleParams = json.RawMessage(`{"practice_difficulty":2}`)
		},
		"/arms/0/rule_params": func(d *Definition) {
			d.Kind = KindRecommendation
			d.Arms[0].RuleParams = json.RawMessage(`{"review_threshold":0.5}`)
		},
	}
	for want, mutate := range cases {
		d := def()
		d.Arms = append([]Arm(nil), d.Arms...)
		mutate(&d)
		if got := fieldsOf(d.Validate()); !strings.Contains(got, want) {
			t.Errorf("%s: %s", want, got)
		}
	}
	two := def()
	two.Arms = []Arm{{Code: "a", AllocationPct: 50, IsControl: true, ModelVersion: "we-1.0"}, {Code: "b", AllocationPct: 40, IsControl: true, ModelVersion: "we-1.0"}}
	if got := fieldsOf(two.Validate()); strings.Count(got, "/arms") < 2 {
		t.Fatalf("two controls and 90 %%: %s", got)
	}
	rec := def()
	rec.Kind = KindRecommendation
	rec.Arms = []Arm{{Code: "control", AllocationPct: 50, IsControl: true}, {Code: "more", AllocationPct: 50, RuleParams: json.RawMessage(`{"max_recommendations":2}`)}}
	if vs := rec.Validate(); len(vs) != 0 {
		t.Fatalf("recommendation: %v", vs)
	}
}

// F035-AC-22, SC-004: stable buckets; every student of a class gets the arm of the class; allocation respected.
func TestAssign(t *testing.T) {
	exp, class := uuid.New(), uuid.New()
	b := Bucket(exp, class)
	if b < 0 || b > 99 || Bucket(exp, class) != b {
		t.Fatal("bucket not stable")
	}
	arms := def().Arms
	if ArmOf(arms, 0).Code != "control" || ArmOf(arms, 49).Code != "control" || ArmOf(arms, 50).Code != "we11" || ArmOf(arms, 99).Code != "we11" {
		t.Fatal("cumulated allocation")
	}
	counts := map[string]int{}
	for i := 0; i < 2000; i++ {
		counts[ArmOf(arms, Bucket(exp, uuid.New())).Code]++
	}
	if counts["control"] < 850 || counts["we11"] < 850 {
		t.Fatalf("allocation %v", counts)
	}
	same := 0
	for range 20 {
		if Bucket(uuid.New(), class) == b {
			same++
		}
	}
	if same == 20 {
		t.Fatal("the experiment id is not mixed in")
	}
	if Control(arms).Code != "control" || ArmLabel("mastery-we-1-1", "we11") != "mastery-we-1-1:we11" {
		t.Fatal("control / label")
	}
}

// F035-AC-23, AC-24, AC-26: transitions and guardrail.
func TestTransitionsAndGuardrail(t *testing.T) {
	if s, err := Start(StatusDraft, StatusShadow); err != nil || s != StatusShadow {
		t.Fatal("draft → shadow")
	}
	if s, err := Start(StatusShadow, StatusLive); err != nil || s != StatusLive {
		t.Fatal("shadow → live")
	}
	if _, err := Start(StatusLive, StatusShadow); !errors.Is(err, ErrState) {
		t.Fatal("live → shadow")
	}
	if _, err := Start(StatusStopped, StatusLive); !errors.Is(err, ErrState) {
		t.Fatal("stopped → live")
	}
	if !Breach(0.80, 0.75, 5) || Breach(0.80, 0.76, 5) || Breach(0.70, 0.90, 5) {
		t.Fatal("guardrail at 5 points")
	}
	if !Running(StatusShadow) || !Running(StatusLive) || Running(StatusDraft) {
		t.Fatal("running")
	}
}
