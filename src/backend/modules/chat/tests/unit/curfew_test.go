package unit

import (
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
)

// F115-AC-22 (domain): platform_curfew 21:30–06:30 ICT united with a school window that only extends it.
func TestCurfewWindows(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	platform, err := policy.ParseWindow("21:30-06:30", ict)
	if err != nil {
		t.Fatal(err)
	}
	school, _ := policy.NewWindow("21:00", "07:00", ict)
	at := func(h, m int) time.Time { return time.Date(2026, 11, 21, h, m, 0, 0, ict) }
	cases := []struct {
		c      policy.Curfew
		now    time.Time
		active bool
		reopen time.Time
	}{
		{policy.Curfew{platform}, at(21, 31), true, at(6, 30).AddDate(0, 0, 1)},
		{policy.Curfew{platform}, at(6, 31), false, time.Time{}},
		{policy.Curfew{platform}, at(3, 0), true, at(6, 30)},
		{policy.Curfew{platform, school}, at(21, 10), true, at(7, 0).AddDate(0, 0, 1)},
		{policy.Curfew{platform, school}, at(6, 45), true, at(7, 0)},
		{policy.Curfew{platform, school}, at(12, 0), false, time.Time{}},
	}
	for i, c := range cases {
		active, reopen := c.c.Active(c.now)
		if active != c.active || !reopen.Equal(c.reopen) {
			t.Errorf("case %d: %v %v, want %v %v", i, active, reopen, c.active, c.reopen)
		}
	}
	if _, err := policy.ParseWindow("25:00-06:00", ict); err == nil {
		t.Fatal("invalid window accepted")
	}
}
