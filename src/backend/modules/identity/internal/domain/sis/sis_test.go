package sis

import "testing"

func TestHashIgnoresUpdatedAtAndKeyOrder(t *testing.T) {
	a := map[string]any{"student_code": "001", "grade_level": 3, "login": map[string]any{"username": "hs.a"}, "updated_at": "2026-01-01T00:00:00Z"}
	b := map[string]any{"login": map[string]any{"username": "hs.a"}, "grade_level": 3, "student_code": "001", "updated_at": "2026-11-20T00:00:00Z"}
	if Hash(a) != Hash(b) {
		t.Fatal("same data with another snapshot time must hash the same (F011-AC-05)")
	}
	c := map[string]any{"student_code": "001", "grade_level": 4}
	if Hash(a) == Hash(c) {
		t.Fatal("different data, same hash")
	}
}

func TestActionFor(t *testing.T) {
	if ActionFor(false, "", "h") != ActionCreate || ActionFor(true, "h", "h") != ActionNoop || ActionFor(true, "x", "h") != ActionUpdate {
		t.Fatal("ActionFor")
	}
}

// F011-AC-23.
func TestStatusEffect(t *testing.T) {
	cases := map[string]StatusChange{
		"active":          {UserStatus: "active", StudentStatus: "active"},
		"inactive":        {UserStatus: "deactivated", StudentStatus: "inactive"},
		"graduated":       {UserStatus: "deactivated", StudentStatus: "inactive"},
		"suspended":       {UserStatus: "locked"},
		"transferred_out": {StudentStatus: "inactive"},
	}
	for s, want := range cases {
		got, err := StatusEffect(s)
		if err != nil || got != want {
			t.Errorf("%s = %+v %v", s, got, err)
		}
	}
	if _, err := StatusEffect("expelled"); err == nil {
		t.Fatal("unknown status accepted")
	}
}

// F011-AC-24.
func TestDeactivationReview(t *testing.T) {
	if above, review := DeactivationReviewNeeded(3, 100, false, 0.05); above || review {
		t.Fatal("3% is below the threshold")
	}
	if above, review := DeactivationReviewNeeded(10, 100, false, 0.05); !above || review {
		t.Fatal("first run above: mark only")
	}
	if above, review := DeactivationReviewNeeded(10, 100, true, 0.05); !above || !review {
		t.Fatal("second consecutive run above: review")
	}
}

func TestFieldHelpers(t *testing.T) {
	d := map[string]any{"name": " 3A ", "grade_level": 3, "start_date": "2026-09-05", "login": map[string]any{"username": " hs.x "}}
	if Str(d, "name") != "3A" || Int(d, "grade_level") != 3 || Date(d, "start_date") == nil || Date(d, "end_date") != nil || LoginUsername(d) != "hs.x" {
		t.Fatal("helpers")
	}
}

func TestNationalIDHash(t *testing.T) {
	k := []byte("k")
	if NationalIDHash(k, " 0123 456 ") != NationalIDHash(k, "0123456") || len(NationalIDHash(k, "1")) != 64 {
		t.Fatal("normalization")
	}
	if NationalIDHash(nil, "0123456") != "" || NationalIDHash(k, " ") != "" || NationalIDHash([]byte("x"), "1") == NationalIDHash(k, "1") {
		t.Fatal("key")
	}
}

// ADR-0095 D6: class and enrollment records are no longer applied.
func TestIgnoredKinds(t *testing.T) {
	if !IgnoredKind(KindClass) || !IgnoredKind(KindEnrollment) || IgnoredKind(KindStudent) || IgnoredKind(KindSchool) {
		t.Fatal("ignored kinds")
	}
	for _, k := range ApplyOrder {
		if IgnoredKind(k) {
			t.Fatalf("%s is applied", k)
		}
	}
}
