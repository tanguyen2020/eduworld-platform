package csvreader

import (
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
)

var now = time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)

func validator(t *testing.T) *contract.Validator {
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestReadStudents(t *testing.T) {
	csv := "\xEF\xBB\xBFstudent_external_id,school_external_id,student_code,full_name,date_of_birth,grade_level,status,login_username,teacher_name\n" +
		"HS-000123,TH-DEMO-001,2026000123,Nguyễn Văn An,2018-03-14,3,active,hs.thdemo.000123,Cô Lan\n"
	b, err := Read(contract.KindStudent, strings.NewReader(csv), validator(t), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Errors) != 0 || len(b.Records) != 1 {
		t.Fatalf("batch = %+v", b)
	}
	r := b.Records[0]
	if r.Row != 2 || r.Data["grade_level"] != 3 || r.Data["login"].(map[string]any)["username"] != "hs.thdemo.000123" {
		t.Fatalf("record = %+v", r)
	}
	// F011-AC-03: the teacher column is ignored and never stored.
	if _, ok := r.Data["teacher_name"]; ok || len(b.IgnoredColumns) != 1 || b.IgnoredColumns[0] != "teacher_name" {
		t.Fatalf("ignored = %v data = %v", b.IgnoredColumns, r.Data)
	}
	if r.Data["updated_at"] != now.Format(time.RFC3339) {
		t.Fatalf("updated_at = %v", r.Data["updated_at"])
	}
}

// F011-AC-02: violations per row and column; the caller rejects the file.
func TestReadViolations(t *testing.T) {
	csv := "student_external_id,school_external_id,student_code,full_name,grade_level,status\n" +
		"HS-1,TH-1,001,An,3,active\n" +
		"HS-2,TH-1,002,,ten,active\n" +
		"HS-3,TH-1,003,Chi,3,expelled\n" +
		"HS-4,TH-1\n"
	b, err := Read(contract.KindStudent, strings.NewReader(csv), validator(t), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Records) != 1 {
		t.Fatalf("valid records = %d", len(b.Records))
	}
	byRow := map[int][]string{}
	for _, e := range b.Errors {
		if e.Code != ErrorCode {
			t.Fatalf("code = %s", e.Code)
		}
		byRow[e.Row] = append(byRow[e.Row], e.Column)
	}
	has := func(row int, col string) bool {
		for _, c := range byRow[row] {
			if c == col {
				return true
			}
		}
		return false
	}
	if !has(3, "grade_level") || !has(3, "full_name") || !has(4, "status") || len(byRow[5]) != 1 {
		t.Fatalf("errors = %+v", b.Errors)
	}
}

func TestReadEmptyAndKinds(t *testing.T) {
	if _, err := Read(contract.KindClass, strings.NewReader(""), validator(t), now); err == nil {
		t.Fatal("empty file accepted")
	}
	b, err := Read(contract.KindEnrollment, strings.NewReader("student_external_id,class_external_id,start_date,end_date,status\nHS-1,C-1,2026-09-05,,active\n"),
		validator(t), now)
	if err != nil || len(b.Records) != 1 || b.Records[0].Data["end_date"] != nil {
		t.Fatalf("enrollment = %+v %v", b, err)
	}
	if FileKinds["students.csv"] != contract.KindStudent {
		t.Fatal("file kinds")
	}
}
