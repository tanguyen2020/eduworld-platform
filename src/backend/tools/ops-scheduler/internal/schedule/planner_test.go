package schedule_test

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/schedule"
)

const sample = `version: 1
stack: eduworld
timezone: Asia/Ho_Chi_Minh
lead_minutes: 15
windows:
  - { name: morning, start: "06:45", end: "08:00" }
  - { name: afternoon, start: "13:15", end: "14:30" }
  - { name: evening, start: "19:00", end: "21:00" }
services:
  - { name: core-api, baseline: 6, peak: 16 }
  - { name: realtime, baseline: 4, peak: 6 }
`

func ict(t *testing.T, hhmm string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04", "2026-11-26 "+hhmm, loc)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// F130-AC-22: peak from lead_minutes (15) before a warm_up_windows window until its end, baseline otherwise.
func TestPlannerNextPeakBeforeWindowBaselineAfter(t *testing.T) {
	s, err := schedule.Parse([]byte(sample), map[string]bool{"core-api": true, "realtime": true})
	if err != nil {
		t.Fatal(err)
	}
	p, err := schedule.NewPlanner(s)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		at, window string
		core       int
	}{
		{"06:29", "", 6},
		{"06:30", "morning", 16}, // 15 minutes before 06:45
		{"07:59", "morning", 16},
		{"08:00", "", 6}, // window end is exclusive
		{"13:00", "afternoon", 16},
		{"14:30", "", 6},
		{"18:45", "evening", 16},
		{"21:00", "", 6},
		{"02:00", "", 6},
	}
	for _, c := range cases {
		got := p.Next(ict(t, c.at))
		if got[0].Service != "core-api" || got[0].Replicas != c.core || got[0].Window != c.window {
			t.Errorf("%s: core-api = %+v, want %d in %q", c.at, got[0], c.core, c.window)
		}
		wantRT := 4
		if c.window != "" {
			wantRT = 6
		}
		if got[1].Replicas != wantRT {
			t.Errorf("%s: realtime = %d, want %d", c.at, got[1].Replicas, wantRT)
		}
	}
	// UTC input is converted to ICT: 23:30 UTC = 06:30 ICT next day.
	if got := p.Next(time.Date(2026, 11, 25, 23, 30, 0, 0, time.UTC)); got[0].Replicas != 16 {
		t.Fatalf("UTC instant not converted to ICT: %+v", got[0])
	}
}
