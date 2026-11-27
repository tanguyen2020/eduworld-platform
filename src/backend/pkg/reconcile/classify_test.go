package reconcile

import (
	"testing"
	"time"
)

// F134-AC-01, AC-02 (TC-SVC-014): classification table of spec Clarifications 2026-11-16.
func TestClassify(t *testing.T) {
	T := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
	now := T.Add(2 * time.Hour)
	settle := 10 * time.Minute
	old := now.Add(-time.Hour)
	recent := now.Add(-time.Minute)
	before, after := T.Add(-time.Minute), T.Add(time.Minute)
	cases := []struct {
		name string
		o    Observation
		want Class
	}{
		{"both sides agree", Observation{Direction: Outbound, InSource: true, PublishedAt: &old, Processed: true, EventTime: before}, ""},
		{"nothing anywhere", Observation{Direction: Outbound, EventTime: after}, ""},
		{"source lost after T (outbound)", Observation{Direction: Outbound, Processed: true, EventTime: after}, LostAfterRestore},
		{"source lost at T exactly", Observation{Direction: Outbound, Processed: true, EventTime: T}, Unexplained},
		{"source lost before T", Observation{Direction: Outbound, Processed: true, EventTime: before}, Unexplained},
		{"dead-lettered event lost after T", Observation{Direction: Outbound, DeadLettered: true, EventTime: after}, LostAfterRestore},
		{"inbound source missing is never explained", Observation{Direction: Inbound, Processed: true, EventTime: after}, Unexplained},
		{"dead lettered", Observation{Direction: Outbound, InSource: true, PublishedAt: &old, DeadLettered: true, EventTime: before}, DeadLettered},
		{"not published yet", Observation{Direction: Inbound, InSource: true, EventTime: after}, PendingPublish},
		{"published within settle", Observation{Direction: Inbound, InSource: true, PublishedAt: &recent, EventTime: after}, PendingDelivery},
		{"published past settle, not processed", Observation{Direction: Inbound, InSource: true, PublishedAt: &old, EventTime: after}, Unexplained},
		{"processed and dead-lettered wins processed", Observation{Direction: Outbound, InSource: true, PublishedAt: &old, Processed: true, DeadLettered: true, EventTime: before}, ""},
	}
	for _, c := range cases {
		if got := Classify(c.o, T, settle, now); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// F134-AC-04: window limits checked before any module is called.
func TestWindowValidate(t *testing.T) {
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	ret := 30 * 24 * time.Hour
	ok := Window{From: now.Add(-48 * time.Hour), To: now, RestorePoint: now.Add(-24 * time.Hour), Settle: 10 * time.Minute}
	if err := ok.Validate(ret, now); err != nil {
		t.Fatalf("valid window refused: %v", err)
	}
	bad := map[string]Window{
		"longer than outbox_retention": {From: now.Add(-31 * 24 * time.Hour), To: now, RestorePoint: now.Add(-time.Hour)},
		"from after to":                {From: now, To: now.Add(-time.Hour), RestorePoint: now.Add(-30 * time.Minute)},
		"from equals to":               {From: now, To: now, RestorePoint: now},
		"T before the window":          {From: now.Add(-time.Hour), To: now, RestorePoint: now.Add(-2 * time.Hour)},
		"T after the window":           {From: now.Add(-time.Hour), To: now.Add(-30 * time.Minute), RestorePoint: now},
		"missing T":                    {From: now.Add(-time.Hour), To: now},
		"from older than retention":    {From: now.Add(-40 * 24 * time.Hour), To: now.Add(-20 * 24 * time.Hour), RestorePoint: now.Add(-30 * 24 * time.Hour)},
		"negative settle":              {From: now.Add(-time.Hour), To: now, RestorePoint: now.Add(-time.Minute), Settle: -time.Second},
	}
	for name, w := range bad {
		if err := w.Validate(ret, now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestReportExitCode(t *testing.T) {
	r := Report{Status: "complete"}
	if r.ExitCode() != ExitOK {
		t.Fatal("clean report must exit 0")
	}
	r.Counts.LostAfterRestore = 3
	if r.ExitCode() != ExitOK {
		t.Fatal("explained mismatches only must exit 0")
	}
	r.Status = "incomplete"
	if r.ExitCode() != ExitIncomplete {
		t.Fatal("incomplete must exit 3")
	}
	r.Counts.Unexplained = 1
	if r.ExitCode() != ExitUnexplained {
		t.Fatal("unexplained must exit 2")
	}
}
