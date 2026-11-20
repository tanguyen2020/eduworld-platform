package settings

import (
	"testing"
	"time"
)

var ict = time.FixedZone("ICT", 7*3600)

func TestValidateDailyPlayLimit(t *testing.T) {
	for _, v := range []int{15, 120, 300} {
		if err := ValidateDailyPlayLimit(v); err != nil {
			t.Fatalf("%d: %v", v, err)
		}
	}
	for _, v := range []int{0, 14, 301, 1440} {
		if err := ValidateDailyPlayLimit(v); err == nil {
			t.Fatalf("%d accepted", v)
		}
	}
}

func TestSplitByDayOverMidnight(t *testing.T) {
	end := time.Date(2026, 11, 21, 0, 30, 0, 0, ict) // 23:15 → 00:30 ICT
	got := SplitByDay(end, 75*time.Minute, ict)
	if got[PlayDate{2026, 11, 20}] != 45*time.Minute || got[PlayDate{2026, 11, 21}] != 30*time.Minute || len(got) != 2 {
		t.Fatalf("split: %v", got)
	}
	if len(SplitByDay(end, 0, ict)) != 0 {
		t.Fatal("zero duration")
	}
	if one := SplitByDay(end.UTC(), time.Minute, ict); one[PlayDate{2026, 11, 21}] != time.Minute {
		t.Fatalf("UTC end: %v", one)
	}
}

func TestNextMidnightAndLimit(t *testing.T) {
	now := time.Date(2026, 11, 20, 16, 59, 0, 0, time.UTC) // 23:59 ICT
	if got := NextMidnight(now, ict); !got.Equal(time.Date(2026, 11, 21, 0, 0, 0, 0, ict)) {
		t.Fatalf("next midnight %v", got)
	}
	if DateOf(now, ict).String() != "2026-11-20" {
		t.Fatal("date")
	}
	limit := 60
	if DailyLimitReached(nil, 10*time.Hour) || DailyLimitReached(&limit, 59*time.Minute) || !DailyLimitReached(&limit, time.Hour) {
		t.Fatal("limit")
	}
}
