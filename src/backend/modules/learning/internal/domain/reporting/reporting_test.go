package reporting

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func day(s string) *time.Time {
	t, _ := time.Parse(time.DateOnly, s)
	return &t
}

// TestValidate checks F044-AC-07: report types per route, formats, required parameters, range ≤ dashboard_max_range_days.
func TestValidate(t *testing.T) {
	cfg := DefaultConfig()
	id := uuid.New()
	cases := []struct {
		name  string
		r     Request
		field string
		err   error
	}{
		{"lms ok", Request{Scope: ScopeLMS, Type: TypeAssignmentResults, Format: FormatXLSX, AssignmentID: &id}, "", nil},
		{"lms needs assignment", Request{Scope: ScopeLMS, Type: TypeAssignmentResults, Format: FormatCSV}, "assignment_id", nil},
		{"bad format", Request{Scope: ScopeLMS, Type: TypeCourseResults, Format: "docx", CourseID: &id}, "format", nil},
		{"pdf tenant", Request{Scope: ScopeTenant, Type: TypeLearningSummary, Format: FormatPDF, From: day("2026-09-01"), To: day("2026-10-01")}, "format", nil},
		{"wrong route", Request{Scope: ScopeTenant, Type: TypeSkillMap, Format: FormatCSV}, "report_type", nil},
		{"later feature", Request{Scope: ScopePlatform, Type: TypeGameSummary, Format: FormatCSV}, "report_type", nil},
		{"efficacy needs study", Request{Scope: ScopePlatform, Type: TypeEfficacyDataset, Format: FormatCSV, From: day("2026-09-01"), To: day("2026-10-01")}, "params/study_id", nil},
		{"efficacy csv only", Request{Scope: ScopePlatform, Type: TypeEfficacyDataset, Format: FormatXLSX, StudyID: &id, From: day("2026-09-01"), To: day("2026-10-01")}, "format", nil},
		{"efficacy ok", Request{Scope: ScopePlatform, Type: TypeEfficacyDataset, Format: FormatCSV, StudyID: &id, From: day("2026-09-01"), To: day("2026-10-01")}, "", nil},
		{"efficacy not tenant", Request{Scope: ScopeTenant, Type: TypeEfficacyDataset, Format: FormatCSV, StudyID: &id}, "report_type", nil},
		{"reference", Request{Scope: ScopeLMS, Type: TypeRegulationReference, Format: FormatPDF, SchoolID: &id, SubjectID: &id}, "period", nil},
		{"tenant dates", Request{Scope: ScopeTenant, Type: TypeClassProgress, Format: FormatCSV, From: day("2026-09-01")}, "to", nil},
		{"range", Request{Scope: ScopeTenant, Type: TypeClassProgress, Format: FormatCSV, From: day("2024-01-01"), To: day("2026-01-05")}, "", ErrRangeTooLarge},
		{"reversed", Request{Scope: ScopeTenant, Type: TypeClassProgress, Format: FormatCSV, From: day("2026-02-01"), To: day("2026-01-05")}, "to", nil},
	}
	for _, c := range cases {
		err := c.r.Validate(cfg)
		var fe *FieldError
		switch {
		case c.err != nil:
			if !errors.Is(err, c.err) {
				t.Errorf("%s: err = %v, want %v", c.name, err, c.err)
			}
		case c.field == "":
			if err != nil {
				t.Errorf("%s: err = %v", c.name, err)
			}
		case !errors.As(err, &fe) || fe.Field != c.field:
			t.Errorf("%s: err = %v, want field %s", c.name, err, c.field)
		}
	}
}

// TestKeysAndStates checks the storage key, the watermark and the link rules (F044-AC-02, F044-AC-03).
func TestKeysAndStates(t *testing.T) {
	tenant, rid := uuid.New(), uuid.New()
	if k := StorageKey(&tenant, rid, FormatXLSX); k != "exports/"+tenant.String()+"/"+rid.String()+".xlsx" {
		t.Fatal(k)
	}
	if k := StorageKey(nil, rid, FormatCSV); !strings.HasPrefix(k, "exports/platform/") {
		t.Fatal(k)
	}
	user := uuid.New()
	w := Watermark(user, time.Date(2026, 12, 7, 1, 0, 0, 0, time.UTC))
	if !strings.Contains(w, user.String()) || !strings.Contains(w, "2026-12-07 08:00") {
		t.Fatal(w)
	}
	now := time.Now()
	exp := now.Add(time.Minute)
	if !Downloadable(StatusSucceeded, &exp, now) || Downloadable(StatusSucceeded, &exp, exp) || Downloadable(StatusRunning, &exp, now) {
		t.Fatal("Downloadable")
	}
	if !Active(StatusQueued) || !Active(StatusRunning) || Active(StatusFailed) {
		t.Fatal("Active")
	}
}
