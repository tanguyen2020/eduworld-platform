package enrollment

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTransferKinds(t *testing.T) {
	tenantA, tenantB := uuid.New(), uuid.New()
	school1, school2 := uuid.New(), uuid.New()
	start := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	on := time.Date(2026, 11, 19, 23, 30, 0, 0, time.UTC)
	cases := []struct {
		name   string
		target Placement
		kind   Kind
	}{
		{"class (AC-13)", Placement{tenantA, school1, uuid.New()}, KindClass},
		{"school same tenant (AC-14)", Placement{tenantA, school2, uuid.New()}, KindSchool},
		{"other tenant (AC-15)", Placement{tenantB, uuid.New(), uuid.New()}, KindTenant},
	}
	for _, c := range cases {
		cur, err := Open(uuid.New(), uuid.New(), Placement{tenantA, school1, uuid.New()}, start, SourceSIS)
		if err != nil {
			t.Fatal(err)
		}
		tr, err := TransferTo(cur, uuid.New(), c.target, on, SourceManual)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if tr.Kind != c.kind || tr.Closed.Status != StatusTransferred || tr.Opened.Status != StatusActive {
			t.Fatalf("%s: %+v", c.name, tr)
		}
		if !tr.Closed.EndDate.Equal(tr.Opened.StartDate) || tr.Opened.StartDate != time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC) {
			t.Fatalf("%s: dates end=%v start=%v", c.name, tr.Closed.EndDate, tr.Opened.StartDate)
		}
		if tr.Opened.StudentID != cur.StudentID || tr.Opened.Placement != c.target {
			t.Fatalf("%s: opened = %+v", c.name, tr.Opened)
		}
	}
}

func TestTransferRules(t *testing.T) {
	at := Placement{uuid.New(), uuid.New(), uuid.New()}
	start := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	cur, _ := Open(uuid.New(), uuid.New(), at, start, SourceSIS)
	if _, err := TransferTo(cur, uuid.New(), at, start, SourceSIS); !errors.Is(err, ErrSameClass) {
		t.Fatalf("same class = %v", err)
	}
	other := Placement{at.TenantID, at.SchoolID, uuid.New()}
	if _, err := TransferTo(cur, uuid.New(), other, start.AddDate(0, 0, -1), SourceSIS); !errors.Is(err, ErrBeforeStart) {
		t.Fatalf("before start = %v", err)
	}
	if cur.Status != StatusActive {
		t.Fatal("a failed transfer must leave the enrollment active")
	}
	if err := cur.Close(StatusEnded, start); err != nil {
		t.Fatal(err)
	}
	if _, err := TransferTo(cur, uuid.New(), other, start, SourceSIS); !errors.Is(err, ErrNotActive) {
		t.Fatalf("closed enrollment = %v", err)
	}
	if _, err := Open(uuid.New(), uuid.Nil, at, start, SourceSIS); !errors.Is(err, ErrInvalid) {
		t.Fatal("open without student accepted")
	}
}
