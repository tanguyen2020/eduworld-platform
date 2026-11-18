package contract

import (
	"strings"
	"testing"
)

func student() map[string]any {
	return map[string]any{"student_external_id": "HS-000123", "school_external_id": "TH-DEMO-001", "student_code": "2026000123",
		"full_name": "Nguyễn Văn An", "date_of_birth": "2018-03-14", "grade_level": 3, "status": "active",
		"login": map[string]any{"username": "hs.thdemo.000123"}, "updated_at": "2026-11-01T00:00:00Z"}
}

func TestValidator(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if v.Version() == "" {
		t.Fatal("contract version missing")
	}
	if got := v.Validate(KindStudent, student()); got != nil {
		t.Fatalf("valid student: %+v", got)
	}
	// F011-AC-02: missing required field, wrong type, wrong enum → violations with the field.
	bad := student()
	delete(bad, "full_name")
	bad["grade_level"] = 12
	bad["status"] = "expelled"
	got := v.Validate(KindStudent, bad)
	fields := map[string]bool{}
	for _, x := range got {
		fields[x.Field] = true
	}
	for _, f := range []string{"full_name", "grade_level", "status"} {
		if !fields[f] {
			t.Errorf("no violation for %s in %+v", f, got)
		}
	}
	// A column outside the contract is unknown (F011-AC-03 ignores it before validation).
	if v.Known(KindStudent, "teacher_name") || !v.Known(KindStudent, "student_code") {
		t.Fatal("Known() wrong")
	}
	extra := student()
	extra["teacher_name"] = "x"
	if got := v.Validate(KindStudent, extra); len(got) == 0 {
		t.Fatal("additionalProperties must be rejected by the contract (the reader drops unknown columns first)")
	}
	cls := map[string]any{"class_external_id": "C-2026-3A", "school_external_id": "TH-DEMO-001", "academic_year": "2026-27",
		"grade_level": 3, "name": "3A", "updated_at": "2026-11-01T00:00:00Z"}
	if got := v.Validate(KindClass, cls); len(got) != 1 || !strings.Contains(got[0].Field, "academic_year") {
		t.Fatalf("bad academic_year: %+v", got)
	}
}
